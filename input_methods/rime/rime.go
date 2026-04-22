// RIME 输入法 Go 实现
// 对齐 python/input_methods/rime/rime_ime.py
package rime

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gaboolic/moqi-ime/imecore"
)

const (
	APP         = "Moqi"
	APP_VERSION = "0.01"

	ID_MODE_ICON          = 1
	ID_ASCII_MODE         = 2
	ID_FULL_SHAPE         = 3
	ID_ASCII_PUNCT        = 4
	ID_TRADITIONALIZATION = 5
	ID_DEPLOY             = 10
	ID_SYNC               = 11
	ID_SYNC_DIR           = 12
	ID_SHARED_DIR         = 13
	ID_USER_DIR           = 14
	ID_LOG_DIR            = 16
	ID_UPDATE_CONFIG      = 17
	ID_SCHEMA_BASE        = 1000
	ID_SWITCH_BASE        = 2000
	ID_SCHEME_SET_BASE    = 3000

	ID_APPEARANCE_INLINE_PREEDIT               = 100
	ID_APPEARANCE_FONT_14                      = 110
	ID_APPEARANCE_FONT_16                      = 111
	ID_APPEARANCE_FONT_18                      = 112
	ID_APPEARANCE_FONT_20                      = 113
	ID_APPEARANCE_FONT_22                      = 114
	ID_APPEARANCE_COMMENT_FONT_14              = 115
	ID_APPEARANCE_COMMENT_FONT_16              = 116
	ID_APPEARANCE_COMMENT_FONT_18              = 117
	ID_APPEARANCE_COMMENT_FONT_20              = 118
	ID_APPEARANCE_COMMENT_FONT_22              = 119
	ID_APPEARANCE_BG_WHITE                     = 120
	ID_APPEARANCE_BG_WARM                      = 121
	ID_APPEARANCE_BG_BLUE                      = 122
	ID_APPEARANCE_HL_BLUE                      = 130
	ID_APPEARANCE_HL_GRAY                      = 131
	ID_APPEARANCE_HL_GREEN                     = 132
	ID_APPEARANCE_TEXT_BLACK                   = 140
	ID_APPEARANCE_TEXT_DARKGRAY                = 141
	ID_APPEARANCE_TEXT_BLUE                    = 142
	ID_APPEARANCE_HLTEXT_BLACK                 = 145
	ID_APPEARANCE_HLTEXT_WHITE                 = 146
	ID_APPEARANCE_HLTEXT_BLUE                  = 147
	ID_APPEARANCE_THEME_DEFAULT                = 150
	ID_APPEARANCE_THEME_2                      = 151
	ID_APPEARANCE_THEME_MOQI                   = 152
	ID_APPEARANCE_THEME_PURPLE                 = 153
	ID_APPEARANCE_THEME_WALLGRAY               = 154
	ID_APPEARANCE_THEME_ORANGE                 = 155
	ID_APPEARANCE_THEME_REDPLUM                = 156
	ID_APPEARANCE_THEME_SHACHENG               = 157
	ID_APPEARANCE_THEME_GLOBE                  = 158
	ID_APPEARANCE_THEME_SOYMILK                = 159
	ID_APPEARANCE_THEME_CHRYSANTHEMUM          = 160
	ID_APPEARANCE_THEME_QINHUANGDAO            = 161
	ID_APPEARANCE_THEME_BUBBLEGUM              = 162
	ID_APPEARANCE_THEME_PEPSI                  = 163
	ID_APPEARANCE_FONT_FAMILY_SEGOE_UI         = 194
	ID_APPEARANCE_FONT_FAMILY_YAHEI_UI         = 195
	ID_APPEARANCE_FONT_FAMILY_DENGXIAN         = 196
	ID_APPEARANCE_FONT_FAMILY_SIMSUN           = 197
	ID_APPEARANCE_COMMENT_FONT_FAMILY_CONSOLAS = 198
	ID_APPEARANCE_COMMENT_FONT_FAMILY_YAHEI_UI = 199
	ID_APPEARANCE_COMMENT_FONT_FAMILY_DENGXIAN = 200
	ID_APPEARANCE_COMMENT_FONT_FAMILY_SIMSUN   = 201
	ID_APPEARANCE_LAYOUT_VERTICAL              = 170
	ID_APPEARANCE_LAYOUT_HORIZONTAL            = 171
	ID_APPEARANCE_PER_ROW_3                    = 180
	ID_APPEARANCE_PER_ROW_5                    = 181
	ID_APPEARANCE_PER_ROW_7                    = 182
	ID_APPEARANCE_PER_ROW_9                    = 183
	ID_APPEARANCE_SPACING_0                    = 184
	ID_APPEARANCE_SPACING_10                   = 185
	ID_APPEARANCE_SPACING_20                   = 186
	ID_APPEARANCE_SPACING_30                   = 187
	ID_APPEARANCE_SPACING_40                   = 188
	ID_APPEARANCE_SPACING_50                   = 189
	ID_APPEARANCE_CAND_COUNT_3                 = 190
	ID_APPEARANCE_CAND_COUNT_5                 = 191
	ID_APPEARANCE_CAND_COUNT_7                 = 192
	ID_APPEARANCE_CAND_COUNT_9                 = 193
	ID_SHARED_INPUT_STATE                      = 210
	ID_INPUT_AUTO_PAIR_QUOTES                  = 220
	ID_INPUT_SEMICOLON_SELECT_SECOND           = 221

	aiSelectKeys     = "123456789"
	aiHotkeyKeyCode  = 0x47 // G
	aiCandidateLimit = 3
	secondSelectChar = ';'
)

type Style struct {
	DisplayTrayIcon                bool
	CandidateFormat                string
	CandidatePerRow                int
	CandidateCount                 int
	CandidateUseCursor             bool
	CandidateTheme                 string
	CandidateBackgroundColor       string
	CandidateHighlightColor        string
	CandidateTextColor             string
	CandidateHighlightTextColor    string
	CandidateCommentColor          string
	CandidateCommentHighlightColor string
	CandidateSpacing               int
	FontFace                       string
	FontPoint                      int
	CandidateCommentFontFace       string
	CandidateCommentFontPoint      int
	InlinePreedit                  string
	SoftCursor                     bool
}

type candidateItem struct {
	Text    string
	Comment string
}

type rimeState struct {
	CommitString    string
	Composition     string
	RawInput        string
	PageNo          int
	CursorPos       int
	SelStart        int
	SelEnd          int
	Candidates      []candidateItem
	CandidateCursor int
	SelectKeys      string
	AsciiMode       bool
	FullShape       bool
}

type rimeBackend interface {
	Initialize(sharedDir, userDir string, firstRun bool) bool
	Redeploy(sharedDir, userDir string) bool
	SyncUserData() bool
	HasSession() bool
	EnsureSession() bool
	DestroySession()
	ClearComposition()
	ProcessKey(req *imecore.Request, translatedKeyCode, modifiers int) bool
	State() rimeState
	SetOption(name string, value bool)
	GetOption(name string) bool
	SaveOptions() []string
	SchemaSwitches() []RimeSwitch
	SchemaList() []RimeSchema
	CurrentSchemaID() string
	SelectSchema(schemaID string) bool
	SetCandidatePageSize(pageSize int) bool
	SelectCandidate(index int) bool
	HighlightCandidate(index int) bool
	ChangePage(backward bool) bool
}

type IME struct {
	mu sync.Mutex
	*imecore.TextServiceBase
	iconDir                      string
	style                        Style
	selectKeys                   string
	lastKeyDownCode              int
	lastKeySkip                  int
	lastKeyDownRet               bool
	lastKeyUpCode                int
	lastKeyUpRet                 bool
	keyComposing                 bool
	backend                      rimeBackend
	aiEnabled                    bool
	aiActive                     bool
	aiPending                    bool
	aiPrompt                     string
	aiCandidates                 []string
	aiCandidateCursor            int
	aiError                      string
	aiTriggerPending             bool
	aiConsumeKeyUpCode           int
	aiPreviousCommit             string
	aiActions                    []aiAction
	aiCurrentAction              *aiAction
	aiReviewGenerator            func(aiGenerateRequest) ([]string, error)
	aiResultCh                   chan aiAsyncResult
	asyncResponseSender          func(*imecore.Response)
	aiRequestSeq                 uint64
	appearanceVersion            uint64
	autoPairRulesVersion         uint64
	inputStateShared             bool
	sharedOptions                map[string]bool
	sharedInputStateNeedsApply   bool
	autoPairQuotes               bool
	semicolonSelectSecond        bool
	rawInputTracked              string
	customPhraseComposition      string
	customPhraseCandidates       []candidateItem
	customPhraseCursor           int
	customPhraseConsumeKeyUpCode int
	superAbbrevConsumeKeyUpCode  int
	secondSelectConsumeKeyUpCode int
}

type aiAsyncResult struct {
	RequestSeq uint64
	Prompt     string
	Action     aiAction
	Candidates []string
	Err        error
}

func defaultStyle() Style {
	return Style{
		DisplayTrayIcon:                true,
		CandidateFormat:                "{0} {1}",
		CandidatePerRow:                1,
		CandidateCount:                 9,
		CandidateUseCursor:             true,
		CandidateTheme:                 "default",
		CandidateBackgroundColor:       "#ffffff",
		CandidateHighlightColor:        "#c6ddf9",
		CandidateTextColor:             "#000000",
		CandidateHighlightTextColor:    "#000000",
		CandidateCommentColor:          "#000000",
		CandidateCommentHighlightColor: "#000000",
		CandidateSpacing:               20,
		FontFace:                       "Segoe UI",
		FontPoint:                      20,
		CandidateCommentFontFace:       "Consolas",
		CandidateCommentFontPoint:      18,
		InlinePreedit:                  "composition",
		SoftCursor:                     false,
	}
}

var deployConfigFileFunc = DeployConfigFile
var startMaintenanceFunc = StartMaintenance
var joinMaintenanceThreadFunc = JoinMaintenanceThread

func New(client *imecore.Client) imecore.TextService {
	cfg, err := loadAIConfig()
	if err != nil {
		log.Printf("加载 AI 配置失败: %v", err)
	}
	generator := newConfiguredAIReviewGenerator(cfg)
	actions := defaultAIActions(cfg)
	ime := &IME{
		TextServiceBase:   imecore.NewTextServiceBase(client),
		style:             defaultStyle(),
		aiEnabled:         generator != nil && len(actions) > 0,
		aiActions:         actions,
		aiReviewGenerator: generator,
		aiResultCh:        make(chan aiAsyncResult, 4),
	}
	ime.loadAppearancePrefs()
	return ime
}

func (ime *IME) SetAIReviewGenerator(generator func(aiGenerateRequest) ([]string, error)) {
	ime.aiReviewGenerator = generator
	if len(ime.aiActions) == 0 && generator != nil {
		ime.aiActions = []aiAction{defaultAIAction()}
	}
	ime.aiEnabled = generator != nil && len(ime.aiActions) > 0
	if generator == nil {
		ime.resetAIState()
	}
}

func (ime *IME) SetAsyncResponseSender(sender func(*imecore.Response)) {
	ime.asyncResponseSender = sender
}

func (ime *IME) ensureAIResultCh() chan aiAsyncResult {
	if ime.aiResultCh == nil {
		ime.aiResultCh = make(chan aiAsyncResult, 4)
	}
	return ime.aiResultCh
}

