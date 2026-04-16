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
		return identityBertRerankResult(len(input.Candidates)), nil
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

func TestBertAsyncRefreshesCandidateOrder(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "拟"}, {Text: "呢"}}

	started := make(chan struct{}, 1)
	release := make(chan struct{})
	updates := make(chan *imecore.Response, 1)
	ime.asyncResponseSender = func(resp *imecore.Response) {
		updates <- resp
	}
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			started <- struct{}{}
			<-release
			return bertRerankResult{Order: []int{1, 0, 2}}, nil
		},
	}, testBertConfig())

	resp := imecore.NewResponse(1, true)
	ime.fillResponseFromBackendState(resp, false)
	if got := resp.CandidateList[0]; got != "你" {
		t.Fatalf("expected original candidates before async result, got %#v", resp.CandidateList)
	}
	<-started
	close(release)

	select {
	case update := <-updates:
		if len(update.CandidateList) < 3 {
			t.Fatalf("expected async update candidates, got %#v", update.CandidateList)
		}
		if update.CandidateList[0] != "拟" || update.CandidateList[1] != "你" {
			t.Fatalf("expected reranked async candidates, got %#v", update.CandidateList)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for async BERT update")
	}
}

func TestBertOverlaySelectionCommitsVisibleTopCandidate(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "拟"}, {Text: "呢"}}
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			return bertRerankResult{Order: []int{1, 0, 2}}, nil
		},
	}, testBertConfig())

	resp := imecore.NewResponse(1, true)
	ime.fillResponseFromBackendState(resp, false)
	waitForBertAsyncCompletion(t, ime)

	filterResp := ime.filterKeyDown(&imecore.Request{
		SeqNum:   2,
		KeyCode:  int('1'),
		CharCode: int('1'),
	}, imecore.NewResponse(2, true))
	if filterResp.ReturnValue != 1 {
		t.Fatalf("expected BERT overlay to intercept selection key, got %#v", filterResp)
	}

	keyResp := ime.onKeyDown(&imecore.Request{
		SeqNum:   3,
		KeyCode:  int('1'),
		CharCode: int('1'),
	}, imecore.NewResponse(3, true))
	if keyResp.CommitString != "拟" {
		t.Fatalf("expected visible top candidate to commit, got %q", keyResp.CommitString)
	}
}

func TestBertOverlayArrowAndSpaceCommitVisibleCursorCandidate(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "拟"}, {Text: "呢"}}
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			return bertRerankResult{Order: []int{1, 0, 2}}, nil
		},
	}, testBertConfig())

	resp := imecore.NewResponse(1, true)
	ime.fillResponseFromBackendState(resp, false)
	waitForBertAsyncCompletion(t, ime)

	downFilter := ime.filterKeyDown(&imecore.Request{SeqNum: 2, KeyCode: vkDown}, imecore.NewResponse(2, true))
	if downFilter.ReturnValue != 1 {
		t.Fatalf("expected down key intercepted by BERT overlay, got %#v", downFilter)
	}
	downResp := ime.onKeyDown(&imecore.Request{SeqNum: 3, KeyCode: vkDown}, imecore.NewResponse(3, true))
	if downResp.CandidateCursor != 1 {
		t.Fatalf("expected cursor to move to second visible candidate, got %#v", downResp.CandidateCursor)
	}

	spaceFilter := ime.filterKeyDown(&imecore.Request{SeqNum: 4, KeyCode: vkSpace}, imecore.NewResponse(4, true))
	if spaceFilter.ReturnValue != 1 {
		t.Fatalf("expected space intercepted by BERT overlay, got %#v", spaceFilter)
	}
	spaceResp := ime.onKeyDown(&imecore.Request{SeqNum: 5, KeyCode: vkSpace}, imecore.NewResponse(5, true))
	if spaceResp.CommitString != "你" {
		t.Fatalf("expected second visible candidate to commit on space, got %q", spaceResp.CommitString)
	}
}

func TestBertAsyncResultDropsStaleResponse(t *testing.T) {
	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	firstStarted := make(chan struct{}, 1)
	secondStarted := make(chan struct{}, 1)
	firstRelease := make(chan struct{})
	secondRelease := make(chan struct{})
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			switch input.Composition {
			case "ni":
				firstStarted <- struct{}{}
				<-firstRelease
				return bertRerankResult{Order: []int{1, 0}}, nil
			case "nihao":
				secondStarted <- struct{}{}
				<-secondRelease
				return bertRerankResult{Order: []int{1, 0}}, nil
			default:
				return identityBertRerankResult(len(input.Candidates)), nil
			}
		},
	}, testBertConfig())

	backend.composition = "ni"
	backend.candidates = []candidateItem{{Text: "你"}, {Text: "拟"}}
	ime.fillResponseFromBackendState(imecore.NewResponse(1, true), false)
	<-firstStarted

	backend.composition = "nihao"
	backend.candidates = []candidateItem{{Text: "你好"}, {Text: "你号"}}
	ime.fillResponseFromBackendState(imecore.NewResponse(2, true), false)
	<-secondStarted

	close(firstRelease)
	time.Sleep(20 * time.Millisecond)
	ime.consumeBertAsyncResult()
	close(secondRelease)
	waitForBertAsyncCompletion(t, ime)

	showResp := imecore.NewResponse(3, true)
	ime.fillResponseFromCurrentState(showResp)
	if len(showResp.CandidateList) < 2 {
		t.Fatalf("expected refreshed candidates after second result, got %#v", showResp.CandidateList)
	}
	if showResp.CandidateList[0] != "你号" || showResp.CandidateList[1] != "你好" {
		t.Fatalf("expected only latest async result to apply, got %#v", showResp.CandidateList)
	}
}

func TestBertRerankPreservesCustomPhrasePriority(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	resetCustomPhraseCacheForTest()
	writeTestCustomPhraseFile(t, appData, "# 置顶短语\nalpha\ta\t10\n")

	ime := newIsolatedTestIME(t)
	backend := ime.backend.(*testBackend)
	backend.composition = "a"
	backend.rawInput = "a"
	backend.candidates = []candidateItem{{Text: "阿"}, {Text: "啊"}}
	ime.SetBertReranker(&fakeBertReranker{
		rerank: func(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
			return bertRerankResult{Order: []int{1, 0}}, nil
		},
	}, testBertConfig())

	ime.fillResponseFromBackendState(imecore.NewResponse(1, true), false)
	waitForBertAsyncCompletion(t, ime)

	showResp := imecore.NewResponse(2, true)
	ime.fillResponseFromCurrentState(showResp)
	if len(showResp.CandidateList) < 3 {
		t.Fatalf("expected custom phrase plus backend candidates, got %#v", showResp.CandidateList)
	}
	if showResp.CandidateList[0] != "alpha" || showResp.CandidateList[1] != "啊" || showResp.CandidateList[2] != "阿" {
		t.Fatalf("expected custom phrase to stay first while backend reranks below it, got %#v", showResp.CandidateList)
	}
}
