package rime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const appearanceConfigFileName = "appearance_config.json"
const rimeDefaultCustomConfigFileName = "default.custom.yaml"

type appearanceConfig struct {
	CandidateTheme              *string `json:"candidate_theme,omitempty"`
	FontPoint                   *int    `json:"font_point,omitempty"`
	InlinePreedit               *bool   `json:"inline_preedit,omitempty"`
	CandidatePerRow             *int    `json:"candidate_per_row,omitempty"`
	CandidateCount              *int    `json:"candidate_count,omitempty"`
	CandidateBackgroundColor    *string `json:"candidate_background_color,omitempty"`
	CandidateHighlightColor     *string `json:"candidate_highlight_color,omitempty"`
	CandidateTextColor          *string `json:"candidate_text_color,omitempty"`
	CandidateHighlightTextColor *string `json:"candidate_highlight_text_color,omitempty"`
}

var appearanceState struct {
	mu      sync.RWMutex
	version uint64
	cfg     appearanceConfig
	loaded  bool
}

func cloneStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func cloneIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func cloneBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}

func cloneAppearanceConfig(cfg appearanceConfig) appearanceConfig {
	return appearanceConfig{
		CandidateTheme:              cloneStringPtr(cfg.CandidateTheme),
		FontPoint:                   cloneIntPtr(cfg.FontPoint),
		InlinePreedit:               cloneBoolPtr(cfg.InlinePreedit),
		CandidatePerRow:             cloneIntPtr(cfg.CandidatePerRow),
		CandidateCount:              cloneIntPtr(cfg.CandidateCount),
		CandidateBackgroundColor:    cloneStringPtr(cfg.CandidateBackgroundColor),
		CandidateHighlightColor:     cloneStringPtr(cfg.CandidateHighlightColor),
		CandidateTextColor:          cloneStringPtr(cfg.CandidateTextColor),
		CandidateHighlightTextColor: cloneStringPtr(cfg.CandidateHighlightTextColor),
	}
}

func sharedAppearanceConfig() (appearanceConfig, uint64, bool) {
	appearanceState.mu.RLock()
	defer appearanceState.mu.RUnlock()
	if !appearanceState.loaded {
		return appearanceConfig{}, 0, false
	}
	return cloneAppearanceConfig(appearanceState.cfg), appearanceState.version, true
}

func setSharedAppearanceConfig(cfg appearanceConfig) uint64 {
	appearanceState.mu.Lock()
	defer appearanceState.mu.Unlock()
	appearanceState.version++
	appearanceState.cfg = cloneAppearanceConfig(cfg)
	appearanceState.loaded = true
	return appearanceState.version
}

func resetSharedAppearanceConfigForTest() {
	appearanceState.mu.Lock()
	defer appearanceState.mu.Unlock()
	appearanceState.version = 0
	appearanceState.cfg = appearanceConfig{}
	appearanceState.loaded = false
}

