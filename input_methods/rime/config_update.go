package rime

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gaboolic/moqi-ime/imecore"
)

var (
	gitLookPathFunc = exec.LookPath
	gitIsRepoFunc   = gitIsRepo
	gitPullFunc     = gitPull
)

type configUpdateState struct {
	mu      sync.Mutex
	running bool
}

var sharedConfigUpdateState configUpdateState

func resetConfigUpdateStateForTest() {
	sharedConfigUpdateState.mu.Lock()
	sharedConfigUpdateState.running = false
	sharedConfigUpdateState.mu.Unlock()
}

func trayNotification(message string, icon imecore.TrayNotificationIcon) *imecore.TrayNotification {
	return &imecore.TrayNotification{
		Title:   "Rime",
		Message: message,
		Icon:    icon,
	}
}

func (ime *IME) sendAsyncTrayNotification(notification *imecore.TrayNotification) {
	if notification == nil || ime.asyncResponseSender == nil {
		return
	}
	ime.asyncResponseSender(&imecore.Response{
		Success:          true,
		TrayNotification: notification,
	})
}

func gitIsRepo(dir string) (bool, error) {
	output, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return false, nil
	}
	topLevel := strings.TrimSpace(string(output))
	if topLevel == "" {
		return false, nil
	}
	return filepath.Clean(topLevel) == filepath.Clean(dir), nil
}

func gitPull(dir string) (string, error) {
	output, err := exec.Command("git", "-C", dir, "pull").CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func (ime *IME) updateConfigAsync(resp *imecore.Response) bool {
	if _, err := gitLookPathFunc("git"); err != nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("未检测到 Git 命令", imecore.TrayNotificationIconError)
		}
		return false
	}

	userDir := ime.userDir()
	isRepo, err := gitIsRepoFunc(userDir)
	if err != nil || !isRepo {
		if resp != nil {
			resp.TrayNotification = trayNotification("当前方案集目录不是 Git 仓库", imecore.TrayNotificationIconError)
		}
		return false
	}

	sharedConfigUpdateState.mu.Lock()
	if sharedConfigUpdateState.running {
		sharedConfigUpdateState.mu.Unlock()
		if resp != nil {
			resp.TrayNotification = trayNotification("更新配置已在进行中", imecore.TrayNotificationIconInfo)
		}
		return false
	}
	sharedConfigUpdateState.running = true
	sharedConfigUpdateState.mu.Unlock()

	if resp != nil {
		resp.TrayNotification = trayNotification("开始更新配置...", imecore.TrayNotificationIconInfo)
	}

	go func(targetDir string) {
		defer func() {
			sharedConfigUpdateState.mu.Lock()
			sharedConfigUpdateState.running = false
			sharedConfigUpdateState.mu.Unlock()
		}()

		output, err := gitPullFunc(targetDir)
		if err != nil {
			message := "更新配置失败"
			if output != "" {
				message = "更新配置失败: " + summarizeGitOutput(output)
			}
			ime.sendAsyncTrayNotification(trayNotification(message, imecore.TrayNotificationIconError))
			return
		}

		message := "更新配置成功"
		if output != "" {
			message = summarizeGitSuccessMessage(output)
		}
		ime.sendAsyncTrayNotification(trayNotification(message, imecore.TrayNotificationIconInfo))
	}(userDir)

	return true
}

func summarizeGitOutput(output string) string {
	lines := splitNonEmptyLines(output)
	if len(lines) == 0 {
		return "请检查 Git 输出"
	}
	return lastLineWithinLimit(lines, 48)
}

func summarizeGitSuccessMessage(output string) string {
	lines := splitNonEmptyLines(output)
	if len(lines) == 0 {
		return "更新配置成功"
	}
	last := strings.ToLower(lines[len(lines)-1])
	switch {
	case strings.Contains(last, "already up to date"):
		return "配置已是最新"
	case strings.Contains(last, "already up-to-date"):
		return "配置已是最新"
	default:
		return "更新配置成功"
	}
}

func splitNonEmptyLines(output string) []string {
	rawLines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func lastLineWithinLimit(lines []string, maxLen int) string {
	if len(lines) == 0 {
		return ""
	}
	last := lines[len(lines)-1]
	runes := []rune(last)
	if len(runes) <= maxLen {
		return last
	}
	return string(runes[:maxLen]) + "..."
}
