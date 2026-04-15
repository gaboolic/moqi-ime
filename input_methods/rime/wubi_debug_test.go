//go:build windows

package rime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDebugWubiSequences(t *testing.T) {
	userDir := filepath.Clean(`C:\Users\gbl\AppData\Roaming\Moqi\rime-wubi86-jidian`)
	if info, err := os.Stat(userDir); err != nil || !info.IsDir() {
		t.Fatalf("user dir unavailable: %q err=%v", userDir, err)
	}

	dataDirCandidates := []string{
		filepath.Join(`C:\Program Files (x86)\MoqiIM\moqi-ime`, "input_methods", "rime", "data"),
		filepath.Join(`D:\vscode\moqi-input-method-projs\moqi-ime`, "input_methods", "rime", "data"),
	}

	var dataDir string
	for _, candidate := range dataDirCandidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			dataDir = candidate
			break
		}
	}
	if dataDir == "" {
		t.Fatal("usable Rime data directory is required")
	}

	if !RimeInit(dataDir, userDir, APP, APP_VERSION, false) {
		t.Fatal("RimeInit failed")
	}
	defer Finalize()

	sessionID, ok := StartSession()
	if !ok || sessionID == 0 {
		t.Fatal("StartSession failed")
	}
	defer EndSession(sessionID)

	SetOption(sessionID, "ascii_mode", false)
	ClearComposition(sessionID)

	if schema := GetCurrentSchema(sessionID); schema != "" {
		t.Logf("schema=%s", schema)
	}

	for _, input := range []string{"ggtts", "ggttgg", "ggttggtt"} {
		t.Run(input, func(t *testing.T) {
			ClearComposition(sessionID)
			for i, key := range []rune(input) {
				handled := ProcessKey(sessionID, int(key), 0)
				commit, _ := GetCommit(sessionID)
				composition, _ := GetComposition(sessionID)
				menu, _ := GetMenu(sessionID)
				t.Logf("step=%d key=%q handled=%t commit=%q composition=%q candidates=%v", i+1, key, handled, commit.Text, composition.Preedit, menu.Candidates)
			}
		})
	}
}
