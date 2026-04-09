package rime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

const appearanceConfigFileName = "appearance_config.json"

type appearanceConfig struct {
	CandidateTheme              *string `json:"candidate_theme,omitempty"`
	FontPoint                   *int    `json:"font_point,omitempty"`
	InlinePreedit               *bool   `json:"inline_preedit,omitempty"`
	CandidateBackgroundColor    *string `json:"candidate_background_color,omitempty"`
	CandidateHighlightColor     *string `json:"candidate_highlight_color,omitempty"`
	CandidateTextColor          *string `json:"candidate_text_color,omitempty"`
	CandidateHighlightTextColor *string `json:"candidate_highlight_text_color,omitempty"`
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

func (ime *IME) loadAppearancePrefs() {
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
	if cfg.CandidateTheme != nil && !ime.applyThemePreset(*cfg.CandidateTheme) {
		ime.style.CandidateTheme = strings.ToLower(strings.TrimSpace(*cfg.CandidateTheme))
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
	if cfg.CandidateBackgroundColor != nil && normalizeColor(*cfg.CandidateBackgroundColor) != "" {
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateBackgroundColor = normalizeColor(*cfg.CandidateBackgroundColor)
	}
	if cfg.CandidateHighlightColor != nil && normalizeColor(*cfg.CandidateHighlightColor) != "" {
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightColor = normalizeColor(*cfg.CandidateHighlightColor)
	}
	if cfg.CandidateTextColor != nil && normalizeColor(*cfg.CandidateTextColor) != "" {
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateTextColor = normalizeColor(*cfg.CandidateTextColor)
	}
	if cfg.CandidateHighlightTextColor != nil && normalizeColor(*cfg.CandidateHighlightTextColor) != "" {
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightTextColor = normalizeColor(*cfg.CandidateHighlightTextColor)
	}
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
	backgroundColor := ime.style.CandidateBackgroundColor
	highlightColor := ime.style.CandidateHighlightColor
	textColor := ime.style.CandidateTextColor
	highlightTextColor := ime.style.CandidateHighlightTextColor
	cfg := appearanceConfig{
		CandidateTheme:              &theme,
		FontPoint:                   &fontPoint,
		InlinePreedit:               &inlinePreedit,
		CandidateBackgroundColor:    &backgroundColor,
		CandidateHighlightColor:     &highlightColor,
		CandidateTextColor:          &textColor,
		CandidateHighlightTextColor: &highlightTextColor,
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
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
		"candPerRow":             ime.style.CandidatePerRow,
		"candUseCursor":          ime.style.CandidateUseCursor,
		"candBackgroundColor":    normalizeColor(ime.style.CandidateBackgroundColor),
		"candHighlightColor":     normalizeColor(ime.style.CandidateHighlightColor),
		"candTextColor":          normalizeColor(ime.style.CandidateTextColor),
		"candHighlightTextColor": normalizeColor(ime.style.CandidateHighlightTextColor),
		"inlinePreedit":          ime.inlinePreeditEnabled(),
	}
}

func (ime *IME) applyAppearanceCommand(commandID int) bool {
	switch commandID {
	case ID_APPEARANCE_INLINE_PREEDIT:
		if ime.inlinePreeditEnabled() {
			ime.style.InlinePreedit = "external"
		} else {
			ime.style.InlinePreedit = "composition"
		}
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
	default:
		return false
	}
	ime.saveAppearancePrefs()
	return true
}
