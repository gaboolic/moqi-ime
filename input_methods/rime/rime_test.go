package rime

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

func TestMain(m *testing.M) {
	tempAppData, err := os.MkdirTemp("", "moqi-ime-rime-test-*")
	if err == nil {
		_ = os.Setenv("APPDATA", tempAppData)
	}
	code := m.Run()
	if err == nil {
		_ = os.RemoveAll(tempAppData)
	}
	os.Exit(code)
}

func writeTestAIConfig(t *testing.T, appData string, body string) {
	t.Helper()
	configPath := filepath.Join(appData, APP, aiConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create AI config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write AI config: %v", err)
	}
}

func writeTestBertConfig(t *testing.T, appData, body string) {
	t.Helper()
	configPath := filepath.Join(appData, APP, bertConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create BERT config dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write BERT config: %v", err)
	}
}

func writeTestSchemeSetConfig(t *testing.T, appData, current string) {
	t.Helper()
	configPath := filepath.Join(appData, APP, schemeSetConfigFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create scheme set config dir: %v", err)
	}
	body, err := json.MarshalIndent(schemeSetConfig{Current: current}, "", "  ")
	if err != nil {
		t.Fatalf("marshal scheme set config: %v", err)
	}
	if err := os.WriteFile(configPath, body, 0o644); err != nil {
		t.Fatalf("write scheme set config: %v", err)
	}
}

func writeTestCustomPhraseFile(t *testing.T, appData, body string) string {
	t.Helper()
	configPath := filepath.Join(appData, APP, customPhraseFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create custom phrase dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write custom phrase file: %v", err)
	}
	return configPath
}

func writeTestSuperAbbrevFile(t *testing.T, appData, body string) string {
	t.Helper()
	configPath := filepath.Join(appData, APP, superAbbrevFileName)
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatalf("create super abbrev dir: %v", err)
	}
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write super abbrev file: %v", err)
	}
	return configPath
}

type testDictEntry struct {
	code  string
	words []candidateItem
}

type testBackend struct {
	session                   bool
	composition               string
	rawInput                  string
	pageNo                    int
	candidates                []candidateItem
	commitString              string
	asciiMode                 bool
	fullShape                 bool
	options                   map[string]bool
	saveOptions               []string
	schemaSwitches            map[string][]RimeSwitch
	schemas                   []RimeSchema
	currentSchemaID           string
	selectSchemaCalls         []string
	pageSizeCalls             []int
	pageSizeOK                bool
	redeployCalls             int
	redeploySharedDir         string
	redeployUserDir           string
	redeployOK                bool
	syncCalls                 int
	syncOK                    bool
	setOptionCalls            int
	getOptionCalls            int
	bertCandidatesForCodeFunc func(code string, limit int) []candidateItem
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
		options: map[string]bool{
			"ascii_mode":         false,
			"full_shape":         false,
			"ascii_punct":        false,
			"traditionalization": false,
		},
		saveOptions: []string{"ascii_mode", "traditionalization", "ascii_punct", "full_shape"},
		schemaSwitches: map[string][]RimeSwitch{
			"rime_frost": {
				{Name: "ascii_mode", States: []string{"中文", "西文"}},
				{Name: "traditionalization", States: []string{"简体", "繁体"}},
				{Name: "ascii_punct", States: []string{"中文标点", "英文标点"}},
				{Name: "full_shape", States: []string{"半角", "全角"}},
			},
			"rime_frost_double_pinyin": {
				{Name: "ascii_mode", States: []string{"中文", "西文"}},
				{Name: "traditionalization", States: []string{"简体", "繁体"}},
				{Name: "ascii_punct", States: []string{"中文标点", "英文标点"}},
				{Name: "full_shape", States: []string{"半角", "全角"}},
			},
			"rime_frost_double_pinyin_flypy": {
				{Name: "ascii_mode", States: []string{"中文", "西文"}},
				{Name: "traditionalization", States: []string{"简体", "繁体"}},
				{Name: "ascii_punct", States: []string{"中文标点", "英文标点"}},
				{Name: "full_shape", States: []string{"半角", "全角"}},
			},
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

func (b *testBackend) HasSession() bool {
	return b.session
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
	b.rawInput = ""
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
	if keyCode == vkShift && modifiers == releaseMask {
		b.SetOption("ascii_mode", !b.GetOption("ascii_mode"))
		return false
	}
	if keyCode == vkCapital && modifiers&releaseMask == 0 {
		b.SetOption("ascii_mode", !b.GetOption("ascii_mode"))
		return true
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
		RawInput:        b.rawInput,
		PageNo:          b.pageNo,
		CursorPos:       len(b.composition),
		Candidates:      append([]candidateItem(nil), b.candidates...),
		CandidateCursor: 0,
		SelectKeys:      "1234567890",
		AsciiMode:       b.asciiMode,
		FullShape:       b.fullShape,
	}
	if state.RawInput == "" {
		state.RawInput = b.composition
	}
	b.commitString = ""
	return state
}

func (b *testBackend) SetOption(name string, value bool) {
	b.setOptionCalls++
	if b.options == nil {
		b.options = map[string]bool{}
	}
	b.options[name] = value
	switch name {
	case "ascii_mode":
		b.asciiMode = value
	case "full_shape":
		b.fullShape = value
	}
}

func (b *testBackend) GetOption(name string) bool {
	b.getOptionCalls++
	if b.options != nil {
		if value, ok := b.options[name]; ok {
			return value
		}
	}
	switch name {
	case "ascii_mode":
		return b.asciiMode
	case "full_shape":
		return b.fullShape
	default:
		return false
	}
}

func (b *testBackend) SaveOptions() []string {
	return append([]string(nil), b.saveOptions...)
}

func (b *testBackend) SchemaSwitches() []RimeSwitch {
	return append([]RimeSwitch(nil), b.schemaSwitches[b.currentSchemaID]...)
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

func (b *testBackend) SelectCandidate(index int) bool {
	if index < 0 || index >= len(b.candidates) {
		return false
	}
	b.commitString = b.candidates[index].Text
	b.composition = ""
	b.candidates = nil
	return true
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

func (b *testBackend) bertCandidatesForCode(code string, limit int) []candidateItem {
	if b.bertCandidatesForCodeFunc != nil {
		return b.bertCandidatesForCodeFunc(code, limit)
	}
	code = strings.TrimSpace(strings.ToLower(strings.ReplaceAll(code, "'", "")))
	if code == "" || limit <= 0 {
		return nil
	}
	for _, entry := range testDictionary() {
		if entry.code != code {
			continue
		}
		candidates := make([]candidateItem, 0, min(limit, len(entry.words)))
		for _, word := range entry.words {
			candidates = append(candidates, word)
			if len(candidates) >= limit {
				break
			}
		}
		return candidates
	}
	return nil
}

func testDictionary() []testDictEntry {
	return []testDictEntry{
		{code: "ni", words: []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}, {Text: "尼"}, {Text: "拟"}}},
		{code: "nihao", words: []candidateItem{{Text: "你好"}, {Text: "你号"}, {Text: "拟好"}}},
		{code: "nimen", words: []candidateItem{{Text: "你们"}}},
		{code: "zhong", words: []candidateItem{{Text: "中"}, {Text: "种"}, {Text: "重"}}},
		{code: "zhongwen", words: []candidateItem{{Text: "中文"}}},
		{code: "ta", words: []candidateItem{{Text: "她"}, {Text: "他"}}},
		{code: "zhi", words: []candidateItem{{Text: "只"}, {Text: "之"}}},
		{code: "shi", words: []candidateItem{{Text: "是"}, {Text: "时"}}},
		{code: "wo", words: []candidateItem{{Text: "我"}}},
		{code: "de", words: []candidateItem{{Text: "的"}}},
		{code: "meimei", words: []candidateItem{{Text: "妹妹"}}},
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
	if ime.style.CandidateTheme != "default" || ime.style.FontPoint != 20 || ime.style.CandidateCommentFontPoint != 18 {
		t.Fatalf("expected default theme defaults, got theme=%q font=%d commentFont=%d",
			ime.style.CandidateTheme, ime.style.FontPoint, ime.style.CandidateCommentFontPoint)
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
	t.Setenv("APPDATA", t.TempDir())
	resetCustomPhraseCacheForTest()
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

func TestOnKeyDownSemicolonSelectsSecondCandidate(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetCustomPhraseCacheForTest()
	ime := newIsolatedTestIME(t)
	ime.semicolonSelectSecond = true
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "呢"}, {Text: "泥"}}
	ime.keyComposing = true

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   41,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	}, imecore.NewResponse(41, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon selection to be handled, got %d", filterResp.ReturnValue)
	}

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   42,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	}, imecore.NewResponse(42, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected semicolon onKeyDown to succeed, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "呢" {
		t.Fatalf("expected semicolon to commit second candidate 呢, got %q", resp.CommitString)
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected state reset after semicolon selection")
	}
}

func TestOnKeyDownSemicolonSelectsSecondCandidateWhenKeyEncodingDiffersBetweenStages(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetCustomPhraseCacheForTest()
	ime := newIsolatedTestIME(t)
	ime.semicolonSelectSecond = true
	backend := ime.backend.(*testBackend)
	backend.composition = "n"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "那么"}, {Text: "能不能"}, {Text: "哪里"}}
	ime.keyComposing = true

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   43,
		KeyCode:  vkOem1,
		CharCode: 0,
	}, imecore.NewResponse(43, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon filterKeyDown handled with OEM keycode, got %d", filterResp.ReturnValue)
	}

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   44,
		KeyCode:  0,
		CharCode: int(';'),
	}, imecore.NewResponse(44, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected semicolon onKeyDown handled with charcode-only event, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "那么" {
		t.Fatalf("expected semicolon to commit second visible candidate 那么, got %q", resp.CommitString)
	}
}

