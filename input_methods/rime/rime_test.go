package rime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

func writeTestAIConfig(t *testing.T, appData string, body string) {
	t.Helper()
	configPath := filepath.Join(appData, APP, "Rime", aiConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create AI config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write AI config: %v", err)
	}
}

type testDictEntry struct {
	code  string
	words []candidateItem
}

type testBackend struct {
	session           bool
	composition       string
	candidates        []candidateItem
	commitString      string
	asciiMode         bool
	fullShape         bool
	schemas           []RimeSchema
	currentSchemaID   string
	selectSchemaCalls []string
	pageSizeCalls     []int
	pageSizeOK        bool
	redeployCalls     int
	redeploySharedDir string
	redeployUserDir   string
	redeployOK        bool
	syncCalls         int
	syncOK            bool
}

func newTestBackend() *testBackend {
	return &testBackend{
		redeployOK: true,
		syncOK:     true,
		schemas: []RimeSchema{
			{ID: "rime_frost", Name: "白霜拼音"},
			{ID: "rime_frost_double_pinyin", Name: "自然码双拼"},
			{ID: "rime_frost_double_pinyin_flypy", Name: "小鹤双拼"},
		},
		currentSchemaID: "rime_frost",
		pageSizeOK:      false,
	}
}

func (b *testBackend) Initialize(sharedDir, userDir string, firstRun bool) bool {
	return true
}

func (b *testBackend) Redeploy(sharedDir, userDir string) bool {
	b.redeployCalls++
	b.redeploySharedDir = sharedDir
	b.redeployUserDir = userDir
	b.DestroySession()
	return b.redeployOK
}

func (b *testBackend) SyncUserData() bool {
	b.syncCalls++
	return b.syncOK
}

func (b *testBackend) EnsureSession() bool {
	b.session = true
	return true
}

func (b *testBackend) DestroySession() {
	b.session = false
	b.ClearComposition()
}

func (b *testBackend) ClearComposition() {
	b.composition = ""
	b.candidates = nil
	b.commitString = ""
}

func (b *testBackend) ProcessKey(req *imecore.Request, translatedKeyCode, modifiers int) bool {
	b.commitString = ""
	keyCode := req.KeyCode
	charCode := req.CharCode
	if charCode == 0 && keyCode >= 'A' && keyCode <= 'Z' {
		charCode = keyCode + 32
	}
	if b.asciiMode && b.composition == "" && charCode >= 0x20 {
		return false
	}
	if modifiers&releaseMask != 0 {
		return false
	}

	switch keyCode {
	case vkBack:
		if b.composition == "" {
			return false
		}
		b.composition = trimLastRuneForTest(b.composition)
		b.refreshCandidates()
		return true
	case vkEscape:
		if b.composition == "" {
			return false
		}
		b.ClearComposition()
		return true
	case vkReturn, vkSpace:
		if b.composition == "" {
			return false
		}
		b.commitString = b.currentCommit()
		b.composition = ""
		b.candidates = nil
		return true
	}

	if b.composition != "" && keyCode >= '1' && keyCode <= '9' {
		index := keyCode - '1'
		if index >= 0 && index < len(b.candidates) {
			b.commitString = b.candidates[index].Text
			b.composition = ""
			b.candidates = nil
			return true
		}
	}

	if charCode >= 'a' && charCode <= 'z' {
		b.composition += string(rune(charCode))
		b.refreshCandidates()
		return true
	}
	if charCode == '\'' && b.composition != "" && !strings.HasSuffix(b.composition, "'") {
		b.composition += "'"
		b.refreshCandidates()
		return true
	}
	if b.composition != "" && charCode >= 0x20 && charCode != '\'' {
		b.commitString = b.currentCommit() + string(rune(charCode))
		b.composition = ""
		b.candidates = nil
		return true
	}
	return false
}

func (b *testBackend) State() rimeState {
	state := rimeState{
		CommitString:    b.commitString,
		Composition:     b.composition,
		CursorPos:       len(b.composition),
		Candidates:      append([]candidateItem(nil), b.candidates...),
		CandidateCursor: 0,
		SelectKeys:      "1234567890",
		AsciiMode:       b.asciiMode,
		FullShape:       b.fullShape,
	}
	b.commitString = ""
	return state
}

func (b *testBackend) SetOption(name string, value bool) {
	switch name {
	case "ascii_mode":
		b.asciiMode = value
	case "full_shape":
		b.fullShape = value
	}
}

func (b *testBackend) GetOption(name string) bool {
	switch name {
	case "ascii_mode":
		return b.asciiMode
	case "full_shape":
		return b.fullShape
	default:
		return false
	}
}

func (b *testBackend) SchemaList() []RimeSchema {
	return append([]RimeSchema(nil), b.schemas...)
}

func (b *testBackend) CurrentSchemaID() string {
	return b.currentSchemaID
}

func (b *testBackend) SelectSchema(schemaID string) bool {
	for _, schema := range b.schemas {
		if schema.ID != schemaID {
			continue
		}
		b.currentSchemaID = schemaID
		b.selectSchemaCalls = append(b.selectSchemaCalls, schemaID)
		return true
	}
	return false
}

func (b *testBackend) SetCandidatePageSize(pageSize int) bool {
	b.pageSizeCalls = append(b.pageSizeCalls, pageSize)
	return b.pageSizeOK
}

func (b *testBackend) currentCommit() string {
	if len(b.candidates) > 0 {
		return b.candidates[0].Text
	}
	return strings.ReplaceAll(b.composition, "'", "")
}