func (ime *IME) HandleRequest(req *imecore.Request) *imecore.Response {
	ime.mu.Lock()
	defer ime.mu.Unlock()

	resp := imecore.NewResponse(req.SeqNum, true)
	if ime.syncAppearancePrefs() {
		ime.sharedInputStateNeedsApply = ime.inputStateShared
		resp.CustomizeUI = ime.customizeUIMap()
	}
	if ime.syncAutoPairRules() {
		resp.CustomizeUI = ime.customizeUIMap()
	}
	ime.consumeAIAsyncResult(resp)
	ime.consumeBackendNotification(resp)
	traceLogf("RIME 输入法处理请求 client=%s seq=%d method=%s", ime.Client.ID, req.SeqNum, req.Method)

	switch req.Method {
	case "onActivate":
		return ime.onActivate(req, resp)
	case "onDeactivate":
		return ime.onDeactivate(req, resp)
	case "filterKeyDown":
		return ime.filterKeyDown(req, resp)
	case "onKeyDown":
		return ime.onKeyDown(req, resp)
	case "filterKeyUp":
		return ime.filterKeyUp(req, resp)
	case "onKeyUp":
		return ime.onKeyUp(req, resp)
	case "onCompositionTerminated":
		return ime.onCompositionTerminated(req, resp)
	case "onCommand":
		return ime.onCommand(req, resp)
	case "onMenu":
		return ime.onMenu(req, resp)
	case "highlightCandidate":
		return ime.highlightCandidate(req, resp)
	case "selectCandidate":
		return ime.selectCandidate(req, resp)
	case "changePage":
		return ime.changePage(req, resp)
	default:
		resp.ReturnValue = 0
		return resp
	}
}

func (ime *IME) onActivate(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	log.Println("RIME 输入法已激活")
	ime.createSession(resp)
	ime.addButtons(resp)
	ime.updateLangStatus(req, resp)
	resp.ReturnValue = 1
	return resp
}

func (ime *IME) onDeactivate(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	log.Println("RIME 输入法已失活")
	ime.destroySession(resp)
	ime.removeButtons(resp)
	resp.ReturnValue = 1
	return resp
}

func (ime *IME) filterKeyDown(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	if ime.handleAIKeyDownFilter(req, resp) {
		return resp
	}
	if ime.handleCustomPhraseKeyDownFilter(req, resp) {
		return resp
	}
	if ime.handleSuperAbbrevKeyDownFilter(req, resp) {
		return resp
	}
	if ime.handleSecondSelectionKeyDownFilter(req, resp) {
		return resp
	}
	if ime.lastKeyDownCode == req.KeyCode {
		ime.lastKeySkip++
		if ime.lastKeySkip >= 2 {
			ime.lastKeyDownCode = 0
			ime.lastKeySkip = 0
		}
	} else {
		ime.lastKeyDownCode = req.KeyCode
		ime.lastKeySkip = 0
		beforeASCII, beforeFullShape, hasInputState := ime.currentInputModeState()
		ime.lastKeyDownRet = ime.processKey(req, false)
		ime.updateLangStatusIfInputStateChanged(req, resp, beforeASCII, beforeFullShape, hasInputState)
	}
	ime.lastKeyUpCode = 0
	resp.ReturnValue = boolToInt(ime.lastKeyDownRet)
	return resp
}

func (ime *IME) filterKeyUp(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	if ime.handleAIKeyUpFilter(req, resp) {
		return resp
	}
	if ime.handleCustomPhraseKeyUpFilter(req, resp) {
		return resp
	}
	if ime.handleSuperAbbrevKeyUpFilter(req, resp) {
		return resp
	}
	if ime.handleSecondSelectionKeyUpFilter(req, resp) {
		return resp
	}
	if ime.lastKeyUpCode == req.KeyCode {
		ime.lastKeyUpCode = 0
	} else {
		ime.lastKeyUpCode = req.KeyCode
		beforeASCII, beforeFullShape, hasInputState := ime.currentInputModeState()
		ime.lastKeyUpRet = ime.processKey(req, true)
		ime.updateLangStatusIfInputStateChanged(req, resp, beforeASCII, beforeFullShape, hasInputState)
	}
	ime.lastKeyDownCode = 0
	ime.lastKeySkip = 0
	resp.ReturnValue = boolToInt(ime.lastKeyUpRet)
	return resp
}

func (ime *IME) currentInputModeState() (asciiMode bool, fullShape bool, ok bool) {
	if ime.backend == nil {
		return false, false, false
	}
	return ime.backend.GetOption("ascii_mode"), ime.backend.GetOption("full_shape"), true
}

func (ime *IME) updateLangStatusIfInputStateChanged(req *imecore.Request, resp *imecore.Response, beforeASCII, beforeFullShape bool, hasInputState bool) {
	if !hasInputState || ime.backend == nil {
		return
	}
	afterASCII := ime.backend.GetOption("ascii_mode")
	afterFullShape := ime.backend.GetOption("full_shape")
	if afterASCII == beforeASCII && afterFullShape == beforeFullShape {
		return
	}
	ime.updateLangStatus(req, resp)
}

func (ime *IME) onKeyDown(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	if ime.handleAIKeyDown(req, resp) {
		return resp
	}
	if ime.handleCustomPhraseKeyDown(req, resp) {
		return resp
	}
	if ime.handleSuperAbbrevKeyDown(req, resp) {
		return resp
	}
	if ime.handleSecondSelectionKeyDown(req, resp) {
		return resp
	}
	if ime.shouldPassThroughModifierOnKey(req, ime.lastKeyDownRet) {
		resp.ReturnValue = 0
		return resp
	}
	resp.ReturnValue = boolToInt(ime.onKey(req, resp))
	return resp
}

func (ime *IME) onKeyUp(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	if ime.handleAIKeyUp(req, resp) {
		return resp
	}
	if ime.handleCustomPhraseKeyUp(req, resp) {
		return resp
	}
	if ime.handleSuperAbbrevKeyUp(req, resp) {
		return resp
	}
	if ime.handleSecondSelectionKeyUp(req, resp) {
		return resp
	}
	if ime.shouldPassThroughModifierOnKey(req, ime.lastKeyUpRet) {
		resp.ReturnValue = 0
		return resp
	}
	resp.ReturnValue = boolToInt(ime.onKey(req, resp))
	return resp
}

func (ime *IME) highlightCandidate(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	resp.ReturnValue = boolToInt(ime.applyCandidateHighlight(req, resp))
	return resp
}

func (ime *IME) selectCandidate(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	resp.ReturnValue = boolToInt(ime.applyCandidateSelection(req, resp))
	return resp
}

func (ime *IME) changePage(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	resp.ReturnValue = boolToInt(ime.applyCandidatePageChange(req, resp))
	return resp
}

func (ime *IME) onCompositionTerminated(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	ime.resetAIState()
	ime.resetCustomPhraseOverlay()
	ime.resetSuperAbbrevOverlay()
	ime.resetSecondSelectionShortcut()
	ime.resetTrackedRawInput()
	if req.Forced {
		ime.destroySession(resp)
	} else {
		ime.clearResponse(resp)
	}
	resp.ReturnValue = 1
	return resp
}

func isSecondSelectionShortcut(req *imecore.Request) bool {
	if req == nil {
		return false
	}
	if req.KeyStates.IsKeyDown(vkShift) || req.KeyStates.IsKeyDown(vkControl) || req.KeyStates.IsKeyDown(vkMenu) {
		return false
	}
	if req.KeyCode == vkOem1 {
		return true
	}
	return req.CharCode == int(secondSelectChar)
}

func isSemicolonDebugEvent(req *imecore.Request) bool {
	if req == nil {
		return false
	}
	return req.KeyCode == vkOem1 || req.CharCode == int(secondSelectChar) || req.CharCode == int('；')
}

func summarizeCandidateTexts(items []candidateItem, limit int) []string {
	if len(items) == 0 || limit <= 0 {
		return nil
	}
	if len(items) > limit {
		items = items[:limit]
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.Text)
	}
	return result
}

func (ime *IME) selectionKeyIndex(req *imecore.Request) (int, bool) {
	if req == nil {
		return 0, false
	}
	if req.KeyCode >= '1' && req.KeyCode <= '9' {
		return req.KeyCode - '1', true
	}
	if req.CharCode >= '1' && req.CharCode <= '9' {
		return req.CharCode - '1', true
	}
	if ime.semicolonSelectSecond && isSecondSelectionShortcut(req) {
		return 1, true
	}
	return 0, false
}

func selectionShortcutConsumeCode(req *imecore.Request) int {
	if req == nil {
		return 0
	}
	if isSecondSelectionShortcut(req) {
		return int(secondSelectChar)
	}
	if req.CharCode >= '1' && req.CharCode <= '9' {
		return req.CharCode
	}
	if req.KeyCode >= '1' && req.KeyCode <= '9' {
		return req.KeyCode
	}
	if req.KeyCode != 0 {
		return req.KeyCode
	}
	return req.CharCode
}

func requestCharCode(req *imecore.Request) int {
	if req == nil {
		return 0
	}
	if req.CharCode != 0 {
		return req.CharCode
	}
	if req.KeyCode >= 'A' && req.KeyCode <= 'Z' {
		return req.KeyCode + 32
	}
	return 0
}

func (ime *IME) resetTrackedRawInput() {
	ime.rawInputTracked = ""
}

func (ime *IME) trimTrackedRawInput() {
	if ime.rawInputTracked == "" {
		return
	}
	runes := []rune(ime.rawInputTracked)
	ime.rawInputTracked = string(runes[:len(runes)-1])
}

func (ime *IME) updateTrackedRawInput(req *imecore.Request, backendRet bool) {
	if req == nil || !backendRet {
		return
	}
	keyCode := req.KeyCode
	charCode := requestCharCode(req)

	switch keyCode {
	case vkBack:
		ime.trimTrackedRawInput()
		return
	case vkEscape, vkReturn, vkSpace:
		ime.resetTrackedRawInput()
		return
	}

	if _, ok := ime.selectionKeyIndex(req); ok {
		ime.resetTrackedRawInput()
		return
	}

	if charCode >= 'a' && charCode <= 'z' {
		ime.rawInputTracked += string(rune(charCode))
		return
	}
	if charCode == '\'' && ime.rawInputTracked != "" && !strings.HasSuffix(ime.rawInputTracked, "'") {
		ime.rawInputTracked += "'"
		return
	}
	if ime.rawInputTracked != "" && charCode >= 0x20 && charCode != '\'' {
		ime.resetTrackedRawInput()
	}
}

func (ime *IME) resetSecondSelectionShortcut() {
	ime.secondSelectConsumeKeyUpCode = 0
}

