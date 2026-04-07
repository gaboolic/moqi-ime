package simplepinyin

import (
	"testing"

	"github.com/gaboolic/moqi-ime/imecore"
)

func newTestIME() *IME {
	return New(&imecore.Client{ID: "test-client"}).(*IME)
}

func TestNewInitialState(t *testing.T) {
	ime := newTestIME()

	if ime.pinyin != "" {
		t.Fatalf("expected empty pinyin, got %q", ime.pinyin)
	}
	if len(ime.candidates) != 0 {
		t.Fatalf("expected no candidates, got %v", ime.candidates)
	}
	if len(ime.dict) == 0 {
		t.Fatal("expected dictionary to be initialized")
	}
}

func TestHandleKeyDownBuildsPinyinAndExactCandidates(t *testing.T) {
	ime := newTestIME()

	for i, char := range []rune{'n', 'i', 'h', 'a', 'o'} {
		resp := ime.handleKeyDown(&imecore.Request{
			SeqNum:   i + 1,
			KeyCode:  int(char - 32),
			CharCode: int(char),
		}, imecore.NewResponse(i+1, true))

		if resp.ReturnValue != 1 {
			t.Fatalf("expected char %q to be handled, got %d", char, resp.ReturnValue)
		}
	}

	if ime.pinyin != "nihao" {
		t.Fatalf("expected pinyin nihao, got %q", ime.pinyin)
	}
	if len(ime.candidates) != 3 {
		t.Fatalf("expected exact candidates for nihao, got %v", ime.candidates)
	}
	if ime.candidates[0] != "你好" {
		t.Fatalf("expected first candidate 你好, got %q", ime.candidates[0])
	}
}

