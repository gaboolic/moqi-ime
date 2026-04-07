package rime

import (
	"errors"
	"strings"
	"testing"

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

func (b *fakeBackend) EnsureSession() bool {
	return true
}

func (b *fakeBackend) DestroySession() {}

func (b *fakeBackend) ClearComposition() {
	b.clearCompositionN++
}

func (b *fakeBackend) ProcessKey(req *imecore.Request, translatedKeyCode, modifiers int) bool {
	b.processKeyN++
	return false
}

func (b *fakeBackend) State() rimeState {
	return b.state
}

func (b *fakeBackend) SetOption(name string, value bool) {}

func (b *fakeBackend) GetOption(name string) bool {
	return false
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
	ime.SetAIReviewGenerator(func(input aiGenerateRequest) ([]string, error) {
		if input.Composition != "咖啡机" {
			t.Fatalf("unexpected AI composition: %q", input.Composition)
		}
		if len(input.Candidates) != 1 || input.Candidates[0] != "咖啡机" {
			t.Fatalf("unexpected AI candidates: %#v", input.Candidates)
		}
		if !strings.Contains(input.Prompt, "{{composition}}") {
			t.Fatalf("expected configured prompt template, got %q", input.Prompt)
		}
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

	keyResp := ime.HandleRequest(&imecore.Request{
		Method:    "onKeyDown",
		SeqNum:    2,
		KeyCode:   aiHotkeyKeyCode,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})
	if keyResp.CompositionString != "咖啡机" {
		t.Fatalf("expected original composition to stay visible, got %#v", keyResp.CompositionString)
	}
	if keyResp.ShowCandidates != true {
		t.Fatalf("expected AI candidates to be shown, got %#v", keyResp)
	}
	if len(keyResp.CandidateList) != 2 {
		t.Fatalf("expected 2 AI candidates, got %#v", keyResp.CandidateList)
	}
	if keyResp.CandidateList[0] != "做出来的咖啡香气很足，口感顺滑。" {
		t.Fatalf("unexpected first AI candidate: %#v", keyResp.CandidateList[0])
	}
	if keyResp.SetSelKeys != aiSelectKeys {
		t.Fatalf("expected AI select keys %q, got %q", aiSelectKeys, keyResp.SetSelKeys)
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
	if keyResp.ShowCandidates != true || len(keyResp.CandidateList) != 1 {
		t.Fatalf("expected generated AI candidate, got %#v", keyResp)
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
	if keyResp.ShowCandidates != true || len(keyResp.CandidateList) != 1 {
		t.Fatalf("expected generated AI candidate, got %#v", keyResp)
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

	filterResp := ime.HandleRequest(&imecore.Request{
		Method:  "filterKeyUp",
		SeqNum:  3,
		KeyCode: aiHotkeyKeyCode,
	})
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected AI hotkey key-up to be handled, got %#v", filterResp)
	}
	if filterResp.ShowCandidates != true || len(filterResp.CandidateList) != 2 {
		t.Fatalf("expected AI candidates to stay visible on filterKeyUp, got %#v", filterResp)
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
	if keyUpResp.ShowCandidates != true || len(keyUpResp.CandidateList) != 2 {
		t.Fatalf("expected AI candidates to stay visible on onKeyUp, got %#v", keyUpResp)
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

	filterResp := ime.HandleRequest(&imecore.Request{
		Method:  "filterKeyDown",
		SeqNum:  3,
		KeyCode: 0x32,
	})
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected AI candidate selection key to be handled, got %#v", filterResp)
	}

	keyResp := ime.HandleRequest(&imecore.Request{
		Method:  "onKeyDown",
		SeqNum:  4,
		KeyCode: 0x32,
	})
	if keyResp.CommitString != "萃取稳定，清洗也方便，很适合家用。" {
		t.Fatalf("expected second AI candidate to commit, got %#v", keyResp.CommitString)
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
	if keyResp.CompositionString != "咖啡机" {
		t.Fatalf("expected fallback composition to remain visible, got %#v", keyResp.CompositionString)
	}
	if keyResp.ShowCandidates != true {
		t.Fatalf("expected fallback candidates from backend, got %#v", keyResp)
	}
	if len(keyResp.CandidateList) != 2 {
		t.Fatalf("expected fallback candidate list, got %#v", keyResp.CandidateList)
	}
	if ime.aiActive {
		t.Fatal("expected AI mode to remain inactive on failure")
	}
}