func (b *testBackend) refreshCandidates() {
	code := strings.ReplaceAll(b.composition, "'", "")
	if code == "" {
		b.candidates = nil
		return
	}
	results := make([]candidateItem, 0, 9)
	seen := make(map[string]struct{})
	appendWords := func(words []candidateItem) {
		for _, word := range words {
			if _, ok := seen[word.Text]; ok {
				continue
			}
			seen[word.Text] = struct{}{}
			results = append(results, word)
			if len(results) == 9 {
				return
			}
		}
	}
	for _, entry := range testDictionary() {
		if entry.code == code {
			appendWords(entry.words)
		}
	}
	for _, entry := range testDictionary() {
		if len(results) == 9 {
			break
		}
		if entry.code != code && strings.HasPrefix(entry.code, code) {
			appendWords(entry.words)
		}
	}
	if len(results) == 0 {
		results = []candidateItem{{Text: code}}
	}
	b.candidates = results
}

func testDictionary() []testDictEntry {
	return []testDictEntry{
		{code: "ni", words: []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}, {Text: "尼"}, {Text: "拟"}}},
		{code: "nihao", words: []candidateItem{{Text: "你好"}, {Text: "你号"}, {Text: "拟好"}}},
		{code: "nimen", words: []candidateItem{{Text: "你们"}}},
		{code: "zhong", words: []candidateItem{{Text: "中"}, {Text: "种"}, {Text: "重"}}},
		{code: "zhongwen", words: []candidateItem{{Text: "中文"}}},
	}
}

func trimLastRuneForTest(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	return string(runes[:len(runes)-1])
}

func newTestIME() *IME {
	return &IME{
		TextServiceBase: imecore.NewTextServiceBase(&imecore.Client{ID: "test-client"}),
		style:           defaultStyle(),
		backend:         newTestBackend(),
	}
}

func newIsolatedTestIME(t *testing.T) *IME {
	t.Helper()
	resetSharedAppearanceConfigForTest()
	return newTestIME()
}

func TestNewInitialState(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	if !ime.style.DisplayTrayIcon {
		t.Fatal("expected tray icon style enabled by default")
	}
	if backend.composition != "" {
		t.Fatalf("expected empty composition, got %q", backend.composition)
	}
	if len(backend.candidates) != 0 {
		t.Fatalf("expected no candidates, got %v", backend.candidates)
	}
	if ime.style.CandidatePerRow != 1 {
		t.Fatalf("expected vertical layout by default, got CandidatePerRow=%d", ime.style.CandidatePerRow)
	}
	if ime.style.CandidateTheme != "default" || ime.style.FontPoint != 20 {
		t.Fatalf("expected default theme defaults, got theme=%q font=%d", ime.style.CandidateTheme, ime.style.FontPoint)
	}
	if ime.style.CandidateBackgroundColor != "#ffffff" || ime.style.CandidateHighlightColor != "#c6ddf9" {
		t.Fatalf("expected default theme colors, got bg=%q hl=%q",
			ime.style.CandidateBackgroundColor, ime.style.CandidateHighlightColor)
	}
	if ime.keyComposing {
		t.Fatal("expected keyComposing to be false initially")
	}
}

func TestFilterKeyDownProcessesKeyWithoutUpdatingUI(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   1,
		KeyCode:  0x4E,
		CharCode: 'n',
	}, imecore.NewResponse(1, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected n to be handled, got %d", resp.ReturnValue)
	}
	if resp.CompositionString != "" || len(resp.CandidateList) != 0 || resp.ShowCandidates {
		t.Fatalf("expected filterKeyDown not to emit UI state, got %#v", resp)
	}
}

func TestFilterKeyDownFallsBackToKeyCodeWhenCharCodeMissing(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  2,
		KeyCode: 0x4E,
	}, imecore.NewResponse(2, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected keyCode-only N to be handled, got %d", resp.ReturnValue)
	}
}

func TestOnKeyDownReflectsBackendStateAfterFilter(t *testing.T) {
	ime := newIsolatedTestIME(t)

	ime.filterKeyDown(&imecore.Request{
		SeqNum:   1,
		KeyCode:  0x4E,
		CharCode: 'n',
	}, imecore.NewResponse(1, true))
	ime.filterKeyDown(&imecore.Request{
		SeqNum:   2,
		KeyCode:  0x49,
		CharCode: 'i',
	}, imecore.NewResponse(2, true))

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   3,
		KeyCode:  0x49,
		CharCode: 'i',
	}, imecore.NewResponse(3, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected onKeyDown to succeed, got %d", resp.ReturnValue)
	}
	if resp.CompositionString != "ni" {
		t.Fatalf("expected composition ni, got %q", resp.CompositionString)
	}
	if len(resp.CandidateList) == 0 || resp.CandidateList[0] != "你" {
		t.Fatalf("expected first exact candidate 你, got %v", resp.CandidateList)
	}
}

func TestOnKeyDownNumberSelectsCandidate(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}}
	ime.keyComposing = true

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  4,
		KeyCode: 0x32,
	}, imecore.NewResponse(4, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected number selection to be handled, got %d", filterResp.ReturnValue)
	}

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:  5,
		KeyCode: 0x32,
	}, imecore.NewResponse(5, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected onKeyDown after selection to succeed, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "呢" {
		t.Fatalf("expected second candidate 呢, got %q", resp.CommitString)
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected state reset after candidate selection")
	}
}

func TestOnKeyDownBackspaceUpdatesComposition(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	ime.filterKeyDown(&imecore.Request{
		SeqNum:  5,
		KeyCode: 0x08,
	}, imecore.NewResponse(5, true))
	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:  6,
		KeyCode: 0x08,
	}, imecore.NewResponse(6, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected backspace to be handled, got %d", resp.ReturnValue)
	}
	if backend.composition != "n" {
		t.Fatalf("expected composition n after backspace, got %q", backend.composition)
	}
	if resp.CompositionString != "n" {
		t.Fatalf("expected response composition n, got %q", resp.CompositionString)
	}
	if len(resp.CandidateList) == 0 {
		t.Fatal("expected candidates to remain after backspace")
	}
}

