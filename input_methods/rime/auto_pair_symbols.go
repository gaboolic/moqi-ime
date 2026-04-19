package rime

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

const (
	autoPairSymbolsFileName    = "moqi_auto_pair_symbols.txt"
	ID_OPEN_AUTO_PAIR_SYMBOLS  = 222
)

var openAutoPairSymbolsTargetFunc = openWithDefaultApp
var autoPairSymbolsState struct {
	mu      sync.Mutex
	version uint64
	modTime time.Time
	size    int64
	rules   []imecore.AutoPairRule
	loaded  bool
}

func builtinAutoPairRules() []imecore.AutoPairRule {
	return []imecore.AutoPairRule{
		{Open: "“", Close: "”"},
		{Open: "‘", Close: "’"},
		{Open: "【", Close: "】"},
		{Open: "《", Close: "》"},
		{Open: "<", Close: ">"},
		{Open: "(", Close: ")"},
		{Open: "（", Close: "）"},
		{Open: "「", Close: "」"},
	}
}

func cloneAutoPairRules(rules []imecore.AutoPairRule) []imecore.AutoPairRule {
	if len(rules) == 0 {
		return nil
	}
	cloned := make([]imecore.AutoPairRule, len(rules))
	copy(cloned, rules)
	return cloned
}

func autoPairSymbolsFilePath() string {
	root := moqiAppDataDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, autoPairSymbolsFileName)
}

func defaultAutoPairSymbolsFileContent() []byte {
	var builder strings.Builder
	builder.WriteString("# 成对符号\n")
	builder.WriteString("# 每行一对符号\n")
	for _, rule := range builtinAutoPairRules() {
		if rule.Open == "" || rule.Close == "" {
			continue
		}
		builder.WriteString(rule.Open)
		builder.WriteString(rule.Close)
		builder.WriteString("\n")
	}
	return []byte(builder.String())
}

func ensureAutoPairSymbolsFileExists() (string, error) {
	path := autoPairSymbolsFilePath()
	if path == "" {
		return "", os.ErrNotExist
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := os.WriteFile(path, defaultAutoPairSymbolsFileContent(), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func loadAutoPairRules() ([]imecore.AutoPairRule, uint64, error) {
	path, err := ensureAutoPairSymbolsFileExists()
	if err != nil {
		return cloneAutoPairRules(builtinAutoPairRules()), 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return cloneAutoPairRules(builtinAutoPairRules()), 0, err
	}

	autoPairSymbolsState.mu.Lock()
	defer autoPairSymbolsState.mu.Unlock()

	if autoPairSymbolsState.loaded &&
		autoPairSymbolsState.modTime.Equal(info.ModTime()) &&
		autoPairSymbolsState.size == info.Size() {
		return cloneAutoPairRules(autoPairSymbolsState.rules), autoPairSymbolsState.version, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return cloneAutoPairRules(builtinAutoPairRules()), autoPairSymbolsState.version, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 256), 64*1024)
	rules := make([]imecore.AutoPairRule, 0)
	seen := make(map[string]struct{})
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var rule imecore.AutoPairRule
		parts := strings.Fields(line)
		switch {
		case len(parts) >= 2:
			rule = imecore.AutoPairRule{
				Open:  strings.TrimSpace(parts[0]),
				Close: strings.TrimSpace(parts[1]),
			}
		default:
			runes := []rune(line)
			if len(runes) != 2 {
				continue
			}
			rule = imecore.AutoPairRule{
				Open:  string(runes[0]),
				Close: string(runes[1]),
			}
		}
		if rule.Open == "" || rule.Close == "" {
			continue
		}
		key := rule.Open + "\x00" + rule.Close
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		rules = append(rules, rule)
	}
	if err := scanner.Err(); err != nil {
		return cloneAutoPairRules(builtinAutoPairRules()), autoPairSymbolsState.version, err
	}
	if len(rules) == 0 {
		rules = builtinAutoPairRules()
	}

	autoPairSymbolsState.loaded = true
	autoPairSymbolsState.modTime = info.ModTime()
	autoPairSymbolsState.size = info.Size()
	autoPairSymbolsState.rules = cloneAutoPairRules(rules)
	autoPairSymbolsState.version++
	return cloneAutoPairRules(autoPairSymbolsState.rules), autoPairSymbolsState.version, nil
}

func resetAutoPairSymbolsStateForTest() {
	autoPairSymbolsState.mu.Lock()
	autoPairSymbolsState.version = 0
	autoPairSymbolsState.modTime = time.Time{}
	autoPairSymbolsState.size = 0
	autoPairSymbolsState.rules = nil
	autoPairSymbolsState.loaded = false
	autoPairSymbolsState.mu.Unlock()
}

func (ime *IME) currentAutoPairRules() []imecore.AutoPairRule {
	rules, version, err := loadAutoPairRules()
	if err != nil {
		log.Printf("加载成对符号配置失败: %v", err)
	}
	if version != 0 {
		ime.autoPairRulesVersion = version
	}
	if len(rules) == 0 {
		return builtinAutoPairRules()
	}
	return rules
}

func (ime *IME) syncAutoPairRules() bool {
	_, version, err := loadAutoPairRules()
	if err != nil {
		log.Printf("同步成对符号配置失败: %v", err)
		return false
	}
	if version == 0 || version == ime.autoPairRulesVersion {
		return false
	}
	ime.autoPairRulesVersion = version
	return true
}

func (ime *IME) openAutoPairSymbolsFile(resp *imecore.Response) bool {
	path, err := ensureAutoPairSymbolsFileExists()
	if err != nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("创建成对符号文件失败", imecore.TrayNotificationIconError)
		}
		return false
	}
	if err := openAutoPairSymbolsTargetFunc(path); err != nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("打开成对符号文件失败", imecore.TrayNotificationIconError)
		}
		return false
	}
	return true
}
