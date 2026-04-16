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
	PreviousCommit         string
	Composition            string
	RawInput               string
	CandidateIndexes       []int
	OriginalCandidateCount int
	PromoteTopOnly         bool
	Candidates             []candidateItem
}

type bertAsyncSnapshot struct {
	State          rimeState
	PreviousCommit string
	RawInput       string
	SchemaID       string
	Key            string
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
		PreviousCommit:         strings.TrimSpace(input.PreviousCommit),
		Composition:            strings.TrimSpace(input.Composition),
		RawInput:               strings.TrimSpace(input.RawInput),
		OriginalCandidateCount: input.OriginalCandidateCount,
		PromoteTopOnly:         input.PromoteTopOnly,
	}
	if len(input.CandidateIndexes) > 0 {
		cloned.CandidateIndexes = append([]int(nil), input.CandidateIndexes...)
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
		if len(input.CandidateIndexes) > maxCandidates {
			input.CandidateIndexes = append([]int(nil), input.CandidateIndexes[:maxCandidates]...)
		}
	}
	filtered := make([]candidateItem, 0, len(input.Candidates))
	filteredIndexes := make([]int, 0, len(input.Candidates))
	for i, candidate := range input.Candidates {
		candidate.Text = strings.TrimSpace(candidate.Text)
		candidate.Comment = strings.TrimSpace(candidate.Comment)
		index := i
		if i < len(input.CandidateIndexes) {
			index = input.CandidateIndexes[i]
		}
		if candidate.Text == "" {
			continue
		}
		filtered = append(filtered, candidate)
		filteredIndexes = append(filteredIndexes, index)
	}
	input.Candidates = filtered
	input.CandidateIndexes = filteredIndexes
	if input.OriginalCandidateCount <= 0 {
		input.OriginalCandidateCount = len(input.Candidates)
	}
	return input
}

func buildBertRequestKey(input bertRerankRequest) string {
	h := sha1.New()
	fmt.Fprintf(h, "prev=%s\x1fcomp=%s\x1fraw=%s", input.PreviousCommit, input.Composition, input.RawInput)
	for i, candidate := range input.Candidates {
		index := i
		if i < len(input.CandidateIndexes) {
			index = input.CandidateIndexes[i]
		}
		fmt.Fprintf(h, "\x1ec%d=%d\x1f%s\x1f%s", i, index, candidate.Text, candidate.Comment)
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

func promoteSingleBertCandidate(scores []bertScore, candidateCount int, minLead float64) bertRerankResult {
	if candidateCount <= 0 {
		return bertRerankResult{}
	}
	filtered := make([]bertScore, 0, len(scores))
	for _, score := range scores {
		if score.Index < 0 || score.Index >= candidateCount {
			continue
		}
		filtered = append(filtered, score)
	}
	if len(filtered) < 2 {
		return identityBertRerankResult(candidateCount)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Score == filtered[j].Score {
			return filtered[i].Index < filtered[j].Index
		}
		return filtered[i].Score > filtered[j].Score
	})
	best := filtered[0]
	next := filtered[1]
	if best.Index <= 0 || best.Score-next.Score < minLead {
		return identityBertRerankResult(candidateCount)
	}
	order := make([]int, 0, candidateCount)
	order = append(order, best.Index)
	for i := 0; i < candidateCount; i++ {
		if i != best.Index {
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

func cloneRimeState(state rimeState) rimeState {
	cloned := state
	if len(state.Candidates) > 0 {
		cloned.Candidates = append([]candidateItem(nil), state.Candidates...)
	}
	return cloned
}

func countWholeSentenceCandidates(candidates []candidateItem, rawInput string) int {
	count := 0
	for _, candidate := range candidates {
		if isWholeSentenceCandidate(strings.TrimSpace(candidate.Text), rawInput) {
			count++
		}
	}
	return count
}

func buildBertStateKey(snapshot bertAsyncSnapshot) string {
	h := sha1.New()
	fmt.Fprintf(h, "schema=%s\x1fprev=%s\x1fcomp=%s\x1fraw=%s",
		snapshot.SchemaID,
		snapshot.PreviousCommit,
		strings.TrimSpace(snapshot.State.Composition),
		snapshot.RawInput,
	)
	for i, candidate := range snapshot.State.Candidates {
		fmt.Fprintf(h, "\x1ec%d=%s\x1f%s", i, strings.TrimSpace(candidate.Text), strings.TrimSpace(candidate.Comment))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (ime *IME) bertSnapshotForState(state rimeState) (bertAsyncSnapshot, bool) {
	if !ime.shouldBertRerankState(state) {
		return bertAsyncSnapshot{}, false
	}
	rawInput := normalizeBertRawInput(ime.customPhraseMatchInput(state))
	if !isSuspectedSentenceInput(rawInput) {
		return bertAsyncSnapshot{}, false
	}
	if countWholeSentenceCandidates(state.Candidates, rawInput) < 2 {
		return bertAsyncSnapshot{}, false
	}
	snapshot := bertAsyncSnapshot{
		State:          cloneRimeState(state),
		PreviousCommit: strings.TrimSpace(ime.aiPreviousCommit),
		RawInput:       rawInput,
	}
	if ime.backend != nil {
		snapshot.SchemaID = strings.TrimSpace(ime.backend.CurrentSchemaID())
	}
	snapshot.Key = buildBertStateKey(snapshot)
	return snapshot, snapshot.Key != ""
}

func (ime *IME) generatedSentenceCandidates(snapshot bertAsyncSnapshot) map[string]struct{} {
	if ime.bertSentenceCache == nil {
		ime.bertSentenceCache = newBertSentenceCandidateCache(defaultBertSentenceCacheTTL)
	}
	cacheKey := buildBertSentenceCacheKey(snapshot.SchemaID, snapshot.RawInput)
	if cacheKey == "" {
		return nil
	}
	sentences := ime.bertSentenceCache.GetOrCompute(cacheKey, func() ([]string, bool) {
		if source := bertSchemaCandidateSourceFromBackend(ime.backend); source != nil {
			return generateSentenceCandidatesWithSchemaSource(source, snapshot.SchemaID, snapshot.RawInput)
		}
		return generateSentenceCandidatesFromSource(bertCandidateSourceFromBackend(ime.backend), snapshot.RawInput), true
	})
	return sentenceSetFromList(sentences)
}

func (ime *IME) buildBertRequest(snapshot bertAsyncSnapshot) bertRerankRequest {
	if !isSuspectedSentenceInput(snapshot.RawInput) {
		return bertRerankRequest{}
	}
	generated := ime.generatedSentenceCandidates(snapshot)
	if len(generated) == 0 {
		return bertRerankRequest{}
	}
	indexes := make([]int, 0, len(snapshot.State.Candidates))
	candidates := make([]candidateItem, 0, len(snapshot.State.Candidates))
	for i, candidate := range snapshot.State.Candidates {
		text := strings.TrimSpace(candidate.Text)
		if !isWholeSentenceCandidate(text, snapshot.RawInput) {
			continue
		}
		if _, ok := generated[text]; !ok {
			continue
		}
		indexes = append(indexes, i)
		candidates = append(candidates, candidate)
	}
	if len(candidates) < 2 {
		return bertRerankRequest{}
	}
	return normalizeBertRerankRequest(bertRerankRequest{
		PreviousCommit:         snapshot.PreviousCommit,
		Composition:            snapshot.State.Composition,
		RawInput:               snapshot.RawInput,
		CandidateIndexes:       indexes,
		OriginalCandidateCount: len(snapshot.State.Candidates),
		PromoteTopOnly:         true,
		Candidates:             candidates,
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