func TestOnKeyDownEscapeClearsComposition(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	ime.filterKeyDown(&imecore.Request{
		SeqNum:  6,
		KeyCode: 0x1B,
	}, imecore.NewResponse(6, true))
	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:  7,
		KeyCode: 0x1B,
	}, imecore.NewResponse(7, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected escape to be handled, got %d", resp.ReturnValue)
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected composition state cleared")
	}
	if resp.CompositionString != "" || resp.ShowCandidates {
		t.Fatalf("expected cleared UI, got %#v", resp)
	}
}

func TestOnKeyDownSpaceCommitsFirstCandidate(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	ime.filterKeyDown(&imecore.Request{
		SeqNum:  7,
		KeyCode: 0x20,
	}, imecore.NewResponse(7, true))
	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:  8,
		KeyCode: 0x20,
	}, imecore.NewResponse(8, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected space to be handled, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "你" {
		t.Fatalf("expected first candidate 你, got %q", resp.CommitString)
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected state reset after commit")
	}
}

func TestOnKeyDownPunctuationCommitsComposition(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	ime.filterKeyDown(&imecore.Request{
		SeqNum:   8,
		KeyCode:  int('.'),
		CharCode: int('.'),
	}, imecore.NewResponse(8, true))
	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   9,
		KeyCode:  int('.'),
		CharCode: int('.'),
	}, imecore.NewResponse(9, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected punctuation to be handled while composing, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "你." {
		t.Fatalf("expected punctuation commit 你., got %q", resp.CommitString)
	}
}

func TestOnKeyDownUnhandledKeyReturnsZero(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   9,
		KeyCode:  0x70,
		CharCode: 0,
	}, imecore.NewResponse(9, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected unrelated key to be ignored, got %d", resp.ReturnValue)
	}
}

func TestOnKeyDownAsciiModePassesThroughWhenIdle(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.backend.SetOption("ascii_mode", true)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   10,
		KeyCode:  int('A'),
		CharCode: int('a'),
	}, imecore.NewResponse(10, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected ascii mode to pass through idle typing, got %d", resp.ReturnValue)
	}
}

func TestControlKeyPassesThroughWhenIdle(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  10,
		KeyCode: vkControl,
	}, imecore.NewResponse(10, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected bare ctrl to pass through, got %d", resp.ReturnValue)
	}
}

// Regression: if filterKeyDown does not handle a bare Ctrl key, onKeyDown must return
// unhandled as well; otherwise the host still thinks the IME consumed the modifier.
func TestOnKeyDownBareControlUnhandledWhenFilterDoesNotHandle(t *testing.T) {
	ime := newIsolatedTestIME(t)
	const seq = 20
	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  seq,
		KeyCode: vkControl,
	}, imecore.NewResponse(seq, true))
	if filterResp.ReturnValue != 0 {
		t.Fatalf("expected filterKeyDown bare Ctrl unhandled, got %d", filterResp.ReturnValue)
	}
	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:  seq + 1,
		KeyCode: vkControl,
	}, imecore.NewResponse(seq+1, true))
	if onResp.ReturnValue != 0 {
		t.Fatalf("expected onKeyDown bare Ctrl unhandled when filter did not handle, got %d", onResp.ReturnValue)
	}
}

func TestOnKeyDownControlShortcutUnhandledWhenFilterDoesNotHandle(t *testing.T) {
	ime := newIsolatedTestIME(t)
	const seq = 22
	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   seq,
		KeyCode:  int('A'),
		CharCode: 1,
	}, imecore.NewResponse(seq, true))
	if filterResp.ReturnValue != 0 {
		t.Fatalf("expected filterKeyDown ctrl+a unhandled, got %d", filterResp.ReturnValue)
	}
	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   seq + 1,
		KeyCode:  int('A'),
		CharCode: 1,
	}, imecore.NewResponse(seq+1, true))
	if onResp.ReturnValue != 0 {
		t.Fatalf("expected onKeyDown ctrl+a unhandled when filter did not handle, got %d", onResp.ReturnValue)
	}
}

// Regression: same contract as TestOnKeyDownBareControlUnhandledWhenFilterDoesNotHandle for key-up / Alt.
func TestOnKeyUpBareAltUnhandledWhenFilterDoesNotHandle(t *testing.T) {
	ime := newIsolatedTestIME(t)
	const seq = 21
	filterResp := ime.filterKeyUp(&imecore.Request{
		SeqNum:  seq,
		KeyCode: vkMenu,
	}, imecore.NewResponse(seq, true))
	if filterResp.ReturnValue != 0 {
		t.Fatalf("expected filterKeyUp bare Alt unhandled, got %d", filterResp.ReturnValue)
	}
	onResp := ime.onKeyUp(&imecore.Request{
		SeqNum:  seq + 1,
		KeyCode: vkMenu,
	}, imecore.NewResponse(seq+1, true))
	if onResp.ReturnValue != 0 {
		t.Fatalf("expected onKeyUp bare Alt unhandled when filter did not handle, got %d", onResp.ReturnValue)
	}
}

func TestOnCommandHandlesKnownAndMissingCommand(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	validResp := ime.onCommand(&imecore.Request{
		SeqNum: 11,
		ID:     imecore.FlexibleID{Int: ID_ASCII_MODE, IsInt: true},
	}, imecore.NewResponse(11, true))
	if validResp.ReturnValue != 1 {
		t.Fatalf("expected known command to be handled, got %d", validResp.ReturnValue)
	}
	if !ime.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii mode toggled on")
	}
	if backend.composition != "ni" {
		t.Fatalf("expected test composition preserved until backend handles key flow, got %q", backend.composition)
	}

	missingResp := ime.onCommand(&imecore.Request{
		SeqNum: 12,
	}, imecore.NewResponse(12, true))
	if missingResp.ReturnValue != 0 {
		t.Fatalf("expected missing commandId to be ignored, got %d", missingResp.ReturnValue)
	}
}

