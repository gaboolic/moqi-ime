//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gaboolic/moqi-ime/input_methods/rime"
)

func resolveRealRimeDataDir(t *testing.T) string {
	t.Helper()

	candidates := []string{
		filepath.Join(`C:\Program Files (x86)\MoqiIM\moqi-ime`, "input_methods", "rime", "data"),
	}

	wd, err := os.Getwd()
	if err == nil {
		candidates = append(candidates, filepath.Clean(filepath.Join(wd, "..", "data")))
	}

	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}

	t.Skip("usable Rime data directory is required")
	return ""
}

func resolveRealRimeUserDir(t *testing.T) string {
	t.Helper()

	appData := os.Getenv("APPDATA")
	if appData == "" {
		t.Skip("APPDATA is not set")
	}

	userDir := filepath.Join(appData, "Moqi", "Rime")
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatalf("create user dir %q: %v", userDir, err)
	}
	return userDir
}

type fileSnapshot struct {
	ModTime time.Time
	Size    int64
}

func snapshotBuildDir(t *testing.T, userDir string) map[string]fileSnapshot {
	t.Helper()

	buildDir := filepath.Join(userDir, "build")
	snapshot := make(map[string]fileSnapshot)
	entries, err := os.ReadDir(buildDir)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot
		}
		t.Fatalf("read build dir %q: %v", buildDir, err)
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatalf("stat build entry %q: %v", entry.Name(), err)
		}
		snapshot[entry.Name()] = fileSnapshot{
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}
	}
	return snapshot
}

func snapshotConfigFiles(userDir string) map[string]fileSnapshot {
	names := []string{"Moqi.yaml", "default.yaml", "installation.yaml"}
	snapshot := make(map[string]fileSnapshot, len(names))
	for _, name := range names {
		path := filepath.Join(userDir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		snapshot[name] = fileSnapshot{
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}
	}
	return snapshot
}

func formatSnapshotDiff(before, after map[string]fileSnapshot) []string {
	seen := make(map[string]struct{}, len(before)+len(after))
	for name := range before {
		seen[name] = struct{}{}
	}
	for name := range after {
		seen[name] = struct{}{}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	diff := make([]string, 0)
	for _, name := range names {
		beforeInfo, beforeOK := before[name]
		afterInfo, afterOK := after[name]
		switch {
		case !beforeOK && afterOK:
			diff = append(diff, fmt.Sprintf("%s created mod=%s size=%d", name, afterInfo.ModTime.Format(time.RFC3339Nano), afterInfo.Size))
		case beforeOK && !afterOK:
			diff = append(diff, fmt.Sprintf("%s removed", name))
		case beforeOK && afterOK && (!beforeInfo.ModTime.Equal(afterInfo.ModTime) || beforeInfo.Size != afterInfo.Size):
			diff = append(diff, fmt.Sprintf("%s changed mod=%s -> %s size=%d -> %d",
				name,
				beforeInfo.ModTime.Format(time.RFC3339Nano),
				afterInfo.ModTime.Format(time.RFC3339Nano),
				beforeInfo.Size,
				afterInfo.Size,
			))
		}
	}
	return diff
}

func TestRealRimeInitDuration(t *testing.T) {
	dataDir := resolveRealRimeDataDir(t)
	userDir := resolveRealRimeUserDir(t)
	beforeBuild := snapshotBuildDir(t, userDir)
	beforeConfigs := snapshotConfigFiles(userDir)

	start := time.Now()
	if !rime.RimeInit(dataDir, userDir, "Moqi", "0.01", false) {
		t.Fatal("RimeInit failed")
	}
	elapsed := time.Since(start)
	t.Cleanup(rime.Finalize)
	afterBuild := snapshotBuildDir(t, userDir)
	afterConfigs := snapshotConfigFiles(userDir)

	t.Logf("RimeInit(fullcheck=false) took %s using dataDir=%q userDir=%q", elapsed, dataDir, userDir)
	for _, line := range formatSnapshotDiff(beforeBuild, afterBuild) {
		t.Logf("build snapshot: %s", line)
	}
	for _, line := range formatSnapshotDiff(beforeConfigs, afterConfigs) {
		t.Logf("config snapshot: %s", line)
	}

	maxMillisText := strings.TrimSpace(os.Getenv("MOQI_RIME_INIT_MAX_MS"))
	if maxMillisText == "" {
		return
	}

	maxMillis, err := strconv.Atoi(maxMillisText)
	if err != nil {
		t.Fatalf("invalid MOQI_RIME_INIT_MAX_MS value %q: %v", maxMillisText, err)
	}

	maxDuration := time.Duration(maxMillis) * time.Millisecond
	if elapsed > maxDuration {
		t.Fatalf("RimeInit(fullcheck=false) took %s, exceeded limit %s", elapsed, maxDuration)
	}
}
