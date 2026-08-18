package rime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gaboolic/moqi-ime/imecore"
)

const appearanceConfigFileName = "appearance_config.json"
const rimeDefaultCustomConfigFileName = "default.custom.yaml"

type appearanceConfig struct {
	CandidateTheme                 *string           `json:"candidate_theme,omitempty"`
	FontFace                       *string           `json:"font_face,omitempty"`
	FontPoint                      *int              `json:"font_point,omitempty"`
	CandidateCommentFontFace       *string           `json:"candidate_comment_font_face,omitempty"`
	CandidateCommentFontPoint      *int              `json:"candidate_comment_font_point,omitempty"`
	InlinePreedit                  *bool             `json:"inline_preedit,omitempty"`
	CandidatePerRow                *int              `json:"candidate_per_row,omitempty"`
	CandidateCount                 *int              `json:"candidate_count,omitempty"`
	CandidateSpacing               *int              `json:"candidate_spacing,omitempty"`
	CandidateBackgroundColor       *string           `json:"candidate_background_color,omitempty"`
	CandidateHighlightColor        *string           `json:"candidate_highlight_color,omitempty"`
	CandidateTextColor             *string           `json:"candidate_text_color,omitempty"`
	CandidateHighlightTextColor    *string           `json:"candidate_highlight_text_color,omitempty"`
	CandidateCommentColor          *string           `json:"candidate_comment_color,omitempty"`
	CandidateCommentHighlightColor *string           `json:"candidate_comment_highlight_color,omitempty"`
	InputStateShared               *bool             `json:"input_state_shared,omitempty"`
	SharedOptions                  map[string]bool   `json:"shared_options,omitempty"`
	SyncedOptions                  map[string]bool   `json:"synced_options,omitempty"`
	CurrentSchemaID                *string           `json:"current_schema_id,omitempty"`
	CurrentSchemaBySchemeSet       map[string]string `json:"current_schema_by_scheme_set,omitempty"`
	SharedAsciiMode                *bool             `json:"shared_ascii_mode,omitempty"`
	SharedFullShape                *bool             `json:"shared_full_shape,omitempty"`
	SharedTraditionalization       *bool             `json:"shared_traditionalization,omitempty"`
	AutoPairQuotes                 *bool             `json:"auto_pair_quotes,omitempty"`
	SemicolonSelectSecond          *bool             `json:"semicolon_select_second,omitempty"`
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
		CandidateTheme:                 cloneStringPtr(cfg.CandidateTheme),
		FontFace:                       cloneStringPtr(cfg.FontFace),
		FontPoint:                      cloneIntPtr(cfg.FontPoint),
		CandidateCommentFontFace:       cloneStringPtr(cfg.CandidateCommentFontFace),
		CandidateCommentFontPoint:      cloneIntPtr(cfg.CandidateCommentFontPoint),
		InlinePreedit:                  cloneBoolPtr(cfg.InlinePreedit),
		CandidatePerRow:                cloneIntPtr(cfg.CandidatePerRow),
		CandidateCount:                 cloneIntPtr(cfg.CandidateCount),
		CandidateSpacing:               cloneIntPtr(cfg.CandidateSpacing),
		CandidateBackgroundColor:       cloneStringPtr(cfg.CandidateBackgroundColor),
		CandidateHighlightColor:        cloneStringPtr(cfg.CandidateHighlightColor),
		CandidateTextColor:             cloneStringPtr(cfg.CandidateTextColor),
		CandidateHighlightTextColor:    cloneStringPtr(cfg.CandidateHighlightTextColor),
		CandidateCommentColor:          cloneStringPtr(cfg.CandidateCommentColor),
		CandidateCommentHighlightColor: cloneStringPtr(cfg.CandidateCommentHighlightColor),
		InputStateShared:               cloneBoolPtr(cfg.InputStateShared),
		SharedOptions:                  cloneBoolMap(cfg.SharedOptions),
		SyncedOptions:                  cloneBoolMap(cfg.SyncedOptions),
		CurrentSchemaID:                cloneStringPtr(cfg.CurrentSchemaID),
		CurrentSchemaBySchemeSet:       cloneStringMap(cfg.CurrentSchemaBySchemeSet),
		SharedAsciiMode:                cloneBoolPtr(cfg.SharedAsciiMode),
		SharedFullShape:                cloneBoolPtr(cfg.SharedFullShape),
		SharedTraditionalization:       cloneBoolPtr(cfg.SharedTraditionalization),
		AutoPairQuotes:                 cloneBoolPtr(cfg.AutoPairQuotes),
		SemicolonSelectSecond:          cloneBoolPtr(cfg.SemicolonSelectSecond),
	}
}

