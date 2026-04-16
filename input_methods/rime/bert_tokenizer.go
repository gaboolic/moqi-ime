package rime

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	bertPadToken = "[PAD]"
	bertUNKToken = "[UNK]"
	bertCLSToken = "[CLS]"
	bertSEPToken = "[SEP]"
)

type bertTokenizer struct {
	vocabMap   map[string]int64
	lowerCase  bool
	padTokenID int64
	unkTokenID int64
	clsTokenID int64
	sepTokenID int64
}

func loadBertTokenizer(vocabPath string, lowerCase bool) (*bertTokenizer, error) {
	file, err := os.Open(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("open BERT vocab %s: %w", vocabPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)
	vocab := make(map[string]int64)
	var index int64
	for scanner.Scan() {
		token := strings.TrimSpace(scanner.Text())
		if token == "" {
			index++
			continue
		}
		if _, exists := vocab[token]; !exists {
			vocab[token] = index
		}
		index++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan BERT vocab %s: %w", vocabPath, err)
	}
	tokenizer := &bertTokenizer{
		vocabMap:   vocab,
		lowerCase:  lowerCase,
		padTokenID: 0,
		unkTokenID: 100,
		clsTokenID: 101,
		sepTokenID: 102,
	}
	if id, ok := vocab[bertPadToken]; ok {
		tokenizer.padTokenID = id
	}
	if id, ok := vocab[bertUNKToken]; ok {
		tokenizer.unkTokenID = id
	}
	if id, ok := vocab[bertCLSToken]; ok {
		tokenizer.clsTokenID = id
	}
	if id, ok := vocab[bertSEPToken]; ok {
		tokenizer.sepTokenID = id
	}
	return tokenizer, nil
}

func (t *bertTokenizer) encodePair(contextText, candidateText string, maxSeqLen int) (inputIDs, attentionMask, tokenTypeIDs []int64) {
	if t == nil {
		return nil, nil, nil
	}
	if maxSeqLen <= 0 {
		maxSeqLen = 96
	}
	contextTokens := t.tokenize(contextText)
	candidateTokens := t.tokenize(candidateText)
	maxContextLen := maxSeqLen - len(candidateTokens) - 3
	if maxContextLen < 0 {
		maxContextLen = 0
	}
	if len(contextTokens) > maxContextLen {
		contextTokens = contextTokens[len(contextTokens)-maxContextLen:]
	}
	inputIDs = make([]int64, 0, 1+len(contextTokens)+1+len(candidateTokens)+1)
	tokenTypeIDs = make([]int64, 0, cap(inputIDs))

	inputIDs = append(inputIDs, t.clsTokenID)
	tokenTypeIDs = append(tokenTypeIDs, 0)
	inputIDs = append(inputIDs, contextTokens...)
	for range contextTokens {
		tokenTypeIDs = append(tokenTypeIDs, 0)
	}
	inputIDs = append(inputIDs, t.sepTokenID)
	tokenTypeIDs = append(tokenTypeIDs, 0)
	inputIDs = append(inputIDs, candidateTokens...)
	for range candidateTokens {
		tokenTypeIDs = append(tokenTypeIDs, 1)
	}
	inputIDs = append(inputIDs, t.sepTokenID)
	tokenTypeIDs = append(tokenTypeIDs, 1)
	if len(inputIDs) > maxSeqLen {
		inputIDs = inputIDs[:maxSeqLen]
		tokenTypeIDs = tokenTypeIDs[:maxSeqLen]
	}
	attentionMask = make([]int64, len(inputIDs))
	for i := range attentionMask {
		attentionMask[i] = 1
	}
	return inputIDs, attentionMask, tokenTypeIDs
}

func (t *bertTokenizer) tokenize(text string) []int64 {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if t.lowerCase {
		text = strings.ToLower(text)
	}
	words := splitBertWords(text)
	tokenIDs := make([]int64, 0, len(words))
	for _, word := range words {
		tokenIDs = append(tokenIDs, t.wordPiece(word)...)
	}
	return tokenIDs
}

func splitBertWords(text string) []string {
	words := make([]string, 0, utf8.RuneCountInString(text))
	var asciiWord strings.Builder
	flushASCII := func() {
		if asciiWord.Len() == 0 {
			return
		}
		words = append(words, asciiWord.String())
		asciiWord.Reset()
	}
	for _, r := range text {
		switch {
		case unicode.IsSpace(r):
			flushASCII()
		case isBertASCIIWordRune(r):
			asciiWord.WriteRune(r)
		default:
			flushASCII()
			words = append(words, string(r))
		}
	}
	flushASCII()
	return words
}

func isBertASCIIWordRune(r rune) bool {
	return r <= unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r))
}

func (t *bertTokenizer) wordPiece(word string) []int64 {
	if word == "" {
		return nil
	}
	if id, ok := t.vocabMap[word]; ok {
		return []int64{id}
	}
	runes := []rune(word)
	if len(runes) == 1 {
		return []int64{t.unkTokenID}
	}
	pieces := make([]int64, 0, len(runes))
	for start := 0; start < len(runes); {
		end := len(runes)
		var matchedID int64 = -1
		next := start + 1
		for end > start {
			piece := string(runes[start:end])
			if start > 0 {
				piece = "##" + piece
			}
			if id, ok := t.vocabMap[piece]; ok {
				matchedID = id
				next = end
				break
			}
			end--
		}
		if matchedID == -1 {
			return []int64{t.unkTokenID}
		}
		pieces = append(pieces, matchedID)
		start = next
	}
	return pieces
}
