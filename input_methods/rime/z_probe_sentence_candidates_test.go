//go:build windows

package rime

import (
	"os"
	"path/filepath"
	"testing"
)

func copyProbeTree(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("ReadDir(%q) failed: %v", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) failed: %v", dst, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			copyProbeTree(t, srcPath, dstPath)
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("ReadFile(%q) failed: %v", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatalf("WriteFile(%q) failed: %v", dstPath, err)
		}
	}
}

func TestProbeSentenceCandidates(t *testing.T) {
	installedDataDir := filepath.Join(`C:\Program Files (x86)\MoqiIM\moqi-ime`, "input_methods", "rime", "data")
	userDir := filepath.Join(`C:\Users\gbl\AppData\Roaming`, APP, "Rime")
	if info, err := os.Stat(installedDataDir); err != nil || !info.IsDir() {
		t.Fatalf("installed data dir missing: %q", installedDataDir)
	}
	if info, err := os.Stat(userDir); err != nil || !info.IsDir() {
		t.Fatalf("user dir missing: %q", userDir)
	}

	probeRoot, err := os.MkdirTemp("", "moqi-rime-probe-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	probeDataDir := filepath.Join(probeRoot, "data")
	copyProbeTree(t, installedDataDir, probeDataDir)

	repoDLL := filepath.Join(`D:\vscode\moqi-input-method-projs\moqi-ime`, "input_methods", "rime", "rime.dll")
	dllBytes, err := os.ReadFile(repoDLL)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", repoDLL, err)
	}
	if err := os.WriteFile(filepath.Join(probeRoot, "rime.dll"), dllBytes, 0o644); err != nil {
		t.Fatalf("WriteFile(rime.dll) failed: %v", err)
	}

	if !RimeInit(probeDataDir, userDir, APP, APP_VERSION, false) {
		t.Fatal("RimeInit failed")
	}
	defer Finalize()

	sessionID, ok := StartSession()
	if !ok || sessionID == 0 {
		t.Fatal("StartSession failed")
	}
	defer EndSession(sessionID)

	if !SelectSchema(sessionID, "rime_frost") {
		t.Fatal("SelectSchema(rime_frost) failed")
	}
	ClearComposition(sessionID)
	SetOption(sessionID, "ascii_mode", false)

	input := "gegeguojiayougegeguojiadeguoge"
	for _, key := range input {
		if !ProcessKey(sessionID, int(key), 0) {
			t.Fatalf("ProcessKey failed for %q", key)
		}
	}

	t.Logf("schema=%q rawInput=%q", GetCurrentSchema(sessionID), GetInput(sessionID))
	if composition, ok := GetComposition(sessionID); ok {
		t.Logf("composition=%#v", composition)
	}
	menu, ok := GetMenu(sessionID)
	if !ok {
		t.Fatal("GetMenu failed")
	}
	t.Logf("numCandidates=%d pageSize=%d pageNo=%d highlighted=%d", menu.NumCandidates, menu.PageSize, menu.PageNo, menu.HighlightedCandidateIndex)
	for i, candidate := range menu.Candidates {
		t.Logf("candidate[%d]=text:%q comment:%q", i, candidate.Text, candidate.Comment)
	}
}