func cloneBoolMap(values map[string]bool) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]bool, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
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

func userAppearanceConfigPath() string {
	root := moqiAppDataDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, appearanceConfigFileName)
}

func legacyUserAppearanceConfigPath() string {
	root := moqiAppDataDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, defaultSchemeSetName, appearanceConfigFileName)
}

func (ime *IME) applyAppearanceConfig(cfg appearanceConfig) {
	themeName := strings.TrimSpace(ime.style.CandidateTheme)
	if cfg.CandidateTheme != nil {
		themeName = strings.ToLower(strings.TrimSpace(*cfg.CandidateTheme))
		if theme, ok := lookupRegisteredTheme(themeName); ok {
			applyThemeAppearanceToStyle(ime, theme)
			themeName = theme.ID
		} else {
			ime.style.CandidateTheme = themeName
		}
	}
	if cfg.FontFace != nil && strings.TrimSpace(*cfg.FontFace) != "" {
		ime.style.FontFace = strings.TrimSpace(*cfg.FontFace)
	}
	if cfg.FontPoint != nil && *cfg.FontPoint > 0 {
		ime.style.FontPoint = *cfg.FontPoint
	}
	if cfg.CandidateCommentFontFace != nil && strings.TrimSpace(*cfg.CandidateCommentFontFace) != "" {
		ime.style.CandidateCommentFontFace = strings.TrimSpace(*cfg.CandidateCommentFontFace)
	}
	if cfg.CandidateCommentFontPoint != nil && *cfg.CandidateCommentFontPoint > 0 {
		ime.style.CandidateCommentFontPoint = *cfg.CandidateCommentFontPoint
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
	if cfg.CandidateSpacing != nil && *cfg.CandidateSpacing >= 0 {
		ime.style.CandidateSpacing = *cfg.CandidateSpacing
	}
	allowCustomColors := !isRegisteredTheme(themeName) || themeName == "custom"
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
	if allowCustomColors {
		ime.style.CandidateCommentColor = ime.style.CandidateTextColor
		ime.style.CandidateCommentHighlightColor = ime.style.CandidateHighlightTextColor
	}
	if allowCustomColors && cfg.CandidateCommentColor != nil && normalizeColor(*cfg.CandidateCommentColor) != "" {
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateCommentColor = normalizeColor(*cfg.CandidateCommentColor)
	}
	if allowCustomColors && cfg.CandidateCommentHighlightColor != nil && normalizeColor(*cfg.CandidateCommentHighlightColor) != "" {
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateCommentHighlightColor = normalizeColor(*cfg.CandidateCommentHighlightColor)
	}
	if cfg.InputStateShared != nil {
		ime.inputStateShared = *cfg.InputStateShared
	}
	if len(cfg.SharedOptions) > 0 {
		ime.sharedOptions = cloneBoolMap(cfg.SharedOptions)
	}
	if len(cfg.SyncedOptions) > 0 {
		ime.syncedOptions = cloneBoolMap(cfg.SyncedOptions)
	}
	if ime.syncedOptions == nil {
		ime.syncedOptions = make(map[string]bool)
	}
	if cfg.CurrentSchemaID != nil {
		ime.syncedSchemaID = strings.TrimSpace(*cfg.CurrentSchemaID)
		ime.setSyncedSchemaIDForCurrentSchemeSet(ime.syncedSchemaID)
	}
	if len(cfg.CurrentSchemaBySchemeSet) > 0 {
		ime.syncedSchemaBySchemeSet = cloneStringMap(cfg.CurrentSchemaBySchemeSet)
		if schemaID := strings.TrimSpace(ime.syncedSchemaBySchemeSet[currentSchemeSetName()]); schemaID != "" {
			ime.syncedSchemaID = schemaID
		}
	}
	if ime.sharedOptions == nil {
		ime.sharedOptions = make(map[string]bool)
	}
	if cfg.SharedAsciiMode != nil {
		ime.sharedOptions["ascii_mode"] = *cfg.SharedAsciiMode
	}
	if cfg.SharedFullShape != nil {
		ime.sharedOptions["full_shape"] = *cfg.SharedFullShape
	}
	if cfg.SharedTraditionalization != nil {
		ime.sharedOptions["traditionalization"] = *cfg.SharedTraditionalization
	}
	if cfg.AutoPairQuotes != nil {
		ime.autoPairQuotes = *cfg.AutoPairQuotes
	}
	if cfg.SemicolonSelectSecond != nil {
		ime.semicolonSelectSecond = *cfg.SemicolonSelectSecond
	}
}

func (ime *IME) loadAppearancePrefs() {
	if cfg, version, ok := sharedAppearanceConfig(); ok {
		ime.applyAppearanceConfig(cfg)
		ime.appearanceVersion = version
		return
	}

	primaryPath := userAppearanceConfigPath()
	if primaryPath == "" {
		return
	}

	var (
		data       []byte
		err        error
		loadedPath string
	)
	for _, path := range []string{primaryPath, legacyUserAppearanceConfigPath()} {
		if path == "" {
			continue
		}
		data, err = os.ReadFile(path)
		if err == nil {
			loadedPath = path
			break
		}
		if !os.IsNotExist(err) {
			return
		}
	}
	if loadedPath == "" {
		ime.saveAppearancePrefsWithReason("loadAppearancePrefs:create_default_config")
		return
	}

	var cfg appearanceConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	ime.applyAppearanceConfig(cfg)
	ime.appearanceVersion = setSharedAppearanceConfig(cfg)
	if loadedPath != primaryPath {
		ime.saveAppearancePrefsWithReason("loadAppearancePrefs:migrate_legacy_config")
	}
}

func (ime *IME) saveAppearancePrefs() {
	ime.saveAppearancePrefsWithReason("unspecified")
}

func (ime *IME) saveAppearancePrefsWithReason(reason string) {
	path := userAppearanceConfigPath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	theme := ime.style.CandidateTheme
	fontFace := ime.style.FontFace
	fontPoint := ime.style.FontPoint
	commentFontFace := ime.style.CandidateCommentFontFace
	commentFontPoint := ime.style.CandidateCommentFontPoint
	inlinePreedit := ime.inlinePreeditEnabled()
	candidatePerRow := ime.style.CandidatePerRow
	candidateCount := ime.style.CandidateCount
	candidateSpacing := ime.style.CandidateSpacing
	cfg := appearanceConfig{
		CandidateTheme:            &theme,
		FontFace:                  &fontFace,
		FontPoint:                 &fontPoint,
		CandidateCommentFontFace:  &commentFontFace,
		CandidateCommentFontPoint: &commentFontPoint,
		InlinePreedit:             &inlinePreedit,
		CandidatePerRow:           &candidatePerRow,
		CandidateCount:            &candidateCount,
		CandidateSpacing:          &candidateSpacing,
	}
	inputStateShared := ime.inputStateShared
	cfg.InputStateShared = &inputStateShared
	autoPairQuotes := ime.autoPairQuotes
	cfg.AutoPairQuotes = &autoPairQuotes
	semicolonSelectSecond := ime.semicolonSelectSecond
	cfg.SemicolonSelectSecond = &semicolonSelectSecond
	if !isRegisteredTheme(theme) || theme == "custom" {
		backgroundColor := ime.style.CandidateBackgroundColor
		highlightColor := ime.style.CandidateHighlightColor
		textColor := ime.style.CandidateTextColor
		highlightTextColor := ime.style.CandidateHighlightTextColor
		commentColor := ime.style.CandidateCommentColor
		commentHighlightColor := ime.style.CandidateCommentHighlightColor
		cfg.CandidateBackgroundColor = &backgroundColor
		cfg.CandidateHighlightColor = &highlightColor
		cfg.CandidateTextColor = &textColor
		cfg.CandidateHighlightTextColor = &highlightTextColor
		cfg.CandidateCommentColor = &commentColor
		cfg.CandidateCommentHighlightColor = &commentHighlightColor
	}
	if ime.inputStateShared {
		cfg.SharedOptions = cloneBoolMap(ime.sharedOptions)
	}
	if len(ime.syncedOptions) > 0 {
		cfg.SyncedOptions = cloneBoolMap(ime.syncedOptions)
	}
	if strings.TrimSpace(ime.syncedSchemaIDForCurrentSchemeSet()) != "" {
		currentSchemaID := strings.TrimSpace(ime.syncedSchemaIDForCurrentSchemeSet())
		cfg.CurrentSchemaID = &currentSchemaID
	}
	if len(ime.syncedSchemaBySchemeSet) > 0 {
		cfg.CurrentSchemaBySchemeSet = cloneStringMap(ime.syncedSchemaBySchemeSet)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return
	}
	debugLogf("saveAppearancePrefs triggered_by=%s path=%q candidate_per_row=%d candidate_count=%d inline_preedit=%t input_state_shared=%t auto_pair_quotes=%t semicolon_select_second=%t theme=%q",
		strings.TrimSpace(reason),
		path,
		candidatePerRow,
		candidateCount,
		inlinePreedit,
		inputStateShared,
		autoPairQuotes,
		semicolonSelectSecond,
		theme,
	)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return
	}
	ime.appearanceVersion = setSharedAppearanceConfig(cfg)
}