func (ime *IME) handleSecondSelectionKeyDownFilter(req *imecore.Request, resp *imecore.Response) bool {
	shortcut := isSecondSelectionShortcut(req)
	if !ime.semicolonSelectSecond || !shortcut {
		if isSemicolonDebugEvent(req) {
			debugLogf("semicolon filter backend ignored enabled=%t shortcut=%t key=%d char=%d shift=%t ctrl=%t alt=%t",
				ime.semicolonSelectSecond,
				shortcut,
				req.KeyCode,
				req.CharCode,
				req.KeyStates.IsKeyDown(vkShift),
				req.KeyStates.IsKeyDown(vkControl),
				req.KeyStates.IsKeyDown(vkMenu),
			)
		}
		return false
	}
	state, ok := ime.currentVisibleBackendState()
	if !ok || strings.TrimSpace(state.Composition) == "" || len(state.Candidates) < 2 {
		debugLogf("semicolon filter backend unavailable ok=%t composition=%q candidates=%v",
			ok,
			state.Composition,
			summarizeCandidateTexts(state.Candidates, 6),
		)
		return false
	}
	ime.secondSelectConsumeKeyUpCode = selectionShortcutConsumeCode(req)
	debugLogf("semicolon filter backend handled consume=%d composition=%q candidates=%v",
		ime.secondSelectConsumeKeyUpCode,
		state.Composition,
		summarizeCandidateTexts(state.Candidates, 6),
	)
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleSecondSelectionKeyUpFilter(req *imecore.Request, resp *imecore.Response) bool {
	if ime.secondSelectConsumeKeyUpCode == 0 || selectionShortcutConsumeCode(req) != ime.secondSelectConsumeKeyUpCode {
		return false
	}
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleSecondSelectionKeyDown(req *imecore.Request, resp *imecore.Response) bool {
	if ime.secondSelectConsumeKeyUpCode == 0 || selectionShortcutConsumeCode(req) != ime.secondSelectConsumeKeyUpCode {
		return false
	}
	state, ok := ime.currentVisibleBackendState()
	if !ok || strings.TrimSpace(state.Composition) == "" || len(state.Candidates) < 2 {
		debugLogf("semicolon onKeyDown backend fallback ok=%t composition=%q candidates=%v",
			ok,
			state.Composition,
			summarizeCandidateTexts(state.Candidates, 6),
		)
		ime.fillResponseFromCurrentState(resp)
		resp.ReturnValue = 1
		return true
	}
	debugLogf("semicolon onKeyDown backend selecting visibleIndex=1 text=%q composition=%q candidates=%v",
		state.Candidates[1].Text,
		state.Composition,
		summarizeCandidateTexts(state.Candidates, 6),
	)
	if ime.commitBackendOverlayCandidate(resp, 1) {
		debugLogf("semicolon onKeyDown backend committed commit=%q", resp.CommitString)
		resp.ReturnValue = 1
		return true
	}
	debugLogf("semicolon onKeyDown backend select failed composition=%q candidates=%v",
		state.Composition,
		summarizeCandidateTexts(state.Candidates, 6),
	)
	ime.fillResponseFromCurrentState(resp)
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleSecondSelectionKeyUp(req *imecore.Request, resp *imecore.Response) bool {
	if ime.secondSelectConsumeKeyUpCode == 0 || selectionShortcutConsumeCode(req) != ime.secondSelectConsumeKeyUpCode {
		return false
	}
	ime.resetSecondSelectionShortcut()
	ime.fillResponseFromCurrentState(resp)
	resp.ReturnValue = 1
	return true
}

func (ime *IME) onCommand(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	commandID := req.ID.IntValue()
	if commandID == 0 && req.Data != nil {
		if raw, ok := req.Data["commandId"].(float64); ok {
			commandID = int(raw)
		}
	}
	if commandID == 0 {
		resp.ReturnValue = 0
		return resp
	}

	ime.createSession(resp)

	switch commandID {
	case ID_ASCII_MODE, ID_MODE_ICON:
		ime.toggleOption("ascii_mode")
	case ID_FULL_SHAPE:
		ime.toggleOption("full_shape")
	case ID_ASCII_PUNCT:
		ime.toggleOption("ascii_punct")
	case ID_TRADITIONALIZATION:
		ime.toggleOption("traditionalization")
	case ID_DEPLOY:
		if !ime.redeploy(req, resp) {
			if resp.TrayNotification == nil {
				resp.TrayNotification = deployTrayNotification(false)
			}
			resp.ReturnValue = 0
			return resp
		}
	case ID_SYNC:
		if ime.backend == nil || !ime.backend.SyncUserData() {
			resp.ReturnValue = 0
			return resp
		}
	case ID_UPDATE_CONFIG:
		if !ime.updateConfigAsync(resp) {
			resp.ReturnValue = 0
			return resp
		}
	case ID_OPEN_CUSTOM_PHRASE:
		if !ime.openCustomPhraseFile(resp) {
			resp.ReturnValue = 0
			return resp
		}
	case ID_OPEN_SUPER_ABBREV:
		if !ime.openSuperAbbrevFile(resp) {
			resp.ReturnValue = 0
			return resp
		}
	case ID_OPEN_AUTO_PAIR_SYMBOLS:
		if !ime.openAutoPairSymbolsFile(resp) {
			resp.ReturnValue = 0
			return resp
		}
	case ID_USER_DIR:
		ime.openPath(ime.userDir())
	case ID_SHARED_DIR:
		ime.openPath(ime.sharedDir())
	case ID_SYNC_DIR:
		ime.openPath(filepath.Join(ime.userDir(), "sync"))
	case ID_LOG_DIR:
		logDir := rimeLogDir()
		if logDir != "" {
			if err := os.MkdirAll(logDir, 0o755); err != nil {
				log.Printf("创建 RIME 日志目录失败 %s: %v", logDir, err)
			}
		}
		ime.openPath(logDir)
	default:
		previousCandidateCount := ime.candidateCount()
		if commandID == ID_SHARED_INPUT_STATE {
			ime.toggleInputStateShared()
			resp.ReturnValue = 1
			ime.updateLangStatus(req, resp)
			return resp
		}
		if commandID == ID_INPUT_AUTO_PAIR_QUOTES {
			ime.autoPairQuotes = !ime.autoPairQuotes
			ime.saveAppearancePrefsWithReason("onCommand:toggle_auto_pair_quotes")
			resp.CustomizeUI = ime.customizeUIMap()
			ime.fillResponseFromCurrentState(resp)
			ime.updateLangStatus(req, resp)
			resp.ReturnValue = 1
			return resp
		}
		if commandID == ID_INPUT_SEMICOLON_SELECT_SECOND {
			ime.semicolonSelectSecond = !ime.semicolonSelectSecond
			ime.saveAppearancePrefsWithReason("onCommand:toggle_semicolon_select_second")
			resp.CustomizeUI = ime.customizeUIMap()
			ime.fillResponseFromCurrentState(resp)
			ime.updateLangStatus(req, resp)
			resp.ReturnValue = 1
			return resp
		}
		if ime.handleSwitchCommand(commandID) {
			ime.resetAIState()
			resp.ReturnValue = 1
			ime.updateLangStatus(req, resp)
			return resp
		}
		if ime.handleSchemeSetCommand(commandID, req, resp) {
			ime.resetAIState()
			resp.ReturnValue = 1
			ime.updateLangStatus(req, resp)
			return resp
		}
		if ime.handleSchemaCommand(commandID) {
			ime.resetAIState()
			resp.ReturnValue = 1
			ime.updateLangStatus(req, resp)
			return resp
		}
		if ime.applyAppearanceCommand(commandID) {
			if isCandidateCountCommand(commandID) && ime.candidateCount() != previousCandidateCount {
				if !ime.applyCandidateCountConfig(resp) {
					resp.ReturnValue = 0
					return resp
				}
			}
			resp.CustomizeUI = ime.customizeUIMap()
			ime.fillResponseFromCurrentState(resp)
			ime.updateLangStatus(req, resp)
			resp.ReturnValue = 1
			return resp
		}
		if ime.isKnownDynamicCommand(commandID) {
			resp.ReturnValue = 0
			return resp
		}
		log.Printf("未知命令: %d", commandID)
		resp.ReturnValue = 0
		return resp
	}

	ime.updateLangStatus(req, resp)
	resp.ReturnValue = 1
	return resp
}

func (ime *IME) onMenu(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	buttonID := req.ID.StringValue()
	if buttonID == "" && req.Data != nil {
		if raw, ok := req.Data["buttonId"].(string); ok {
			buttonID = raw
		} else if raw, ok := req.Data["id"].(string); ok {
			buttonID = raw
		}
	}
	if buttonID != "settings" && buttonID != "windows-mode-icon" {
		resp.ReturnData = []map[string]interface{}{}
		resp.ReturnValue = 0
		return resp
	}

	resp.ReturnData = ime.buildMenu()
	resp.ReturnValue = 1
	return resp
}

func (ime *IME) Init(req *imecore.Request) bool {
	initStart := time.Now()
	firstRun := false
	backendAvailable := false
	defer func() {
		log.Printf("RIME 输入法初始化完成 elapsed=%s firstRun=%t backendAvailable=%t", time.Since(initStart), firstRun, backendAvailable)
	}()

	log.Println("RIME 输入法初始化")
	exePath, err := os.Executable()
	if err != nil {
		log.Printf("获取可执行文件路径失败，原生 RIME 不可用: %v", err)
		return true
	}

	exeDir := filepath.Dir(exePath)
	ime.iconDir = filepath.Join(exeDir, "input_methods", "rime", "icons")
	// After installation this resolves to e.g. C:\Program Files (x86)\Moqi\moqi-ime\input_methods\rime\data.
	sharedDir := filepath.Join(exeDir, "input_methods", "rime", "data")

	appData := os.Getenv("APPDATA")
	if appData == "" {
		log.Println("未找到 APPDATA，原生 RIME 不可用")
		return true
	}
	userDir := ime.userDir()
	info, statErr := os.Stat(userDir)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			if err := os.MkdirAll(userDir, 0o700); err != nil {
				log.Printf("创建用户 RIME 数据目录失败，原生 RIME 不可用: %v", err)
				return true
			}
			firstRun = true
		} else {
			log.Printf("检查用户 RIME 数据目录失败，原生 RIME 不可用: %v", statErr)
			return true
		}
	} else if !info.IsDir() {
		log.Printf("用户 RIME 数据目录不是目录，原生 RIME 不可用: %s", userDir)
		return true
	}

	real := newNativeBackend()
	if real != nil && real.Initialize(sharedDir, userDir, firstRun) {
		ime.backend = real
		backendAvailable = true
	} else {
		ime.backend = nil
	}
	return true
}

func (ime *IME) Close() {
	ime.destroySession(nil)
	log.Println("RIME 输入法关闭")
}

func (ime *IME) BackendAvailable() bool {
	return ime.backend != nil
}

func (ime *IME) processKey(req *imecore.Request, isUp bool) bool {
	ime.createSession(nil)
	if ime.backend == nil {
		ime.logShortcutTrace(req, isUp, 0, 0, false, false)
		return false
	}
	if !isUp {
		ime.keyComposing = ime.isComposing()
	}
	shouldFallbackArrowNavigation := !isUp && req != nil &&
		(req.KeyCode == vkUp || req.KeyCode == vkDown)
	var beforeState rimeState
	if shouldFallbackArrowNavigation {
		beforeState = ime.backend.State()
	}
	translatedKeyCode := translateKeyCode(req)
	modifiers := translateModifiers(req, isUp)
	backendRet := ime.backend.ProcessKey(req, translatedKeyCode, modifiers)
	if shouldFallbackArrowNavigation && modifiers == 0 &&
		ime.applyArrowNavigationFallback(req.KeyCode, beforeState) {
		backendRet = true
	}
	if !isUp {
		ime.updateTrackedRawInput(req, backendRet)
	}
	if ime.shouldSyncSharedInputStateAfterProcessKey(req, isUp) {
		ime.syncSharedInputStateFromBackendIfChanged()
	}
	handled := backendRet
	if backendRet {
		ime.logShortcutTrace(req, isUp, translatedKeyCode, modifiers, backendRet, true)
		return true
	}
	if ime.keyComposing && req.KeyCode == vkReturn {
		handled = true
		ime.logShortcutTrace(req, isUp, translatedKeyCode, modifiers, backendRet, handled)
		return true
	}
	if (req.KeyCode == vkShift || req.KeyCode == vkCapital) &&
		(modifiers == 0 || modifiers == releaseMask) {
		handled = true
		ime.logShortcutTrace(req, isUp, translatedKeyCode, modifiers, backendRet, handled)
		return true
	}
	ime.logShortcutTrace(req, isUp, translatedKeyCode, modifiers, backendRet, handled)
	return false
}

func (ime *IME) applyArrowNavigationFallback(keyCode int, beforeState rimeState) bool {
	if ime.backend == nil || len(beforeState.Candidates) == 0 {
		return false
	}
	afterState := ime.backend.State()
	if len(afterState.Candidates) == 0 {
		return false
	}
	if afterState.CandidateCursor != beforeState.CandidateCursor {
		return false
	}
	target := beforeState.CandidateCursor
	if target < 0 {
		target = 0
	}
	switch keyCode {
	case vkUp:
		if target > 0 {
			target--
		}
	case vkDown:
		if target < len(afterState.Candidates)-1 {
			target++
		}
	default:
		return false
	}
	if target == afterState.CandidateCursor {
		return false
	}
	return ime.backend.HighlightCandidate(target)
}

func (ime *IME) shouldSyncSharedInputStateAfterProcessKey(req *imecore.Request, isUp bool) bool {
	if ime.backend == nil || !ime.inputStateShared || req == nil {
		return false
	}
	switch req.KeyCode {
	case vkShift:
		return isUp
	case vkCapital:
		return !isUp
	default:
		return false
	}
}

func (ime *IME) handleAIKeyDownFilter(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil {
		return false
	}
	if ime.aiActive {
		if ime.isAIHandledKey(req) {
			ime.aiConsumeKeyUpCode = selectionShortcutConsumeCode(req)
			if isSemicolonDebugEvent(req) {
				debugLogf("semicolon filter ai handled consume=%d ai=%v",
					ime.aiConsumeKeyUpCode,
					ime.aiCandidates,
				)
			}
			resp.ReturnValue = 1
			return true
		}
		ime.resetAIState()
	}
	action := ime.matchAIAction(req)
	if action == nil {
		return false
	}
	ime.aiConsumeKeyUpCode = req.KeyCode
	ime.triggerAIReview(action)
	ime.aiTriggerPending = true
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleAIKeyUpFilter(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil {
		return false
	}
	if ime.aiConsumeKeyUpCode != 0 && selectionShortcutConsumeCode(req) == ime.aiConsumeKeyUpCode {
		if ime.aiActive {
			ime.fillAIResponse(resp)
		} else {
			ime.fillResponseFromCurrentState(resp)
		}
		resp.ReturnValue = 1
		return true
	}
	if ime.aiActive && ime.isAIHandledKey(req) {
		ime.fillAIResponse(resp)
		resp.ReturnValue = 1
		return true
	}
	return false
}

func (ime *IME) handleAIKeyDown(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil {
		return false
	}
	if ime.aiTriggerPending {
		ime.aiTriggerPending = false
		if ime.aiActive {
			ime.fillAIResponse(resp)
			resp.ReturnValue = 1
			return true
		}
		resp.ReturnValue = boolToInt(ime.onKey(req, resp))
		return true
	}
	if !ime.aiActive {
		return false
	}
	totalCandidates, aiCandidates := ime.visibleAIOverlayCounts()
	if index, ok := ime.selectionKeyIndex(req); ok {
		if index >= 0 && index < aiCandidates {
			if isSemicolonDebugEvent(req) {
				debugLogf("semicolon onKeyDown ai selecting aiIndex=%d ai=%v", index, ime.aiCandidates)
			}
			ime.aiCandidateCursor = index
			ime.commitAICandidate(resp)
			resp.ReturnValue = 1
			return true
		}
		if index >= aiCandidates && index < totalCandidates {
			if isSemicolonDebugEvent(req) {
				state, _ := ime.currentVisibleBackendState()
				debugLogf("semicolon onKeyDown ai selecting backendIndex=%d ai=%v backend=%v",
					index-aiCandidates,
					ime.aiCandidates,
					summarizeCandidateTexts(state.Candidates, 6),
				)
			}
			if ime.commitBackendOverlayCandidate(resp, index-aiCandidates) {
				resp.ReturnValue = 1
				return true
			}
		}
		ime.fillAIResponse(resp)
		resp.ReturnValue = 1
		return true
	}
	switch req.KeyCode {
	case vkUp:
		if ime.aiCandidateCursor > 0 {
			ime.aiCandidateCursor--
		}
		ime.fillAIResponse(resp)
		resp.ReturnValue = 1
		return true
	case vkDown:
		if ime.aiCandidateCursor < totalCandidates-1 {
			ime.aiCandidateCursor++
		}
		ime.fillAIResponse(resp)
		resp.ReturnValue = 1
		return true
	case vkReturn, vkSpace:
		if ime.aiCandidateCursor < aiCandidates {
			ime.commitAICandidate(resp)
			resp.ReturnValue = 1
			return true
		}
		if ime.commitBackendOverlayCandidate(resp, ime.aiCandidateCursor-aiCandidates) {
			resp.ReturnValue = 1
			return true
		}
		ime.fillAIResponse(resp)
		resp.ReturnValue = 1
		return true
	case vkEscape:
		ime.resetAIState()
		resp.ReturnValue = boolToInt(ime.onKey(req, resp))
		return true
	default:
		ime.resetAIState()
		return false
	}
}

func (ime *IME) handleAIKeyUp(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil {
		return false
	}
	if ime.aiConsumeKeyUpCode != 0 && selectionShortcutConsumeCode(req) == ime.aiConsumeKeyUpCode {
		ime.aiConsumeKeyUpCode = 0
		if ime.aiActive {
			ime.fillAIResponse(resp)
		} else {
			ime.fillResponseFromCurrentState(resp)
		}
		resp.ReturnValue = 1
		return true
	}
	if !ime.aiActive || !ime.isAIHandledKey(req) {
		return false
	}
	ime.fillAIResponse(resp)
	resp.ReturnValue = 1
	return true
}

func (ime *IME) isAIHotkey(req *imecore.Request) bool {
	return ime.matchAIAction(req) != nil
}

func (ime *IME) matchAIAction(req *imecore.Request) *aiAction {
	if !ime.aiEnabled || ime.aiReviewGenerator == nil || req == nil {
		return nil
	}
	for i := range ime.aiActions {
		if ime.aiActions[i].matches(req) {
			return &ime.aiActions[i]
		}
	}
	return nil
}

func (ime *IME) isAIHandledKey(req *imecore.Request) bool {
	if _, ok := ime.selectionKeyIndex(req); ok {
		return true
	}
	if req == nil {
		return false
	}
	return req.KeyCode == vkUp || req.KeyCode == vkDown || req.KeyCode == vkReturn || req.KeyCode == vkSpace || req.KeyCode == vkEscape
}

func (ime *IME) triggerAIReview(action *aiAction) bool {
	if ime.backend == nil || ime.aiReviewGenerator == nil || action == nil {
		return false
	}
	state := ime.backend.State()
	composition := strings.TrimSpace(state.Composition)
	inputCandidates := collectAICandidateTexts(state.Candidates, 3)
	if composition == "" && len(inputCandidates) == 0 {
		return false
	}

	ime.aiRequestSeq++
	requestSeq := ime.aiRequestSeq
	ime.aiPending = true
	ime.aiError = ""
	ime.aiActive = false
	ime.aiPrompt = composition
	ime.aiCandidates = nil
	ime.aiCandidateCursor = 0
	ime.aiCurrentAction = nil

	request := aiGenerateRequest{
		PreviousCommit: ime.aiPreviousCommit,
		Composition:    composition,
		Candidates:     inputCandidates,
		Prompt:         action.Prompt,
	}
	actionCopy := *action
	resultCh := ime.ensureAIResultCh()
	sender := ime.asyncResponseSender
	go func() {
		generatedCandidates, err := ime.aiReviewGenerator(request)
		result := aiAsyncResult{
			RequestSeq: requestSeq,
			Prompt:     composition,
			Action:     actionCopy,
			Err:        err,
		}
		if err == nil {
			result.Candidates = normalizeAICandidates(generatedCandidates)
			if len(result.Candidates) == 0 {
				result.Err = fmt.Errorf("empty AI result")
			}
		}
		if sender != nil {
			var updateResp *imecore.Response
			ime.mu.Lock()
			if ime.applyAIAsyncResult(result) {
				updateResp = imecore.NewResponse(0, true)
				ime.fillAIResponse(updateResp)
				if !updateResp.ShowCandidates && len(updateResp.CandidateList) == 0 && updateResp.CompositionString == "" {
					updateResp = nil
				}
			}
			ime.mu.Unlock()
			if updateResp != nil {
				sender(updateResp)
			}
			return
		}
		resultCh <- result
	}()
	return true
}

func (ime *IME) applyAIAsyncResult(result aiAsyncResult) bool {
	if result.RequestSeq != ime.aiRequestSeq {
		return false
	}
	ime.aiPending = false
	if result.Err != nil {
		ime.aiError = result.Err.Error()
		log.Printf("AI 写好评失败: %v", result.Err)
		ime.resetAIState()
		return false
	}
	ime.aiError = ""
	ime.aiPrompt = result.Prompt
	ime.aiCandidates = append([]string(nil), result.Candidates...)
	ime.aiCandidateCursor = 0
	ime.aiCurrentAction = &aiAction{
		Name:    result.Action.Name,
		Hotkey:  result.Action.Hotkey,
		Prompt:  result.Action.Prompt,
		KeyCode: result.Action.KeyCode,
		Ctrl:    result.Action.Ctrl,
		Alt:     result.Action.Alt,
		Shift:   result.Action.Shift,
	}
	ime.aiActive = len(ime.aiCandidates) > 0
	if ime.backend != nil && ime.backendReady() {
		state := ime.backend.State()
		if strings.TrimSpace(state.Composition) != strings.TrimSpace(result.Prompt) {
			ime.resetAIState()
			return false
		}
	}
	return ime.aiActive
}

func (ime *IME) consumeAIAsyncResult(resp *imecore.Response) {
	resultCh := ime.ensureAIResultCh()
	for {
		select {
		case result := <-resultCh:
			ime.applyAIAsyncResult(result)
		default:
			return
		}
	}
}

func collectAICandidateTexts(items []candidateItem, limit int) []string {
	if limit <= 0 {
		return nil
	}
	candidates := make([]string, 0, limit)
	for _, item := range items {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		candidates = append(candidates, text)
		if len(candidates) == limit {
			break
		}
	}
	return candidates
}

func (ime *IME) currentVisibleBackendState() (rimeState, bool) {
	if ime.backend == nil || !ime.backendReady() {
		return rimeState{}, false
	}
	state := ime.backend.State()
	visibleCandidateCount := ime.candidateCount()
	if visibleCandidateCount > 0 && len(state.Candidates) > visibleCandidateCount {
		state.Candidates = append([]candidateItem(nil), state.Candidates[:visibleCandidateCount]...)
	}
	if state.SelectKeys != "" && visibleCandidateCount > 0 && len(state.SelectKeys) > visibleCandidateCount {
		state.SelectKeys = state.SelectKeys[:visibleCandidateCount]
	}
	return state, true
}

func (ime *IME) visibleAIOverlayCounts() (totalCandidates int, aiCandidates int) {
	state, ok := ime.currentVisibleBackendState()
	if !ok {
		return 0, 0
	}
	visibleLimit := ime.candidateCount()
	if len(ime.aiCandidates) > 0 {
		aiCandidates = 1
	}
	if visibleLimit > 0 && aiCandidates > visibleLimit {
		aiCandidates = visibleLimit
	}
	totalCandidates = aiCandidates + len(state.Candidates)
	if visibleLimit > 0 && totalCandidates > visibleLimit {
		totalCandidates = visibleLimit
	}
	return totalCandidates, aiCandidates
}

func (ime *IME) fillAIResponse(resp *imecore.Response) {
	if resp == nil {
		return
	}
	state, ok := ime.currentVisibleBackendState()
	if !ok {
		ime.clearResponse(resp)
		ime.keyComposing = false
		return
	}
	if state.Composition == "" {
		ime.resetAIState()
		ime.clearResponse(resp)
		ime.keyComposing = false
		return
	}
	if strings.TrimSpace(ime.aiPrompt) != "" && strings.TrimSpace(state.Composition) != strings.TrimSpace(ime.aiPrompt) {
		ime.resetAIState()
		ime.fillResponseFromBackendState(resp, false)
		return
	}
	cursor := state.CursorPos
	resp.CompositionString = state.Composition
	resp.CursorPos = cursor
	resp.CompositionCursor = cursor
	resp.SelStart = state.SelStart
	resp.SelEnd = state.SelEnd

	combined := make([]string, 0, 1+len(state.Candidates))
	if len(ime.aiCandidates) > 0 {
		combined = append(combined, ime.aiCandidates[0])
	}
	combined = append(combined, ime.formatCandidates(state.Candidates)...)
	visibleCandidateCount := ime.candidateCount()
	if visibleCandidateCount > 0 && len(combined) > visibleCandidateCount {
		combined = combined[:visibleCandidateCount]
	}
	resp.CandidateList = combined
	if len(combined) == 0 {
		resp.CandidateCursor = 0
		resp.HasCandidateCursor = false
		resp.ShowCandidates = false
	} else {
		if ime.aiCandidateCursor < 0 {
			ime.aiCandidateCursor = 0
		}
		if ime.aiCandidateCursor >= len(combined) {
			ime.aiCandidateCursor = len(combined) - 1
		}
		resp.CandidateCursor = ime.aiCandidateCursor
		resp.HasCandidateCursor = true
		resp.ShowCandidates = true
		if len(combined) <= len(aiSelectKeys) {
			selKeys := aiSelectKeys[:len(combined)]
			if selKeys != ime.selectKeys {
				resp.SetSelKeys = selKeys
				ime.selectKeys = selKeys
			}
		}
	}
	ime.keyComposing = true
}

func (ime *IME) commitAICandidate(resp *imecore.Response) {
	if resp == nil || len(ime.aiCandidates) == 0 {
		return
	}
	if ime.aiCandidateCursor < 0 || ime.aiCandidateCursor >= len(ime.aiCandidates) {
		ime.aiCandidateCursor = 0
	}
	resp.CommitString = ime.aiCandidates[ime.aiCandidateCursor]
	ime.rememberAICommit(resp.CommitString)
	ime.clearResponse(resp)
	ime.resetAIState()
	ime.resetTrackedRawInput()
	if ime.backend != nil {
		ime.backend.ClearComposition()
	}
}

func (ime *IME) commitBackendOverlayCandidate(resp *imecore.Response, backendIndex int) bool {
	if resp == nil || backendIndex < 0 || backendIndex >= 9 {
		return false
	}
	ime.resetAIState()
	ime.resetCustomPhraseOverlay()
	if ime.backend == nil || !ime.backendReady() {
		return false
	}
	if !ime.backend.SelectCandidate(backendIndex) {
		debugLogf("backend overlay select failed backendIndex=%d", backendIndex)
		return false
	}
	debugLogf("backend overlay select succeeded backendIndex=%d", backendIndex)
	resp.ReturnValue = boolToInt(ime.onKey(&imecore.Request{}, resp))
	return true
}

func (ime *IME) highlightBackendCandidate(resp *imecore.Response, backendIndex int) bool {
	if resp == nil || backendIndex < 0 || backendIndex >= 9 {
		return false
	}
	if ime.backend == nil || !ime.backendReady() {
		return false
	}
	if !ime.backend.HighlightCandidate(backendIndex) {
		return false
	}
	ime.fillResponseFromCurrentState(resp)
	return true
}

func (ime *IME) selectBackendCandidate(resp *imecore.Response, backendIndex int) bool {
	if resp == nil || backendIndex < 0 || backendIndex >= 9 {
		return false
	}
	ime.resetAIState()
	ime.resetCustomPhraseOverlay()
	if ime.backend == nil || !ime.backendReady() {
		return false
	}
	if !ime.backend.SelectCandidate(backendIndex) {
		return false
	}
	resp.ReturnValue = boolToInt(ime.onKey(&imecore.Request{}, resp))
	return true
}

func (ime *IME) applyCandidateHighlight(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || resp == nil || !req.HasCandidateIndex || req.CandidateIndex < 0 {
		return false
	}
	index := req.CandidateIndex
	if ime.aiActive {
		totalCandidates, _ := ime.visibleAIOverlayCounts()
		if index >= totalCandidates {
			return false
		}
		ime.aiCandidateCursor = index
		ime.fillAIResponse(resp)
		return true
	}
	if _, customCandidates, backendIndexes, ok := ime.currentCustomPhraseOverlay(); ok {
		total := len(customCandidates) + len(backendIndexes)
		if index >= total {
			return false
		}
		ime.customPhraseCursor = index
		ime.fillResponseFromCurrentState(resp)
		return true
	}
	return ime.highlightBackendCandidate(resp, index)
}

func (ime *IME) applyCandidateSelection(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || resp == nil || !req.HasCandidateIndex || req.CandidateIndex < 0 {
		return false
	}
	index := req.CandidateIndex
	if ime.aiActive {
		totalCandidates, aiCandidates := ime.visibleAIOverlayCounts()
		if index >= totalCandidates {
			return false
		}
		ime.aiCandidateCursor = index
		if index < aiCandidates {
			ime.commitAICandidate(resp)
			return true
		}
		return ime.commitBackendOverlayCandidate(resp, index-aiCandidates)
	}
	if _, customCandidates, backendIndexes, ok := ime.currentCustomPhraseOverlay(); ok {
		total := len(customCandidates) + len(backendIndexes)
		if index >= total {
			return false
		}
		ime.customPhraseCursor = index
		if index < len(customCandidates) {
			ime.commitCustomPhraseCandidate(resp, customCandidates[index])
			return true
		}
		backendListIndex := index - len(customCandidates)
		if backendListIndex < 0 || backendListIndex >= len(backendIndexes) {
			return false
		}
		return ime.commitBackendOverlayCandidate(resp, backendIndexes[backendListIndex])
	}
	return ime.selectBackendCandidate(resp, index)
}

func (ime *IME) applyCandidatePageChange(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || resp == nil {
		return false
	}
	if ime.aiActive {
		ime.fillAIResponse(resp)
		return false
	}
	if _, _, _, ok := ime.currentCustomPhraseOverlay(); ok {
		ime.fillResponseFromCurrentState(resp)
		return false
	}
	if ime.backend == nil || !ime.backendReady() {
		return false
	}
	if !ime.backend.ChangePage(req.PageBackward) {
		return false
	}
	ime.fillResponseFromCurrentState(resp)
	return true
}

func (ime *IME) resetAIState() {
	ime.aiRequestSeq++
	ime.aiActive = false
	ime.aiPending = false
	ime.aiPrompt = ""
	ime.aiCandidates = nil
	ime.aiCandidateCursor = 0
	ime.aiTriggerPending = false
	ime.aiConsumeKeyUpCode = 0
	ime.aiCurrentAction = nil
}

func normalizeAICandidates(candidates []string) []string {
	normalized := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		value := strings.TrimSpace(candidate)
		value = strings.TrimLeft(value, "-*0123456789.、)） \t")
		value = strings.TrimSpace(strings.Trim(value, `"'`))
		if value == "" {
			continue
		}
		if utf8.RuneCountInString(value) > 120 {
			value = truncateRunes(value, 120)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
		if len(normalized) == aiCandidateLimit {
			break
		}
	}
	return normalized
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit])
}

