package rime

import (
	"testing"

	"github.com/gaboolic/moqi-ime/imecore"
)

func TestTranslateKeyCodeFallsBackToTopRowDigitKeyCode(t *testing.T) {
	req := &imecore.Request{KeyCode: 0x32}

	got := translateKeyCode(req)

	if got != int('2') {
		t.Fatalf("expected top-row digit fallback to '2', got %d", got)
	}
}

func TestTranslateKeyCodeFallsBackToNumpadDigitKeyCode(t *testing.T) {
	req := &imecore.Request{KeyCode: 0x62}

	got := translateKeyCode(req)

	if got != int('2') {
		t.Fatalf("expected numpad digit fallback to '2', got %d", got)
	}
}