func TestOnCommandDeployRedeploysBackend(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 13,
		ID:     imecore.FlexibleID{Int: ID_DEPLOY, IsInt: true},
	}, imecore.NewResponse(13, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected deploy command to succeed, got %d", resp.ReturnValue)
	}
	if backend.redeployCalls != 1 {
		t.Fatalf("expected backend redeployed once, got %d", backend.redeployCalls)
	}
	if backend.redeploySharedDir == "" || backend.redeployUserDir == "" {
		t.Fatalf("expected redeploy paths to be populated, got shared=%q user=%q", backend.redeploySharedDir, backend.redeployUserDir)
	}
	if !backend.session {
		t.Fatal("expected session recreated after redeploy")
	}
	if resp.TrayNotification == nil {
		t.Fatal("expected deploy success tray notification")
	}
	if resp.TrayNotification.Title != "Rime" || resp.TrayNotification.Message != "重新部署成功" {
		t.Fatalf("unexpected deploy success notification: %#v", resp.TrayNotification)
	}
	if resp.TrayNotification.Icon != imecore.TrayNotificationIconInfo {
		t.Fatalf("expected info tray notification, got %q", resp.TrayNotification.Icon)
	}
}

func TestOnCommandDeployFailureReturnsErrorTrayNotification(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.redeployOK = false

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 21,
		ID:     imecore.FlexibleID{Int: ID_DEPLOY, IsInt: true},
	}, imecore.NewResponse(21, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected deploy command to fail, got %d", resp.ReturnValue)
	}
	if backend.redeployCalls != 1 {
		t.Fatalf("expected backend redeployed once, got %d", backend.redeployCalls)
	}
	if resp.TrayNotification == nil {
		t.Fatal("expected deploy failure tray notification")
	}
	if resp.TrayNotification.Title != "Rime" || resp.TrayNotification.Message != "重新部署失败" {
		t.Fatalf("unexpected deploy failure notification: %#v", resp.TrayNotification)
	}
	if resp.TrayNotification.Icon != imecore.TrayNotificationIconError {
		t.Fatalf("expected error tray notification, got %q", resp.TrayNotification.Icon)
	}
}

func TestOnCommandDeployReloadsAIConfig(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	writeTestAIConfig(t, appData, `{
  "api": {
    "base_url": "https://example.com/v1",
    "api_key": "test-key",
    "model": "test-model"
  },
  "actions": [
    {
      "name": "AI 改写",
      "hotkey": "Ctrl+Alt+K",
      "prompt": "请改写 {{composition}}"
    }
  ]
}`)

	ime := newIsolatedTestIME(t)
	ime.aiEnabled = false
	ime.aiActions = nil
	ime.aiReviewGenerator = nil

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 22,
		ID:     imecore.FlexibleID{Int: ID_DEPLOY, IsInt: true},
	}, imecore.NewResponse(22, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected deploy command to succeed, got %d", resp.ReturnValue)
	}
	if !ime.aiEnabled {
		t.Fatal("expected AI to be enabled after reloading ai_config.json")
	}
	if ime.aiReviewGenerator == nil {
		t.Fatal("expected AI review generator reloaded")
	}
	if len(ime.aiActions) != 1 {
		t.Fatalf("expected 1 AI action after reload, got %#v", ime.aiActions)
	}
	if ime.aiActions[0].Name != "AI 改写" || ime.aiActions[0].Hotkey != "Ctrl+Alt+K" {
		t.Fatalf("unexpected AI action after reload: %#v", ime.aiActions[0])
	}
}

func TestOnCommandSyncUserData(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 14,
		ID:     imecore.FlexibleID{Int: ID_SYNC, IsInt: true},
	}, imecore.NewResponse(14, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected sync command to succeed, got %d", resp.ReturnValue)
	}
	if backend.syncCalls != 1 {
		t.Fatalf("expected sync_user_data called once, got %d", backend.syncCalls)
	}
}

func TestOnCommandSelectSchema(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 15,
		ID:     imecore.FlexibleID{Int: schemaCommandID(1), IsInt: true},
	}, imecore.NewResponse(15, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected schema command to succeed, got %d", resp.ReturnValue)
	}
	if backend.currentSchemaID != "rime_frost_double_pinyin" {
		t.Fatalf("expected schema switched to natural double pinyin, got %q", backend.currentSchemaID)
	}
	if len(backend.selectSchemaCalls) != 1 || backend.selectSchemaCalls[0] != "rime_frost_double_pinyin" {
		t.Fatalf("expected select schema call recorded, got %#v", backend.selectSchemaCalls)
	}
}

func TestOnMenuReturnsSettingsMenu(t *testing.T) {
	ime := newIsolatedTestIME(t)

	resp := ime.onMenu(&imecore.Request{
		SeqNum: 16,
		ID:     imecore.FlexibleID{String: "settings"},
	}, imecore.NewResponse(16, true))

	items, ok := resp.ReturnData.([]map[string]interface{})
	if !ok || len(items) == 0 {
		t.Fatalf("expected settings menu items, got %#v", resp.ReturnData)
	}
	if text, ok := items[0]["text"].(string); !ok || text == "" {
		t.Fatalf("expected first menu item text, got %#v", items[0])
	}
}

func TestBuildMenuIncludesSchemaSubmenu(t *testing.T) {
	ime := newIsolatedTestIME(t)

	items := ime.buildMenu()
	var schemaMenu map[string]interface{}
	for _, item := range items {
		text, _ := item["text"].(string)
		if text == "输入方案(&I)" {
			schemaMenu = item
			break
		}
	}
	if schemaMenu == nil {
		t.Fatalf("expected schema submenu in menu, got %#v", items)
	}

	submenu, ok := schemaMenu["submenu"].([]map[string]interface{})
	if !ok || len(submenu) != 3 {
		t.Fatalf("expected 3 schema submenu items, got %#v", schemaMenu["submenu"])
	}
	if submenu[0]["text"] != "白霜拼音" {
		t.Fatalf("expected first schema label, got %#v", submenu[0]["text"])
	}
	if checked, _ := submenu[0]["checked"].(bool); !checked {
		t.Fatalf("expected current schema checked, got %#v", submenu[0])
	}
	if checked, _ := submenu[1]["checked"].(bool); checked {
		t.Fatalf("expected non-current schema unchecked, got %#v", submenu[1])
	}
}

