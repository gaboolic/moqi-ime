package rime

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	bertMinSentenceInputChars    = 8
	bertMinScoreLead             = 0.12
	bertAsyncDebounceDelay       = 180
	bertMaxSentencePaths         = 24
	bertMaxSentenceCombinations  = 24
	bertMaxSegmentCandidateCount = 5
)

type bertCandidateSource interface {
	bertCandidatesForCode(code string, limit int) []candidateItem
}

type bertSchemaCandidateSource interface {
	bertCandidatesForCodeWithSchema(schemaID, code string, limit int) ([]candidateItem, bool)
}

type bertPathEdge struct {
	Start int
	End   int
	Text  string
	Rank  int
}

func bertNormalizedRawInput(state rimeState) string {
	raw := strings.TrimSpace(state.RawInput)
	if raw == "" {
		raw = strings.TrimSpace(state.Composition)
	}
	return normalizeBertRawInput(raw)
}

func normalizeBertRawInput(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.ReplaceAll(raw, " ", "")
	raw = strings.ReplaceAll(raw, "'", "")
	return strings.ToLower(raw)
}

func bertCompactInputLength(raw string) int {
	if raw == "" {
		return 0
	}
	raw = strings.ReplaceAll(raw, "'", "")
	return utf8.RuneCountInString(raw)
}

func isSuspectedSentenceInput(raw string) bool {
	if bertCompactInputLength(raw) < bertMinSentenceInputChars {
		return false
	}
	for _, r := range raw {
		if r == '\'' {
			continue
		}
		if r > unicode.MaxASCII || !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func bertSentenceMinRunes(raw string) int {
	compactLen := bertCompactInputLength(raw)
	if compactLen <= 0 {
		return 4
	}
	minRunes := compactLen / 3
	if minRunes < 4 {
		minRunes = 4
	}
	return minRunes
}

func isWholeSentenceCandidate(text, raw string) bool {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) < bertSentenceMinRunes(raw) {
		return false
	}
	hasHan := false
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			hasHan = true
		case unicode.IsSpace(r):
		case unicode.IsPunct(r):
		default:
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return false
			}
		}
	}
	return hasHan
}

func bertSegmentLengths(rawLen int) []int {
	lengths := []int{9, 6, 5, 4, 7, 8, 10, 3, 2}
	out := make([]int, 0, len(lengths))
	for _, length := range lengths {
		if length <= rawLen {
			out = append(out, length)
		}
	}
	return out
}

func bertSegmentCandidateLimit(segmentLen int) int {
	switch {
	case segmentLen <= 2:
		return 2
	case segmentLen == 3:
		return 4
	default:
		return bertMaxSegmentCandidateCount
	}
}

func calculateBertPathScore(path []bertPathEdge) float64 {
	if len(path) == 0 {
		return 1e9
	}
	totalRank := 0
	for _, edge := range path {
		totalRank += edge.Rank
	}
	return float64(totalRank) + float64(len(path))*0.5
}

func joinBertPath(path []bertPathEdge) string {
	var builder strings.Builder
	for _, edge := range path {
		builder.WriteString(edge.Text)
	}
	return builder.String()
}

func bertCandidateSourceFromBackend(backend rimeBackend) bertCandidateSource {
	if source, ok := backend.(bertCandidateSource); ok {
		return source
	}
	return nil
}

func bertSchemaCandidateSourceFromBackend(backend rimeBackend) bertSchemaCandidateSource {
	if source, ok := backend.(bertSchemaCandidateSource); ok {
		return source
	}
	return nil
}

func normalizeSentenceCacheValues(sentences []string) []string {
	if len(sentences) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(sentences))
	seen := make(map[string]struct{}, len(sentences))
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence == "" {
			continue
		}
		if _, ok := seen[sentence]; ok {
			continue
		}
		seen[sentence] = struct{}{}
		normalized = append(normalized, sentence)
	}
	return normalized
}

func sentenceSetFromList(sentences []string) map[string]struct{} {
	if len(sentences) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(sentences))
	for _, sentence := range sentences {
		if sentence != "" {
			out[sentence] = struct{}{}
		}
	}
	return out
}

func buildBertSentenceCacheKey(schemaID, raw string) string {
	raw = normalizeBertRawInput(raw)
	if !isSuspectedSentenceInput(raw) {
		return ""
	}
	return fmt.Sprintf("%s\x1f%s", strings.TrimSpace(schemaID), raw)
}

