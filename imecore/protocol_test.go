package imecore

import (
	"testing"

	moqipb "github.com/gaboolic/moqi-ime/proto"
)

func TestParseProtoRequestMapsKeyStatesAndIDs(t *testing.T) {
	commandID := uint32(3)
	buttonID := "settings"
	req := ParseProtoRequest(&moqipb.ClientRequest{
		Method:    moqipb.Method_METHOD_ON_COMMAND,
		SeqNum:    1,
		CommandId: &commandID,
		ButtonId:  &buttonID,
		KeyEvent: &moqipb.KeyEvent{
			KeyStates: []uint32{0, 1, 0, 2},
		},
	})

	want := []int{0, 1, 0, 2}
	if len(req.KeyStates) != len(want) {
		t.Fatalf("expected %d key states, got %d", len(want), len(req.KeyStates))
	}
	for i, expected := range want {
		if req.KeyStates[i] != expected {
			t.Fatalf("expected keyStates[%d]=%d, got %d", i, expected, req.KeyStates[i])
		}
	}
	if got := req.ID.IntValue(); got != 3 {
		t.Fatalf("expected numeric id 3, got %d", got)
	}
	if got, _ := req.Data["buttonId"].(string); got != "settings" {
		t.Fatalf("expected buttonId settings, got %#v", req.Data["buttonId"])
	}
}

func TestParseProtoRequestAcceptsGuidStringID(t *testing.T) {
	guid := "{guid}"
	req := ParseProtoRequest(&moqipb.ClientRequest{
		Method: moqipb.Method_METHOD_INIT,
		SeqNum: 1,
		Guid:   &guid,
	})

	if got := req.ID.StringValue(); got != "{guid}" {
		t.Fatalf("expected string id {guid}, got %q", got)
	}
}

func TestBuildProtoResponseIncludesClearedCompositionState(t *testing.T) {
	resp := NewResponse(1, true)
	resp.ReturnValue = 1

	msg, err := BuildProtoResponse("client-1", resp)
	if err != nil {
		t.Fatalf("BuildProtoResponse failed: %v", err)
	}

	if msg.GetCompositionString() != "" {
		t.Fatalf("expected empty compositionString, got %q", msg.GetCompositionString())
	}
	if msg.GetShowCandidates() {
		t.Fatalf("expected showCandidates=false, got true")
	}
	if len(msg.GetCandidateList()) != 0 {
		t.Fatalf("expected empty candidateList, got %#v", msg.GetCandidateList())
	}
}

func TestBuildProtoResponseIncludesCustomizeUIBooleans(t *testing.T) {
	resp := NewResponse(1, true)
	resp.CustomizeUI = map[string]interface{}{
		"autoPairQuotes":        true,
		"semicolonSelectSecond": true,
	}

	msg, err := BuildProtoResponse("client-1", resp)
	if err != nil {
		t.Fatalf("BuildProtoResponse failed: %v", err)
	}

	if msg.GetCustomizeUi() == nil {
		t.Fatal("expected customize_ui to be present")
	}
	if got := msg.GetCustomizeUi().GetAutoPairQuotes(); !got {
		t.Fatalf("expected autoPairQuotes=true, got %v", got)
	}
	if got := msg.GetCustomizeUi().GetSemicolonSelectSecond(); !got {
		t.Fatalf("expected semicolonSelectSecond=true, got %v", got)
	}
}