func TestApplyAppearanceCommandChangesCandidateLayout(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 1

	if !ime.applyAppearanceCommand(ID_APPEARANCE_LAYOUT_HORIZONTAL) {
		t.Fatal("expected horizontal layout command handled")
	}
	if ime.style.CandidatePerRow != 9 {
		t.Fatalf("expected horizontal layout to default to 9 per row, got %d", ime.style.CandidatePerRow)
	}

	if !ime.applyAppearanceCommand(ID_APPEARANCE_PER_ROW_5) {
		t.Fatal("expected per-row command handled")
	}
	if ime.style.CandidatePerRow != 5 {
		t.Fatalf("expected 5 candidates per row, got %d", ime.style.CandidatePerRow)
	}

	if !ime.applyAppearanceCommand(ID_APPEARANCE_PER_ROW_9) {
		t.Fatal("expected per-row 9 command handled")
	}
	if ime.style.CandidatePerRow != 9 {
		t.Fatalf("expected 9 candidates per row, got %d", ime.style.CandidatePerRow)
	}

	if !ime.applyAppearanceCommand(ID_APPEARANCE_LAYOUT_VERTICAL) {
		t.Fatal("expected vertical layout command handled")
	}
	if ime.style.CandidatePerRow != 1 {
		t.Fatalf("expected vertical layout to force 1 per row, got %d", ime.style.CandidatePerRow)
	}
}

func TestEffectiveCandidatePerRowIsCappedByCandidateCount(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 9
	ime.style.CandidateCount = 3

	if got := ime.effectiveCandidatePerRow(); got != 3 {
		t.Fatalf("expected effective per-row capped to 3, got %d", got)
	}

	customizeUI := ime.customizeUIMap()
	candPerRow, ok := customizeUI["candPerRow"].(int)
	if !ok {
		t.Fatalf("expected candPerRow int, got %#v", customizeUI["candPerRow"])
	}
	if candPerRow != 3 {
		t.Fatalf("expected customizeUI candPerRow capped to 3, got %d", candPerRow)
	}
}

func TestOnCommandAppearanceRefreshesCurrentCandidates(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 17,
		ID:     imecore.FlexibleID{Int: ID_APPEARANCE_PER_ROW_5, IsInt: true},
	}, imecore.NewResponse(17, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected appearance command handled, got %d", resp.ReturnValue)
	}
	if ime.style.CandidatePerRow != 5 {
		t.Fatalf("expected per-row updated to 5, got %d", ime.style.CandidatePerRow)
	}
	if resp.CompositionString != "ni" {
		t.Fatalf("expected current composition returned, got %#v", resp.CompositionString)
	}
	if !resp.ShowCandidates || len(resp.CandidateList) == 0 {
		t.Fatalf("expected current candidates returned for immediate refresh, got %#v", resp)
	}
}

func TestOnCommandCandidateCountWritesConfigAndDeploysConfigFile(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	oldDeployConfigFileFunc := deployConfigFileFunc
	oldStartMaintenanceFunc := startMaintenanceFunc
	oldJoinMaintenanceThreadFunc := joinMaintenanceThreadFunc
	deployCalls := 0
	var deployFile string
	var deployKey string
	maintenanceCalls := 0
	joinCalls := 0
	deployConfigFileFunc = func(filePath, key string) bool {
		deployCalls++
		deployFile = filePath
		deployKey = key
		return true
	}
	startMaintenanceFunc = func(fullcheck bool) bool {
		maintenanceCalls++
		return true
	}
	joinMaintenanceThreadFunc = func() {
		joinCalls++
	}
	defer func() {
		deployConfigFileFunc = oldDeployConfigFileFunc
		startMaintenanceFunc = oldStartMaintenanceFunc
		joinMaintenanceThreadFunc = oldJoinMaintenanceThreadFunc
	}()

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{
		{Text: "你"},
		{Text: "呢"},
		{Text: "泥"},
		{Text: "尼"},
		{Text: "拟"},
		{Text: "逆"},
	}

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 18,
		ID:     imecore.FlexibleID{Int: ID_APPEARANCE_CAND_COUNT_3, IsInt: true},
	}, imecore.NewResponse(18, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected candidate count command handled, got %d", resp.ReturnValue)
	}
	if ime.style.CandidateCount != 3 {
		t.Fatalf("expected candidate count updated to 3, got %d", ime.style.CandidateCount)
	}
	if backend.redeployCalls != 0 {
		t.Fatalf("expected candidate count change not to trigger full redeploy, got %d", backend.redeployCalls)
	}
	if deployCalls != 1 {
		t.Fatalf("expected deploy config file called once, got %d", deployCalls)
	}
	if maintenanceCalls != 1 || joinCalls != 1 {
		t.Fatalf("expected maintenance start/join once, got start=%d join=%d", maintenanceCalls, joinCalls)
	}
	if deployFile != "default.yaml" || deployKey != "config_version" {
		t.Fatalf("unexpected deploy config args file=%q key=%q", deployFile, deployKey)
	}
	configPath := filepath.Join(os.Getenv("APPDATA"), APP, "Rime", rimeDefaultCustomConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected %s written, got err=%v", configPath, err)
	}
	if string(data) != "config_version: '3'\npatch:\n  menu/page_size: 3\n" {
		t.Fatalf("unexpected config content: %q", string(data))
	}
}

