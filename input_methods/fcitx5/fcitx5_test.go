package fcitx5

import (
	"testing"

	"github.com/gaboolic/moqi-ime/imecore"
)

func newTestIME() *IME {
	return New(&imecore.Client{ID: "test-client"}).(*IME)
}

func TestNewInitialState(t *testing.T) {
	ime := newTestIME()

	if !ime.style.DisplayTrayIcon {
		t.Fatal("expected tray icon style enabled by default")
	}
	if ime.context != 0 {
		t.Fatal("expected simulated mode by default")
	}
}

func TestFilterKeyDownDelegatesToOnKeyDown(t *testing.T) {
	ime := newTestIME()

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   1,
		KeyCode:  0x48,
		CharCode: 'h',
	}, imecore.NewResponse(1, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected h to be handled, got %d", resp.ReturnValue)
	}
	if resp.CompositionString != "ha" {
		t.Fatalf("expected composition ha, got %q", resp.CompositionString)
	}
	if len(resp.CandidateList) != 5 {
		t.Fatalf("expected 5 candidates, got %v", resp.CandidateList)
	}
}

func TestFilterKeyDownFallsBackToKeyCodeWhenCharCodeMissing(t *testing.T) {
	ime := newTestIME()

	resp := ime.filterKeyDown(&imecore.Request{
		SeqNum:  1,
		KeyCode: 0x48,
	}, imecore.NewResponse(1, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected keyCode-only H to be handled, got %d", resp.ReturnValue)
	}
	if resp.CompositionString != "ha" {
		t.Fatalf("expected composition ha from keyCode fallback, got %q", resp.CompositionString)
	}
}

func TestOnKeyDownARequiresExistingComposition(t *testing.T) {
	ime := newTestIME()

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   2,
		KeyCode:  0x41,
		CharCode: 'a',
	}, imecore.NewResponse(2, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected bare a to be ignored, got %d", resp.ReturnValue)
	}
}

func TestOnKeyDownAWithHaCompositionShowsCandidates(t *testing.T) {
	ime := newTestIME()

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:            3,
		KeyCode:           0x41,
		CharCode:          'a',
		CompositionString: "ha",
	}, imecore.NewResponse(3, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected a with ha composition to be handled, got %d", resp.ReturnValue)
	}
	if resp.CompositionString != "ha" {
		t.Fatalf("expected composition ha, got %q", resp.CompositionString)
	}
	if !resp.ShowCandidates {
		t.Fatal("expected candidates to be shown")
	}
	if resp.CandidateList[0] != "哈" {
		t.Fatalf("expected first candidate 哈, got %q", resp.CandidateList[0])
	}
}

func TestOnKeyDownNumberSelectsCandidate(t *testing.T) {
	ime := newTestIME()

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:         4,
		KeyCode:        0x33,
		CandidateList:  []string{"哈", "呵", "喝", "和", "河"},
		ShowCandidates: true,
	}, imecore.NewResponse(4, true))

	if resp.ReturnValue != 1 {
		t.Fatalf("expected number selection to be handled, got %d", resp.ReturnValue)
	}
	if resp.CommitString != "喝" {
		t.Fatalf("expected third candidate 喝, got %q", resp.CommitString)
	}
}

func TestOnKeyDownRealContextFallsBackToZero(t *testing.T) {
	ime := newTestIME()
	ime.context = 1

	resp := ime.onKeyDown(&imecore.Request{
		SeqNum:   5,
		KeyCode:  0x48,
		CharCode: 'h',
	}, imecore.NewResponse(5, true))

	if resp.ReturnValue != 0 {
		t.Fatalf("expected real-context placeholder path to return 0, got %d", resp.ReturnValue)
	}
}

func TestOnCommandHandlesKnownAndMissingCommand(t *testing.T) {
	ime := newTestIME()

	validResp := ime.onCommand(&imecore.Request{
		SeqNum: 6,
		Data: map[string]interface{}{
			"commandId": float64(ID_FULL_SHAPE),
		},
	}, imecore.NewResponse(6, true))
	if validResp.ReturnValue != 1 {
		t.Fatalf("expected known command to be handled, got %d", validResp.ReturnValue)
	}

	missingResp := ime.onCommand(&imecore.Request{
		SeqNum: 7,
	}, imecore.NewResponse(7, true))
	if missingResp.ReturnValue != 0 {
		t.Fatalf("expected missing commandId to be ignored, got %d", missingResp.ReturnValue)
	}
}

func TestHandleRequestOnDeactivateReturnsHandled(t *testing.T) {
	ime := newTestIME()

	resp := ime.HandleRequest(&imecore.Request{
		SeqNum: 8,
		Method: "onDeactivate",
	})

	if resp.ReturnValue != 1 {
		t.Fatalf("expected onDeactivate to return 1, got %d", resp.ReturnValue)
	}
}