func (ime *IME) applyThemePreset(theme string) bool {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "default":
		ime.style.CandidateTheme = "default"
		ime.style.CandidateBackgroundColor = "#ffffff"
		ime.style.CandidateHighlightColor = "#c6ddf9"
		ime.style.CandidateTextColor = "#000000"
		ime.style.CandidateHighlightTextColor = "#000000"
		return true
	case "theme2":
		ime.style.CandidateTheme = "theme2"
		ime.style.CandidateBackgroundColor = "#000000"
		ime.style.CandidateHighlightColor = "#ff9000"
		ime.style.CandidateTextColor = "#ffffff"
		ime.style.CandidateHighlightTextColor = "#000000"
		return true
	case "moqi":
		ime.style.CandidateTheme = "moqi"
		ime.style.CandidateBackgroundColor = "#000000"
		ime.style.CandidateHighlightColor = "#ffffff"
		ime.style.CandidateTextColor = "#ffffff"
		ime.style.CandidateHighlightTextColor = "#000000"
		return true
	case "purple":
		ime.style.CandidateTheme = "purple"
		ime.style.CandidateBackgroundColor = "#f3e8ff"
		ime.style.CandidateHighlightColor = "#8b5cf6"
		ime.style.CandidateTextColor = "#4c1d95"
		ime.style.CandidateHighlightTextColor = "#ffffff"
		return true
	case "wallgray":
		ime.style.CandidateTheme = "wallgray"
		ime.style.CandidateBackgroundColor = "#d6d3d1"
		ime.style.CandidateHighlightColor = "#94a3b8"
		ime.style.CandidateTextColor = "#44403c"
		ime.style.CandidateHighlightTextColor = "#ffffff"
		return true
	case "orange":
		ime.style.CandidateTheme = "orange"
		ime.style.CandidateBackgroundColor = "#fed7aa"
		ime.style.CandidateHighlightColor = "#ea580c"
		ime.style.CandidateTextColor = "#7c2d12"
		ime.style.CandidateHighlightTextColor = "#ffffff"
		return true
	case "redplum":
		ime.style.CandidateTheme = "redplum"
		ime.style.CandidateBackgroundColor = "#f8efea"
		ime.style.CandidateHighlightColor = "#6f1028"
		ime.style.CandidateTextColor = "#3f1d24"
		ime.style.CandidateHighlightTextColor = "#fff7f5"
		return true
	case "shacheng":
		ime.style.CandidateTheme = "shacheng"
		ime.style.CandidateBackgroundColor = "#2f2424"
		ime.style.CandidateHighlightColor = "#d4af5a"
		ime.style.CandidateTextColor = "#f6dda0"
		ime.style.CandidateHighlightTextColor = "#2b1d14"
		return true
	case "globe":
		ime.style.CandidateTheme = "globe"
		ime.style.CandidateBackgroundColor = "#67d4ff"
		ime.style.CandidateHighlightColor = "#f4c542"
		ime.style.CandidateTextColor = "#083344"
		ime.style.CandidateHighlightTextColor = "#1f2937"
		return true
	case "soymilk":
		ime.style.CandidateTheme = "soymilk"
		ime.style.CandidateBackgroundColor = "#f4efe6"
		ime.style.CandidateHighlightColor = "#a3ad6a"
		ime.style.CandidateTextColor = "#4b4b43"
		ime.style.CandidateHighlightTextColor = "#1f2917"
		return true
	case "chrysanthemum":
		ime.style.CandidateTheme = "chrysanthemum"
		ime.style.CandidateBackgroundColor = "#f7f1dc"
		ime.style.CandidateHighlightColor = "#d6a823"
		ime.style.CandidateTextColor = "#5b4a1d"
		ime.style.CandidateHighlightTextColor = "#fffdf5"
		return true
	case "qinhuangdao":
		ime.style.CandidateTheme = "qinhuangdao"
		ime.style.CandidateBackgroundColor = "#d9edf4"
		ime.style.CandidateHighlightColor = "#5fa7c7"
		ime.style.CandidateTextColor = "#1f4f68"
		ime.style.CandidateHighlightTextColor = "#f8fdff"
		return true
	case "bubblegum":
		ime.style.CandidateTheme = "bubblegum"
		ime.style.CandidateBackgroundColor = "#ffd6eb"
		ime.style.CandidateHighlightColor = "#7ee7d8"
		ime.style.CandidateTextColor = "#7a2e67"
		ime.style.CandidateHighlightTextColor = "#16313a"
		return true
	default:
		return false
	}
}

func isBuiltinTheme(theme string) bool {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "default", "theme2", "moqi", "purple", "wallgray", "orange", "redplum", "shacheng", "globe", "soymilk", "chrysanthemum", "qinhuangdao", "bubblegum":
		return true
	default:
		return false
	}
}

func userAppearanceConfigPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, APP, "Rime", appearanceConfigFileName)
}

func (ime *IME) applyAppearanceConfig(cfg appearanceConfig) {
	themeName := strings.TrimSpace(ime.style.CandidateTheme)
	if cfg.CandidateTheme != nil {
		themeName = strings.ToLower(strings.TrimSpace(*cfg.CandidateTheme))
		if !ime.applyThemePreset(*cfg.CandidateTheme) {
			ime.style.CandidateTheme = themeName
		}
	}
	if cfg.FontPoint != nil && *cfg.FontPoint > 0 {
		ime.style.FontPoint = *cfg.FontPoint
	}
	if cfg.InlinePreedit != nil {
		if *cfg.InlinePreedit {
			ime.style.InlinePreedit = "composition"
		} else {
			ime.style.InlinePreedit = "external"
		}
	}
	if cfg.CandidatePerRow != nil && *cfg.CandidatePerRow > 0 {
		ime.style.CandidatePerRow = *cfg.CandidatePerRow
	}
	if cfg.CandidateCount != nil && *cfg.CandidateCount > 0 {
		ime.style.CandidateCount = *cfg.CandidateCount
	}
	allowCustomColors := !isBuiltinTheme(themeName) || themeName == "custom"
	if allowCustomColors && cfg.CandidateBackgroundColor != nil && normalizeColor(*cfg.CandidateBackgroundColor) != "" {
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateBackgroundColor = normalizeColor(*cfg.CandidateBackgroundColor)
	}
	if allowCustomColors && cfg.CandidateHighlightColor != nil && normalizeColor(*cfg.CandidateHighlightColor) != "" {
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightColor = normalizeColor(*cfg.CandidateHighlightColor)
	}
	if allowCustomColors && cfg.CandidateTextColor != nil && normalizeColor(*cfg.CandidateTextColor) != "" {
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateTextColor = normalizeColor(*cfg.CandidateTextColor)
	}
	if allowCustomColors && cfg.CandidateHighlightTextColor != nil && normalizeColor(*cfg.CandidateHighlightTextColor) != "" {
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightTextColor = normalizeColor(*cfg.CandidateHighlightTextColor)
	}
}

