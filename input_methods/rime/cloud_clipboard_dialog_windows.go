//go:build windows

package rime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gaboolic/moqi-ime/input_methods/rime/cloudclipboard"
)

type cloudClipboardDialogResult struct {
	Enabled        bool   `json:"enabled"`
	BaseURL        string `json:"base_url"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	KeepPassword   bool   `json:"keep_password"`
	SettingsRoot   string `json:"settings_root"`
	MinIntervalSec int    `json:"min_interval_sec"`
	ListHotkey     string `json:"list_hotkey"`
	TestOnly       bool   `json:"test_only"`
}

func promptWebDAVSettings(ctx context.Context, current cloudclipboard.Config, hasPassword bool) (cloudClipboardDialogResult, error) {
	passwordHint := "（未设置，输入后保存）"
	if hasPassword {
		passwordHint = "（已设置，留空表示不修改）"
	}
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$form = New-Object System.Windows.Forms.Form
$form.Text = 'WebDAV 配置'
$form.StartPosition = 'CenterScreen'
$form.Size = New-Object System.Drawing.Size(620, 320)
$form.FormBorderStyle = [System.Windows.Forms.FormBorderStyle]::FixedDialog
$form.MaximizeBox = $false
$form.MinimizeBox = $false

function Add-Label($text, $y) {
  $label = New-Object System.Windows.Forms.Label
  $label.Text = $text
  $label.Location = New-Object System.Drawing.Point(16, $y)
  $label.Size = New-Object System.Drawing.Size(580, 20)
  $form.Controls.Add($label)
}

function Add-Box($y, $value) {
  $box = New-Object System.Windows.Forms.TextBox
  $box.Location = New-Object System.Drawing.Point(16, ($y + 22))
  $box.Size = New-Object System.Drawing.Size(570, 24)
  $box.Text = $value
  $form.Controls.Add($box)
  return $box
}

$y = 12
Add-Label 'WebDAV 地址（飞牛示例 http://192.168.x.x:5005/）' $y | Out-Null
$urlBox = Add-Box $y '%s'
$y += 52

Add-Label '用户名' $y | Out-Null
$userBox = Add-Box $y '%s'
$y += 52

Add-Label ('密码 ' + '%s') $y | Out-Null
$passBox = Add-Box $y ''
$passBox.UseSystemPasswordChar = $true
$y += 52

Add-Label '墨奇设置目录（剪贴板在 clip/ 下）' $y | Out-Null
$rootBox = Add-Box $y '%s'
$y += 52

$okButton = New-Object System.Windows.Forms.Button
$okButton.Text = '确定'
$okButton.Location = New-Object System.Drawing.Point(410, $y)
$okButton.Size = New-Object System.Drawing.Size(80, 30)
$okButton.DialogResult = [System.Windows.Forms.DialogResult]::OK
$form.AcceptButton = $okButton
$form.Controls.Add($okButton)

$cancelButton = New-Object System.Windows.Forms.Button
$cancelButton.Text = '取消'
$cancelButton.Location = New-Object System.Drawing.Point(506, $y)
$cancelButton.Size = New-Object System.Drawing.Size(80, 30)
$cancelButton.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
$form.CancelButton = $cancelButton
$form.Controls.Add($cancelButton)

$result = $form.ShowDialog()
if ($result -ne [System.Windows.Forms.DialogResult]::OK) { exit 2 }
$payload = @{
  enabled = $%t
  base_url = $urlBox.Text
  username = $userBox.Text
  password = $passBox.Text
  keep_password = [bool]($passBox.Text -eq '')
  settings_root = $rootBox.Text
  min_interval_sec = %d
  list_hotkey = '%s'
  test_only = $false
} | ConvertTo-Json -Compress
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Write-Output $payload
`,
		escapePS(current.BaseURL),
		escapePS(current.Username),
		passwordHint,
		escapePS(current.SettingsRoot),
		current.Enabled,
		current.MinIntervalSec,
		escapePS(current.ListHotkey),
	)

	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-EncodedCommand", encodePowerShellCommand(script))
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return cloudClipboardDialogResult{}, errors.New("已取消")
		}
		return cloudClipboardDialogResult{}, fmt.Errorf("打开云剪贴板设置失败: %w", err)
	}
	jsonOutput, err := extractPowerShellJSONOutput(output)
	if err != nil {
		return cloudClipboardDialogResult{}, err
	}
	var result cloudClipboardDialogResult
	if err := json.Unmarshal(jsonOutput, &result); err != nil {
		return cloudClipboardDialogResult{}, fmt.Errorf("读取设置失败: %w", err)
	}
	return result, nil
}