func TestOnKeyDownSemicolonSelectsSecondCandidateWhenOEMKeyHasUnexpectedCharCode(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetCustomPhraseCacheForTest()
	ime := newIsolatedTestIME(t)
	ime.semicolonSelectSecond = true
	backend := ime.backend.(*testBackend)
	backend.composition = "n"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "那么"}, {Text: "能不能"}}
	ime.keyComposing = true

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   45,
		KeyCode:  vkOem1,
		CharCode: int('；'),
	}, imecore.NewResponse(45, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon filterKeyDown handled even with unexpected charCode, got %d", filterResp.ReturnValue)
	}

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   46,
		KeyCode:  vkOem1,
		CharCode: int('；'),
	}, imecore.NewResponse(46, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected semicolon onKeyDown handled even with unexpected charCode, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "那么" {
		t.Fatalf("expected semicolon to commit second visible candidate 那么, got %q", resp.CommitString)
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

func TestBuildMenuIncludesSchemeSetSubmenuBeforeSchemaMenu(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	if err := os.MkdirAll(filepath.Join(appData, APP, "Work"), 0o755); err != nil {
		t.Fatalf("create Work scheme set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appData, APP, defaultSchemeSetName), 0o755); err != nil {
		t.Fatalf("create default scheme set: %v", err)
	}
	writeTestSchemeSetConfig(t, appData, "Work")

	ime := newIsolatedTestIME(t)
	items := ime.buildMenu()
	if len(items) < 2 {
		t.Fatalf("expected menu items, got %#v", items)
	}

	var schemeSetIndex = -1
	var schemaIndex = -1
	var schemeSetMenu map[string]interface{}
	for i, item := range items {
		text, _ := item["text"].(string)
		if text == "切换方案集" {
			schemeSetIndex = i
			schemeSetMenu = item
		}
		if text == "输入方案(&I)" {
			schemaIndex = i
		}
	}
	if schemeSetIndex == -1 || schemaIndex == -1 {
		t.Fatalf("expected both scheme set and schema submenus, got %#v", items)
	}
	if schemeSetIndex >= schemaIndex {
		t.Fatalf("expected scheme set submenu before schema submenu, got schemeSetIndex=%d schemaIndex=%d", schemeSetIndex, schemaIndex)
	}

	submenu, ok := schemeSetMenu["submenu"].([]map[string]interface{})
	if !ok || len(submenu) != 2 {
		t.Fatalf("expected 2 scheme set submenu items, got %#v", schemeSetMenu["submenu"])
	}
	if submenu[0]["text"] != defaultSchemeSetName {
		t.Fatalf("expected default scheme set first, got %#v", submenu[0]["text"])
	}
	if submenu[1]["text"] != "Work" {
		t.Fatalf("expected Work scheme set second, got %#v", submenu[1]["text"])
	}
	if checked, _ := submenu[1]["checked"].(bool); !checked {
		t.Fatalf("expected Work scheme set checked, got %#v", submenu[1])
	}
}

func TestBuildMenuPlacesUpdateConfigBeforeDeploy(t *testing.T) {
	ime := newIsolatedTestIME(t)
	items := ime.buildMenu()

	openIndex := -1
	superIndex := -1
	updateIndex := -1
	deployIndex := -1
	for i, item := range items {
		text, _ := item["text"].(string)
		if text == "打开置顶短语" {
			openIndex = i
		}
		if text == "打开超级简拼" {
			superIndex = i
		}
		if text == "更新配置(&P)" {
			updateIndex = i
		}
		if text == "刷新配置(&R)" {
			deployIndex = i
		}
	}
	if openIndex == -1 || superIndex == -1 || updateIndex == -1 || deployIndex == -1 {
		t.Fatalf("expected custom phrase, super abbrev, update and deploy menu items, got %#v", items)
	}
	if openIndex >= updateIndex {
		t.Fatalf("expected custom phrase item before update config, got openIndex=%d updateIndex=%d", openIndex, updateIndex)
	}
	if superIndex != openIndex+1 {
		t.Fatalf("expected super abbrev item right after custom phrase, got openIndex=%d superIndex=%d", openIndex, superIndex)
	}
	if updateIndex >= deployIndex {
		t.Fatalf("expected update config before deploy, got updateIndex=%d deployIndex=%d", updateIndex, deployIndex)
	}
}

func TestBuildMenuGroupsSchemeSetSchemaUpdateAndDeployWithoutSeparators(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	if err := os.MkdirAll(filepath.Join(appData, APP, defaultSchemeSetName), 0o755); err != nil {
		t.Fatalf("create default scheme set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appData, APP, "Work"), 0o755); err != nil {
		t.Fatalf("create Work scheme set: %v", err)
	}

	ime := newIsolatedTestIME(t)
	items := ime.buildMenu()

	indexByText := map[string]int{}
	for i, item := range items {
		text, _ := item["text"].(string)
		indexByText[text] = i
	}

	group := []string{"切换方案集", "输入方案(&I)", "打开置顶短语", "打开超级简拼", "更新配置(&P)", "刷新配置(&R)"}
	for _, text := range group {
		if _, ok := indexByText[text]; !ok {
			t.Fatalf("expected menu item %q, got %#v", text, items)
		}
	}
	for i := 0; i < len(group)-1; i++ {
		current := indexByText[group[i]]
		next := indexByText[group[i+1]]
		if next != current+1 {
			t.Fatalf("expected %q and %q to be consecutive, got %d and %d", group[i], group[i+1], current, next)
		}
	}
}

func TestOnCommandOpenCustomPhraseCreatesFileAndOpensIt(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	oldOpen := openCustomPhraseTargetFunc
	defer func() {
		openCustomPhraseTargetFunc = oldOpen
	}()

	var openedPath string
	openCustomPhraseTargetFunc = func(path string) error {
		openedPath = path
		return nil
	}

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 100,
		ID:     imecore.FlexibleID{Int: ID_OPEN_CUSTOM_PHRASE, IsInt: true},
	}, imecore.NewResponse(100, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected open custom phrase command handled, got %d", resp.ReturnValue)
	}
	wantPath := filepath.Join(appData, APP, customPhraseFileName)
	if openedPath != wantPath {
		t.Fatalf("expected opened custom phrase path %q, got %q", wantPath, openedPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected custom phrase file created: %v", err)
	}
	if !strings.Contains(string(data), "词汇<Tab>编码<Tab>权重") {
		t.Fatalf("expected default custom phrase template, got %q", string(data))
	}
}

func TestOnCommandOpenSuperAbbrevCreatesFileAndOpensIt(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	oldOpen := openSuperAbbrevTargetFunc
	defer func() {
		openSuperAbbrevTargetFunc = oldOpen
	}()

	var openedPath string
	openSuperAbbrevTargetFunc = func(path string) error {
		openedPath = path
		return nil
	}

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 101,
		ID:     imecore.FlexibleID{Int: ID_OPEN_SUPER_ABBREV, IsInt: true},
	}, imecore.NewResponse(101, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected open super abbrev command handled, got %d", resp.ReturnValue)
	}
	wantPath := filepath.Join(appData, APP, superAbbrevFileName)
	if openedPath != wantPath {
		t.Fatalf("expected opened super abbrev path %q, got %q", wantPath, openedPath)
	}
	data, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected super abbrev file created: %v", err)
	}
	if !strings.Contains(string(data), "词汇<Tab>编码") {
		t.Fatalf("expected default super abbrev template, got %q", string(data))
	}
}

func TestBuildMenuUsesSwitcherSaveOptions(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.saveOptions = []string{"emoji", "full_shape"}
	backend.schemaSwitches[backend.currentSchemaID] = []RimeSwitch{
		{Name: "emoji", States: []string{"常规", "Emoji"}},
		{Name: "full_shape", States: []string{"半角", "全角"}},
		{Name: "ascii_mode", States: []string{"中文", "西文"}},
	}

	items := ime.buildMenu()
	if len(items) < 3 {
		t.Fatalf("expected switch items in menu, got %#v", items)
	}
	if text, _ := items[0]["text"].(string); text != "常规 → Emoji" {
		t.Fatalf("expected emoji switch from save_options first, got %#v", items[0])
	}
	if text, _ := items[1]["text"].(string); text != "半角 → 全角" {
		t.Fatalf("expected full_shape switch from save_options second, got %#v", items[1])
	}
	if _, ok := items[2]["text"].(string); !ok || items[2]["text"] != "" {
		t.Fatalf("expected separator after dynamic switches, got %#v", items[2])
	}
}

func TestOnCommandTogglesDynamicSwitcherOption(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.saveOptions = []string{"emoji"}
	backend.schemaSwitches[backend.currentSchemaID] = []RimeSwitch{
		{Name: "emoji", States: []string{"常规", "Emoji"}},
	}

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 99,
		ID:     imecore.FlexibleID{Int: switchCommandID(0), IsInt: true},
	}, imecore.NewResponse(99, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected dynamic switch command handled, got %d", resp.ReturnValue)
	}
	if !backend.GetOption("emoji") {
		t.Fatal("expected dynamic switch option toggled on")
	}
}

func TestOnCommandSwitchesSchemeSetAndRedeploysWithSelectedUserDir(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	if err := os.MkdirAll(filepath.Join(appData, APP, defaultSchemeSetName), 0o755); err != nil {
		t.Fatalf("create default scheme set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appData, APP, "Work"), 0o755); err != nil {
		t.Fatalf("create Work scheme set: %v", err)
	}

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)

	resp := ime.onCommand(&imecore.Request{
		SeqNum: 100,
		ID:     imecore.FlexibleID{Int: schemeSetCommandID(1), IsInt: true},
	}, imecore.NewResponse(100, true))
	if resp.ReturnValue != 1 {
		t.Fatalf("expected scheme set command handled, got %d", resp.ReturnValue)
	}
	if backend.redeployCalls != 1 {
		t.Fatalf("expected scheme set switch to redeploy once, got %d", backend.redeployCalls)
	}
	wantUserDir := filepath.Join(appData, APP, "Work")
	if backend.redeployUserDir != wantUserDir {
		t.Fatalf("expected redeploy user dir %q, got %q", wantUserDir, backend.redeployUserDir)
	}
	if got := currentSchemeSetName(); got != "Work" {
		t.Fatalf("expected current scheme set Work after switch, got %q", got)
	}
}

func TestOnCommandUpdateConfigFailsWithoutGit(t *testing.T) {
	oldLookPath := gitLookPathFunc
	oldIsRepo := gitIsRepoFunc
	defer func() {
		gitLookPathFunc = oldLookPath
		gitIsRepoFunc = oldIsRepo
	}()
	resetConfigUpdateStateForTest()

	gitLookPathFunc = func(file string) (string, error) {
		return "", errors.New("missing git")
	}
	gitIsRepoFunc = func(dir string) (bool, error) {
		t.Fatalf("did not expect git repo check when git is missing")
		return false, nil
	}

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 101,
		ID:     imecore.FlexibleID{Int: ID_UPDATE_CONFIG, IsInt: true},
	}, imecore.NewResponse(101, true))
	if resp.ReturnValue != 0 {
		t.Fatalf("expected update config command to fail without git, got %d", resp.ReturnValue)
	}
	if resp.TrayNotification == nil || resp.TrayNotification.Message != "未检测到 Git 命令" {
		t.Fatalf("unexpected tray notification: %#v", resp.TrayNotification)
	}
}

