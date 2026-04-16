//go:build windows && cgo

package rime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

func resolveRealBertDataDir(t *testing.T) string {
	t.Helper()

	candidates := []string{
		filepath.Join(`C:\Program Files (x86)\MoqiIM\moqi-ime`, "input_methods", "rime", "data"),
		filepath.Join(`D:\vscode\moqi-input-method-projs\moqi-ime`, "input_methods", "rime", "data"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	t.Skip("usable Rime data directory is required")
	return ""
}

func resolveRealBertUserDir(t *testing.T) string {
	t.Helper()

	userDir := strings.TrimSpace(os.Getenv("MOQI_REAL_BERT_USER_DIR"))
	if userDir == "" {
		appData := strings.TrimSpace(os.Getenv("APPDATA"))
		if appData == "" {
			t.Skip("APPDATA is not set")
			return ""
		}
		userDir = filepath.Join(appData, APP, "Rime")
	}
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		t.Fatalf("create real BERT user dir %q: %v", userDir, err)
	}
	return userDir
}

func resolveRealBertConfigPath(t *testing.T) string {
	t.Helper()

	candidates := []string{}
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData != "" {
		candidates = append(candidates, filepath.Join(appData, APP, bertConfigFileName))
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if ok {
		candidates = append(candidates, filepath.Clean(filepath.Join(filepath.Dir(thisFile), bertConfigFileName)))
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	t.Skip("usable BERT config file is required")
	return ""
}

func loadRealBertConfigForTest(t *testing.T) *bertRuntimeConfig {
	t.Helper()

	configPath := resolveRealBertConfigPath(t)
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read BERT config %q: %v", configPath, err)
	}
	cfg, err := parseBertConfigJSON(data, filepath.Dir(configPath))
	if err != nil {
		t.Fatalf("parse BERT config %q: %v", configPath, err)
	}
	return cfg
}

func newRealBertIME(t *testing.T) *IME {
	t.Helper()

	if os.Getenv("MOQI_REAL_BERT_SEQUENCE") != "1" {
		t.Skip("set MOQI_REAL_BERT_SEQUENCE=1 to run real BERT sequence test")
	}

	dataDir := resolveRealBertDataDir(t)
	userDir := resolveRealBertUserDir(t)
	cfg := loadRealBertConfigForTest(t)
	if cfg == nil {
		t.Skip("BERT config is missing")
	}
	if missing := bertMissingAssetPaths(cfg); len(missing) > 0 {
		t.Skipf("BERT assets are missing: %v", missing)
	}

	reranker, err := newConfiguredBertReranker(cfg)
	if err != nil {
		t.Fatalf("create BERT reranker: %v", err)
	}

	ime := &IME{
		TextServiceBase:   imecore.NewTextServiceBase(&imecore.Client{ID: "real-bert-sequence"}),
		style:             defaultStyle(),
		bertEnabled:       true,
		bertConfig:        cfg,
		bertReranker:      reranker,
		bertCache:         newBertRerankCache(defaultBertCacheTTL),
		bertSentenceCache: newBertSentenceCandidateCache(defaultBertSentenceCacheTTL),
		bertResultCh:      make(chan bertAsyncResult, 8),
		aiResultCh:        make(chan aiAsyncResult, 4),
	}
	if cfg.CacheTTL > 0 {
		ime.bertCache = newBertRerankCache(cfg.CacheTTL)
		ime.bertSentenceCache = newBertSentenceCandidateCache(cfg.CacheTTL)
	}

	backend := newNativeBackend()
	if backend == nil {
		t.Fatal("native backend is unavailable")
	}
	if !backend.Initialize(dataDir, userDir, false) {
		t.Fatal("native backend initialize failed")
	}
	ime.backend = backend
	ime.createSession(nil)

	t.Cleanup(func() {
		ime.Close()
		Finalize()
	})
	return ime
}

func keyCodeForRune(r rune) int {
	if r >= 'a' && r <= 'z' {
		return int(r - 'a' + 'A')
	}
	return int(r)
}

func logResponseState(t *testing.T, label string, resp *imecore.Response) {
	t.Helper()
	if resp == nil {
		t.Logf("%s: <nil>", label)
		return
	}
	t.Logf("%s: return=%d composition=%q show=%t candidates=%v commit=%q cursor=%d",
		label,
		resp.ReturnValue,
		resp.CompositionString,
		resp.ShowCandidates,
		resp.CandidateList,
		resp.CommitString,
		resp.CandidateCursor,
	)
}

func logBackendState(t *testing.T, ime *IME, label string) {
	t.Helper()
	state, ok := ime.currentVisibleBackendState()
	if !ok {
		t.Logf("%s backend: <unavailable>", label)
		return
	}
	texts := make([]string, 0, len(state.Candidates))
	for _, candidate := range state.Candidates {
		texts = append(texts, strings.TrimSpace(candidate.Text))
	}
	t.Logf("%s backend: composition=%q raw=%q page=%d candidates=%v",
		label,
		state.Composition,
		state.RawInput,
		state.PageNo,
		texts,
	)
}

func drainAsyncBertUpdates(t *testing.T, updates <-chan *imecore.Response, wait time.Duration) {
	t.Helper()
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		select {
		case resp := <-updates:
			logResponseState(t, "async_bert", resp)
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func sendPrintableKey(t *testing.T, ime *IME, updates <-chan *imecore.Response, seq *int, r rune) *imecore.Response {
	t.Helper()
	keyCode := keyCodeForRune(r)
	charCode := int(r)

	filterDown := ime.HandleRequest(&imecore.Request{
		Method:   "filterKeyDown",
		SeqNum:   *seq,
		KeyCode:  keyCode,
		CharCode: charCode,
	})
	logResponseState(t, fmt.Sprintf("filterKeyDown(%c)", r), filterDown)
	*seq = *seq + 1

	down := ime.HandleRequest(&imecore.Request{
		Method:   "onKeyDown",
		SeqNum:   *seq,
		KeyCode:  keyCode,
		CharCode: charCode,
	})
	logResponseState(t, fmt.Sprintf("onKeyDown(%c)", r), down)
	logBackendState(t, ime, fmt.Sprintf("after onKeyDown(%c)", r))
	*seq = *seq + 1

	filterUp := ime.HandleRequest(&imecore.Request{
		Method:   "filterKeyUp",
		SeqNum:   *seq,
		KeyCode:  keyCode,
		CharCode: charCode,
	})
	logResponseState(t, fmt.Sprintf("filterKeyUp(%c)", r), filterUp)
	*seq = *seq + 1

	up := ime.HandleRequest(&imecore.Request{
		Method:   "onKeyUp",
		SeqNum:   *seq,
		KeyCode:  keyCode,
		CharCode: charCode,
	})
	logResponseState(t, fmt.Sprintf("onKeyUp(%c)", r), up)
	logBackendState(t, ime, fmt.Sprintf("after onKeyUp(%c)", r))
	*seq = *seq + 1

	drainAsyncBertUpdates(t, updates, 250*time.Millisecond)
	return up
}

func TestRealBertSequence_gegeguoujijay(t *testing.T) {
	ime := newRealBertIME(t)
	updates := make(chan *imecore.Response, 16)
	ime.SetAsyncResponseSender(func(resp *imecore.Response) {
		updates <- resp
	})

	activateResp := ime.HandleRequest(&imecore.Request{
		Method: "onActivate",
		SeqNum: 1,
	})
	logResponseState(t, "onActivate", activateResp)
	logBackendState(t, ime, "after activate")

	seq := 2
	finalResp := (*imecore.Response)(nil)
	sequence := strings.TrimSpace(os.Getenv("MOQI_REAL_BERT_SEQUENCE_INPUT"))
	if sequence == "" {
		sequence = "gegeguoujijay"
	}
	t.Logf("testing sequence: %q", sequence)
	for _, r := range sequence {
		finalResp = sendPrintableKey(t, ime, updates, &seq, r)
	}

	if finalResp == nil {
		t.Fatal("expected final response")
	}
	state, ok := ime.currentVisibleBackendState()
	if !ok {
		t.Fatal("expected backend state after sequence")
	}

	t.Logf("final immediate response: composition=%q show=%t candidates=%v", finalResp.CompositionString, finalResp.ShowCandidates, finalResp.CandidateList)
	t.Logf("final backend state: composition=%q raw=%q candidates=%d", state.Composition, state.RawInput, len(state.Candidates))

	if strings.TrimSpace(state.Composition) != "" && len(state.Candidates) > 0 && !finalResp.ShowCandidates {
		t.Fatalf("candidate window disappeared unexpectedly: final response hid candidates while backend still had composition=%q candidates=%d",
			state.Composition,
			len(state.Candidates),
		)
	}
}
