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

	aiSelectKeys     = "123456789"
	aiHotkeyKeyCode  = 0x47 // G
	aiCandidateLimit = 3
)

type Style struct {
	DisplayTrayIcon    bool
	CandidateFormat    string
	CandidatePerRow    int
	CandidateUseCursor bool
	FontFace           string
	FontPoint          int
	InlinePreedit      string
	SoftCursor         bool
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
	EnsureSession() bool
	DestroySession()
	ClearComposition()
	ProcessKey(req *imecore.Request, translatedKeyCode, modifiers int) bool
	State() rimeState
	SetOption(name string, value bool)
	GetOption(name string) bool
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
	return &IME{
		TextServiceBase: imecore.NewTextServiceBase(client),
		style: Style{
			DisplayTrayIcon:    true,
			CandidateFormat:    "{0} {1}",
			CandidatePerRow:    1,
			CandidateUseCursor: false,
			FontFace:           "MingLiu",
			FontPoint:          20,
			InlinePreedit:      "composition",
			SoftCursor:         false,
		},
		aiEnabled:         generator != nil && len(actions) > 0,
		aiActions:         actions,
		aiReviewGenerator: generator,
	}
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
		log.Println("重新部署尚未实现")
	case ID_SYNC:
		log.Println("同步用户数据尚未实现")
	case ID_USER_DIR:
		ime.openPath(ime.userDir())
	case ID_SHARED_DIR:
		ime.openPath(ime.sharedDir())
	case ID_SYNC_DIR:
		ime.openPath(filepath.Join(ime.userDir(), "sync"))
	case ID_LOG_DIR:
		ime.openPath(filepath.Join(os.Getenv("LOCALAPPDATA"), APP, "Logs"))
	default:
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
	if statErr != nil || !info.IsDir() {
		log.Println("未找到用户 RIME 数据目录，原生 RIME 不可用")
		return true
	}

	real := newNativeBackend()
	if real != nil && real.Initialize(sharedDir, userDir, false) {
		ime.backend = real
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
		resp.CustomizeUI = map[string]interface{}{
			"candFontName":  ime.style.FontFace,
			"candFontSize":  ime.style.FontPoint,
			"candPerRow":    ime.style.CandidatePerRow,
			"candUseCursor": ime.style.CandidateUseCursor,
		}
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
				Tooltip:   "中西文切换",
				CommandID: ID_MODE_ICON,
			})
		}
	}
	if iconPath := ime.iconPath(langIconName(asciiMode)); iconPath != "" {
		resp.AddButton = append(resp.AddButton, imecore.ButtonInfo{
			ID:        "switch-lang",
			Icon:      iconPath,
			Text:      "中西文切换",
			Tooltip:   "中西文切换",
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

	return []map[string]interface{}{
		{"id": ID_ASCII_MODE, "text": asciiText},
		{"id": ID_TRADITIONALIZATION, "text": traditionalizationText},
		{"id": ID_ASCII_PUNCT, "text": punctText},
		{"id": ID_FULL_SHAPE, "text": shapeText},
		{"text": ""},
		{"id": ID_DEPLOY, "text": "重新部署(&D)"},
		{"id": ID_SYNC, "text": "同步(&S)"},
		{"text": "打开文件夹(&O)", "submenu": []map[string]interface{}{
			{"id": ID_USER_DIR, "text": "用户文件夹"},
			{"id": ID_SHARED_DIR, "text": "共享文件夹"},
			{"id": ID_SYNC_DIR, "text": "同步文件夹"},
			{"id": ID_LOG_DIR, "text": "日志文件夹"},
		}},
	}
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