func (ime *IME) logShortcutTrace(req *imecore.Request, isUp bool, translatedKeyCode, modifiers int, backendRet, handled bool) {
	if req == nil {
		return
	}
	if !isTraceLoggingEnabled() {
		return
	}
	if modifiers&controlMask == 0 && modifiers&altMask == 0 && req.KeyCode != vkControl && req.KeyCode != vkMenu {
		return
	}

	eventType := "down"
	if isUp {
		eventType = "up"
	}
	log.Printf(
		"RIME 快捷键追踪 event=%s keyCode=%d charCode=%d translatedKey=%d modifiers=%d ctrl=%t alt=%t backendRet=%t handled=%t composing=%t",
		eventType,
		req.KeyCode,
		req.CharCode,
		translatedKeyCode,
		modifiers,
		(modifiers&controlMask) != 0 || req.KeyCode == vkControl,
		(modifiers&altMask) != 0 || req.KeyCode == vkMenu,
		backendRet,
		handled,
		ime.keyComposing,
	)
}

func (ime *IME) shouldPassThroughModifierOnKey(req *imecore.Request, filterHandled bool) bool {
	if req == nil || filterHandled {
		return false
	}
	if req.KeyCode == vkControl || req.KeyCode == vkMenu {
		return true
	}
	if req.CharCode > 0 && req.CharCode < 0x20 {
		return true
	}
	return req.KeyStates.IsKeyDown(vkControl) || req.KeyStates.IsKeyDown(vkMenu)
}