func (ime *IME) loadAppearancePrefs() {
	if cfg, version, ok := sharedAppearanceConfig(); ok {
		ime.applyAppearanceConfig(cfg)
		ime.appearanceVersion = version
		return
	}
	path := userAppearanceConfigPath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cfg appearanceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	ime.applyAppearanceConfig(cfg)
	ime.appearanceVersion = setSharedAppearanceConfig(cfg)
}

func (ime *IME) saveAppearancePrefs() {
	path := userAppearanceConfigPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	theme := ime.style.CandidateTheme
	fontPoint := ime.style.FontPoint
	inlinePreedit := ime.inlinePreeditEnabled()
	candidatePerRow := ime.style.CandidatePerRow
	candidateCount := ime.style.CandidateCount
	cfg := appearanceConfig{
		CandidateTheme:  &theme,
		FontPoint:       &fontPoint,
		InlinePreedit:   &inlinePreedit,
		CandidatePerRow: &candidatePerRow,
		CandidateCount:  &candidateCount,
	}
	if !isBuiltinTheme(theme) || theme == "custom" {
		backgroundColor := ime.style.CandidateBackgroundColor
		highlightColor := ime.style.CandidateHighlightColor
		textColor := ime.style.CandidateTextColor
		highlightTextColor := ime.style.CandidateHighlightTextColor
		cfg.CandidateBackgroundColor = &backgroundColor
		cfg.CandidateHighlightColor = &highlightColor
		cfg.CandidateTextColor = &textColor
		cfg.CandidateHighlightTextColor = &highlightTextColor
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return
	}
	ime.appearanceVersion = setSharedAppearanceConfig(cfg)
}

func (ime *IME) syncAppearancePrefs() {
	cfg, version, ok := sharedAppearanceConfig()
	if !ok || version == ime.appearanceVersion {
		return
	}
	ime.applyAppearanceConfig(cfg)
	ime.appearanceVersion = version
}

func normalizeColor(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "#") {
		value = "#" + value
	}
	if len(value) != 7 {
		return ""
	}
	for _, ch := range value[1:] {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return ""
		}
	}
	return value
}

func (ime *IME) inlinePreeditEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(ime.style.InlinePreedit), "composition")
}

func (ime *IME) customizeUIMap() map[string]interface{} {
	return map[string]interface{}{
		"candFontName":           ime.style.FontFace,
		"candFontSize":           ime.style.FontPoint,
		"candPerRow":             ime.effectiveCandidatePerRow(),
		"candUseCursor":          ime.style.CandidateUseCursor,
		"candBackgroundColor":    normalizeColor(ime.style.CandidateBackgroundColor),
		"candHighlightColor":     normalizeColor(ime.style.CandidateHighlightColor),
		"candTextColor":          normalizeColor(ime.style.CandidateTextColor),
		"candHighlightTextColor": normalizeColor(ime.style.CandidateHighlightTextColor),
		"inlinePreedit":          ime.inlinePreeditEnabled(),
	}
}

func (ime *IME) isHorizontalCandidateLayout() bool {
	return ime.style.CandidatePerRow > 1
}

func (ime *IME) horizontalCandidatePerRow() int {
	switch ime.style.CandidatePerRow {
	case 3, 5, 7, 9:
		return ime.style.CandidatePerRow
	default:
		return 3
	}
}

func (ime *IME) effectiveCandidatePerRow() int {
	if !ime.isHorizontalCandidateLayout() {
		return 1
	}
	return min(ime.horizontalCandidatePerRow(), ime.candidateCount())
}

func (ime *IME) candidateCount() int {
	switch ime.style.CandidateCount {
	case 3, 5, 7, 9:
		return ime.style.CandidateCount
	default:
		return 9
	}
}

func isCandidateCountCommand(commandID int) bool {
	switch commandID {
	case ID_APPEARANCE_CAND_COUNT_3, ID_APPEARANCE_CAND_COUNT_5, ID_APPEARANCE_CAND_COUNT_7, ID_APPEARANCE_CAND_COUNT_9:
		return true
	default:
		return false
	}
}

func (ime *IME) writeCandidateCountConfig() bool {
	userDir := ime.userDir()
	if userDir == "" {
		return false
	}
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return false
	}
	content := fmt.Sprintf("config_version: '%d'\npatch:\n  menu/page_size: %d\n", ime.candidateCount(), ime.candidateCount())
	path := filepath.Join(userDir, rimeDefaultCustomConfigFileName)
	return os.WriteFile(path, []byte(content), 0o644) == nil
}

