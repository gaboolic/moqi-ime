package rime

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gaboolic/moqi-ime/imecore"
)

const aiConfigFileName = "ai_config.json"

type aiGenerateRequest struct {
	PreviousCommit string
	Composition    string
	Candidates     []string
	Prompt         string
}

type aiFileConfig struct {
	API     aiAPIConfig        `json:"api"`
	Actions []aiActionFileSpec `json:"actions"`
}

type aiAPIConfig struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

type aiActionFileSpec struct {
	Name   string `json:"name"`
	Hotkey string `json:"hotkey"`
	Prompt string `json:"prompt"`
}

type aiRuntimeConfig struct {
	API     aiAPIConfig
	Actions []aiAction
}

type aiAction struct {
	Name    string
	Hotkey  string
	Prompt  string
	KeyCode int
	Ctrl    bool
	Alt     bool
	Shift   bool
}

func loadAIConfig() (*aiRuntimeConfig, error) {
	if err := ensureUserAIConfigCopied(); err != nil {
		return nil, err
	}
	for _, path := range aiConfigSearchPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read AI config %s: %w", path, err)
		}
		return parseAIConfigJSON(data)
	}
	return envAIConfig(), nil
}

func aiConfigSearchPaths() []string {
	paths := make([]string, 0, 2)
	if path := userAIConfigPath(); path != "" {
		paths = append(paths, path)
	}
	if path := bundledAIConfigPath(); path != "" {
		paths = append(paths, path)
	}
	return paths
}

func userAIConfigPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, APP, "Rime", aiConfigFileName)
}

func bundledAIConfigPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exePath), "input_methods", "rime", aiConfigFileName)
}

func ensureUserAIConfigCopied() error {
	return copyAIConfigIfMissing(userAIConfigPath(), bundledAIConfigPath())
}

func copyAIConfigIfMissing(dstPath, srcPath string) error {
	if dstPath == "" || srcPath == "" {
		return nil
	}
	if _, err := os.Stat(dstPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat AI config %s: %w", dstPath, err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open bundled AI config %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create AI config directory %s: %w", filepath.Dir(dstPath), err)
	}

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("create user AI config %s: %w", dstPath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy AI config to %s: %w", dstPath, err)
	}
	return nil
}

func envAIConfig() *aiRuntimeConfig {
	api := envAIAPIConfig()
	if api.BaseURL == "" || api.APIKey == "" || api.Model == "" {
		return nil
	}
	return &aiRuntimeConfig{
		API:     api,
		Actions: []aiAction{defaultAIAction()},
	}
}

func envAIAPIConfig() aiAPIConfig {
	return aiAPIConfig{
		BaseURL: strings.TrimRight(strings.TrimSpace(os.Getenv("MOQI_AI_BASE_URL")), "/"),
		APIKey:  strings.TrimSpace(os.Getenv("MOQI_AI_API_KEY")),
		Model:   strings.TrimSpace(os.Getenv("MOQI_AI_MODEL")),
	}
}

func fillAIAPIConfigFromEnv(api aiAPIConfig) aiAPIConfig {
	envAPI := envAIAPIConfig()
	if api.BaseURL == "" {
		api.BaseURL = envAPI.BaseURL
	}
	if api.APIKey == "" {
		api.APIKey = envAPI.APIKey
	}
	if api.Model == "" {
		api.Model = envAPI.Model
	}
	return api
}

func parseAIConfigJSON(data []byte) (*aiRuntimeConfig, error) {
	var raw aiFileConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode AI config JSON: %w", err)
	}

	cfg := &aiRuntimeConfig{
		API: aiAPIConfig{
			BaseURL: strings.TrimRight(strings.TrimSpace(raw.API.BaseURL), "/"),
			APIKey:  strings.TrimSpace(raw.API.APIKey),
			Model:   strings.TrimSpace(raw.API.Model),
		},
	}
	cfg.API = fillAIAPIConfigFromEnv(cfg.API)
	if cfg.API.BaseURL == "" || cfg.API.APIKey == "" || cfg.API.Model == "" {
		return nil, fmt.Errorf("AI config requires api.base_url, api.api_key and api.model")
	}

	if len(raw.Actions) == 0 {
		cfg.Actions = []aiAction{defaultAIAction()}
		return cfg, nil
	}

	cfg.Actions = make([]aiAction, 0, len(raw.Actions))
	for i, spec := range raw.Actions {
		action, err := newAIAction(spec)
		if err != nil {
			return nil, fmt.Errorf("parse AI action %d: %w", i, err)
		}
		cfg.Actions = append(cfg.Actions, action)
	}
	return cfg, nil
}

