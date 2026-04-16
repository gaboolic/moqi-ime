package rime

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	defaultBertMinSentenceInputChars = 8
	bertMinScoreLead                 = 0.12
	defaultBertAsyncDebounceDelayMS  = 180
)

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

func normalizeBertMinSentenceInputChars(minChars int) int {
	if minChars <= 0 {
		return defaultBertMinSentenceInputChars
	}
	return minChars
}

func isSuspectedSentenceInput(raw string, minChars int) bool {
	if bertCompactInputLength(raw) < normalizeBertMinSentenceInputChars(minChars) {
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