func (ime *IME) onKey(req *imecore.Request, resp *imecore.Response) bool {
	if ime.aiActive {
		ime.fillAIResponse(resp)
		return true
	}
	if ime.backend == nil {
		ime.clearResponse(resp)
		ime.keyComposing = false
		return true
	}
	if !ime.backendReady() {
		ime.clearResponse(resp)
		ime.keyComposing = false
		return false
	}
	ime.updateLangStatus(req, resp)
	handled := ime.fillResponseFromBackendState(resp, true)
	ime.syncSharedInputStateFromBackendIfChanged()
	return handled
}

func (ime *IME) rememberAICommit(commit string) {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return
	}
	ime.aiPreviousCommit = commit
}

func (ime *IME) createSession(resp *imecore.Response) {
	if ime.backend == nil || !ime.backendReady() {
		return
	}
	hadSession := ime.backend.HasSession()
	if !ime.backend.EnsureSession() {
		return
	}
	if ime.inputStateShared && (!hadSession || ime.sharedInputStateNeedsApply) {
		ime.applySharedInputStateToBackend()
		ime.sharedInputStateNeedsApply = false
	}
	if ime.candidateCount() != 9 {
		_ = ime.backend.SetCandidatePageSize(ime.candidateCount())
	}
	if resp != nil {
		resp.CustomizeUI = ime.customizeUIMap()
	}
}

func (ime *IME) destroySession(resp *imecore.Response) {
	ime.resetAIState()
	ime.resetCustomPhraseOverlay()
	ime.resetSecondSelectionShortcut()
	ime.resetTrackedRawInput()
	ime.clearResponse(resp)
	if ime.backend != nil {
		ime.backend.ClearComposition()
		ime.backend.DestroySession()
	}
	ime.keyComposing = false
	ime.selectKeys = ""
}

func (ime *IME) applyCandidateCountConfig(resp *imecore.Response) bool {
	if ime.backend != nil && ime.backend.SetCandidatePageSize(ime.candidateCount()) {
		ime.keyComposing = false
		ime.selectKeys = ""
		return true
	}
	if !ime.writeCandidateCountConfig() {
		return false
	}
	maintenanceStarted := startMaintenanceFunc(false)
	if !deployConfigFileFunc("default.yaml", "config_version") {
		if maintenanceStarted {
			joinMaintenanceThreadFunc()
		}
		return false
	}
	if maintenanceStarted {
		joinMaintenanceThreadFunc()
	}
	ime.resetAIState()
	if ime.backend != nil {
		ime.backend.DestroySession()
	}
	ime.keyComposing = false
	ime.selectKeys = ""
	ime.createSession(resp)
	return true
}

func (ime *IME) redeploy(req *imecore.Request, resp *imecore.Response) bool {
	sharedDir := ime.sharedDir()
	userDir := ime.userDir()
	if sharedDir == "" || userDir == "" {
		log.Printf("重新部署失败，sharedDir=%q userDir=%q", sharedDir, userDir)
		return false
	}

	if ime.backend == nil {
		ime.backend = newNativeBackend()
	}
	if ime.backend == nil {
		log.Println("重新部署失败，原生 RIME 后端不可用")
		return false
	}
	if err := ime.reloadAIConfig(); err != nil {
		log.Printf("重新加载 AI 配置失败，将继续部署: %v", err)
	}
	ime.destroySession(resp)

	if native, ok := ime.backend.(*nativeBackend); ok {
		return native.Redeploy(sharedDir, userDir)
	}

	if !ime.backend.Redeploy(sharedDir, userDir) {
		log.Printf("重新部署失败，sharedDir=%q userDir=%q", sharedDir, userDir)
		return false
	}
	resp.TrayNotification = deployTrayNotification(true)
	ime.createSession(resp)
	return ime.onKey(req, resp)
}