func TestHandleKeyDownUsesFallbackCandidatesForUnknownPinyin(t *testing.T) {
	ime := newTestIME()

	resp := ime.handleKeyDown(&imecore.Request{
		SeqNum:   1,
		KeyCode:  int('a' - 32),
		CharCode: int('a'),
	}, imecore.NewResponse(1, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected unknown pinyin char to be handled, got %d", resp.ReturnValue)
	}
	if resp.CompositionString != "a" {
		t.Fatalf("expected composition a, got %q", resp.CompositionString)
	}
	if len(resp.CandidateList) != 3 {
		t.Fatalf("expected fallback candidates, got %v", resp.CandidateList)
	}
	if resp.CandidateList[0] != "测试" {
		t.Fatalf("expected fallback candidate 测试, got %q", resp.CandidateList[0])
	}
	if !resp.ShowCandidates {
		t.Fatal("expected fallback candidates to be shown")
	}
}

func TestHandleKeyDownFallsBackToKeyCodeWhenCharCodeMissing(t *testing.T) {
	ime := newTestIME()

	resp := ime.handleKeyDown(&imecore.Request{
		SeqNum:  1,
		KeyCode: 0x4E,
	}, imecore.NewResponse(1, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected keyCode-only N to be handled, got %d", resp.ReturnValue)
	}
	if resp.CompositionString != "n" {
		t.Fatalf("expected composition n from keyCode fallback, got %q", resp.CompositionString)
	}
}

func TestHandleKeyDownBackspaceUpdatesComposition(t *testing.T) {
	ime := newTestIME()
	ime.pinyin = "ni"
	ime.candidates = []string{"你", "泥", "尼"}

	resp := ime.handleKeyDown(&imecore.Request{
		SeqNum:  2,
		KeyCode: 0x08,
	}, imecore.NewResponse(2, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected backspace to be handled, got %d", resp.ReturnValue)
	}
	if ime.pinyin != "n" {
		t.Fatalf("expected pinyin n after backspace, got %q", ime.pinyin)
	}
	if resp.CompositionString != "n" {
		t.Fatalf("expected composition n, got %q", resp.CompositionString)
	}
	if len(resp.CandidateList) == 0 {
		t.Fatal("expected fallback candidates after backspace")
	}
}

func TestHandleKeyDownBackspaceClearsSingleRune(t *testing.T) {
	ime := newTestIME()
	ime.pinyin = "a"
	ime.candidates = []string{"测试"}

	resp := ime.handleKeyDown(&imecore.Request{
		SeqNum:  3,
		KeyCode: 0x08,
	}, imecore.NewResponse(3, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected backspace to be handled, got %d", resp.ReturnValue)
	}
	if ime.pinyin != "" {
		t.Fatalf("expected pinyin to clear, got %q", ime.pinyin)
	}
	if resp.CompositionString != "" {
		t.Fatalf("expected empty composition, got %q", resp.CompositionString)
	}
	if resp.ShowCandidates {
		t.Fatal("expected candidate window to close")
	}
}

func TestHandleKeyDownEscapeClearsState(t *testing.T) {
	ime := newTestIME()
	ime.pinyin = "nihao"
	ime.candidates = []string{"你好", "您好"}

	resp := ime.handleKeyDown(&imecore.Request{
		SeqNum:  4,
		KeyCode: 0x1B,
	}, imecore.NewResponse(4, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected escape to be handled, got %d", resp.ReturnValue)
	}
	if ime.pinyin != "" {
		t.Fatalf("expected pinyin to clear, got %q", ime.pinyin)
	}
	if ime.candidates != nil {
		t.Fatalf("expected candidates to clear, got %v", ime.candidates)
	}
	if resp.CompositionString != "" || resp.ShowCandidates {
		t.Fatalf("expected cleared UI, got %#v", resp)
	}
}

func TestHandleKeyDownNumberSelectsCandidate(t *testing.T) {
	ime := newTestIME()
	ime.pinyin = "nihao"
	ime.candidates = []string{"你好", "您好", "你号"}

	resp := ime.handleKeyDown(&imecore.Request{
		SeqNum:  5,
		KeyCode: 0x32,
	}, imecore.NewResponse(5, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected number selection to be handled, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "您好" {
		t.Fatalf("expected second candidate 您好, got %q", resp.CommitString)
	}
	if ime.pinyin != "" || ime.candidates != nil {
		t.Fatal("expected ime state to reset after candidate selection")
	}
	if resp.CompositionString != "" || resp.ShowCandidates {
		t.Fatalf("expected composition and candidates cleared, got %#v", resp)
	}
}

func TestHandleKeyDownEnterCommitsFirstCandidate(t *testing.T) {
	ime := newTestIME()
	ime.pinyin = "nihao"
	ime.candidates = []string{"你好", "您好", "你号"}

	resp := ime.handleKeyDown(&imecore.Request{
		SeqNum:  6,
		KeyCode: 0x0D,
	}, imecore.NewResponse(6, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected enter to be handled, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "你好" {
		t.Fatalf("expected enter to commit first candidate 你好, got %q", resp.CommitString)
	}
	if ime.pinyin != "" || ime.candidates != nil {
		t.Fatal("expected ime state to reset after enter commit")
	}
}

func TestHandleKeyDownEnterCommitsRawPinyinWithoutCandidates(t *testing.T) {
	ime := newTestIME()
	ime.pinyin = "raw"
	ime.candidates = nil

	resp := ime.handleKeyDown(&imecore.Request{
		SeqNum:  7,
		KeyCode: 0x0D,
	}, imecore.NewResponse(7, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected enter to be handled, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "raw" {
		t.Fatalf("expected raw pinyin commit, got %q", resp.CommitString)
	}
}

func TestHandleKeyDownUnhandledKeyReturnsZero(t *testing.T) {
	ime := newTestIME()

	resp := ime.handleKeyDown(&imecore.Request{
		SeqNum:   8,
		KeyCode:  0x70,
		CharCode: 0,
	}, imecore.NewResponse(8, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected unrelated key to be ignored, got %d", resp.ReturnValue)
	}
}

func TestHandleRequestCompositionTerminatedResetsState(t *testing.T) {
	ime := newTestIME()
	ime.pinyin = "nihao"
	ime.candidates = []string{"你好", "您好"}

	resp := ime.HandleRequest(&imecore.Request{
		SeqNum: 9,
		Method: "onCompositionTerminated",
	})

	if !resp.Success {
		t.Fatal("expected composition termination response to succeed")
	}
	if ime.pinyin != "" {
		t.Fatalf("expected pinyin to reset, got %q", ime.pinyin)
	}
	if ime.candidates != nil {
		t.Fatalf("expected candidates to reset, got %v", ime.candidates)
	}
}