func TestOnCommandUpdateConfigFailsWhenUserDirNotGitRepo(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	oldLookPath := gitLookPathFunc
	oldIsRepo := gitIsRepoFunc
	defer func() {
		gitLookPathFunc = oldLookPath
		gitIsRepoFunc = oldIsRepo
	}()
	resetConfigUpdateStateForTest()

	gitLookPathFunc = func(file string) (string, error) {
		return "git", nil
	}
	gitIsRepoFunc = func(dir string) (bool, error) {
		want := filepath.Join(appData, APP, defaultSchemeSetName)
		if dir != want {
			t.Fatalf("expected git repo check dir %q, got %q", want, dir)
		}
		return false, nil
	}

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 102,
		ID:     imecore.FlexibleID{Int: ID_UPDATE_CONFIG, IsInt: true},
	}, imecore.NewResponse(102, true))
	if resp.ReturnValue != 0 {
		t.Fatalf("expected update config command to fail for non-git dir, got %d", resp.ReturnValue)
	}
	if resp.TrayNotification == nil || resp.TrayNotification.Message != "当前方案集目录不是 Git 仓库" {
		t.Fatalf("unexpected tray notification: %#v", resp.TrayNotification)
	}
}

func TestOnCommandUpdateConfigRunsGitPullAsyncAndNotifies(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	oldLookPath := gitLookPathFunc
	oldIsRepo := gitIsRepoFunc
	oldPull := gitPullFunc
	defer func() {
		gitLookPathFunc = oldLookPath
		gitIsRepoFunc = oldIsRepo
		gitPullFunc = oldPull
		resetConfigUpdateStateForTest()
	}()
	resetConfigUpdateStateForTest()

	userDir := filepath.Join(appData, APP, defaultSchemeSetName)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("create user dir: %v", err)
	}

	gitLookPathFunc = func(file string) (string, error) {
		return "git", nil
	}
	gitIsRepoFunc = func(dir string) (bool, error) {
		return dir == userDir, nil
	}
	pullDone := make(chan struct{})
	var pullDir string
	gitPullFunc = func(dir string) (string, error) {
		pullDir = dir
		time.Sleep(120 * time.Millisecond)
		close(pullDone)
		return "Already up to date.", nil
	}

	ime := newIsolatedTestIME(t)
	asyncResponses := make(chan *imecore.Response, 1)
	ime.SetAsyncResponseSender(func(resp *imecore.Response) {
		asyncResponses <- resp
	})

	start := time.Now()
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 103,
		ID:     imecore.FlexibleID{Int: ID_UPDATE_CONFIG, IsInt: true},
	}, imecore.NewResponse(103, true))
	if elapsed := time.Since(start); elapsed > 60*time.Millisecond {
		t.Fatalf("expected update config command to return quickly, took %s", elapsed)
	}
	if resp.ReturnValue != 1 {
		t.Fatalf("expected update config command to succeed, got %d", resp.ReturnValue)
	}
	if resp.TrayNotification == nil || resp.TrayNotification.Message != "开始更新配置..." {
		t.Fatalf("unexpected initial tray notification: %#v", resp.TrayNotification)
	}

	select {
	case <-pullDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async git pull")
	}

	if pullDir != userDir {
		t.Fatalf("expected git pull dir %q, got %q", userDir, pullDir)
	}

	select {
	case asyncResp := <-asyncResponses:
		if asyncResp.TrayNotification == nil {
			t.Fatalf("expected async tray notification, got %#v", asyncResp)
		}
		if asyncResp.TrayNotification.Message != "配置已是最新" {
			t.Fatalf("unexpected async tray notification: %#v", asyncResp.TrayNotification)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for async tray notification")
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
	configPath := filepath.Join(os.Getenv("APPDATA"), APP, defaultSchemeSetName, rimeDefaultCustomConfigFileName)
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
	configPath := filepath.Join(os.Getenv("APPDATA"), APP, defaultSchemeSetName, rimeDefaultCustomConfigFileName)
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		t.Fatalf("expected no config file write on runtime success, got err=%v", err)
	}
}

