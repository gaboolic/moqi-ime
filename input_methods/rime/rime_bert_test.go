package rime

import (
	"context"
	"testing"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

type fakeBertReranker struct {
	rerank func(context.Context, bertRerankRequest) (bertRerankResult, error)
	closeN int
}

func (f *fakeBertReranker) Rerank(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
	if f.rerank == nil {
		return identityBertRerankResult(input.OriginalCandidateCount), nil
	}
	return f.rerank(ctx, input)
}

func (f *fakeBertReranker) Close() error {
	f.closeN++
	return nil
}

func waitForBertAsyncCompletion(t *testing.T, ime *IME) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ime.consumeBertAsyncResult()
		if !ime.bertPending {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for BERT async completion")
}

func testBertConfig() *bertRuntimeConfig {
	return &bertRuntimeConfig{
		Enabled:           true,
		Provider:          bertProviderCrossEncoder,
		MaxSequenceLength: 96,
		MaxCandidates:     5,
		LeftContextRunes:  48,
		CacheTTL:          time.Minute,
	}
}

func seedSentenceRerankState(ime *IME) *testBackend {
	backend := ime.backend.(*testBackend)
	backend.composition = "tazhishiwodemeimei"
	backend.rawInput = "tazhishiwodemeimei"
	backend.candidates = []candidateItem{
		{Text: "他只是"},
		{Text: "沃德"},
		{Text: "她只是我的妹妹"},
		{Text: "他只是我的妹妹"},
		{Text: "妹妹"},
	}
	return backend
}

func TestBertShortInputDoesNotTriggerSentenceRerank(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.rawInput = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "拟"}, {Text: "呢"}}

	called := false
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			called = true
			return identityBertRerankResult(input.OriginalCandidateCount), nil
		},
	}, testBertConfig())
	ime.bertEnabled = true

	resp := imecore.NewResponse(1, true)
	ime.fillResponseFromBackendState(resp, false)
	if called {
		t.Fatal("expected short input to skip BERT rerank")
	}
	if got := resp.CandidateList[0]; got != "你" {
		t.Fatalf("expected original candidates before async result, got %#v", resp.CandidateList)
	}
}

func TestBertSentenceRequestUsesGeneratedWholeSentenceCandidatesOnly(t *testing.T) {
	ime := newIsolatedTestIME(t)
	seedSentenceRerankState(ime)

	started := make(chan bertRerankRequest, 1)
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			started <- cloneBertRequest(input)
			return identityBertRerankResult(input.OriginalCandidateCount), nil
		},
	}, testBertConfig())
	ime.bertEnabled = true

	ime.fillResponseFromBackendState(imecore.NewResponse(1, true), false)
	waitForBertAsyncCompletion(t, ime)

	select {
	case input := <-started:
		if !input.PromoteTopOnly {
			t.Fatal("expected sentence mode to only allow a single promotion")
		}
		if input.OriginalCandidateCount != 5 {
			t.Fatalf("expected original candidate count preserved, got %d", input.OriginalCandidateCount)
		}
		if len(input.Candidates) != 2 {
			t.Fatalf("expected only whole-sentence candidates to be scored, got %#v", input.Candidates)
		}
		if input.Candidates[0].Text != "她只是我的妹妹" || input.Candidates[1].Text != "他只是我的妹妹" {
			t.Fatalf("unexpected sentence candidates: %#v", input.Candidates)
		}
		if len(input.CandidateIndexes) != 2 || input.CandidateIndexes[0] != 2 || input.CandidateIndexes[1] != 3 {
			t.Fatalf("unexpected original candidate indexes: %#v", input.CandidateIndexes)
		}
	default:
		t.Fatal("expected BERT sentence rerank request")
	}
}

func TestBertAsyncPromotesSingleSentenceCandidateOnly(t *testing.T) {
	ime := newIsolatedTestIME(t)
	seedSentenceRerankState(ime)
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			return bertRerankResult{Order: []int{3, 0, 1, 2, 4}}, nil
		},
	}, testBertConfig())
	ime.bertEnabled = true

	ime.fillResponseFromBackendState(imecore.NewResponse(1, true), false)
	waitForBertAsyncCompletion(t, ime)

	showResp := imecore.NewResponse(2, true)
	ime.fillResponseFromCurrentState(showResp)
	if len(showResp.CandidateList) < 5 {
		t.Fatalf("expected updated sentence candidates, got %#v", showResp.CandidateList)
	}
	want := []string{"他只是我的妹妹", "他只是", "沃德", "她只是我的妹妹", "妹妹"}
	for i, text := range want {
		if showResp.CandidateList[i] != text {
			t.Fatalf("expected only one candidate promoted, got %#v", showResp.CandidateList)
		}
	}
}