func generateSentenceCandidatesFromSource(source bertCandidateSource, raw string) []string {
	if source == nil {
		return nil
	}
	raw = normalizeBertRawInput(raw)
	if !isSuspectedSentenceInput(raw) {
		return nil
	}
	rawLen := len(raw)
	wordGraph := make(map[int]map[int][]bertPathEdge, rawLen)
	for start := 0; start < rawLen; start++ {
		lengths := bertSegmentLengths(rawLen - start)
		if len(lengths) == 0 {
			continue
		}
		for _, segmentLen := range lengths {
			end := start + segmentLen
			if end > rawLen {
				continue
			}
			code := raw[start:end]
			candidates := source.bertCandidatesForCode(code, bertSegmentCandidateLimit(segmentLen))
			if len(candidates) == 0 {
				continue
			}
			if wordGraph[start] == nil {
				wordGraph[start] = make(map[int][]bertPathEdge)
			}
			edges := make([]bertPathEdge, 0, len(candidates))
			for rank, candidate := range candidates {
				text := strings.TrimSpace(candidate.Text)
				if text == "" {
					continue
				}
				edges = append(edges, bertPathEdge{
					Start: start,
					End:   end,
					Text:  text,
					Rank:  rank,
				})
			}
			if len(edges) > 0 {
				wordGraph[start][end] = edges
			}
		}
	}

	pathsByPos := map[int][][]bertPathEdge{0: {nil}}
	for pos := 0; pos < rawLen; pos++ {
		currentPaths := pathsByPos[pos]
		if len(currentPaths) == 0 {
			continue
		}
		edgesByEnd := wordGraph[pos]
		if len(edgesByEnd) == 0 {
			continue
		}
		for end, edges := range edgesByEnd {
			for _, path := range currentPaths {
				for _, edge := range edges {
					newPath := append(append([]bertPathEdge(nil), path...), edge)
					pathsByPos[end] = append(pathsByPos[end], newPath)
				}
			}
			if len(pathsByPos[end]) > bertMaxSentencePaths {
				paths := pathsByPos[end]
				sortBertPaths(paths)
				pathsByPos[end] = paths[:bertMaxSentencePaths]
			}
		}
	}

	completePaths := pathsByPos[rawLen]
	if len(completePaths) == 0 {
		return nil
	}
	sortBertPaths(completePaths)
	if len(completePaths) > bertMaxSentenceCombinations {
		completePaths = completePaths[:bertMaxSentenceCombinations]
	}

	sentences := make([]string, 0, len(completePaths))
	for _, path := range completePaths {
		sentence := strings.TrimSpace(joinBertPath(path))
		if !isWholeSentenceCandidate(sentence, raw) {
			continue
		}
		sentences = append(sentences, sentence)
	}
	return normalizeSentenceCacheValues(sentences)
}

func generateSentenceCandidatesWithSchemaSource(source bertSchemaCandidateSource, schemaID, raw string) ([]string, bool) {
	if source == nil {
		return nil, true
	}
	raw = normalizeBertRawInput(raw)
	if !isSuspectedSentenceInput(raw) {
		return nil, true
	}
	rawLen := len(raw)
	wordGraph := make(map[int]map[int][]bertPathEdge, rawLen)
	for start := 0; start < rawLen; start++ {
		lengths := bertSegmentLengths(rawLen - start)
		if len(lengths) == 0 {
			continue
		}
		for _, segmentLen := range lengths {
			end := start + segmentLen
			if end > rawLen {
				continue
			}
			code := raw[start:end]
			candidates, ok := source.bertCandidatesForCodeWithSchema(schemaID, code, bertSegmentCandidateLimit(segmentLen))
			if !ok {
				return nil, false
			}
			if len(candidates) == 0 {
				continue
			}
			if wordGraph[start] == nil {
				wordGraph[start] = make(map[int][]bertPathEdge)
			}
			edges := make([]bertPathEdge, 0, len(candidates))
			for rank, candidate := range candidates {
				text := strings.TrimSpace(candidate.Text)
				if text == "" {
					continue
				}
				edges = append(edges, bertPathEdge{
					Start: start,
					End:   end,
					Text:  text,
					Rank:  rank,
				})
			}
			if len(edges) > 0 {
				wordGraph[start][end] = edges
			}
		}
	}
	pathsByPos := map[int][][]bertPathEdge{0: {nil}}
	for pos := 0; pos < rawLen; pos++ {
		currentPaths := pathsByPos[pos]
		if len(currentPaths) == 0 {
			continue
		}
		edgesByEnd := wordGraph[pos]
		if len(edgesByEnd) == 0 {
			continue
		}
		for end, edges := range edgesByEnd {
			for _, path := range currentPaths {
				for _, edge := range edges {
					newPath := append(append([]bertPathEdge(nil), path...), edge)
					pathsByPos[end] = append(pathsByPos[end], newPath)
				}
			}
			if len(pathsByPos[end]) > bertMaxSentencePaths {
				paths := pathsByPos[end]
				sortBertPaths(paths)
				pathsByPos[end] = paths[:bertMaxSentencePaths]
			}
		}
	}
	completePaths := pathsByPos[rawLen]
	if len(completePaths) == 0 {
		return nil, true
	}
	sortBertPaths(completePaths)
	if len(completePaths) > bertMaxSentenceCombinations {
		completePaths = completePaths[:bertMaxSentenceCombinations]
	}
	sentences := make([]string, 0, len(completePaths))
	for _, path := range completePaths {
		sentence := strings.TrimSpace(joinBertPath(path))
		if !isWholeSentenceCandidate(sentence, raw) {
			continue
		}
		sentences = append(sentences, sentence)
	}
	return normalizeSentenceCacheValues(sentences), true
}

func sortBertPaths(paths [][]bertPathEdge) {
	sort.SliceStable(paths, func(i, j int) bool {
		left := calculateBertPathScore(paths[i])
		right := calculateBertPathScore(paths[j])
		if left == right {
			return joinBertPath(paths[i]) < joinBertPath(paths[j])
		}
		return left < right
	})
}