func TestBuildMenuIncludesCandidateLayoutSubmenus(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidatePerRow = 5
	ime.style.CandidateCommentFontPoint = 18

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
	var commentFontMenu map[string]interface{}
	for _, item := range submenu {
		text, _ := item["text"].(string)
		if text == "候选排列" {
			layoutMenu = item
		}
		if text == "每行候选数" {
			perRowMenu = item
		}
		if text == "注释文字大小" {
			commentFontMenu = item
		}
	}
	if layoutMenu == nil || perRowMenu == nil || commentFontMenu == nil {
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

	commentFontItems, ok := commentFontMenu["submenu"].([]map[string]interface{})
	if !ok || len(commentFontItems) != 5 {
		t.Fatalf("expected 5 comment font items, got %#v", commentFontMenu["submenu"])
	}
	if checked, _ := commentFontItems[2]["checked"].(bool); !checked {
		t.Fatalf("expected comment font 18 checked, got %#v", commentFontItems[2])
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

func TestBuildMenuIncludesSharedInputStateToggle(t *testing.T) {
	ime := newIsolatedTestIME(t)

	items := ime.buildMenu()
	found := false
	for _, item := range items {
		if text, _ := item["text"].(string); text == "输入状态共享" {
			found = true
			if checked, _ := item["checked"].(bool); checked {
				t.Fatalf("expected input state sharing disabled by default, got %#v", item)
			}
		}
	}
	if !found {
		t.Fatalf("expected shared input state toggle in menu, got %#v", items)
	}
}

func TestBuildMenuIncludesInputSettingsSubmenu(t *testing.T) {
	ime := newIsolatedTestIME(t)

	items := ime.buildMenu()
	var inputSettingsMenu map[string]interface{}
	for _, item := range items {
		if text, _ := item["text"].(string); text == "输入设置" {
			inputSettingsMenu = item
			break
		}
	}
	if inputSettingsMenu == nil {
		t.Fatalf("expected input settings menu, got %#v", items)
	}

	submenu, ok := inputSettingsMenu["submenu"].([]map[string]interface{})
	if !ok || len(submenu) != 3 {
		t.Fatalf("expected three input settings items, got %#v", inputSettingsMenu["submenu"])
	}
	if text, _ := submenu[0]["text"].(string); text != "自动插入成对引号" {
		t.Fatalf("unexpected input settings item: %#v", submenu[0])
	}
	if checked, _ := submenu[0]["checked"].(bool); checked {
		t.Fatalf("expected auto pair quotes disabled by default, got %#v", submenu[0])
	}
	if text, _ := submenu[1]["text"].(string); text != "分号键次选" {
		t.Fatalf("unexpected second input settings item: %#v", submenu[1])
	}
	if checked, _ := submenu[1]["checked"].(bool); checked {
		t.Fatalf("expected semicolon select second disabled by default, got %#v", submenu[1])
	}
	if text, _ := submenu[2]["text"].(string); text != "BERT 整句优化" {
		t.Fatalf("unexpected third input settings item: %#v", submenu[2])
	}
	if checked, _ := submenu[2]["checked"].(bool); checked {
		t.Fatalf("expected BERT rerank disabled by default, got %#v", submenu[2])
	}
}

func TestOnCommandTogglesAutoPairQuotes(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 21,
		ID:     imecore.FlexibleID{Int: ID_INPUT_AUTO_PAIR_QUOTES, IsInt: true},
	}, imecore.NewResponse(21, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected auto pair quotes command handled, got %d", resp.ReturnValue)
	}
	if !ime.autoPairQuotes {
		t.Fatal("expected auto pair quotes enabled")
	}
	if got, ok := resp.CustomizeUI["autoPairQuotes"].(bool); !ok || !got {
		t.Fatalf("expected customizeUI autoPairQuotes true, got %#v", resp.CustomizeUI["autoPairQuotes"])
	}

	configPath := filepath.Join(appData, APP, appearanceConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected appearance config written to disk: %v", err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("expected valid appearance config json: %v", err)
	}
	if got := persisted["auto_pair_quotes"]; got != true {
		t.Fatalf("expected persisted auto_pair_quotes true, got %#v", got)
	}
}

func TestOnCommandTogglesSemicolonSelectSecond(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 22,
		ID:     imecore.FlexibleID{Int: ID_INPUT_SEMICOLON_SELECT_SECOND, IsInt: true},
	}, imecore.NewResponse(22, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected semicolon select second command handled, got %d", resp.ReturnValue)
	}
	if !ime.semicolonSelectSecond {
		t.Fatal("expected semicolon select second enabled after toggle")
	}
	if got, ok := resp.CustomizeUI["semicolonSelectSecond"].(bool); !ok || !got {
		t.Fatalf("expected customizeUI semicolonSelectSecond true, got %#v", resp.CustomizeUI["semicolonSelectSecond"])
	}

	configPath := filepath.Join(appData, APP, appearanceConfigFileName)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected appearance config written to disk: %v", err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("expected valid appearance config json: %v", err)
	}
	if got := persisted["semicolon_select_second"]; got != true {
		t.Fatalf("expected persisted semicolon_select_second true, got %#v", got)
	}
}