func (ime *IME) syncAppearancePrefs() bool {
	cfg, version, ok := sharedAppearanceConfig()
	if !ok || version == ime.appearanceVersion {
		return false
	}
	ime.applyAppearanceConfig(cfg)
	ime.appearanceVersion = version
	return true
}

func (ime *IME) sendAsyncAppearanceUpdate(notification *imecore.TrayNotification) {
	if ime.asyncResponseSender == nil {
		return
	}
	resp := imecore.NewResponse(0, true)
	resp.CustomizeUI = ime.customizeUIMap()
	ime.fillResponseFromCurrentState(resp)
	if notification != nil {
		resp.TrayNotification = notification
	}
	ime.asyncResponseSender(resp)
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
		"candFontName":              ime.style.FontFace,
		"candFontSize":              ime.style.FontPoint,
		"candCommentFontName":       ime.style.CandidateCommentFontFace,
		"candCommentFontSize":       ime.style.CandidateCommentFontPoint,
		"candPerRow":                ime.effectiveCandidatePerRow(),
		"candSpacing":               ime.style.CandidateSpacing,
		"candUseCursor":             ime.style.CandidateUseCursor,
		"candBackgroundColor":       normalizeColor(ime.style.CandidateBackgroundColor),
		"candHighlightColor":        normalizeColor(ime.style.CandidateHighlightColor),
		"candTextColor":             normalizeColor(ime.style.CandidateTextColor),
		"candHighlightTextColor":    normalizeColor(ime.style.CandidateHighlightTextColor),
		"candCommentColor":          normalizeColor(ime.style.CandidateCommentColor),
		"candCommentHighlightColor": normalizeColor(ime.style.CandidateCommentHighlightColor),
		"inlinePreedit":             ime.inlinePreeditEnabled(),
		"autoPairQuotes":            ime.autoPairQuotes,
		"autoPairRules":             ime.currentAutoPairRules(),
		"semicolonSelectSecond":     ime.semicolonSelectSecond,
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
		return 9
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
	if ime.handleThemeCommand(commandID) {
		ime.saveAppearancePrefsWithReason(fmt.Sprintf("applyAppearanceCommand:commandID=%d", commandID))
		return true
	}
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
	case ID_APPEARANCE_SPACING_0:
		ime.style.CandidateSpacing = 0
	case ID_APPEARANCE_SPACING_10:
		ime.style.CandidateSpacing = 10
	case ID_APPEARANCE_SPACING_20:
		ime.style.CandidateSpacing = 20
	case ID_APPEARANCE_SPACING_30:
		ime.style.CandidateSpacing = 30
	case ID_APPEARANCE_SPACING_40:
		ime.style.CandidateSpacing = 40
	case ID_APPEARANCE_SPACING_50:
		ime.style.CandidateSpacing = 50
	case ID_APPEARANCE_CAND_COUNT_3:
		ime.style.CandidateCount = 3
	case ID_APPEARANCE_CAND_COUNT_5:
		ime.style.CandidateCount = 5
	case ID_APPEARANCE_CAND_COUNT_7:
		ime.style.CandidateCount = 7
	case ID_APPEARANCE_CAND_COUNT_9:
		ime.style.CandidateCount = 9
	case ID_APPEARANCE_FONT_8:
		ime.style.FontPoint = 8
	case ID_APPEARANCE_FONT_10:
		ime.style.FontPoint = 10
	case ID_APPEARANCE_FONT_12:
		ime.style.FontPoint = 12
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
	case ID_APPEARANCE_FONT_24:
		ime.style.FontPoint = 24
	case ID_APPEARANCE_FONT_26:
		ime.style.FontPoint = 26
	case ID_APPEARANCE_FONT_28:
		ime.style.FontPoint = 28
	case ID_APPEARANCE_FONT_30:
		ime.style.FontPoint = 30
	case ID_APPEARANCE_FONT_FAMILY_SEGOE_UI:
		ime.style.FontFace = "Segoe UI"
	case ID_APPEARANCE_FONT_FAMILY_YAHEI_UI:
		ime.style.FontFace = "Microsoft YaHei UI"
	case ID_APPEARANCE_FONT_FAMILY_DENGXIAN:
		ime.style.FontFace = "DengXian"
	case ID_APPEARANCE_FONT_FAMILY_SIMSUN:
		ime.style.FontFace = "SimSun"
	case ID_APPEARANCE_COMMENT_FONT_8:
		ime.style.CandidateCommentFontPoint = 8
	case ID_APPEARANCE_COMMENT_FONT_10:
		ime.style.CandidateCommentFontPoint = 10
	case ID_APPEARANCE_COMMENT_FONT_12:
		ime.style.CandidateCommentFontPoint = 12
	case ID_APPEARANCE_COMMENT_FONT_14:
		ime.style.CandidateCommentFontPoint = 14
	case ID_APPEARANCE_COMMENT_FONT_16:
		ime.style.CandidateCommentFontPoint = 16
	case ID_APPEARANCE_COMMENT_FONT_18:
		ime.style.CandidateCommentFontPoint = 18
	case ID_APPEARANCE_COMMENT_FONT_20:
		ime.style.CandidateCommentFontPoint = 20
	case ID_APPEARANCE_COMMENT_FONT_22:
		ime.style.CandidateCommentFontPoint = 22
	case ID_APPEARANCE_COMMENT_FONT_24:
		ime.style.CandidateCommentFontPoint = 24
	case ID_APPEARANCE_COMMENT_FONT_26:
		ime.style.CandidateCommentFontPoint = 26
	case ID_APPEARANCE_COMMENT_FONT_28:
		ime.style.CandidateCommentFontPoint = 28
	case ID_APPEARANCE_COMMENT_FONT_30:
		ime.style.CandidateCommentFontPoint = 30
	case ID_APPEARANCE_COMMENT_FONT_FAMILY_CONSOLAS:
		ime.style.CandidateCommentFontFace = "Consolas"
	case ID_APPEARANCE_COMMENT_FONT_FAMILY_YAHEI_UI:
		ime.style.CandidateCommentFontFace = "Microsoft YaHei UI"
	case ID_APPEARANCE_COMMENT_FONT_FAMILY_DENGXIAN:
		ime.style.CandidateCommentFontFace = "DengXian"
	case ID_APPEARANCE_COMMENT_FONT_FAMILY_SIMSUN:
		ime.style.CandidateCommentFontFace = "SimSun"
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
		ime.style.CandidateCommentColor = ime.style.CandidateTextColor
	case ID_APPEARANCE_TEXT_DARKGRAY:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateTextColor = "#333333"
		ime.style.CandidateCommentColor = ime.style.CandidateTextColor
	case ID_APPEARANCE_TEXT_BLUE:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateTextColor = "#1d4ed8"
		ime.style.CandidateCommentColor = ime.style.CandidateTextColor
	case ID_APPEARANCE_HLTEXT_BLACK:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightTextColor = "#000000"
		ime.style.CandidateCommentHighlightColor = ime.style.CandidateHighlightTextColor
	case ID_APPEARANCE_HLTEXT_WHITE:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightTextColor = "#ffffff"
		ime.style.CandidateCommentHighlightColor = ime.style.CandidateHighlightTextColor
	case ID_APPEARANCE_HLTEXT_BLUE:
		ime.style.CandidateTheme = "custom"
		ime.style.CandidateHighlightTextColor = "#1d4ed8"
		ime.style.CandidateCommentHighlightColor = ime.style.CandidateHighlightTextColor
	default:
		return false
	}
	ime.saveAppearancePrefsWithReason(fmt.Sprintf("applyAppearanceCommand:commandID=%d", commandID))
	return true
}
