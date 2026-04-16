package rime

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

type bertRerankRequest struct {
	PreviousCommit string
	Composition    string
	RawInput       string
	Candidates     []candidateItem
}

type bertScore struct {
	Index int
	Text  string
	Score float64
}

type bertRerankResult struct {
	Order  []int
	Scores []bertScore
}

type bertAsyncResult struct {
	RequestSeq uint64
	Key        string
	Result     bertRerankResult
	Err        error
}

type bertReranker interface {
	Rerank(context.Context, bertRerankRequest) (bertRerankResult, error)
	Close() error
}

func cloneBertRequest(input bertRerankRequest) bertRerankRequest {
	cloned := bertRerankRequest{
		PreviousCommit: strings.TrimSpace(input.PreviousCommit),
		Composition:    strings.TrimSpace(input.Composition),
		RawInput:       strings.TrimSpace(input.RawInput),
	}
	if len(input.Candidates) > 0 {
		cloned.Candidates = append([]candidateItem(nil), input.Candidates...)
	}
	return cloned
}

func cloneBertRerankResult(result bertRerankResult) bertRerankResult {
	cloned := bertRerankResult{}
	if len(result.Order) > 0 {
		cloned.Order = append([]int(nil), result.Order...)
	}
	if len(result.Scores) > 0 {
		cloned.Scores = append([]bertScore(nil), result.Scores...)
	}
	return cloned
}

func normalizeBertRerankRequest(input bertRerankRequest, maxCandidates int) bertRerankRequest {
	input = cloneBertRequest(input)
	if maxCandidates > 0 && len(input.Candidates) > maxCandidates {
		input.Candidates = append([]candidateItem(nil), input.Candidates[:maxCandidates]...)
	}
	filtered := make([]candidateItem, 0, len(input.Candidates))
	for _, candidate := range input.Candidates {
		candidate.Text = strings.TrimSpace(candidate.Text)
		candidate.Comment = strings.TrimSpace(candidate.Comment)
		if candidate.Text == "" {
			continue
		}
		filtered = append(filtered, candidate)
	}
	input.Candidates = filtered
	return input
}

func buildBertRequestKey(input bertRerankRequest) string {
	h := sha1.New()
	fmt.Fprintf(h, "prev=%s\x1fcomp=%s\x1fraw=%s", input.PreviousCommit, input.Composition, input.RawInput)
	for i, candidate := range input.Candidates {
		fmt.Fprintf(h, "\x1ec%d=%s\x1f%s", i, candidate.Text, candidate.Comment)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func identityBertRerankResult(count int) bertRerankResult {
	order := make([]int, count)
	scores := make([]bertScore, 0, count)
	for i := 0; i < count; i++ {
		order[i] = i
		scores = append(scores, bertScore{Index: i})
	}
	return bertRerankResult{
		Order:  order,
		Scores: scores,
	}
}

func reorderCandidateItems(candidates []candidateItem, result bertRerankResult) []candidateItem {
	if len(candidates) == 0 {
		return nil
	}
	if len(result.Order) == 0 {
		return append([]candidateItem(nil), candidates...)
	}
	used := make([]bool, len(candidates))
	reordered := make([]candidateItem, 0, len(candidates))
	for _, index := range result.Order {
		if index < 0 || index >= len(candidates) || used[index] {
			continue
		}
		used[index] = true
		reordered = append(reordered, candidates[index])
	}
	for i := range candidates {
		if !used[i] {
			reordered = append(reordered, candidates[i])
		}
	}
	return reordered
}

func sortBertScores(scores []bertScore, candidateCount int) bertRerankResult {
	if len(scores) == 0 {
		return identityBertRerankResult(candidateCount)
	}
	filtered := make([]bertScore, 0, len(scores))
	for _, score := range scores {
		if score.Index < 0 || score.Index >= candidateCount {
			continue
		}
		filtered = append(filtered, score)
	}
	if len(filtered) == 0 {
		return identityBertRerankResult(candidateCount)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Score == filtered[j].Score {
			return filtered[i].Index < filtered[j].Index
		}
		return filtered[i].Score > filtered[j].Score
	})
	order := make([]int, 0, candidateCount)
	for _, score := range filtered {
		order = append(order, score.Index)
	}
	for i := 0; i < candidateCount; i++ {
		found := false
		for _, index := range order {
			if index == i {
				found = true
				break
			}
		}
		if !found {
			order = append(order, i)
		}
	}
	return bertRerankResult{
		Order:  order,
		Scores: filtered,
	}
}

func shortBertFailureResult(candidateCount int) bertRerankResult {
	return identityBertRerankResult(candidateCount)
}

func (ime *IME) buildBertRequest(state rimeState) bertRerankRequest {
	return normalizeBertRerankRequest(bertRerankRequest{
		PreviousCommit: ime.aiPreviousCommit,
		Composition:    state.Composition,
		RawInput:       ime.customPhraseMatchInput(state),
		Candidates:     state.Candidates,
	}, ime.bertMaxCandidates())
}

func (ime *IME) bertMaxCandidates() int {
	if ime.bertConfig == nil || ime.bertConfig.MaxCandidates <= 0 {
		return 0
	}
	return min(ime.bertConfig.MaxCandidates, ime.candidateCount())
}

func defaultBertTimeout() time.Duration {
	return 1500 * time.Millisecond
}