func promptCloudClipboardSettings(ctx context.Context, current cloudclipboard.Config) (cloudClipboardDialogResult, error) {
	intervals := []int{1, 3, 5, 10, 60}
	intervalLabels := []string{"1 秒", "3 秒", "5 秒", "10 秒", "60 秒"}
	defaultIntervalIndex := 3
	for i, sec := range intervals {
		if sec == current.MinIntervalSec {
			defaultIntervalIndex = i
			break
		}
	}
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing

$form = New-Object System.Windows.Forms.Form
$form.Text = '云剪贴板设置'
$form.StartPosition = 'CenterScreen'
$form.Size = New-Object System.Drawing.Size(520, 230)
$form.FormBorderStyle = [System.Windows.Forms.FormBorderStyle]::FixedDialog
$form.MaximizeBox = $false
$form.MinimizeBox = $false

function Add-Label($text, $y) {
  $label = New-Object System.Windows.Forms.Label
  $label.Text = $text
  $label.Location = New-Object System.Drawing.Point(16, $y)
  $label.Size = New-Object System.Drawing.Size(470, 20)
  $form.Controls.Add($label)
}

$y = 12
Add-Label '上传最小间隔' $y | Out-Null
$intervalBox = New-Object System.Windows.Forms.ComboBox
$intervalBox.Location = New-Object System.Drawing.Point(16, ($y + 22))
$intervalBox.Size = New-Object System.Drawing.Size(200, 24)
$intervalBox.DropDownStyle = [System.Windows.Forms.ComboBoxStyle]::DropDownList
$intervalLabels = @('%s')
foreach ($label in $intervalLabels) { [void]$intervalBox.Items.Add($label) }
$intervalBox.SelectedIndex = %d
$form.Controls.Add($intervalBox)
$y += 58

Add-Label '列出云剪贴板快捷键' $y | Out-Null
$hotkeyBox = New-Object System.Windows.Forms.TextBox
$hotkeyBox.Location = New-Object System.Drawing.Point(16, ($y + 22))
$hotkeyBox.Size = New-Object System.Drawing.Size(470, 24)
$hotkeyBox.Text = '%s'
$form.Controls.Add($hotkeyBox)
$y += 58

$okButton = New-Object System.Windows.Forms.Button
$okButton.Text = '确定'
$okButton.Location = New-Object System.Drawing.Point(310, $y)
$okButton.Size = New-Object System.Drawing.Size(80, 30)
$okButton.DialogResult = [System.Windows.Forms.DialogResult]::OK
$form.AcceptButton = $okButton
$form.Controls.Add($okButton)

$cancelButton = New-Object System.Windows.Forms.Button
$cancelButton.Text = '取消'
$cancelButton.Location = New-Object System.Drawing.Point(406, $y)
$cancelButton.Size = New-Object System.Drawing.Size(80, 30)
$cancelButton.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
$form.CancelButton = $cancelButton
$form.Controls.Add($cancelButton)

$result = $form.ShowDialog()
if ($result -ne [System.Windows.Forms.DialogResult]::OK) { exit 2 }
$payload = @{
  enabled = $%t
  base_url = '%s'
  username = '%s'
  password = ''
  keep_password = $true
  settings_root = '%s'
  min_interval_sec = @(%s)[$intervalBox.SelectedIndex]
  list_hotkey = $hotkeyBox.Text
  test_only = $false
} | ConvertTo-Json -Compress
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
Write-Output $payload
`,
		strings.Join(intervalLabels, "','"),
		defaultIntervalIndex,
		escapePS(current.ListHotkey),
		current.Enabled,
		escapePS(current.BaseURL),
		escapePS(current.Username),
		escapePS(current.SettingsRoot),
		strings.Trim(strings.Join(intSliceToStrings(intervals), ","), " "),
	)

	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-EncodedCommand", encodePowerShellCommand(script))
	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			return cloudClipboardDialogResult{}, errors.New("已取消")
		}
		return cloudClipboardDialogResult{}, fmt.Errorf("打开云剪贴板设置失败: %w", err)
	}
	jsonOutput, err := extractPowerShellJSONOutput(output)
	if err != nil {
		return cloudClipboardDialogResult{}, err
	}
	var result cloudClipboardDialogResult
	if err := json.Unmarshal(jsonOutput, &result); err != nil {
		return cloudClipboardDialogResult{}, fmt.Errorf("读取设置失败: %w", err)
	}
	return result, nil
}

func escapePS(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func intSliceToStrings(values []int) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = fmt.Sprintf("%d", v)
	}
	return out
}

func dialogResultToConfig(result cloudClipboardDialogResult, previous cloudclipboard.Config) cloudclipboard.Config {
	cfg := previous
	cfg.Enabled = result.Enabled
	cfg.BaseURL = cloudclipboard.NormalizeBaseURL(result.BaseURL)
	cfg.Username = strings.TrimSpace(result.Username)
	cfg.SettingsRoot = cloudclipboard.NormalizeDir(result.SettingsRoot)
	if result.MinIntervalSec > 0 {
		cfg.MinIntervalSec = result.MinIntervalSec
	}
	if hk := strings.TrimSpace(result.ListHotkey); hk != "" {
		cfg.ListHotkey = hk
	}
	if result.Password != "" {
		cfg.Password = result.Password
	} else if !result.KeepPassword {
		cfg.Password = ""
	}
	return cfg
}

func webDAVDialogResultToConfig(result cloudClipboardDialogResult, previous cloudclipboard.Config) cloudclipboard.Config {
	cfg := previous
	cfg.BaseURL = cloudclipboard.NormalizeBaseURL(result.BaseURL)
	cfg.Username = strings.TrimSpace(result.Username)
	cfg.SettingsRoot = cloudclipboard.NormalizeDir(result.SettingsRoot)
	if result.Password != "" {
		cfg.Password = result.Password
	} else if !result.KeepPassword {
		cfg.Password = ""
	}
	return cfg
}