func TestOnCommandCandidateCountUsesRuntimePageSizeWhenAvailable(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	oldDeployConfigFileFunc := deployConfigFileFunc
	oldStartMaintenanceFunc := startMaintenanceFunc
	oldJoinMaintenanceThreadFunc := joinMaintenanceThreadFunc
	deployConfigFileFunc = func(filePath, key string) bool {
		t.Fatalf("did not expect deploy config file call, got file=%q key=%q", filePath, key)
		return false
	}
	startMaintenanceFunc = func(fullcheck bool) bool {
		t.Fatalf("did not expect maintenance start")
		return false
	}
	joinMaintenanceThreadFunc = func() {
		t.Fatalf("did not expect maintenance join")
	}
	defer func() {
		deployConfigFileFunc = oldDeployConfigFileFunc
		startMaintenanceFunc = oldStartMaintenanceFunc
		joinMaintenanceThreadFunc = oldJoinMaintenanceThreadFunc
	}()

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.pageSizeOK = true
	backend.composition = "ni"
	backend.candidates = []candidateItem{
		{Text: "你"},
		{Text: "呢"},
		{Text: "泥"},
		{Text: "尼"},
	}

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 20,
		ID:     imecore.FlexibleID{Int: ID_APPEARANCE_CAND_COUNT_3, IsInt: true},
	}, imecore.NewResponse(20, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected candidate count command handled, got %d", resp.ReturnValue)
	}
	if len(backend.pageSizeCalls) == 0 || backend.pageSizeCalls[len(backend.pageSizeCalls)-1] != 3 {
		t.Fatalf("expected runtime page size call with 3, got %#v", backend.pageSizeCalls)
	}
	configPath := filepath.Join(os.Getenv("APPDATA"), APP, "Rime", rimeDefaultCustomConfigFileName)
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected no config file write on runtime success, got err=%v", err)
	}
}

func TestBuildMenuIncludesCandidateLayoutSubmenus(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 5

	items := ime.buildMenu()
	var appearanceMenu map[string]interface{}
	for _, item := range items {
		text, _ := item["text"].(string)
		if text == "外观(&A)" {
			appearanceMenu = item
			break
		}
	}
	if appearanceMenu == nil {
		t.Fatalf("expected appearance menu, got %#v", items)
	}

	submenu, ok := appearanceMenu["submenu"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected appearance submenu, got %#v", appearanceMenu["submenu"])
	}

	var layoutMenu map[string]interface{}
	var perRowMenu map[string]interface{}
	for _, item := range submenu {
		text, _ := item["text"].(string)
		if text == "候选排列" {
			layoutMenu = item
		}
		if text == "每行候选数" {
			perRowMenu = item
		}
	}
	if layoutMenu == nil || perRowMenu == nil {
		t.Fatalf("expected layout menus, got %#v", submenu)
	}

	layoutItems, ok := layoutMenu["submenu"].([]map[string]interface{})
	if !ok || len(layoutItems) != 2 {
		t.Fatalf("expected 2 layout items, got %#v", layoutMenu["submenu"])
	}
	if checked, _ := layoutItems[1]["checked"].(bool); !checked {
		t.Fatalf("expected horizontal layout checked, got %#v", layoutItems[1])
	}

	perRowItems, ok := perRowMenu["submenu"].([]map[string]interface{})
	if !ok || len(perRowItems) != 4 {
		t.Fatalf("expected 4 per-row items, got %#v", perRowMenu["submenu"])
	}
	if checked, _ := perRowItems[1]["checked"].(bool); !checked {
		t.Fatalf("expected 5-per-row checked, got %#v", perRowItems[1])
	}
	if enabled, _ := perRowMenu["enabled"].(bool); !enabled {
		t.Fatalf("expected per-row menu enabled in horizontal mode, got %#v", perRowMenu)
	}
}

func TestBuildMenuCapsPerRowHighlightByCandidateCount(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 9
	ime.style.CandidateCount = 3

	items := ime.buildMenu()
	var appearanceMenu map[string]interface{}
	for _, item := range items {
		text, _ := item["text"].(string)
		if text == "外观(&A)" {
			appearanceMenu = item
			break
		}
	}
	if appearanceMenu == nil {
		t.Fatalf("expected appearance menu, got %#v", items)
	}

	submenu, ok := appearanceMenu["submenu"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected appearance submenu, got %#v", appearanceMenu["submenu"])
	}

	var perRowMenu map[string]interface{}
	for _, item := range submenu {
		text, _ := item["text"].(string)
		if text == "每行候选数" {
			perRowMenu = item
			break
		}
	}
	if perRowMenu == nil {
		t.Fatalf("expected per-row menu, got %#v", submenu)
	}

	perRowItems, ok := perRowMenu["submenu"].([]map[string]interface{})
	if !ok || len(perRowItems) != 4 {
		t.Fatalf("expected 4 per-row items, got %#v", perRowMenu["submenu"])
	}

	if checked, _ := perRowItems[0]["checked"].(bool); !checked {
		t.Fatalf("expected 3-per-row checked when candidate count is 3, got %#v", perRowItems[0])
	}
	if checked, _ := perRowItems[3]["checked"].(bool); checked {
		t.Fatalf("expected 9-per-row unchecked when candidate count is 3, got %#v", perRowItems[3])
	}
}