func (ime *IME) backendReady() bool {
	if ime.backend == nil {
		return false
	}
	if native, ok := ime.backend.(*nativeBackend); ok {
		return native.Available()
	}
	return true
}

func (ime *IME) consumeBackendNotification(resp *imecore.Response) {
	if resp == nil {
		return
	}
	native, ok := ime.backend.(*nativeBackend)
	if !ok {
		return
	}
	if resp.TrayNotification == nil {
		resp.TrayNotification = native.ConsumeNotification()
	}
}

func deployTrayNotification(success bool) *imecore.TrayNotification {
	notification := &imecore.TrayNotification{
		Title: "Rime",
		Icon:  imecore.TrayNotificationIconInfo,
	}
	if success {
		notification.Message = "重新部署成功"
		return notification
	}
	notification.Message = "重新部署失败"
	notification.Icon = imecore.TrayNotificationIconError
	return notification
}

func schemeSetTrayNotification(message string, icon imecore.TrayNotificationIcon) *imecore.TrayNotification {
	return trayNotification(message, icon)
}

func (ime *IME) sendSchemeSetCompletionNotificationAsync(backend rimeBackend) {
	if ime.asyncResponseSender == nil {
		return
	}
	if native, ok := backend.(*nativeBackend); ok {
		go func() {
			ticker := time.NewTicker(50 * time.Millisecond)
			defer ticker.Stop()
			timeout := time.NewTimer(30 * time.Second)
			defer timeout.Stop()
			for {
				select {
				case <-ticker.C:
					if !native.Available() {
						continue
					}
					notification := native.ConsumeNotification()
					if notification != nil && notification.Icon == imecore.TrayNotificationIconError {
						ime.sendAsyncTrayNotification(schemeSetTrayNotification("方案集切换失败", imecore.TrayNotificationIconError))
						return
					}
					ime.sendAsyncTrayNotification(schemeSetTrayNotification("方案集切换成功", imecore.TrayNotificationIconInfo))
					return
				case <-timeout.C:
					return
				}
			}
		}()
		return
	}
	go func() {
		time.Sleep(10 * time.Millisecond)
		ime.sendAsyncTrayNotification(schemeSetTrayNotification("方案集切换成功", imecore.TrayNotificationIconInfo))
	}()
}

func (ime *IME) reloadAIConfig() error {
	cfg, err := loadAIConfig()
	if err != nil {
		return err
	}
	ime.aiReviewGenerator = newConfiguredAIReviewGenerator(cfg)
	ime.aiActions = defaultAIActions(cfg)
	ime.aiEnabled = ime.aiReviewGenerator != nil && len(ime.aiActions) > 0
	ime.resetAIState()
	log.Printf("AI 配置已重新加载 enabled=%t actions=%d", ime.aiEnabled, len(ime.aiActions))
	return nil
}

func (ime *IME) clearResponse(resp *imecore.Response) {
	if resp == nil {
		return
	}
	resp.CompositionString = ""
	resp.CursorPos = 0
	resp.CompositionCursor = 0
	resp.CandidateList = []string{}
	resp.CandidateEntries = []imecore.CandidateEntry{}
	resp.CandidateCursor = 0
	resp.HasCandidateCursor = false
	resp.ShowCandidates = false
}

func (ime *IME) fillResponseFromBackendState(resp *imecore.Response, allowCommit bool) bool {
	if resp == nil {
		return true
	}
	state, ok := ime.currentVisibleBackendState()
	if !ok {
		ime.resetCustomPhraseOverlay()
		ime.clearResponse(resp)
		ime.keyComposing = false
		return false
	}
	if allowCommit && state.CommitString != "" {
		resp.CommitString = state.CommitString
		ime.rememberAICommit(state.CommitString)
		ime.resetTrackedRawInput()
	}
	if state.Composition == "" {
		ime.resetCustomPhraseOverlay()
		ime.resetTrackedRawInput()
		ime.clearResponse(resp)
		ime.keyComposing = false
		return true
	}
	if state.SelectKeys != "" && state.SelectKeys != ime.selectKeys {
		resp.SetSelKeys = state.SelectKeys
		ime.selectKeys = state.SelectKeys
	}
	resp.CompositionString = state.Composition
	resp.CursorPos = state.CursorPos
	resp.CompositionCursor = state.CursorPos
	resp.SelStart = state.SelStart
	resp.SelEnd = state.SelEnd
	customPhraseCandidates := ime.visibleCustomPhraseCandidatesForState(state)
	if _, overlay, ok := ime.currentSuperAbbrevOverlay(); ok {
		if len(customPhraseCandidates) > 0 {
			customPhraseCandidates = applySuperAbbrevOverlayToCandidates(customPhraseCandidates, overlay)
		} else {
			state = applySuperAbbrevOverlay(state, overlay)
		}
	}
	if len(customPhraseCandidates) > 0 && len(state.Candidates) > 0 {
		state.Candidates = filterDuplicateCandidatesByText(state.Candidates, customPhraseCandidates)
	}
	remainingCandidateCount := ime.candidateCount() - len(customPhraseCandidates)
	if remainingCandidateCount < 0 {
		customPhraseCandidates = customPhraseCandidates[:ime.candidateCount()]
		remainingCandidateCount = 0
	}
	if len(state.Candidates) > remainingCandidateCount {
		state.Candidates = append([]candidateItem(nil), state.Candidates[:remainingCandidateCount]...)
	}
	if len(state.Candidates) > 0 || len(customPhraseCandidates) > 0 {
		resp.CandidateList = append(ime.formatCandidates(customPhraseCandidates), ime.formatCandidates(state.Candidates)...)
		resp.CandidateEntries = append(ime.candidateEntries(customPhraseCandidates), ime.candidateEntries(state.Candidates)...)
		if len(customPhraseCandidates) > 0 {
			if ime.customPhraseCursor < 0 {
				ime.customPhraseCursor = 0
			} else if ime.customPhraseCursor >= len(resp.CandidateList) {
				ime.customPhraseCursor = len(resp.CandidateList) - 1
			}
			resp.CandidateCursor = ime.customPhraseCursor
		} else if state.CandidateCursor < 0 {
			resp.CandidateCursor = 0
		} else if state.CandidateCursor >= len(state.Candidates) {
			resp.CandidateCursor = len(state.Candidates) - 1
		} else {
			resp.CandidateCursor = state.CandidateCursor
		}
		resp.HasCandidateCursor = true
		resp.ShowCandidates = true
		if len(customPhraseCandidates) > 0 && len(resp.CandidateList) <= len(aiSelectKeys) {
			selKeys := aiSelectKeys[:len(resp.CandidateList)]
			if selKeys != ime.selectKeys {
				resp.SetSelKeys = selKeys
				ime.selectKeys = selKeys
			}
		}
	} else {
		resp.CandidateList = []string{}
		resp.CandidateEntries = []imecore.CandidateEntry{}
		resp.CandidateCursor = 0
		resp.HasCandidateCursor = false
		resp.ShowCandidates = false
	}
	ime.keyComposing = true
	return true
}

func (ime *IME) fillResponseFromCurrentState(resp *imecore.Response) {
	if ime.aiActive {
		ime.fillAIResponse(resp)
		return
	}
	ime.fillResponseFromBackendState(resp, false)
}

func (ime *IME) isComposing() bool {
	if ime.backend == nil {
		return false
	}
	state := ime.backend.State()
	return state.Composition != "" || len(state.Candidates) > 0
}

func (ime *IME) toggleOption(name string) {
	if ime.backend == nil {
		return
	}
	ime.backend.SetOption(name, !ime.backend.GetOption(name))
	if ime.inputStateShared && ime.isSharedInputStateOption(name) {
		ime.captureSharedInputStateFromBackend()
		ime.saveAppearancePrefsWithReason(fmt.Sprintf("toggleOption:shared_option:%s", name))
	}
}

func schemaCommandID(index int) int {
	return ID_SCHEMA_BASE + index
}

func schemaCommandIndex(commandID int) (int, bool) {
	index := commandID - ID_SCHEMA_BASE
	if index < 0 {
		return 0, false
	}
	return index, true
}

func switchCommandID(index int) int {
	return ID_SWITCH_BASE + index
}

func switchCommandIndex(commandID int) (int, bool) {
	index := commandID - ID_SWITCH_BASE
	if index < 0 {
		return 0, false
	}
	return index, true
}

func schemeSetCommandID(index int) int {
	return ID_SCHEME_SET_BASE + index
}

func schemeSetCommandIndex(commandID int) (int, bool) {
	index := commandID - ID_SCHEME_SET_BASE
	if index < 0 {
		return 0, false
	}
	return index, true
}

func (ime *IME) menuSwitches() []RimeSwitch {
	if ime.backend == nil {
		return nil
	}
	savedOptions := ime.backend.SaveOptions()
	if len(savedOptions) == 0 {
		return nil
	}
	switches := ime.backend.SchemaSwitches()
	if len(switches) == 0 {
		return nil
	}
	switchByName := make(map[string]RimeSwitch, len(switches))
	for _, sw := range switches {
		name := strings.TrimSpace(sw.Name)
		if name == "" {
			continue
		}
		sw.Name = name
		switchByName[name] = sw
	}
	menuSwitches := make([]RimeSwitch, 0, len(savedOptions))
	seen := make(map[string]struct{}, len(savedOptions))
	for _, name := range savedOptions {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		sw, ok := switchByName[name]
		if !ok {
			continue
		}
		menuSwitches = append(menuSwitches, sw)
		seen[name] = struct{}{}
	}
	return menuSwitches
}

func (ime *IME) isKnownDynamicCommand(commandID int) bool {
	if index, ok := switchCommandIndex(commandID); ok {
		switches := ime.menuSwitches()
		if index >= 0 && index < len(switches) {
			return true
		}
	}
	if index, ok := schemeSetCommandIndex(commandID); ok {
		names := availableSchemeSets()
		if index >= 0 && index < len(names) {
			return true
		}
	}
	if ime.backend != nil {
		if index, ok := schemaCommandIndex(commandID); ok {
			schemas := ime.backend.SchemaList()
			if index >= 0 && index < len(schemas) {
				return true
			}
		}
	}
	return false
}

func switchMenuText(sw RimeSwitch, enabled bool) string {
	switch len(sw.States) {
	case 0:
		return sw.Name
	case 1:
		return sw.States[0]
	default:
		if enabled {
			return fmt.Sprintf("%s → %s", sw.States[1], sw.States[0])
		}
		return fmt.Sprintf("%s → %s", sw.States[0], sw.States[1])
	}
}

func (ime *IME) handleSwitchCommand(commandID int) bool {
	if ime.backend == nil {
		return false
	}
	index, ok := switchCommandIndex(commandID)
	if !ok {
		return false
	}
	switches := ime.menuSwitches()
	if index < 0 || index >= len(switches) {
		return false
	}
	name := strings.TrimSpace(switches[index].Name)
	if name == "" {
		return false
	}
	ime.toggleOption(name)
	return true
}

func (ime *IME) handleSchemeSetCommand(commandID int, req *imecore.Request, resp *imecore.Response) bool {
	index, ok := schemeSetCommandIndex(commandID)
	if !ok {
		return false
	}
	names := availableSchemeSets()
	if index < 0 || index >= len(names) {
		return false
	}
	target := names[index]
	current := currentSchemeSetName()
	if target == current {
		return true
	}

	root := moqiAppDataDir()
	if root == "" {
		return false
	}
	targetDir := filepath.Join(root, target)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		log.Printf("创建方案集目录失败 %s: %v", targetDir, err)
		return false
	}
	if !saveCurrentSchemeSetName(target) {
		return false
	}
	resp.TrayNotification = schemeSetTrayNotification("方案集切换中...", imecore.TrayNotificationIconInfo)
	if ime.redeploy(req, resp) {
		if _, ok := ime.backend.(*nativeBackend); ok {
			ime.sendSchemeSetCompletionNotificationAsync(ime.backend)
			return true
		}
		if ime.asyncResponseSender != nil {
			resp.TrayNotification = schemeSetTrayNotification("方案集切换中...", imecore.TrayNotificationIconInfo)
			ime.sendSchemeSetCompletionNotificationAsync(ime.backend)
			return true
		}
		resp.TrayNotification = schemeSetTrayNotification("方案集切换成功", imecore.TrayNotificationIconInfo)
		return true
	}
	_ = saveCurrentSchemeSetName(current)
	resp.TrayNotification = schemeSetTrayNotification("方案集切换失败", imecore.TrayNotificationIconError)
	return false
}

