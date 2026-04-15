package rime

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

type fakeBackend struct {
	state             rimeState
	clearCompositionN int
	processKeyN       int
}

func (b *fakeBackend) Initialize(sharedDir, userDir string, firstRun bool) bool {
	return true
}

func (b *fakeBackend) Redeploy(sharedDir, userDir string) bool {
	return true
}

func (b *fakeBackend) SyncUserData() bool {
	return true
}

func (b *fakeBackend) EnsureSession() bool {
	return true
}

func (b *fakeBackend) DestroySession() {}

func (b *fakeBackend) ClearComposition() {
	b.clearCompositionN++
}

func (b *fakeBackend) ProcessKey(req *imecore.Request, translatedKeyCode, modifiers int) bool {
	b.processKeyN++
	if req != nil && req.KeyCode >= '1' && req.KeyCode <= '9' {
		index := req.KeyCode - '1'
		if index >= 0 && index < len(b.state.Candidates) {
			b.state.CommitString = b.state.Candidates[index].Text
			b.state.Composition = ""
			b.state.Candidates = nil
			return true
		}
	}
	return false
}

func (b *fakeBackend) State() rimeState {
	state := b.state
	b.state.CommitString = ""
	return state
}

func (b *fakeBackend) SetOption(name string, value bool) {}

func (b *fakeBackend) GetOption(name string) bool {
	return false
}

func (b *fakeBackend) SaveOptions() []string {
	return nil
}

func (b *fakeBackend) SchemaSwitches() []RimeSwitch {
	return nil
}

func (b *fakeBackend) SchemaList() []RimeSchema {
	return nil
}

func (b *fakeBackend) CurrentSchemaID() string {
	return ""
}

func (b *fakeBackend) SelectSchema(schemaID string) bool {
	return false
}

func (b *fakeBackend) SetCandidatePageSize(pageSize int) bool {
	return false
}

func (b *fakeBackend) SelectCandidate(index int) bool {
	if index < 0 || index >= len(b.state.Candidates) {
		return false
	}
	b.state.CommitString = b.state.Candidates[index].Text
	b.state.Composition = ""
	b.state.Candidates = nil
	return true
}

func newTestIMEWithBackend(backend rimeBackend) *IME {
	ime := New(&imecore.Client{}).(*IME)
	ime.backend = backend
	return ime
}

func keyStatesWithDown(keys ...int) imecore.KeyStates {
	states := make(imecore.KeyStates, 256)
	for _, key := range keys {
		states[key] = 1 << 7
	}
	return states
}

func waitForAIAsyncCompletion(t *testing.T, ime *IME) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ime.consumeAIAsyncResult(nil)
		if !ime.aiPending {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for AI async completion")
}