func newAIAction(spec aiActionFileSpec) (aiAction, error) {
	hotkey := strings.TrimSpace(spec.Hotkey)
	if hotkey == "" {
		hotkey = defaultAIAction().Hotkey
	}
	action, err := parseAIHotkey(hotkey)
	if err != nil {
		return aiAction{}, err
	}
	action.Name = strings.TrimSpace(spec.Name)
	if action.Name == "" {
		action.Name = "AI"
	}
	action.Prompt = strings.TrimSpace(spec.Prompt)
	if action.Prompt == "" {
		action.Prompt = defaultAIUserPromptTemplate()
	}
	return action, nil
}

func defaultAIAction() aiAction {
	action, _ := parseAIHotkey(fmt.Sprintf("Ctrl+Shift+%s", string(rune(aiHotkeyKeyCode))))
	action.Name = "写好评"
	action.Prompt = defaultAIUserPromptTemplate()
	return action
}

func defaultAIActions(cfg *aiRuntimeConfig) []aiAction {
	if cfg == nil || len(cfg.Actions) == 0 {
		return nil
	}
	actions := make([]aiAction, len(cfg.Actions))
	copy(actions, cfg.Actions)
	return actions
}

func defaultAIUserPromptTemplate() string {
	return "请围绕“{{composition}}”生成最多 3 条适合直接发布的中文好评，每条 20 字左右。"
}

func parseAIHotkey(hotkey string) (aiAction, error) {
	parts := strings.Split(hotkey, "+")
	action := aiAction{
		Hotkey: strings.TrimSpace(hotkey),
	}
	if len(parts) == 0 {
		return aiAction{}, fmt.Errorf("empty hotkey")
	}

	var mainKey string
	for _, part := range parts {
		token := strings.ToUpper(strings.TrimSpace(part))
		if token == "" {
			continue
		}
		switch token {
		case "CTRL", "CONTROL":
			action.Ctrl = true
		case "ALT", "MENU":
			action.Alt = true
		case "SHIFT":
			action.Shift = true
		default:
			if mainKey != "" {
				return aiAction{}, fmt.Errorf("hotkey %q has multiple main keys", hotkey)
			}
			mainKey = token
		}
	}
	if mainKey == "" {
		return aiAction{}, fmt.Errorf("hotkey %q is missing main key", hotkey)
	}

	keyCode, err := parseAIHotkeyMainKey(mainKey)
	if err != nil {
		return aiAction{}, err
	}
	action.KeyCode = keyCode
	action.Hotkey = normalizedAIHotkey(action)
	return action, nil
}

func parseAIHotkeyMainKey(token string) (int, error) {
	if len(token) == 1 {
		ch := token[0]
		if ch >= 'A' && ch <= 'Z' {
			return int(ch), nil
		}
		if ch >= '0' && ch <= '9' {
			return int(ch), nil
		}
	}
	return 0, fmt.Errorf("unsupported AI hotkey key %q", token)
}

func normalizedAIHotkey(action aiAction) string {
	parts := make([]string, 0, 4)
	if action.Ctrl {
		parts = append(parts, "Ctrl")
	}
	if action.Alt {
		parts = append(parts, "Alt")
	}
	if action.Shift {
		parts = append(parts, "Shift")
	}
	parts = append(parts, strings.ToUpper(string(rune(action.KeyCode))))
	return strings.Join(parts, "+")
}

func (action aiAction) matches(req *imecore.Request) bool {
	if req == nil || req.KeyCode != action.KeyCode {
		return false
	}
	return req.KeyStates.IsKeyDown(vkControl) == action.Ctrl &&
		req.KeyStates.IsKeyDown(vkMenu) == action.Alt &&
		req.KeyStates.IsKeyDown(vkShift) == action.Shift
}