func (ime *IME) schemaMenuItems() []map[string]interface{} {
	if ime.backend == nil {
		return nil
	}
	schemas := ime.backend.SchemaList()
	if len(schemas) == 0 {
		return nil
	}
	currentSchemaID := ime.backend.CurrentSchemaID()
	items := make([]map[string]interface{}, 0, len(schemas))
	for i, schema := range schemas {
		text := strings.TrimSpace(schema.Name)
		if text == "" {
			text = schema.ID
		}
		if text == "" {
			continue
		}
		items = append(items, map[string]interface{}{
			"id":      schemaCommandID(i),
			"text":    text,
			"checked": schema.ID != "" && schema.ID == currentSchemaID,
		})
	}
	return items
}

func (ime *IME) handleSchemaCommand(commandID int) bool {
	if ime.backend == nil {
		return false
	}
	index, ok := schemaCommandIndex(commandID)
	if !ok {
		return false
	}
	schemas := ime.backend.SchemaList()
	if index < 0 || index >= len(schemas) {
		return false
	}
	schemaID := strings.TrimSpace(schemas[index].ID)
	if schemaID == "" {
		return false
	}
	if !ime.backend.SelectSchema(schemaID) {
		return false
	}
	if ime.inputStateShared {
		ime.applySharedInputStateToBackend()
		ime.syncSharedInputStateFromBackendIfChanged()
	}
	return true
}

func (ime *IME) updateLangStatus(req *imecore.Request, resp *imecore.Response) {
	if !ime.style.DisplayTrayIcon || ime.backend == nil {
		return
	}
	asciiMode := ime.backend.GetOption("ascii_mode")
	fullShape := ime.backend.GetOption("full_shape")
	capsOn := req != nil && req.KeyStates.IsKeyToggled(vkCapital)

	if ime.Client != nil && ime.Client.IsWindows8Above {
		if iconPath := ime.iconPath(modeIconName(asciiMode, fullShape, capsOn)); iconPath != "" {
			resp.ChangeButton = append(resp.ChangeButton, imecore.ButtonInfo{
				ID:   "windows-mode-icon",
				Icon: iconPath,
			})
		}
	}
	if iconPath := ime.iconPath(langIconName(asciiMode)); iconPath != "" {
		resp.ChangeButton = append(resp.ChangeButton, imecore.ButtonInfo{
			ID:   "switch-lang",
			Icon: iconPath,
		})
	}
	if iconPath := ime.iconPath(shapeIconName(fullShape)); iconPath != "" {
		resp.ChangeButton = append(resp.ChangeButton, imecore.ButtonInfo{
			ID:   "switch-shape",
			Icon: iconPath,
		})
	}
}

func (ime *IME) shareableOptionNames() []string {
	if ime.backend == nil {
		return nil
	}
	names := ime.backend.SaveOptions()
	if len(names) == 0 {
		return nil
	}
	unique := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		unique = append(unique, name)
	}
	return unique
}

func (ime *IME) isSharedInputStateOption(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, candidate := range ime.shareableOptionNames() {
		if candidate == name {
			return true
		}
	}
	return false
}

func (ime *IME) captureSharedInputStateFromBackend() {
	if ime.backend == nil {
		return
	}
	if ime.sharedOptions == nil {
		ime.sharedOptions = make(map[string]bool)
	}
	for _, name := range ime.shareableOptionNames() {
		ime.sharedOptions[name] = ime.backend.GetOption(name)
	}
}

func (ime *IME) applySharedInputStateToBackend() {
	if ime.backend == nil || !ime.inputStateShared {
		return
	}
	for name, value := range ime.sharedOptions {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		ime.backend.SetOption(name, value)
	}
}

func (ime *IME) syncSharedInputStateFromBackendIfChanged() {
	if ime.backend == nil || !ime.inputStateShared {
		return
	}
	if ime.sharedOptions == nil {
		ime.sharedOptions = make(map[string]bool)
	}
	changed := false
	for _, name := range ime.shareableOptionNames() {
		value := ime.backend.GetOption(name)
		if current, ok := ime.sharedOptions[name]; ok && current == value {
			continue
		}
		ime.sharedOptions[name] = value
		changed = true
	}
	if !changed {
		return
	}
	ime.saveAppearancePrefsWithReason("captureSharedInputStateFromBackend")
}

func (ime *IME) toggleInputStateShared() {
	ime.inputStateShared = !ime.inputStateShared
	if ime.inputStateShared {
		ime.captureSharedInputStateFromBackend()
	}
	ime.saveAppearancePrefsWithReason("toggleInputStateShared")
}

func (ime *IME) addButtons(resp *imecore.Response) {
	if !ime.style.DisplayTrayIcon || ime.backend == nil {
		return
	}
	asciiMode := ime.backend.GetOption("ascii_mode")
	fullShape := ime.backend.GetOption("full_shape")
	if ime.Client != nil && ime.Client.IsWindows8Above {
		if iconPath := ime.iconPath(modeIconName(asciiMode, fullShape, false)); iconPath != "" {
			resp.AddButton = append(resp.AddButton, imecore.ButtonInfo{
				ID:        "windows-mode-icon",
				Icon:      iconPath,
				Tooltip:   "中英文切换",
				CommandID: ID_MODE_ICON,
			})
		}
	}
	if iconPath := ime.iconPath(langIconName(asciiMode)); iconPath != "" {
		resp.AddButton = append(resp.AddButton, imecore.ButtonInfo{
			ID:        "switch-lang",
			Icon:      iconPath,
			Text:      "中英文切换",
			Tooltip:   "中英文切换",
			CommandID: ID_ASCII_MODE,
		})
	}
	if iconPath := ime.iconPath(shapeIconName(fullShape)); iconPath != "" {
		resp.AddButton = append(resp.AddButton, imecore.ButtonInfo{
			ID:        "switch-shape",
			Icon:      iconPath,
			Text:      "全半角切换",
			Tooltip:   "全角/半角切换",
			CommandID: ID_FULL_SHAPE,
		})
	}
	if iconPath := ime.iconPath("config.ico"); iconPath != "" {
		resp.AddButton = append(resp.AddButton, imecore.ButtonInfo{
			ID:   "settings",
			Icon: iconPath,
			Text: "设置",
			Type: "menu",
		})
	}
}

func (ime *IME) removeButtons(resp *imecore.Response) {
	if !ime.style.DisplayTrayIcon || resp == nil {
		return
	}
	resp.RemoveButton = append(resp.RemoveButton, "switch-lang", "switch-shape", "settings")
	if ime.Client != nil && ime.Client.IsWindows8Above {
		resp.RemoveButton = append(resp.RemoveButton, "windows-mode-icon")
	}
}

func (ime *IME) formatCandidates(candidates []candidateItem) []string {
	formatted := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		text := candidate.Text
		if candidate.Comment != "" {
			if strings.Contains(ime.style.CandidateFormat, "{0}") && strings.Contains(ime.style.CandidateFormat, "{1}") {
				text = strings.ReplaceAll(ime.style.CandidateFormat, "{0}", candidate.Text)
				text = strings.ReplaceAll(text, "{1}", candidate.Comment)
			} else {
				text = candidate.Text + " " + candidate.Comment
			}
		}
		formatted = append(formatted, text)
	}
	return formatted
}

func candidateTextKey(candidate candidateItem) string {
	return strings.TrimSpace(candidate.Text)
}