func TestAIHotkeyShowsGeneratedCandidates(t *testing.T) {
	backend := &fakeBackend{
		state: rimeState{
			Composition: "咖啡机",
			Candidates: []candidateItem{
				{Text: "咖啡机"},
			},
		},
	}
	ime := newTestIMEWithBackend(backend)
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	ime.SetAIReviewGenerator(func(input aiGenerateRequest) ([]string, error) {
		if input.Composition != "咖啡机" {
			t.Fatalf("unexpected AI composition: %q", input.Composition)
		}
		if len(input.Candidates) != 1 || input.Candidates[0] != "咖啡机" {
			t.Fatalf("unexpected AI candidates: %#v", input.Candidates)
		}
		if !strings.Contains(input.Prompt, "{{candidate_1}}") {
			t.Fatalf("expected configured prompt template, got %q", input.Prompt)
		}
		started <- struct{}{}
		<-release
		return []string{
			"做出来的咖啡香气很足，口感顺滑。",
			"操作简单，早上出门前也能快速来一杯。",
		}, nil
	})

	filterResp := ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected AI hotkey to be handled, got %#v", filterResp)
	}
	if backend.processKeyN != 0 {
		t.Fatalf("expected AI hotkey to bypass backend.ProcessKey, got %d calls", backend.processKeyN)
	}
	<-started

	keyResp := ime.HandleRequest(&imecore.Request{
		Method:    "onKeyDown",
		SeqNum:    2,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	if keyResp.CompositionString != "咖啡机" {
		t.Fatalf("expected original composition to stay visible, got %#v", keyResp.CompositionString)
	}
	if keyResp.ShowCandidates != true || len(keyResp.CandidateList) != 1 || keyResp.CandidateList[0] != "咖啡机" {
		t.Fatalf("expected pending AI request to keep backend candidates visible, got %#v", keyResp)
	}
	if ime.aiActive {
		t.Fatal("expected AI overlay to remain inactive while request is pending")
	}

	filterKeyUpResp := ime.HandleRequest(&imecore.Request{
		Method:  "filterKeyUp",
		SeqNum:  3,
		KeyCode: aiHotkeyKeyCode,
	})
	if filterKeyUpResp.ReturnValue != 1 {
		t.Fatalf("expected pending AI key-up filter to be handled, got %#v", filterKeyUpResp)
	}
	if filterKeyUpResp.CompositionString != "咖啡机" || !filterKeyUpResp.ShowCandidates || len(filterKeyUpResp.CandidateList) != 1 {
		t.Fatalf("expected pending AI key-up filter to preserve backend candidates, got %#v", filterKeyUpResp)
	}

	keyUpResp := ime.HandleRequest(&imecore.Request{
		Method:  "onKeyUp",
		SeqNum:  4,
		KeyCode: aiHotkeyKeyCode,
	})
	if keyUpResp.ReturnValue != 1 {
		t.Fatalf("expected pending AI key-up to be handled, got %#v", keyUpResp)
	}
	if keyUpResp.CompositionString != "咖啡机" || !keyUpResp.ShowCandidates || len(keyUpResp.CandidateList) != 1 {
		t.Fatalf("expected pending AI key-up to preserve backend candidates, got %#v", keyUpResp)
	}

	close(release)
	waitForAIAsyncCompletion(t, ime)

	showResp := imecore.NewResponse(5, true)
	ime.fillAIResponse(showResp)
	if showResp.CompositionString != "咖啡机" {
		t.Fatalf("expected original composition to stay visible, got %#v", showResp.CompositionString)
	}
	if !showResp.ShowCandidates {
		t.Fatalf("expected merged candidates to be shown, got %#v", showResp)
	}
	if len(showResp.CandidateList) != 2 {
		t.Fatalf("expected AI candidates to be prepended, got %#v", showResp.CandidateList)
	}
	if showResp.CandidateList[0] != "做出来的咖啡香气很足，口感顺滑。" || showResp.CandidateList[1] != "咖啡机" {
		t.Fatalf("unexpected merged AI candidates: %#v", showResp.CandidateList)
	}
	if showResp.SetSelKeys != "12" {
		t.Fatalf("expected merged select keys %q, got %q", "12", showResp.SetSelKeys)
	}
}

func TestAIHotkeyPassesTopThreeCandidatesToGenerator(t *testing.T) {
	backend := &fakeBackend{
		state: rimeState{
			Composition: "kafeiji",
			Candidates: []candidateItem{
				{Text: "咖啡机"},
				{Text: "咖啡壶"},
				{Text: "咖啡杯"},
				{Text: "咖啡豆"},
			},
		},
	}
	ime := newTestIMEWithBackend(backend)
	ime.SetAIReviewGenerator(func(input aiGenerateRequest) ([]string, error) {
		if input.Composition != "kafeiji" {
			t.Fatalf("unexpected composition: %q", input.Composition)
		}
		if len(input.Candidates) != 3 {
			t.Fatalf("expected top 3 candidates, got %#v", input.Candidates)
		}
		if input.Candidates[0] != "咖啡机" || input.Candidates[1] != "咖啡壶" || input.Candidates[2] != "咖啡杯" {
			t.Fatalf("unexpected top candidates: %#v", input.Candidates)
		}
		return []string{"咖啡机很好用，体验很顺手。"}, nil
	})

	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	keyResp := ime.HandleRequest(&imecore.Request{
		Method:    "onKeyDown",
		SeqNum:    2,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	if !keyResp.ShowCandidates {
		t.Fatalf("expected backend candidates while AI is pending, got %#v", keyResp)
	}
	waitForAIAsyncCompletion(t, ime)
	showResp := imecore.NewResponse(3, true)
	ime.fillAIResponse(showResp)
	if showResp.ShowCandidates != true || len(showResp.CandidateList) != 5 {
		t.Fatalf("expected generated AI candidate prepended to backend list, got %#v", showResp)
	}
	if showResp.CandidateList[0] != "咖啡机很好用，体验很顺手。" {
		t.Fatalf("expected AI candidate first, got %#v", showResp.CandidateList)
	}
}

func TestAIHotkeyPassesPreviousCommitToGenerator(t *testing.T) {
	backend := &fakeBackend{
		state: rimeState{
			Composition: "tangkongchuanshengranghaizitizhewan",
			Candidates: []candidateItem{
				{Text: "糖空传声让孩子提着万"},
				{Text: "糖空传声让孩子提着碗"},
			},
		},
	}
	ime := newTestIMEWithBackend(backend)
	ime.aiPreviousCommit = "人们在春节和元宵节期间也制做冰灯摆在门前"
	ime.SetAIReviewGenerator(func(input aiGenerateRequest) ([]string, error) {
		if input.PreviousCommit != "人们在春节和元宵节期间也制做冰灯摆在门前" {
			t.Fatalf("unexpected previous commit: %q", input.PreviousCommit)
		}
		return []string{"糖空传声让孩子提着碗"}, nil
	})

	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	keyResp := ime.HandleRequest(&imecore.Request{
		Method:    "onKeyDown",
		SeqNum:    2,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	if !keyResp.ShowCandidates {
		t.Fatalf("expected backend candidates while AI is pending, got %#v", keyResp)
	}
	waitForAIAsyncCompletion(t, ime)
	showResp := imecore.NewResponse(3, true)
	ime.fillAIResponse(showResp)
	if showResp.ShowCandidates != true || len(showResp.CandidateList) != 3 {
		t.Fatalf("expected generated AI candidate prepended to backend list, got %#v", showResp)
	}
	if showResp.CandidateList[0] != "糖空传声让孩子提着碗" {
		t.Fatalf("expected AI candidate first, got %#v", showResp.CandidateList)
	}
}

func TestAIHotkeyKeyUpKeepsGeneratedCandidatesVisible(t *testing.T) {
	backend := &fakeBackend{
		state: rimeState{
			Composition: "咖啡机",
		},
	}
	ime := newTestIMEWithBackend(backend)
	ime.SetAIReviewGenerator(func(input aiGenerateRequest) ([]string, error) {
		return []string{
			"操作简单，打出来的咖啡很香。",
			"清洗方便，早上用着很省心。",
		}, nil
	})

	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	ime.HandleRequest(&imecore.Request{
		Method:    "onKeyDown",
		SeqNum:    2,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	waitForAIAsyncCompletion(t, ime)

	filterResp := ime.HandleRequest(&imecore.Request{
		Method:  "filterKeyUp",
		SeqNum:  3,
		KeyCode: aiHotkeyKeyCode,
	})
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected AI hotkey key-up to be handled, got %#v", filterResp)
	}
	if filterResp.ShowCandidates != true || len(filterResp.CandidateList) != 1 {
		t.Fatalf("expected AI candidates to stay visible on filterKeyUp, got %#v", filterResp)
	}
	if filterResp.CandidateList[0] != "操作简单，打出来的咖啡很香。" {
		t.Fatalf("expected AI candidate to stay first on filterKeyUp, got %#v", filterResp.CandidateList)
	}
	if ime.aiConsumeKeyUpCode != aiHotkeyKeyCode {
		t.Fatalf("expected key-up code to stay pending until onKeyUp, got %d", ime.aiConsumeKeyUpCode)
	}

	keyUpResp := ime.HandleRequest(&imecore.Request{
		Method:  "onKeyUp",
		SeqNum:  4,
		KeyCode: aiHotkeyKeyCode,
	})
	if keyUpResp.ReturnValue != 1 {
		t.Fatalf("expected AI hotkey onKeyUp to be handled, got %#v", keyUpResp)
	}
	if keyUpResp.ShowCandidates != true || len(keyUpResp.CandidateList) != 1 {
		t.Fatalf("expected AI candidates to stay visible on onKeyUp, got %#v", keyUpResp)
	}
	if keyUpResp.CandidateList[0] != "操作简单，打出来的咖啡很香。" {
		t.Fatalf("expected AI candidate to stay first on onKeyUp, got %#v", keyUpResp.CandidateList)
	}
	if ime.aiConsumeKeyUpCode != 0 {
		t.Fatalf("expected key-up code to be cleared after onKeyUp, got %d", ime.aiConsumeKeyUpCode)
	}
}

func TestAISelectionCommitsCandidateAndClearsComposition(t *testing.T) {
	backend := &fakeBackend{
		state: rimeState{Composition: "咖啡机"},
	}
	ime := newTestIMEWithBackend(backend)
	ime.SetAIReviewGenerator(func(input aiGenerateRequest) ([]string, error) {
		return []string{
			"香味浓郁，整体体验超出预期。",
			"萃取稳定，清洗也方便，很适合家用。",
		}, nil
	})

	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	ime.HandleRequest(&imecore.Request{
		Method:    "onKeyDown",
		SeqNum:    2,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	waitForAIAsyncCompletion(t, ime)

	filterResp := ime.HandleRequest(&imecore.Request{
		Method:  "filterKeyDown",
		SeqNum:  3,
		KeyCode: 0x31,
	})
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected AI candidate selection key to be handled, got %#v", filterResp)
	}

	keyResp := ime.HandleRequest(&imecore.Request{
		Method:  "onKeyDown",
		SeqNum:  4,
		KeyCode: 0x31,
	})
	if keyResp.CommitString != "香味浓郁，整体体验超出预期。" {
		t.Fatalf("expected first AI candidate to commit, got %#v", keyResp.CommitString)
	}
	if keyResp.ShowCandidates != false {
		t.Fatalf("expected AI candidate window to close, got %#v", keyResp)
	}
	if backend.clearCompositionN != 1 {
		t.Fatalf("expected backend composition to be cleared once, got %d", backend.clearCompositionN)
	}
	if ime.aiActive {
		t.Fatal("expected AI mode to end after commit")
	}
}

func TestAISpaceCommitsCurrentCandidateAndClearsComposition(t *testing.T) {
	backend := &fakeBackend{
		state: rimeState{Composition: "咖啡机"},
	}
	ime := newTestIMEWithBackend(backend)
	ime.SetAIReviewGenerator(func(input aiGenerateRequest) ([]string, error) {
		return []string{
			"第一条默认候选。",
			"第二条候选。",
		}, nil
	})

	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	ime.HandleRequest(&imecore.Request{
		Method:    "onKeyDown",
		SeqNum:    2,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	waitForAIAsyncCompletion(t, ime)

	filterResp := ime.HandleRequest(&imecore.Request{
		Method:  "filterKeyDown",
		SeqNum:  3,
		KeyCode: vkSpace,
	})
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected AI space key to be handled, got %#v", filterResp)
	}

	keyResp := ime.HandleRequest(&imecore.Request{
		Method:  "onKeyDown",
		SeqNum:  4,
		KeyCode: vkSpace,
	})
	if keyResp.CommitString != "第一条默认候选。" {
		t.Fatalf("expected first AI candidate to commit on space, got %#v", keyResp.CommitString)
	}
	if keyResp.ShowCandidates != false {
		t.Fatalf("expected AI candidate window to close after space commit, got %#v", keyResp)
	}
	if backend.clearCompositionN != 1 {
		t.Fatalf("expected backend composition to be cleared once, got %d", backend.clearCompositionN)
	}
	if ime.aiActive {
		t.Fatal("expected AI mode to end after space commit")
	}
}

func TestAIOverlayShiftsBackendCandidatesAndAllowsSelectingThem(t *testing.T) {
	backend := &fakeBackend{
		state: rimeState{
			Composition: "咖啡机",
			Candidates: []candidateItem{
				{Text: "原候选一"},
				{Text: "原候选二"},
			},
		},
	}
	ime := newTestIMEWithBackend(backend)
	ime.semicolonSelectSecond = true
	ime.SetAIReviewGenerator(func(input aiGenerateRequest) ([]string, error) {
		return []string{"AI 置顶候选"}, nil
	})

	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	ime.HandleRequest(&imecore.Request{
		Method:    "onKeyDown",
		SeqNum:    2,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	waitForAIAsyncCompletion(t, ime)

	showResp := imecore.NewResponse(3, true)
	ime.fillAIResponse(showResp)
	if len(showResp.CandidateList) != 3 {
		t.Fatalf("expected AI candidate plus shifted backend candidates, got %#v", showResp.CandidateList)
	}
	if showResp.CandidateList[0] != "AI 置顶候选" || showResp.CandidateList[1] != "原候选一" || showResp.CandidateList[2] != "原候选二" {
		t.Fatalf("unexpected shifted candidate order: %#v", showResp.CandidateList)
	}

	filterResp := ime.HandleRequest(&imecore.Request{
		Method:  "filterKeyDown",
		SeqNum:  4,
		KeyCode: '2',
	})
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected shifted backend candidate selection key to be handled, got %#v", filterResp)
	}

	keyResp := ime.HandleRequest(&imecore.Request{
		Method:  "onKeyDown",
		SeqNum:  5,
		KeyCode: '2',
	})
	if keyResp.CommitString != "原候选一" {
		t.Fatalf("expected shifted backend candidate to commit original first candidate, got %#v", keyResp.CommitString)
	}
	if ime.aiActive {
		t.Fatal("expected AI overlay to end after selecting backend candidate")
	}
}

func TestAIOverlaySemicolonSelectsSecondVisibleCandidate(t *testing.T) {
	backend := &fakeBackend{
		state: rimeState{
			Composition: "咖啡机",
			Candidates: []candidateItem{
				{Text: "原候选一"},
				{Text: "原候选二"},
			},
		},
	}
	ime := newTestIMEWithBackend(backend)
	ime.semicolonSelectSecond = true
	ime.SetAIReviewGenerator(func(input aiGenerateRequest) ([]string, error) {
		return []string{"AI 置顶候选"}, nil
	})

	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	ime.HandleRequest(&imecore.Request{
		Method:    "onKeyDown",
		SeqNum:    2,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	waitForAIAsyncCompletion(t, ime)

	filterResp := ime.HandleRequest(&imecore.Request{
		Method:   "filterKeyDown",
		SeqNum:   4,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	})
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected AI semicolon selection key to be handled, got %#v", filterResp)
	}

	keyResp := ime.HandleRequest(&imecore.Request{
		Method:   "onKeyDown",
		SeqNum:   5,
		KeyCode:  vkOem1,
		CharCode: int(';'),
	})
	if keyResp.CommitString != "原候选一" {
		t.Fatalf("expected semicolon to commit second visible candidate 原候选一, got %#v", keyResp.CommitString)
	}
	if ime.aiActive {
		t.Fatal("expected AI overlay to end after semicolon selection")
	}
}

func TestAIHotkeyFailureFallsBackToOriginalComposition(t *testing.T) {
	backend := &fakeBackend{
		state: rimeState{
			Composition: "咖啡机",
			Candidates: []candidateItem{
				{Text: "咖啡机"},
				{Text: "咖啡壶"},
			},
			SelectKeys: "1234",
		},
	}
	ime := newTestIMEWithBackend(backend)
	ime.SetAIReviewGenerator(func(input aiGenerateRequest) ([]string, error) {
		return nil, errors.New("mock AI failure")
	})

	filterResp := ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected failed AI hotkey to still be consumed, got %#v", filterResp)
	}

	keyResp := ime.HandleRequest(&imecore.Request{
		Method:    "onKeyDown",
		SeqNum:    2,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	if keyResp.CompositionString != "咖啡机" || !keyResp.ShowCandidates || len(keyResp.CandidateList) != 2 {
		t.Fatalf("expected backend candidates to remain visible while AI is pending, got %#v", keyResp)
	}
	waitForAIAsyncCompletion(t, ime)
	keyUpResp := ime.HandleRequest(&imecore.Request{
		Method:  "onKeyUp",
		SeqNum:  3,
		KeyCode: aiHotkeyKeyCode,
	})
	if keyUpResp.CompositionString != "咖啡机" {
		t.Fatalf("expected fallback composition to remain visible, got %#v", keyUpResp.CompositionString)
	}
	if !keyUpResp.ShowCandidates || len(keyUpResp.CandidateList) != 2 {
		t.Fatalf("expected fallback candidate list, got %#v", keyUpResp.CandidateList)
	}
	if ime.aiActive {
		t.Fatal("expected AI mode to remain inactive on failure")
	}
}