func TestPromoteSingleBertCandidateRequiresClearLead(t *testing.T) {
	result := promoteSingleBertCandidate([]bertScore{
		{Index: 3, Score: 0.61},
		{Index: 2, Score: 0.55},
	}, 5, bertMinScoreLead)
	if !sameIntSlice(result.Order, identityBertRerankResult(5).Order) {
		t.Fatalf("expected close scores to keep original order, got %#v", result.Order)
	}

	result = promoteSingleBertCandidate([]bertScore{
		{Index: 3, Score: 0.76},
		{Index: 2, Score: 0.51},
	}, 5, bertMinScoreLead)
	want := []int{3, 0, 1, 2, 4}
	if !sameIntSlice(result.Order, want) {
		t.Fatalf("expected a single clear winner promotion, got %#v", result.Order)
	}
}

func TestBertSentenceRerankPreservesCustomPhrasePriority(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ttazhishiwodemeimei\t10\n")

	ime := newIsolatedTestIME(t)
	seedSentenceRerankState(ime)
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			return bertRerankResult{Order: []int{3, 0, 1, 2, 4}}, nil
		},
	}, testBertConfig())
	ime.bertEnabled = true

	ime.fillResponseFromBackendState(imecore.NewResponse(1, true), false)
	waitForBertAsyncCompletion(t, ime)

	showResp := imecore.NewResponse(2, true)
	ime.fillResponseFromCurrentState(showResp)
	if len(showResp.CandidateList) < 4 {
		t.Fatalf("expected custom phrase plus backend candidates, got %#v", showResp.CandidateList)
	}
	if showResp.CandidateList[0] != "alpha" || showResp.CandidateList[1] != "他只是我的妹妹" {
		t.Fatalf("expected custom phrase to stay first while backend keeps single-sentence promotion, got %#v", showResp.CandidateList)
	}
}

func TestBertSentenceGenerationRunsAsynchronously(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := seedSentenceRerankState(ime)

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	backend.bertCandidatesForCodeFunc = func(code string, limit int) []candidateItem {
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		return (&testBackend{}).bertCandidatesForCode(code, limit)
	}

	rerankCalled := make(chan struct{}, 1)
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			rerankCalled <- struct{}{}
			return bertRerankResult{Order: []int{3, 0, 1, 2, 4}}, nil
		},
	}, testBertConfig())
	ime.bertEnabled = true

	resp := imecore.NewResponse(1, true)
	start := time.Now()
	ime.fillResponseFromBackendState(resp, false)
	if elapsed := time.Since(start); elapsed > 40*time.Millisecond {
		t.Fatalf("expected BERT path generation to stay off the request path, took %s", elapsed)
	}
	if resp.CandidateList[0] != "他只是" {
		t.Fatalf("expected original candidates before async work completes, got %#v", resp.CandidateList)
	}

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected async sentence generation to start in background")
	}

	select {
	case <-rerankCalled:
		t.Fatal("expected reranker to wait for sentence generation")
	default:
	}

	close(release)
	waitForBertAsyncCompletion(t, ime)

	select {
	case <-rerankCalled:
	case <-time.After(time.Second):
		t.Fatal("expected reranker to run after sentence generation completes")
	}
}

func TestBertSentenceGenerationCacheAvoidsRepeatedScanForSameInput(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := seedSentenceRerankState(ime)

	scanCalls := 0
	backend.bertCandidatesForCodeFunc = func(code string, limit int) []candidateItem {
		scanCalls++
		return (&testBackend{}).bertCandidatesForCode(code, limit)
	}

	rerankCalls := 0
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			rerankCalls++
			return identityBertRerankResult(input.OriginalCandidateCount), nil
		},
	}, testBertConfig())
	ime.bertEnabled = true

	ime.fillResponseFromBackendState(imecore.NewResponse(1, true), false)
	waitForBertAsyncCompletion(t, ime)
	firstScanCalls := scanCalls
	if firstScanCalls == 0 {
		t.Fatal("expected initial sentence generation scan")
	}

	ime.aiPreviousCommit = "上一句"
	ime.fillResponseFromBackendState(imecore.NewResponse(2, true), false)
	waitForBertAsyncCompletion(t, ime)

	if rerankCalls != 2 {
		t.Fatalf("expected reranker to run again for a new context, got %d calls", rerankCalls)
	}
	if scanCalls != firstScanCalls {
		t.Fatalf("expected sentence cache to reuse generated paths, scan calls before=%d after=%d", firstScanCalls, scanCalls)
	}
}

func TestBertSentenceCacheDoesNotPoisonOnSkippedComputation(t *testing.T) {
	cache := newBertSentenceCandidateCache(time.Minute)
	computeCalls := 0

	got := cache.GetOrCompute("raw-key", func() ([]string, bool) {
		computeCalls++
		return nil, false
	})
	if got != nil {
		t.Fatalf("expected skipped computation to return nil, got %#v", got)
	}

	got = cache.GetOrCompute("raw-key", func() ([]string, bool) {
		computeCalls++
		return []string{"哥哥国家有"}, true
	})
	if len(got) != 1 || got[0] != "哥哥国家有" {
		t.Fatalf("expected second computation to run and cache result, got %#v", got)
	}
	if computeCalls != 2 {
		t.Fatalf("expected skipped result not cached, got computeCalls=%d", computeCalls)
	}
}