func TestBuildMenuDisablesPerRowSubmenuInVerticalLayout(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 1

	items := ime.buildMenu()
	var appearanceMenu map[string]interface{}
	for _, item := range items {
		text, _ := item["text"].(string)
		if text == "外观(&A)" {
			appearanceMenu = item
			break
		}
	}
	if appearanceMenu == nil {
		t.Fatalf("expected appearance menu, got %#v", items)
	}

	submenu, ok := appearanceMenu["submenu"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected appearance submenu, got %#v", appearanceMenu["submenu"])
	}

	var perRowMenu map[string]interface{}
	for _, item := range submenu {
		text, _ := item["text"].(string)
		if text == "每行候选数" {
			perRowMenu = item
			break
		}
	}
	if perRowMenu == nil {
		t.Fatalf("expected per-row menu, got %#v", submenu)
	}

	if enabled, _ := perRowMenu["enabled"].(bool); enabled {
		t.Fatalf("expected per-row menu disabled in vertical mode, got %#v", perRowMenu)
	}
	perRowItems, ok := perRowMenu["submenu"].([]map[string]interface{})
	if !ok || len(perRowItems) != 4 {
		t.Fatalf("expected 4 per-row items, got %#v", perRowMenu["submenu"])
	}
	for _, item := range perRowItems {
		if enabled, _ := item["enabled"].(bool); enabled {
			t.Fatalf("expected per-row item disabled in vertical mode, got %#v", item)
		}
	}
}

func TestBuildMenuIncludesCandidateCountSubmenu(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidateCount = 7

	items := ime.buildMenu()
	var appearanceMenu map[string]interface{}
	for _, item := range items {
		text, _ := item["text"].(string)
		if text == "外观(&A)" {
			appearanceMenu = item
			break
		}
	}
	if appearanceMenu == nil {
		t.Fatalf("expected appearance menu, got %#v", items)
	}

	submenu, ok := appearanceMenu["submenu"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected appearance submenu, got %#v", appearanceMenu["submenu"])
	}

	var countMenu map[string]interface{}
	for _, item := range submenu {
		text, _ := item["text"].(string)
		if text == "总候选数量" {
			countMenu = item
			break
		}
	}
	if countMenu == nil {
		t.Fatalf("expected candidate count menu, got %#v", submenu)
	}

	countItems, ok := countMenu["submenu"].([]map[string]interface{})
	if !ok || len(countItems) != 4 {
		t.Fatalf("expected 4 candidate count items, got %#v", countMenu["submenu"])
	}
	if checked, _ := countItems[2]["checked"].(bool); !checked {
		t.Fatalf("expected 7-count checked, got %#v", countItems[2])
	}
}

func TestFillResponseFromBackendStateAppliesCandidateCount(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidateCount = 5
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{
		{Text: "你"},
		{Text: "呢"},
		{Text: "泥"},
		{Text: "尼"},
		{Text: "拟"},
		{Text: "逆"},
	}

	resp := imecore.NewResponse(19, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) != 5 {
		t.Fatalf("expected 5 candidates after truncation, got %#v", resp.CandidateList)
	}
	if resp.SetSelKeys != "12345" {
		t.Fatalf("expected select keys truncated to 12345, got %q", resp.SetSelKeys)
	}
}

func TestHandleRequestCompositionTerminatedResetsState(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	resp := ime.HandleRequest(&imecore.Request{
		SeqNum: 13,
		Method: "onCompositionTerminated",
	})

	if !resp.Success {
		t.Fatal("expected composition termination response to succeed")
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected state reset on composition termination")
	}
}

func TestHandleRequestOnDeactivateReturnsHandled(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	resp := ime.HandleRequest(&imecore.Request{
		SeqNum: 14,
		Method: "onDeactivate",
	})

	if resp.ReturnValue != 1 {
		t.Fatalf("expected onDeactivate to return 1, got %d", resp.ReturnValue)
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected deactivate to clear composition state")
	}
}

func TestHandleRequestSyncsAppearanceAcrossInstances(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	first := newTestIME()
	second := newTestIME()

	if !first.applyAppearanceCommand(ID_APPEARANCE_THEME_PURPLE) {
		t.Fatal("expected theme command handled")
	}

	resp := second.HandleRequest(&imecore.Request{
		SeqNum: 15,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})

	if resp.ReturnValue != 1 {
		t.Fatalf("expected onMenu handled, got %d", resp.ReturnValue)
	}
	if second.style.CandidateTheme != "purple" {
		t.Fatalf("expected second instance theme synced to purple, got %q", second.style.CandidateTheme)
	}
	if second.style.CandidateBackgroundColor != "#f3e8ff" {
		t.Fatalf("expected synced background color, got %q", second.style.CandidateBackgroundColor)
	}
}

func TestLoadAppearancePrefsKeepsPresetThemeAfterPersist(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	first := newTestIME()
	if !first.applyAppearanceCommand(ID_APPEARANCE_THEME_PURPLE) {
		t.Fatal("expected theme command handled")
	}

	resetSharedAppearanceConfigForTest()
	second := newTestIME()
	second.loadAppearancePrefs()

	if second.style.CandidateTheme != "purple" {
		t.Fatalf("expected persisted preset theme purple, got %q", second.style.CandidateTheme)
	}
	if second.style.CandidateBackgroundColor != "#f3e8ff" {
		t.Fatalf("expected persisted preset background color, got %q", second.style.CandidateBackgroundColor)
	}
}