func filterDuplicateCandidatesByText(candidates []candidateItem, existing []candidateItem) []candidateItem {
	if len(candidates) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(existing))
	for _, candidate := range existing {
		key := candidateTextKey(candidate)
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	filtered := make([]candidateItem, 0, len(candidates))
	for _, candidate := range candidates {
		key := candidateTextKey(candidate)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
		}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func (ime *IME) candidateEntries(candidates []candidateItem) []imecore.CandidateEntry {
	entries := make([]imecore.CandidateEntry, 0, len(candidates))
	for _, candidate := range candidates {
		entries = append(entries, imecore.CandidateEntry{
			Text:    candidate.Text,
			Comment: candidate.Comment,
		})
	}
	return entries
}

func (ime *IME) iconPath(name string) string {
	if ime.iconDir == "" || name == "" {
		return ""
	}
	iconPath := filepath.Join(ime.iconDir, name)
	if _, err := os.Stat(iconPath); err != nil {
		return ""
	}
	return iconPath
}

func (ime *IME) buildMenu() []map[string]interface{} {
	menuSwitches := ime.menuSwitches()
	schemeSetItems := schemeSetMenuItems()
	schemaItems := ime.schemaMenuItems()
	items := make([]map[string]interface{}, 0, len(menuSwitches)+len(schemeSetItems)+10)
	for i, sw := range menuSwitches {
		enabled := ime.backend != nil && ime.backend.GetOption(sw.Name)
		items = append(items, map[string]interface{}{
			"id":      switchCommandID(i),
			"text":    switchMenuText(sw, enabled),
			"checked": enabled,
		})
	}
	if len(menuSwitches) > 0 {
		items = append(items, map[string]interface{}{"text": ""})
	}
	if len(schemeSetItems) > 0 {
		items = append(items, map[string]interface{}{
			"text":    "切换方案集",
			"submenu": schemeSetItems,
		})
	}
	if len(schemaItems) > 0 {
		items = append(items, map[string]interface{}{
			"text":    "输入方案(&I)",
			"submenu": schemaItems,
		})
	}
	if len(schemeSetItems) > 0 || len(schemaItems) > 0 {
		items = append(items,
			map[string]interface{}{"id": ID_OPEN_CUSTOM_PHRASE, "text": "打开置顶短语"},
			map[string]interface{}{"id": ID_OPEN_SUPER_ABBREV, "text": "打开超级简拼"},
			map[string]interface{}{"id": ID_UPDATE_CONFIG, "text": "更新配置(&P)"},
			map[string]interface{}{"id": ID_DEPLOY, "text": "刷新配置(&R)"},
			map[string]interface{}{"text": ""},
		)
	}
	items = append(items,
		map[string]interface{}{"id": ID_SHARED_INPUT_STATE, "text": "输入状态共享", "checked": ime.inputStateShared},
		map[string]interface{}{"text": "外观(&A)", "submenu": []map[string]interface{}{
			{"text": "切换主题", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_THEME_DEFAULT, "text": "默认主题", "checked": ime.style.CandidateTheme == "default"},
				{"id": ID_APPEARANCE_THEME_2, "text": "橘白", "checked": ime.style.CandidateTheme == "theme2"},
				{"id": ID_APPEARANCE_THEME_MOQI, "text": "墨奇", "checked": ime.style.CandidateTheme == "moqi"},
				{"id": ID_APPEARANCE_THEME_PURPLE, "text": "很有韵味", "checked": ime.style.CandidateTheme == "purple"},
				{"id": ID_APPEARANCE_THEME_WALLGRAY, "text": "墙灰", "checked": ime.style.CandidateTheme == "wallgray"},
				{"id": ID_APPEARANCE_THEME_ORANGE, "text": "橙狗", "checked": ime.style.CandidateTheme == "orange"},
				{"id": ID_APPEARANCE_THEME_REDPLUM, "text": "老红梅", "checked": ime.style.CandidateTheme == "redplum"},
				{"id": ID_APPEARANCE_THEME_SHACHENG, "text": "沙城老窖", "checked": ime.style.CandidateTheme == "shacheng"},
				{"id": ID_APPEARANCE_THEME_GLOBE, "text": "地球仪", "checked": ime.style.CandidateTheme == "globe"},
				{"id": ID_APPEARANCE_THEME_SOYMILK, "text": "豆浆杯", "checked": ime.style.CandidateTheme == "soymilk"},
				{"id": ID_APPEARANCE_THEME_CHRYSANTHEMUM, "text": "菊花茶", "checked": ime.style.CandidateTheme == "chrysanthemum"},
				{"id": ID_APPEARANCE_THEME_QINHUANGDAO, "text": "秦皇岛", "checked": ime.style.CandidateTheme == "qinhuangdao"},
				{"id": ID_APPEARANCE_THEME_BUBBLEGUM, "text": "歪比巴卜", "checked": ime.style.CandidateTheme == "bubblegum"},
				{"id": ID_APPEARANCE_THEME_PEPSI, "text": "百事可乐", "checked": ime.style.CandidateTheme == "pepsi"},
			}},
			{"id": ID_APPEARANCE_INLINE_PREEDIT, "text": "行内预编辑", "checked": ime.inlinePreeditEnabled()},
			{"text": "候选排列", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_LAYOUT_VERTICAL, "text": "竖排", "checked": !ime.isHorizontalCandidateLayout()},
				{"id": ID_APPEARANCE_LAYOUT_HORIZONTAL, "text": "横排", "checked": ime.isHorizontalCandidateLayout()},
			}},
			{"text": "每行候选数", "enabled": ime.isHorizontalCandidateLayout(), "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_PER_ROW_3, "text": "3", "checked": ime.effectiveCandidatePerRow() == 3, "enabled": ime.isHorizontalCandidateLayout()},
				{"id": ID_APPEARANCE_PER_ROW_5, "text": "5", "checked": ime.effectiveCandidatePerRow() == 5, "enabled": ime.isHorizontalCandidateLayout()},
				{"id": ID_APPEARANCE_PER_ROW_7, "text": "7", "checked": ime.effectiveCandidatePerRow() == 7, "enabled": ime.isHorizontalCandidateLayout()},
				{"id": ID_APPEARANCE_PER_ROW_9, "text": "9", "checked": ime.effectiveCandidatePerRow() == 9, "enabled": ime.isHorizontalCandidateLayout()},
			}},
			{"text": "候选间距", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_SPACING_0, "text": "0", "checked": ime.style.CandidateSpacing == 0},
				{"id": ID_APPEARANCE_SPACING_10, "text": "10", "checked": ime.style.CandidateSpacing == 10},
				{"id": ID_APPEARANCE_SPACING_20, "text": "20", "checked": ime.style.CandidateSpacing == 20},
				{"id": ID_APPEARANCE_SPACING_30, "text": "30", "checked": ime.style.CandidateSpacing == 30},
				{"id": ID_APPEARANCE_SPACING_40, "text": "40", "checked": ime.style.CandidateSpacing == 40},
				{"id": ID_APPEARANCE_SPACING_50, "text": "50", "checked": ime.style.CandidateSpacing == 50},
			}},
			{"text": "总候选数量", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_CAND_COUNT_3, "text": "3", "checked": ime.candidateCount() == 3},
				{"id": ID_APPEARANCE_CAND_COUNT_5, "text": "5", "checked": ime.candidateCount() == 5},
				{"id": ID_APPEARANCE_CAND_COUNT_7, "text": "7", "checked": ime.candidateCount() == 7},
				{"id": ID_APPEARANCE_CAND_COUNT_9, "text": "9", "checked": ime.candidateCount() == 9},
			}},
			{"text": "字体大小", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_FONT_14, "text": "14", "checked": ime.style.FontPoint == 14},
				{"id": ID_APPEARANCE_FONT_16, "text": "16", "checked": ime.style.FontPoint == 16},
				{"id": ID_APPEARANCE_FONT_18, "text": "18", "checked": ime.style.FontPoint == 18},
				{"id": ID_APPEARANCE_FONT_20, "text": "20", "checked": ime.style.FontPoint == 20},
				{"id": ID_APPEARANCE_FONT_22, "text": "22", "checked": ime.style.FontPoint == 22},
			}},
			{"text": "候选文字字体", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_FONT_FAMILY_SEGOE_UI, "text": "Segoe UI", "checked": strings.EqualFold(ime.style.FontFace, "Segoe UI")},
				{"id": ID_APPEARANCE_FONT_FAMILY_YAHEI_UI, "text": "微软雅黑 UI", "checked": strings.EqualFold(ime.style.FontFace, "Microsoft YaHei UI")},
				{"id": ID_APPEARANCE_FONT_FAMILY_DENGXIAN, "text": "等线", "checked": strings.EqualFold(ime.style.FontFace, "DengXian")},
				{"id": ID_APPEARANCE_FONT_FAMILY_SIMSUN, "text": "宋体", "checked": strings.EqualFold(ime.style.FontFace, "SimSun")},
			}},
			{"text": "注释文字大小", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_COMMENT_FONT_14, "text": "14", "checked": ime.style.CandidateCommentFontPoint == 14},
				{"id": ID_APPEARANCE_COMMENT_FONT_16, "text": "16", "checked": ime.style.CandidateCommentFontPoint == 16},
				{"id": ID_APPEARANCE_COMMENT_FONT_18, "text": "18", "checked": ime.style.CandidateCommentFontPoint == 18},
				{"id": ID_APPEARANCE_COMMENT_FONT_20, "text": "20", "checked": ime.style.CandidateCommentFontPoint == 20},
				{"id": ID_APPEARANCE_COMMENT_FONT_22, "text": "22", "checked": ime.style.CandidateCommentFontPoint == 22},
			}},
			{"text": "注释文字字体", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_COMMENT_FONT_FAMILY_CONSOLAS, "text": "Consolas", "checked": strings.EqualFold(ime.style.CandidateCommentFontFace, "Consolas")},
				{"id": ID_APPEARANCE_COMMENT_FONT_FAMILY_YAHEI_UI, "text": "微软雅黑 UI", "checked": strings.EqualFold(ime.style.CandidateCommentFontFace, "Microsoft YaHei UI")},
				{"id": ID_APPEARANCE_COMMENT_FONT_FAMILY_DENGXIAN, "text": "等线", "checked": strings.EqualFold(ime.style.CandidateCommentFontFace, "DengXian")},
				{"id": ID_APPEARANCE_COMMENT_FONT_FAMILY_SIMSUN, "text": "宋体", "checked": strings.EqualFold(ime.style.CandidateCommentFontFace, "SimSun")},
			}},
			{"text": "候选框背景", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_BG_WHITE, "text": "白色", "checked": strings.EqualFold(ime.style.CandidateBackgroundColor, "#ffffff")},
				{"id": ID_APPEARANCE_BG_WARM, "text": "暖白", "checked": strings.EqualFold(ime.style.CandidateBackgroundColor, "#fff7e8")},
				{"id": ID_APPEARANCE_BG_BLUE, "text": "浅蓝", "checked": strings.EqualFold(ime.style.CandidateBackgroundColor, "#f3f8ff")},
			}},
			{"text": "高亮颜色", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_HL_BLUE, "text": "浅蓝", "checked": strings.EqualFold(ime.style.CandidateHighlightColor, "#c6ddf9")},
				{"id": ID_APPEARANCE_HL_GRAY, "text": "浅灰", "checked": strings.EqualFold(ime.style.CandidateHighlightColor, "#e5e7eb")},
				{"id": ID_APPEARANCE_HL_GREEN, "text": "浅绿", "checked": strings.EqualFold(ime.style.CandidateHighlightColor, "#d9f2e6")},
			}},
			{"text": "字体颜色", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_TEXT_BLACK, "text": "黑色", "checked": strings.EqualFold(ime.style.CandidateTextColor, "#000000")},
				{"id": ID_APPEARANCE_TEXT_DARKGRAY, "text": "深灰", "checked": strings.EqualFold(ime.style.CandidateTextColor, "#333333")},
				{"id": ID_APPEARANCE_TEXT_BLUE, "text": "深蓝", "checked": strings.EqualFold(ime.style.CandidateTextColor, "#1d4ed8")},
			}},
			{"text": "高亮文字颜色", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_HLTEXT_BLACK, "text": "黑色", "checked": strings.EqualFold(ime.style.CandidateHighlightTextColor, "#000000")},
				{"id": ID_APPEARANCE_HLTEXT_WHITE, "text": "白色", "checked": strings.EqualFold(ime.style.CandidateHighlightTextColor, "#ffffff")},
				{"id": ID_APPEARANCE_HLTEXT_BLUE, "text": "深蓝", "checked": strings.EqualFold(ime.style.CandidateHighlightTextColor, "#1d4ed8")},
			}},
		}},
		map[string]interface{}{"text": "输入设置", "submenu": []map[string]interface{}{
			{"id": ID_INPUT_AUTO_PAIR_QUOTES, "text": "自动插入成对符号", "checked": ime.autoPairQuotes},
			{"id": ID_OPEN_AUTO_PAIR_SYMBOLS, "text": "打开成对符号设置"},
			{"id": ID_INPUT_SEMICOLON_SELECT_SECOND, "text": "分号键次选", "checked": ime.semicolonSelectSecond},
		}},
		map[string]interface{}{"text": "打开文件夹(&O)", "submenu": []map[string]interface{}{
			{"id": ID_USER_DIR, "text": "用户文件夹"},
			{"id": ID_SHARED_DIR, "text": "共享文件夹"},
			{"id": ID_SYNC_DIR, "text": "同步文件夹"},
			{"id": ID_LOG_DIR, "text": "日志文件夹"},
		}},
	)
	return items
}

func (ime *IME) AIHotkeyDescription() string {
	if len(ime.aiActions) == 0 {
		return fmt.Sprintf("Ctrl+Shift+%s", string(rune(aiHotkeyKeyCode)))
	}
	hotkeys := make([]string, 0, len(ime.aiActions))
	for _, action := range ime.aiActions {
		if action.Hotkey == "" {
			continue
		}
		hotkeys = append(hotkeys, action.Hotkey)
	}
	if len(hotkeys) == 0 {
		return fmt.Sprintf("Ctrl+Shift+%s", string(rune(aiHotkeyKeyCode)))
	}
	return strings.Join(hotkeys, " / ")
}

func (ime *IME) sharedDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exePath), "input_methods", "rime", "data")
}

func (ime *IME) userDir() string {
	root := moqiAppDataDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, currentSchemeSetName())
}

func rimeLogDir() string {
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		return ""
	}
	return filepath.Join(localAppData, "MoqiIM", "Log")
}

func (ime *IME) openPath(path string) {
	if path == "" {
		return
	}
	if err := exec.Command("explorer.exe", path).Start(); err != nil {
		log.Printf("打开路径失败 %s: %v", path, err)
	}
}

func (ime *IME) openURL(rawURL string) {
	if rawURL == "" {
		return
	}
	if err := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", rawURL).Start(); err != nil {
		log.Printf("打开链接失败 %s: %v", rawURL, err)
	}
}

func modeIconName(asciiMode, fullShape, capsOn bool) string {
	lang := "chi"
	if asciiMode {
		lang = "eng"
	}
	shape := "half"
	if fullShape {
		shape = "full"
	}
	caps := "off"
	if capsOn {
		caps = "on"
	}
	return lang + "_" + shape + "_caps" + caps + ".ico"
}

func langIconName(asciiMode bool) string {
	if asciiMode {
		return "eng.ico"
	}
	return "chi.ico"
}

func shapeIconName(fullShape bool) string {
	if fullShape {
		return "full.ico"
	}
	return "half.ico"
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