func (ime *IME) applyAppearanceCommand(commandID int) bool {
	switch commandID {
	case ID_APPEARANCE_INLINE_PREEDIT:
		if ime.inlinePreeditEnabled() {
			ime.style.InlinePreedit = "external"
		} else {
			ime.style.InlinePreedit = "composition"
		}
	case ID_APPEARANCE_LAYOUT_VERTICAL:
		ime.style.CandidatePerRow = 1
	case ID_APPEARANCE_LAYOUT_HORIZONTAL:
		ime.style.CandidatePerRow = ime.horizontalCandidatePerRow()
	case ID_APPEARANCE_PER_ROW_3:
		ime.style.CandidatePerRow = 3
	case ID_APPEARANCE_PER_ROW_5:
		ime.style.CandidatePerRow = 5
	case ID_APPEARANCE_PER_ROW_7:
		ime.style.CandidatePerRow = 7
	case ID_APPEARANCE_PER_ROW_9:
		ime.style.CandidatePerRow = 9
	case ID_APPEARANCE_CAND_COUNT_3:
		ime.style.CandidateCount = 3
	case ID_APPEARANCE_CAND_COUNT_5:
		ime.style.CandidateCount = 5
	case ID_APPEARANCE_CAND_COUNT_7:
		ime.style.CandidateCount = 7
	case ID_APPEARANCE_CAND_COUNT_9:
		ime.style.CandidateCount = 9
	case ID_APPEARANCE_FONT_14:
		ime.style.FontPoint = 14
	case ID_APPEARANCE_FONT_16:
		ime.style.FontPoint = 16
	case ID_APPEARANCE_FONT_18:
		ime.style.FontPoint = 18
	case ID_APPEARANCE_FONT_20:
		ime.style.FontPoint = 20
	case ID_APPEARANCE_FONT_22:
		ime.style.FontPoint = 22
	case ID_APPEARANCE_BG_WHITE:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateBackgroundColor = "#ffffff"
	case ID_APPEARANCE_BG_WARM:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateBackgroundColor = "#fff7e8"
	case ID_APPEARANCE_BG_BLUE:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateBackgroundColor = "#f3f8ff"
	case ID_APPEARANCE_HL_BLUE:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightColor = "#c6ddf9"
	case ID_APPEARANCE_HL_GRAY:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightColor = "#e5e7eb"
	case ID_APPEARANCE_HL_GREEN:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightColor = "#d9f2e6"
	case ID_APPEARANCE_TEXT_BLACK:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateTextColor = "#000000"
	case ID_APPEARANCE_TEXT_DARKGRAY:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateTextColor = "#333333"
	case ID_APPEARANCE_TEXT_BLUE:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateTextColor = "#1d4ed8"
	case ID_APPEARANCE_HLTEXT_BLACK:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightTextColor = "#000000"
	case ID_APPEARANCE_HLTEXT_WHITE:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightTextColor = "#ffffff"
	case ID_APPEARANCE_HLTEXT_BLUE:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightTextColor = "#1d4ed8"
	case ID_APPEARANCE_THEME_DEFAULT:
		ime.applyThemePreset("default")
	case ID_APPEARANCE_THEME_2:
		ime.applyThemePreset("theme2")
	case ID_APPEARANCE_THEME_MOQI:
		ime.applyThemePreset("moqi")
	case ID_APPEARANCE_THEME_PURPLE:
		ime.applyThemePreset("purple")
	case ID_APPEARANCE_THEME_WALLGRAY:
		ime.applyThemePreset("wallgray")
	case ID_APPEARANCE_THEME_ORANGE:
		ime.applyThemePreset("orange")
	case ID_APPEARANCE_THEME_REDPLUM:
		ime.applyThemePreset("redplum")
	case ID_APPEARANCE_THEME_SHACHENG:
		ime.applyThemePreset("shacheng")
	case ID_APPEARANCE_THEME_GLOBE:
		ime.applyThemePreset("globe")
	case ID_APPEARANCE_THEME_SOYMILK:
		ime.applyThemePreset("soymilk")
	case ID_APPEARANCE_THEME_CHRYSANTHEMUM:
		ime.applyThemePreset("chrysanthemum")
	case ID_APPEARANCE_THEME_QINHUANGDAO:
		ime.applyThemePreset("qinhuangdao")
	case ID_APPEARANCE_THEME_BUBBLEGUM:
		ime.applyThemePreset("bubblegum")
	default:
		return false
	}
	ime.saveAppearancePrefs()
	return true
}
