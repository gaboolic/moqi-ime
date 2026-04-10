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
	ID_SCHEMA_BASE        = 1000

	ID_APPEARANCE_INLINE_PREEDIT      = 100
	ID_APPEARANCE_FONT_14             = 110
	ID_APPEARANCE_FONT_16             = 111
	ID_APPEARANCE_FONT_18             = 112
	ID_APPEARANCE_FONT_20             = 113
	ID_APPEARANCE_FONT_22             = 114
	ID_APPEARANCE_BG_WHITE            = 120
	ID_APPEARANCE_BG_WARM             = 121
	ID_APPEARANCE_BG_BLUE             = 122
	ID_APPEARANCE_HL_BLUE             = 130
	ID_APPEARANCE_HL_GRAY             = 131
	ID_APPEARANCE_HL_GREEN            = 132
	ID_APPEARANCE_TEXT_BLACK          = 140
	ID_APPEARANCE_TEXT_DARKGRAY       = 141
	ID_APPEARANCE_TEXT_BLUE           = 142
	ID_APPEARANCE_HLTEXT_BLACK        = 145
	ID_APPEARANCE_HLTEXT_WHITE        = 146
	ID_APPEARANCE_HLTEXT_BLUE         = 147
	ID_APPEARANCE_THEME_DEFAULT       = 150
	ID_APPEARANCE_THEME_2             = 151
	ID_APPEARANCE_THEME_MOQI          = 152
	ID_APPEARANCE_THEME_PURPLE        = 153
	ID_APPEARANCE_THEME_WALLGRAY      = 154
	ID_APPEARANCE_THEME_ORANGE        = 155
	ID_APPEARANCE_THEME_REDPLUM       = 156
	ID_APPEARANCE_THEME_SHACHENG      = 157
	ID_APPEARANCE_THEME_GLOBE         = 158
	ID_APPEARANCE_THEME_SOYMILK       = 159
	ID_APPEARANCE_THEME_CHRYSANTHEMUM = 160
	ID_APPEARANCE_THEME_QINHUANGDAO   = 161
	ID_APPEARANCE_THEME_BUBBLEGUM     = 162

	aiSelectKeys     = "123456789"
	aiHotkeyKeyCode  = 0x47 // G
	aiCandidateLimit = 3
)

type Style struct {
	DisplayTrayIcon             bool
	CandidateFormat             string
	CandidatePerRow             int
	CandidateUseCursor          bool
	CandidateTheme              string
	CandidateBackgroundColor    string
	CandidateHighlightColor     string
	CandidateTextColor          string
	CandidateHighlightTextColor string
	FontFace                    string
	FontPoint                   int
	InlinePreedit               string
	SoftCursor                  bool
}

type candidateItem struct {
	Text    string
	Comment string
}

type rimeState struct {
	CommitString    string
	Composition     string
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
	EnsureSession() bool
	DestroySession()
	ClearComposition()
	ProcessKey(req *imecore.Request, translatedKeyCode, modifiers int) bool
	State() rimeState
	SetOption(name string, value bool)
	GetOption(name string) bool
	SchemaList() []RimeSchema
	CurrentSchemaID() string
	SelectSchema(schemaID string) bool
}

type IME struct {
	*imecore.TextServiceBase
	iconDir            string
	style              Style
	selectKeys         string
	lastKeyDownCode    int
	lastKeySkip        int
	lastKeyDownRet     bool
	lastKeyUpCode      int
	lastKeyUpRet       bool
	keyComposing       bool
	backend            rimeBackend
	aiEnabled          bool
	aiActive           bool
	aiPending          bool
	aiPrompt           string
	aiCandidates       []string
	aiCandidateCursor  int
	aiError            string
	aiTriggerPending   bool
	aiConsumeKeyUpCode int
	aiPreviousCommit   string
	aiActions          []aiAction
	aiCurrentAction    *aiAction
	aiReviewGenerator  func(aiGenerateRequest) ([]string, error)
}