func TestAppearanceSettingsPersistToDisk(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()

	ime := newTestIME()
	if !ime.applyAppearanceCommand(ID_APPEARANCE_FONT_22) {
		t.Fatal("expected font size command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_LAYOUT_HORIZONTAL) {
		t.Fatal("expected layout command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_PER_ROW_7) {
		t.Fatal("expected per-row command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_CAND_COUNT_5) {
		t.Fatal("expected candidate count command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_BG_BLUE) {
		t.Fatal("expected background color command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_HL_GREEN) {
		t.Fatal("expected highlight color command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_TEXT_BLUE) {
		t.Fatal("expected text color command handled")
	}
	if !ime.applyAppearanceCommand(ID_APPEARANCE_HLTEXT_WHITE) {
		t.Fatal("expected highlight text color command handled")
	}

	configPath := filepath.Join(appData, APP, "Rime", appearanceConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected appearance config written to disk: %v", err)
	}

	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("expected valid appearance config json: %v", err)
	}
	if got := persisted["font_point"]; got != float64(22) {
		t.Fatalf("expected persisted font_point 22, got %#v", got)
	}
	if got := persisted["candidate_per_row"]; got != float64(7) {
		t.Fatalf("expected persisted candidate_per_row 7, got %#v", got)
	}
	if got := persisted["candidate_count"]; got != float64(5) {
		t.Fatalf("expected persisted candidate_count 5, got %#v", got)
	}
	if got := persisted["candidate_background_color"]; got != "#f3f8ff" {
		t.Fatalf("expected persisted background color, got %#v", got)
	}
	if got := persisted["candidate_highlight_color"]; got != "#d9f2e6" {
		t.Fatalf("expected persisted highlight color, got %#v", got)
	}
	if got := persisted["candidate_text_color"]; got != "#1d4ed8" {
		t.Fatalf("expected persisted text color, got %#v", got)
	}
	if got := persisted["candidate_highlight_text_color"]; got != "#ffffff" {
		t.Fatalf("expected persisted highlight text color, got %#v", got)
	}

	resetSharedAppearanceConfigForTest()
	reloaded := newTestIME()
	reloaded.loadAppearancePrefs()

	if reloaded.style.FontPoint != 22 {
		t.Fatalf("expected reloaded font size 22, got %d", reloaded.style.FontPoint)
	}
	if reloaded.style.CandidatePerRow != 7 {
		t.Fatalf("expected reloaded per-row 7, got %d", reloaded.style.CandidatePerRow)
	}
	if reloaded.style.CandidateCount != 5 {
		t.Fatalf("expected reloaded candidate count 5, got %d", reloaded.style.CandidateCount)
	}
	if reloaded.style.CandidateBackgroundColor != "#f3f8ff" {
		t.Fatalf("expected reloaded background color, got %q", reloaded.style.CandidateBackgroundColor)
	}
	if reloaded.style.CandidateHighlightColor != "#d9f2e6" {
		t.Fatalf("expected reloaded highlight color, got %q", reloaded.style.CandidateHighlightColor)
	}
	if reloaded.style.CandidateTextColor != "#1d4ed8" {
		t.Fatalf("expected reloaded text color, got %q", reloaded.style.CandidateTextColor)
	}
	if reloaded.style.CandidateHighlightTextColor != "#ffffff" {
		t.Fatalf("expected reloaded highlight text color, got %q", reloaded.style.CandidateHighlightTextColor)
	}
	if reloaded.style.CandidateTheme != "custom" {
		t.Fatalf("expected reloaded theme custom for custom colors, got %q", reloaded.style.CandidateTheme)
	}
}

func TestLoadAppearancePrefsCreatesDefaultConfigWhenMissing(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()

	ime := newTestIME()
	ime.loadAppearancePrefs()

	configPath := filepath.Join(appData, APP, "Rime", appearanceConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected default appearance config written to disk: %v", err)
	}

	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("expected valid appearance config json: %v", err)
	}
	if got := persisted["candidate_per_row"]; got != float64(1) {
		t.Fatalf("expected persisted candidate_per_row 1 for vertical default, got %#v", got)
	}
	if got := persisted["candidate_count"]; got != float64(9) {
		t.Fatalf("expected persisted candidate_count 9, got %#v", got)
	}
	if got := persisted["font_point"]; got != float64(20) {
		t.Fatalf("expected persisted font_point 20, got %#v", got)
	}
	if got := persisted["candidate_theme"]; got != "default" {
		t.Fatalf("expected persisted candidate_theme default, got %#v", got)
	}
	if ime.style.CandidatePerRow != 1 {
		t.Fatalf("expected in-memory style to stay vertical by default, got %d", ime.style.CandidatePerRow)
	}
}

func TestRimeLogDirUsesMoqiIMLogUnderLocalAppData(t *testing.T) {
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)

	got := rimeLogDir()
	want := filepath.Join(localAppData, "MoqiIM", "Log")
	if got != want {
		t.Fatalf("expected rime log dir %q, got %q", want, got)
	}
}

func TestRimeLogDirReturnsEmptyWithoutLocalAppData(t *testing.T) {
	t.Setenv("LOCALAPPDATA", "")

	if got := rimeLogDir(); got != "" {
		t.Fatalf("expected empty rime log dir when LOCALAPPDATA unset, got %q", got)
	}
}

func TestNativeBackendRedeployRunsAsync(t *testing.T) {
	resetNativeRuntimeStateForTest()
	oldRedeployFunc := rimeRedeployFunc
	oldInitOK := rimeInitOK
	rimeInitOK = true
	done := make(chan struct{})
	rimeRedeployFunc = func(datadir, userdir, appname, appver string) bool {
		time.Sleep(120 * time.Millisecond)
		close(done)
		return true
	}
	defer func() {
		rimeRedeployFunc = oldRedeployFunc
		rimeInitOK = oldInitOK
		resetNativeRuntimeStateForTest()
	}()

	backend := &nativeBackend{}
	start := time.Now()
	if !backend.Redeploy("shared", "user") {
		t.Fatal("expected redeploy to start")
	}
	if elapsed := time.Since(start); elapsed > 60*time.Millisecond {
		t.Fatalf("expected async redeploy to return quickly, took %s", elapsed)
	}
	if backend.Available() {
		t.Fatal("expected backend unavailable while redeploy is in progress")
	}
	if notification := backend.ConsumeNotification(); notification != nil {
		t.Fatalf("expected no notification before async redeploy completes, got %#v", notification)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async redeploy to finish")
	}

	if !backend.Available() {
		t.Fatal("expected backend available after redeploy completes")
	}
	notification := backend.ConsumeNotification()
	if notification == nil {
		t.Fatal("expected completion notification after async redeploy")
	}
	if notification.Message != "重新部署成功" {
		t.Fatalf("unexpected completion notification: %#v", notification)
	}
	if backend.ConsumeNotification() != nil {
		t.Fatal("expected completion notification to be consumed once")
	}
}