func TestOnCommandEnableBertWithoutModelShowsDownloadHint(t *testing.T) {
	appData := t.TempDir()
	programFiles := filepath.Join(t.TempDir(), "ProgramFilesX86")
	t.Setenv("APPDATA", appData)
	t.Setenv("PROGRAMFILES(X86)", programFiles)
	resetSharedAppearanceConfigForTest()
	writeTestBertConfig(t, appData, `{
  "enabled": false,
  "provider": "onnx_cross_encoder",
  "model_path": "bert/model.onnx",
  "vocab_path": "bert/vocab.txt",
  "runtime_library_path": "bert/onnxruntime.dll"
}`)

	ime := newIsolatedTestIME(t)
	resp := ime.onCommand(&imecore.Request{
		SeqNum: 23,
		ID:     imecore.FlexibleID{Int: ID_INPUT_BERT_RERANK, IsInt: true},
	}, imecore.NewResponse(23, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected BERT toggle command handled, got %d", resp.ReturnValue)
	}
	if ime.bertEnabled {
		t.Fatal("expected BERT to remain disabled when model files are missing")
	}
	if resp.TrayNotification == nil || resp.TrayNotification.Message != "BERT 模型文件缺失，请先手动下载" {
		t.Fatalf("unexpected tray notification: %#v", resp.TrayNotification)
	}
	if resp.ShowMessage == nil {
		t.Fatal("expected missing-model hint message")
	}
	if !strings.Contains(resp.ShowMessage.Message, bertModelDownloadURL) {
		t.Fatalf("expected download URL in message, got %q", resp.ShowMessage.Message)
	}
	if !strings.Contains(resp.ShowMessage.Message, filepath.Join(programFiles, "MoqiIM", "bert")) {
		t.Fatalf("expected install path in message, got %q", resp.ShowMessage.Message)
	}
	if got, ok := resp.CustomizeUI["bertEnabled"].(bool); !ok || got {
		t.Fatalf("expected customizeUI bertEnabled false, got %#v", resp.CustomizeUI["bertEnabled"])
	}
}

func TestFillResponseFromBackendStatePrependsCustomPhraseCandidates(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\nalps\tal\t5\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.candidates = []candidateItem{
		{Text: "阿"},
		{Text: "啊"},
	}

	resp := imecore.NewResponse(30, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) < 3 {
		t.Fatalf("expected custom and backend candidates, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "alpha" || resp.CandidateList[1] != "alps" {
		t.Fatalf("expected custom phrase candidates prepended, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[2] != "阿" {
		t.Fatalf("expected backend candidates appended after custom phrases, got %#v", resp.CandidateList)
	}
	if resp.CandidateCursor != 0 {
		t.Fatalf("expected custom phrase candidate cursor default to 0, got %d", resp.CandidateCursor)
	}
	if resp.SetSelKeys != "1234" {
		t.Fatalf("expected select keys recomputed for combined candidates, got %q", resp.SetSelKeys)
	}
}

func TestFillResponseFromBackendStateDeduplicatesCustomPhraseAgainstBackendCandidates(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n传\tc\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "c"
	backend.candidates = []candidateItem{
		{Text: "传"},
		{Text: "船"},
		{Text: "串"},
	}

	resp := imecore.NewResponse(31, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) < 3 {
		t.Fatalf("expected deduplicated custom and backend candidates, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "传" {
		t.Fatalf("expected custom phrase to stay first, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[1] != "船" || resp.CandidateList[2] != "串" {
		t.Fatalf("expected backend duplicates removed after custom phrase, got %#v", resp.CandidateList)
	}
	for i := 1; i < len(resp.CandidateList); i++ {
		if resp.CandidateList[i] == "传" {
			t.Fatalf("expected duplicated backend candidate removed, got %#v", resp.CandidateList)
		}
	}
}

func TestFillResponseFromBackendStateMatchesCustomPhraseByRawInput(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n娘\tnl\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "niang"
	backend.rawInput = "nl"
	backend.candidates = []candidateItem{
		{Text: "娘"},
		{Text: "酿"},
	}

	resp := imecore.NewResponse(301, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) < 1 {
		t.Fatalf("expected custom phrase candidate, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "娘" {
		t.Fatalf("expected raw input nl to match custom phrase before backend candidates, got %#v", resp.CandidateList)
	}
}

func TestFillResponseFromBackendStateMatchesCustomPhraseByTrackedRawInput(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n娘\tnl\t10\n")

	ime := newIsolatedTestIME(t)
	ime.rawInputTracked = "nl"
	backend := ime.backend.(*testBackend)
	backend.composition = "niang"
	backend.rawInput = ""
	backend.candidates = []candidateItem{
		{Text: "娘"},
		{Text: "酿"},
	}

	resp := imecore.NewResponse(302, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) < 1 {
		t.Fatalf("expected custom phrase candidate, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "娘" {
		t.Fatalf("expected tracked raw input nl to match custom phrase before backend candidates, got %#v", resp.CandidateList)
	}
}

func TestFillResponseFromBackendStateShowsSuperAbbrevInFirstComment(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"
	backend.candidates = []candidateItem{
		{Text: "发"},
		{Text: "法"},
	}

	resp := imecore.NewResponse(3021, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateEntries) < 1 {
		t.Fatalf("expected candidate entries, got %#v", resp.CandidateEntries)
	}
	if resp.CandidateEntries[0].Text != "发" || resp.CandidateEntries[0].Comment != "法" {
		t.Fatalf("expected first candidate comment to show super abbrev, got %#v", resp.CandidateEntries[0])
	}
}

func TestFillResponseFromBackendStateShowsSuperAbbrevOnPrependedCustomPhrase(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n发生\tfa\t10\n")
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"
	backend.candidates = []candidateItem{
		{Text: "发"},
		{Text: "法"},
	}

	resp := imecore.NewResponse(30215, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateEntries) < 1 {
		t.Fatalf("expected candidate entries, got %#v", resp.CandidateEntries)
	}
	if resp.CandidateEntries[0].Text != "发生" || resp.CandidateEntries[0].Comment != "法" {
		t.Fatalf("expected super abbrev comment on prepended custom phrase, got %#v", resp.CandidateEntries[0])
	}
}

func TestFillResponseFromBackendStateInjectsSuperAbbrevCandidateWhenNoBackendCandidate(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"

	resp := imecore.NewResponse(3022, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateEntries) != 1 {
		t.Fatalf("expected synthetic super abbrev candidate, got %#v", resp.CandidateEntries)
	}
	if resp.CandidateEntries[0].Text != "法" || resp.CandidateEntries[0].Comment != superAbbrevCommitMark {
		t.Fatalf("unexpected synthetic super abbrev candidate %#v", resp.CandidateEntries[0])
	}
}

func TestFillResponseFromBackendStateSkipsSuperAbbrevForLuaFilterComposition(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n的\td\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa`d"
	backend.rawInput = "d"
	backend.candidates = []candidateItem{
		{Text: "法"},
		{Text: "珐"},
	}

	resp := imecore.NewResponse(3023, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateEntries) < 1 {
		t.Fatalf("expected backend candidates to remain visible, got %#v", resp.CandidateEntries)
	}
	if resp.CandidateEntries[0].Text != "法" || resp.CandidateEntries[0].Comment != "" {
		t.Fatalf("expected lua filter composition to skip super abbrev overlay, got %#v", resp.CandidateEntries[0])
	}
}

func TestFillResponseFromBackendStateSkipsCustomPhraseOverlayForLuaFilterComposition(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n的\td\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa`d"
	backend.rawInput = "d"
	backend.candidates = []candidateItem{
		{Text: "法"},
		{Text: "珐"},
	}

	resp := imecore.NewResponse(303, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) < 1 {
		t.Fatalf("expected backend candidates to remain visible, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "法" {
		t.Fatalf("expected lua filter candidates to stay ahead of custom phrases, got %#v", resp.CandidateList)
	}
}

func TestSuperAbbrevTabCommitsOverlayText(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"
	backend.candidates = []candidateItem{{Text: "发"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  401,
		KeyCode: vkTab,
	}, imecore.NewResponse(401, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected tab intercepted for super abbrev, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:  402,
		KeyCode: vkTab,
	}, imecore.NewResponse(402, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected tab commit handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "法" {
		t.Fatalf("expected super abbrev committed by tab, got %q", onResp.CommitString)
	}
	if backend.composition != "" {
		t.Fatalf("expected backend composition cleared after super abbrev commit, got %q", backend.composition)
	}
}

func TestSuperAbbrevPeriodCommitsOverlayText(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"
	backend.candidates = []candidateItem{{Text: "发"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   403,
		KeyCode:  vkOemPeriod,
		CharCode: int('.'),
	}, imecore.NewResponse(403, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected period intercepted for super abbrev, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   404,
		KeyCode:  vkOemPeriod,
		CharCode: int('.'),
	}, imecore.NewResponse(404, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected period commit handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "法" {
		t.Fatalf("expected super abbrev committed by period, got %q", onResp.CommitString)
	}
}

func TestSuperAbbrevTabCommitsOverlayTextWhenCustomPhraseIsFirst(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSuperAbbrevCacheForTest()
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\n发生\tfa\t10\n")
	writeTestSuperAbbrevFile(t, appData, "# 超级简拼\n法\tfa\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "fa"
	backend.rawInput = "fa"
	backend.candidates = []candidateItem{{Text: "发"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  405,
		KeyCode: vkTab,
	}, imecore.NewResponse(405, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected tab intercepted for super abbrev with custom phrase first, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:  406,
		KeyCode: vkTab,
	}, imecore.NewResponse(406, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected tab commit handled with custom phrase first, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "法" {
		t.Fatalf("expected super abbrev committed by tab with custom phrase first, got %q", onResp.CommitString)
	}
}

func TestFillResponseFromBackendStateDoesNotShowCustomPhraseAfterPaging(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.rawInput = "a"
	backend.pageNo = 1
	backend.candidates = []candidateItem{
		{Text: "阿"},
		{Text: "啊"},
	}

	resp := imecore.NewResponse(302, true)
	ime.fillResponseFromBackendState(resp, false)

	if len(resp.CandidateList) != 2 {
		t.Fatalf("expected only backend candidates after paging, got %#v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "阿" || resp.CandidateList[1] != "啊" {
		t.Fatalf("expected paged results to exclude custom phrases, got %#v", resp.CandidateList)
	}
}

func TestCustomPhraseSelectionCommitsFirstCustomCandidate(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.candidates = []candidateItem{{Text: "阿"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   31,
		KeyCode:  int('1'),
		CharCode: int('1'),
	}, imecore.NewResponse(31, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected number key intercepted for custom phrase, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   32,
		KeyCode:  int('1'),
		CharCode: int('1'),
	}, imecore.NewResponse(32, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected custom phrase number selection handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "alpha" {
		t.Fatalf("expected custom phrase committed, got %q", onResp.CommitString)
	}
	if backend.composition != "" {
		t.Fatalf("expected backend composition cleared after custom phrase commit, got %q", backend.composition)
	}
}

func TestCustomPhraseOverlayCanSelectBackendCandidateAfterCustomOnes(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.candidates = []candidateItem{{Text: "阿"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   33,
		KeyCode:  int('2'),
		CharCode: int('2'),
	}, imecore.NewResponse(33, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected second number key intercepted, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   34,
		KeyCode:  int('2'),
		CharCode: int('2'),
	}, imecore.NewResponse(34, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected backend overlay candidate selection handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "阿" {
		t.Fatalf("expected backend first candidate committed after custom phrase slot, got %q", onResp.CommitString)
	}
}

func TestCustomPhraseOverlayDoesNotInterceptEnter(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.candidates = []candidateItem{{Text: "阿"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  35,
		KeyCode: vkReturn,
	}, imecore.NewResponse(35, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected enter key handled by backend flow, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:  36,
		KeyCode: vkReturn,
	}, imecore.NewResponse(36, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected enter keydown handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "阿" {
		t.Fatalf("expected enter to commit backend first candidate 阿, got %q", onResp.CommitString)
	}
}

func TestCustomPhraseOverlaySemicolonSelectsSecondVisibleCandidate(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	ime.semicolonSelectSecond = true
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.candidates = []candidateItem{{Text: "阿"}}

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   35,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	}, imecore.NewResponse(35, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon key intercepted for custom phrase overlay, got %d", filterResp.ReturnValue)
	}

	onResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   36,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	}, imecore.NewResponse(36, true))
	if onResp.ReturnValue != 1 {
		t.Fatalf("expected semicolon overlay selection handled, got %d", onResp.ReturnValue)
	}
	if onResp.CommitString != "阿" {
		t.Fatalf("expected semicolon to select second visible candidate 阿, got %q", onResp.CommitString)
	}
}

func TestFillResponseFromBackendStateAppliesCandidateCount(t *testing.T) {
	ime := newIsolatedTestIME(t)
	ime.style.CandidateCount = 5
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{
		{Text: "你", Comment: "pron"},
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
	if len(resp.CandidateEntries) != 5 {
		t.Fatalf("expected 5 candidate entries after truncation, got %#v", resp.CandidateEntries)
	}
	if resp.CandidateEntries[0].Text != "你" || resp.CandidateEntries[0].Comment != "pron" {
		t.Fatalf("expected first candidate entry to preserve comment, got %#v", resp.CandidateEntries[0])
	}
	if resp.SetSelKeys != "12345" {
		t.Fatalf("expected select keys truncated to 12345, got %q", resp.SetSelKeys)
	}
}

func TestHandleRequestCompositionTerminatedPreservesBackendStateWhenNotForced(t *testing.T) {
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
	if backend.composition != "ni" {
		t.Fatalf("expected non-forced termination to preserve backend composition, got %q", backend.composition)
	}
	if len(backend.candidates) == 0 {
		t.Fatal("expected non-forced termination to preserve backend candidates")
	}
}

func TestHandleRequestCompositionTerminatedForcedResetsState(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.refreshCandidates()

	resp := ime.HandleRequest(&imecore.Request{
		SeqNum: 13,
		Method: "onCompositionTerminated",
		Forced: true,
	})

	if !resp.Success {
		t.Fatal("expected forced composition termination response to succeed")
	}
	if backend.composition != "" || backend.candidates != nil {
		t.Fatal("expected forced composition termination to reset backend state")
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

func TestHandleRequestSyncsSharedInputStateAcrossInstances(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	first := newTestIME()
	first.inputStateShared = true
	first.backend.SetOption("ascii_mode", true)
	first.backend.SetOption("ascii_punct", true)
	first.backend.SetOption("traditionalization", true)
	first.backend.SetOption("full_shape", true)
	first.captureSharedInputStateFromBackend()
	first.saveAppearancePrefs()

	second := newTestIME()
	resp := second.HandleRequest(&imecore.Request{
		SeqNum: 16,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})

	if resp.ReturnValue != 1 {
		t.Fatalf("expected onMenu handled, got %d", resp.ReturnValue)
	}
	if !second.inputStateShared {
		t.Fatal("expected second instance to enable shared input state")
	}
	second.createSession(nil)
	if !second.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode synced to enabled")
	}
	if !second.backend.GetOption("ascii_punct") {
		t.Fatal("expected ascii_punct synced to enabled")
	}
	if !second.backend.GetOption("traditionalization") {
		t.Fatal("expected traditionalization synced to enabled")
	}
	if !second.backend.GetOption("full_shape") {
		t.Fatal("expected full_shape synced to enabled")
	}
}

func TestHandleRequestSyncsDynamicSharedOptionsAcrossInstances(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	first := newTestIME()
	first.inputStateShared = true
	firstBackend := first.backend.(*testBackend)
	firstBackend.saveOptions = []string{"emoji", "ascii_mode"}
	firstBackend.schemaSwitches[firstBackend.currentSchemaID] = []RimeSwitch{
		{Name: "emoji", States: []string{"常规", "Emoji"}},
		{Name: "ascii_mode", States: []string{"中文", "西文"}},
	}
	first.backend.SetOption("emoji", true)
	first.backend.SetOption("ascii_mode", true)
	first.captureSharedInputStateFromBackend()
	first.saveAppearancePrefs()

	second := newTestIME()
	secondBackend := second.backend.(*testBackend)
	secondBackend.saveOptions = []string{"emoji", "ascii_mode"}
	secondBackend.schemaSwitches[secondBackend.currentSchemaID] = []RimeSwitch{
		{Name: "emoji", States: []string{"常规", "Emoji"}},
		{Name: "ascii_mode", States: []string{"中文", "西文"}},
	}

	resp := second.HandleRequest(&imecore.Request{
		SeqNum: 17,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})

	if resp.ReturnValue != 1 {
		t.Fatalf("expected onMenu handled, got %d", resp.ReturnValue)
	}
	second.createSession(nil)
	if !second.backend.GetOption("emoji") {
		t.Fatal("expected dynamic emoji option synced to enabled")
	}
	if !second.backend.GetOption("ascii_mode") {
		t.Fatal("expected ascii_mode synced to enabled")
	}
}

func TestProcessKeySyncsSharedInputStateAfterShiftAndCapsToggle(t *testing.T) {
	testCases := []struct {
		name     string
		req      *imecore.Request
		isUp     bool
		expected bool
	}{
		{
			name: "shift",
			req: &imecore.Request{
				KeyCode:   vkShift,
				KeyStates: make(imecore.KeyStates, 256),
			},
			isUp:     true,
			expected: true,
		},
		{
			name: "caps",
			req: &imecore.Request{
				KeyCode:   vkCapital,
				KeyStates: make(imecore.KeyStates, 256),
			},
			isUp:     false,
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APPDATA", t.TempDir())
			resetSharedAppearanceConfigForTest()

			first := newTestIME()
			first.inputStateShared = true
			first.captureSharedInputStateFromBackend()
			first.saveAppearancePrefs()

			first.processKey(tc.req, tc.isUp)
			if got := first.backend.GetOption("ascii_mode"); got != tc.expected {
				t.Fatalf("expected first instance ascii_mode=%t after %s toggle, got %t", tc.expected, tc.name, got)
			}

			second := newTestIME()
			resp := second.HandleRequest(&imecore.Request{
				SeqNum: 18,
				Method: "onMenu",
				ID:     imecore.FlexibleID{String: "settings"},
			})
			if resp.ReturnValue != 1 {
				t.Fatalf("expected onMenu handled, got %d", resp.ReturnValue)
			}
			second.createSession(nil)

			if got := second.backend.GetOption("ascii_mode"); got != tc.expected {
				t.Fatalf("expected second instance ascii_mode=%t after %s shared sync, got %t", tc.expected, tc.name, got)
			}
		})
	}
}

func TestCreateSessionAppliesSharedInputStateOnlyForNewSession(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	ime := newTestIME()
	ime.inputStateShared = true
	ime.sharedOptions = map[string]bool{
		"ascii_mode":  true,
		"full_shape":  true,
		"ascii_punct": true,
	}
	backend := ime.backend.(*testBackend)

	ime.createSession(nil)
	firstApplyCalls := backend.setOptionCalls
	if firstApplyCalls != len(ime.sharedOptions) {
		t.Fatalf("expected first createSession to apply %d shared options, got %d", len(ime.sharedOptions), firstApplyCalls)
	}

	ime.createSession(nil)
	if backend.setOptionCalls != firstApplyCalls {
		t.Fatalf("expected repeated createSession to skip shared apply, got %d total SetOption calls", backend.setOptionCalls)
	}
}

func TestProcessKeySkipsSharedStateSyncForRegularTyping(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	ime := newTestIME()
	ime.inputStateShared = true
	ime.sharedOptions = map[string]bool{
		"ascii_mode": false,
	}
	backend := ime.backend.(*testBackend)

	ime.createSession(nil)
	backend.setOptionCalls = 0
	backend.getOptionCalls = 0

	handled := ime.processKey(&imecore.Request{
		KeyCode:   'N',
		CharCode:  'n',
		KeyStates: make(imecore.KeyStates, 256),
	}, false)
	if !handled {
		t.Fatal("expected regular typing key to be handled")
	}
	if backend.setOptionCalls != 0 {
		t.Fatalf("expected regular typing to avoid shared SetOption calls, got %d", backend.setOptionCalls)
	}
	if backend.getOptionCalls != 0 {
		t.Fatalf("expected regular typing to avoid shared GetOption calls, got %d", backend.getOptionCalls)
	}
}

func TestCreateSessionAppliesSharedInputStateAfterSharedConfigUpdateWithExistingSession(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())
	resetSharedAppearanceConfigForTest()

	second := newTestIME()
	second.inputStateShared = true
	second.createSession(nil)
	secondBackend := second.backend.(*testBackend)
	secondBackend.setOptionCalls = 0

	first := newTestIME()
	first.inputStateShared = true
	first.backend.SetOption("ascii_mode", true)
	first.captureSharedInputStateFromBackend()
	first.saveAppearancePrefs()

	resp := second.HandleRequest(&imecore.Request{
		SeqNum: 19,
		Method: "onMenu",
		ID:     imecore.FlexibleID{String: "settings"},
	})
	if resp.ReturnValue != 1 {
		t.Fatalf("expected onMenu handled, got %d", resp.ReturnValue)
	}
	if !second.sharedInputStateNeedsApply {
		t.Fatal("expected shared input state update to mark session for reapply")
	}

	second.createSession(nil)
	if second.sharedInputStateNeedsApply {
		t.Fatal("expected shared input state apply marker cleared after createSession")
	}
	if !second.backend.GetOption("ascii_mode") {
		t.Fatal("expected existing session to receive updated ascii_mode")
	}
	if secondBackend.setOptionCalls == 0 {
		t.Fatal("expected createSession to reapply shared state for existing session after config update")
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
	if !ime.applyAppearanceCommand(ID_APPEARANCE_COMMENT_FONT_18) {
		t.Fatal("expected comment font size command handled")
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
	ime.autoPairQuotes = true
	ime.semicolonSelectSecond = true
	ime.saveAppearancePrefs()

	configPath := filepath.Join(appData, APP, appearanceConfigFileName)
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
	if got := persisted["candidate_comment_font_point"]; got != float64(18) {
		t.Fatalf("expected persisted candidate_comment_font_point 18, got %#v", got)
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
	if got := persisted["auto_pair_quotes"]; got != true {
		t.Fatalf("expected persisted auto_pair_quotes true, got %#v", got)
	}
	if got := persisted["semicolon_select_second"]; got != true {
		t.Fatalf("expected persisted semicolon_select_second true, got %#v", got)
	}

	resetSharedAppearanceConfigForTest()
	reloaded := newTestIME()
	reloaded.loadAppearancePrefs()

	if reloaded.style.FontPoint != 22 {
		t.Fatalf("expected reloaded font size 22, got %d", reloaded.style.FontPoint)
	}
	if reloaded.style.CandidateCommentFontPoint != 18 {
		t.Fatalf("expected reloaded comment font size 18, got %d", reloaded.style.CandidateCommentFontPoint)
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
	if !reloaded.autoPairQuotes {
		t.Fatal("expected reloaded auto pair quotes enabled")
	}
	if !reloaded.semicolonSelectSecond {
		t.Fatal("expected reloaded semicolon select second enabled")
	}
}

func TestLoadAppearancePrefsCreatesDefaultConfigWhenMissing(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetSharedAppearanceConfigForTest()

	ime := newTestIME()
	ime.loadAppearancePrefs()

	configPath := filepath.Join(appData, APP, appearanceConfigFileName)
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
	if got := persisted["candidate_comment_font_point"]; got != float64(18) {
		t.Fatalf("expected persisted candidate_comment_font_point 18, got %#v", got)
	}
	if got := persisted["candidate_theme"]; got != "default" {
		t.Fatalf("expected persisted candidate_theme default, got %#v", got)
	}
	if got := persisted["input_state_shared"]; got != false {
		t.Fatalf("expected persisted input_state_shared false, got %#v", got)
	}
	if ime.style.CandidatePerRow != 1 {
		t.Fatalf("expected in-memory style to stay vertical by default, got %d", ime.style.CandidatePerRow)
	}
	if ime.inputStateShared {
		t.Fatal("expected shared input state disabled by default")
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