func New(client *imecore.Client) imecore.TextService {
	cfg, err := loadAIConfig()
	if err != nil {
		log.Printf("加载 AI 配置失败: %v", err)
	}
	generator := newConfiguredAIReviewGenerator(cfg)
	actions := defaultAIActions(cfg)
	ime := &IME{
		TextServiceBase: imecore.NewTextServiceBase(client),
		style: Style{
			DisplayTrayIcon:             true,
			CandidateFormat:             "{0} {1}",
			CandidatePerRow:             1,
			CandidateUseCursor:          true,
			CandidateTheme:              "default",
			CandidateBackgroundColor:    "#ffffff",
			CandidateHighlightColor:     "#c6ddf9",
			CandidateTextColor:          "#000000",
			CandidateHighlightTextColor: "#000000",
			FontFace:                    "Segoe UI",
			FontPoint:                   16,
			InlinePreedit:               "composition",
			SoftCursor:                  false,
		},
		aiEnabled:         generator != nil && len(actions) > 0,
		aiActions:         actions,
		aiReviewGenerator: generator,
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

func (ime *IME) HandleRequest(req *imecore.Request) *imecore.Response {
	resp := imecore.NewResponse(req.SeqNum, true)
	log.Printf("RIME 输入法处理请求 client=%s seq=%d method=%s", ime.Client.ID, req.SeqNum, req.Method)

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
	if ime.lastKeyDownCode == req.KeyCode {
		ime.lastKeySkip++
		if ime.lastKeySkip >= 2 {
			ime.lastKeyDownCode = 0
			ime.lastKeySkip = 0
		}
	} else {
		ime.lastKeyDownCode = req.KeyCode
		ime.lastKeySkip = 0
		ime.lastKeyDownRet = ime.processKey(req, false)
	}
	ime.lastKeyUpCode = 0
	resp.ReturnValue = boolToInt(ime.lastKeyDownRet)
	return resp
}

func (ime *IME) filterKeyUp(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	if ime.handleAIKeyUpFilter(req, resp) {
		return resp
	}
	if ime.lastKeyUpCode == req.KeyCode {
		ime.lastKeyUpCode = 0
	} else {
		ime.lastKeyUpCode = req.KeyCode
		ime.lastKeyUpRet = ime.processKey(req, true)
	}
	ime.lastKeyDownCode = 0
	ime.lastKeySkip = 0
	resp.ReturnValue = boolToInt(ime.lastKeyUpRet)
	return resp
}

func (ime *IME) onKeyDown(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	if ime.handleAIKeyDown(req, resp) {
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
	if ime.shouldPassThroughModifierOnKey(req, ime.lastKeyUpRet) {
		resp.ReturnValue = 0
		return resp
	}
	resp.ReturnValue = boolToInt(ime.onKey(req, resp))
	return resp
}

func (ime *IME) onCompositionTerminated(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	ime.resetAIState()
	if req.Forced {
		ime.destroySession(resp)
	} else if ime.backend != nil {
		ime.backend.ClearComposition()
		ime.clearResponse(resp)
	}
	resp.ReturnValue = 1
	return resp
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
			resp.ReturnValue = 0
			return resp
		}
	case ID_SYNC:
		if ime.backend == nil || !ime.backend.SyncUserData() {
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
		ime.openPath(filepath.Join(os.Getenv("LOCALAPPDATA"), APP, "Logs"))
	default:
		if ime.handleSchemaCommand(commandID) {
			ime.resetAIState()
			resp.ReturnValue = 1
			ime.updateLangStatus(req, resp)
			return resp
		}
		if ime.applyAppearanceCommand(commandID) {
			resp.CustomizeUI = ime.customizeUIMap()
			ime.updateLangStatus(req, resp)
			resp.ReturnValue = 1
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
	userDir := filepath.Join(appData, APP, "Rime")
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
	translatedKeyCode := translateKeyCode(req)
	modifiers := translateModifiers(req, isUp)
	backendRet := ime.backend.ProcessKey(req, translatedKeyCode, modifiers)
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

func (ime *IME) handleAIKeyDownFilter(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil {
		return false
	}
	if ime.aiActive {
		if ime.isAIHandledKey(req.KeyCode) {
			ime.aiConsumeKeyUpCode = req.KeyCode
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
	if ime.aiConsumeKeyUpCode != 0 && req.KeyCode == ime.aiConsumeKeyUpCode {
		if ime.aiActive {
			ime.fillAIResponse(resp)
		}
		resp.ReturnValue = 1
		return true
	}
	if ime.aiActive && ime.isAIHandledKey(req.KeyCode) {
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
	switch {
	case ime.isAISelectionKey(req.KeyCode):
		index := req.KeyCode - 0x31
		if index >= 0 && index < len(ime.aiCandidates) {
			ime.aiCandidateCursor = index
			ime.commitAICandidate(resp)
			resp.ReturnValue = 1
			return true
		}
		ime.fillAIResponse(resp)
		resp.ReturnValue = 1
		return true
	case req.KeyCode == vkUp:
		if ime.aiCandidateCursor > 0 {
			ime.aiCandidateCursor--
		}
		ime.fillAIResponse(resp)
		resp.ReturnValue = 1
		return true
	case req.KeyCode == vkDown:
		if ime.aiCandidateCursor < len(ime.aiCandidates)-1 {
			ime.aiCandidateCursor++
		}
		ime.fillAIResponse(resp)
		resp.ReturnValue = 1
		return true
	case req.KeyCode == vkReturn || req.KeyCode == vkSpace:
		ime.commitAICandidate(resp)
		resp.ReturnValue = 1
		return true
	case req.KeyCode == vkEscape:
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
	if ime.aiConsumeKeyUpCode != 0 && req.KeyCode == ime.aiConsumeKeyUpCode {
		ime.aiConsumeKeyUpCode = 0
		if ime.aiActive {
			ime.fillAIResponse(resp)
		}
		resp.ReturnValue = 1
		return true
	}
	if !ime.aiActive || !ime.isAIHandledKey(req.KeyCode) {
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

func (ime *IME) isAISelectionKey(keyCode int) bool {
	return keyCode >= 0x31 && keyCode <= 0x39
}

func (ime *IME) isAIHandledKey(keyCode int) bool {
	return ime.isAISelectionKey(keyCode) || keyCode == vkUp || keyCode == vkDown || keyCode == vkReturn || keyCode == vkSpace || keyCode == vkEscape
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

	ime.aiPending = true
	ime.aiError = ""
	generatedCandidates, err := ime.aiReviewGenerator(aiGenerateRequest{
		PreviousCommit: ime.aiPreviousCommit,
		Composition:    composition,
		Candidates:     inputCandidates,
		Prompt:         action.Prompt,
	})
	ime.aiPending = false
	if err != nil {
		ime.aiError = err.Error()
		log.Printf("AI 写好评失败: %v", err)
		ime.resetAIState()
		return false
	}

	normalized := normalizeAICandidates(generatedCandidates)
	if len(normalized) == 0 {
		ime.aiError = "empty AI result"
		log.Println("AI 写好评失败: 未返回候选")
		ime.resetAIState()
		return false
	}

	ime.aiPrompt = composition
	ime.aiCandidates = normalized
	ime.aiCandidateCursor = 0
	ime.aiCurrentAction = action
	ime.aiActive = true
	return true
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

func (ime *IME) fillAIResponse(resp *imecore.Response) {
	if resp == nil {
		return
	}
	cursor := utf8.RuneCountInString(ime.aiPrompt)
	resp.CompositionString = ime.aiPrompt
	resp.CursorPos = cursor
	resp.CompositionCursor = cursor
	resp.SelStart = 0
	resp.SelEnd = cursor
	resp.CandidateList = append([]string(nil), ime.aiCandidates...)
	resp.CandidateCursor = ime.aiCandidateCursor
	resp.ShowCandidates = len(ime.aiCandidates) > 0
	if aiSelectKeys != ime.selectKeys {
		resp.SetSelKeys = aiSelectKeys
		ime.selectKeys = aiSelectKeys
	}
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
	if ime.backend != nil {
		ime.backend.ClearComposition()
	}
}

func (ime *IME) resetAIState() {
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
	ime.updateLangStatus(req, resp)
	state := ime.backend.State()
	if state.CommitString != "" {
		resp.CommitString = state.CommitString
		ime.rememberAICommit(state.CommitString)
	}
	if state.Composition == "" {
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

	if len(state.Candidates) > 0 {
		resp.CandidateList = ime.formatCandidates(state.Candidates)
		resp.CandidateCursor = state.CandidateCursor
		resp.ShowCandidates = true
	} else {
		resp.ShowCandidates = false
	}
	ime.keyComposing = true
	return true
}

func (ime *IME) rememberAICommit(commit string) {
	commit = strings.TrimSpace(commit)
	if commit == "" {
		return
	}
	ime.aiPreviousCommit = commit
}

func (ime *IME) createSession(resp *imecore.Response) {
	if ime.backend == nil {
		return
	}
	if !ime.backend.EnsureSession() {
		return
	}
	if resp != nil {
		resp.CustomizeUI = ime.customizeUIMap()
	}
}

func (ime *IME) destroySession(resp *imecore.Response) {
	ime.resetAIState()
	ime.clearResponse(resp)
	if ime.backend != nil {
		ime.backend.ClearComposition()
		ime.backend.DestroySession()
	}
	ime.keyComposing = false
	ime.selectKeys = ""
}

func (ime *IME) redeploy(req *imecore.Request, resp *imecore.Response) bool {
	ime.destroySession(resp)

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
	if !ime.backend.Redeploy(sharedDir, userDir) {
		log.Printf("重新部署失败，sharedDir=%q userDir=%q", sharedDir, userDir)
		return false
	}

	ime.createSession(resp)
	return ime.onKey(req, resp)
}

func (ime *IME) clearResponse(resp *imecore.Response) {
	if resp == nil {
		return
	}
	resp.CompositionString = ""
	resp.CursorPos = 0
	resp.CompositionCursor = 0
	resp.CandidateList = []string{}
	resp.CandidateCursor = 0
	resp.ShowCandidates = false
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
	return ime.backend.SelectSchema(schemaID)
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
	asciiMode := ime.backend != nil && ime.backend.GetOption("ascii_mode")
	fullShape := ime.backend != nil && ime.backend.GetOption("full_shape")
	asciiPunct := ime.backend != nil && ime.backend.GetOption("ascii_punct")
	traditionalization := ime.backend != nil && ime.backend.GetOption("traditionalization")
	schemaItems := ime.schemaMenuItems()

	asciiText := "中文 → 英文"
	if asciiMode {
		asciiText = "英文 → 中文"
	}
	shapeText := "半角 → 全角"
	if fullShape {
		shapeText = "全角 → 半角"
	}
	punctText := "中文标点 → 英文标点"
	if asciiPunct {
		punctText = "英文标点 → 中文标点"
	}
	traditionalizationText := "简体 → 繁体"
	if traditionalization {
		traditionalizationText = "繁体 → 简体"
	}

	items := []map[string]interface{}{
		{"id": ID_ASCII_MODE, "text": asciiText},
		{"id": ID_TRADITIONALIZATION, "text": traditionalizationText},
		{"id": ID_ASCII_PUNCT, "text": punctText},
		{"id": ID_FULL_SHAPE, "text": shapeText},
		{"text": ""},
	}
	if len(schemaItems) > 0 {
		items = append(items, map[string]interface{}{
			"text":    "输入方案(&I)",
			"submenu": schemaItems,
		})
		items = append(items, map[string]interface{}{"text": ""})
	}
	items = append(items,
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
			}},
			{"id": ID_APPEARANCE_INLINE_PREEDIT, "text": "行内预编辑", "checked": ime.inlinePreeditEnabled()},
			{"text": "字体大小", "submenu": []map[string]interface{}{
				{"id": ID_APPEARANCE_FONT_14, "text": "14", "checked": ime.style.FontPoint == 14},
				{"id": ID_APPEARANCE_FONT_16, "text": "16", "checked": ime.style.FontPoint == 16},
				{"id": ID_APPEARANCE_FONT_18, "text": "18", "checked": ime.style.FontPoint == 18},
				{"id": ID_APPEARANCE_FONT_20, "text": "20", "checked": ime.style.FontPoint == 20},
				{"id": ID_APPEARANCE_FONT_22, "text": "22", "checked": ime.style.FontPoint == 22},
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
		map[string]interface{}{"id": ID_DEPLOY, "text": "重新部署(&D)"},
		map[string]interface{}{"id": ID_SYNC, "text": "同步(&S)"},
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
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, APP, "Rime")
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
