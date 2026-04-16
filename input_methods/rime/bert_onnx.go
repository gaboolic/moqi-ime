//go:build windows && cgo

package rime

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var bertONNXRuntimeState struct {
	mu                 sync.Mutex
	initialized        bool
	runtimeLibraryPath string
}

type bertOnnxReranker struct {
	mu            sync.Mutex
	cfg           *bertRuntimeConfig
	tokenizer     *bertTokenizer
	session       *ort.DynamicAdvancedSession
	inputNames    []string
	outputNames   []string
	maxSeqLen     int
	maxCandidates int
}

func newConfiguredBertReranker(cfg *bertRuntimeConfig) (bertReranker, error) {
	if cfg == nil {
		return nil, nil
	}
	switch cfg.Provider {
	case bertProviderCrossEncoder:
		return newBertOnnxReranker(cfg)
	default:
		return nil, fmt.Errorf("unsupported BERT provider %q", cfg.Provider)
	}
}

func newBertOnnxReranker(cfg *bertRuntimeConfig) (*bertOnnxReranker, error) {
	if cfg == nil {
		return nil, fmt.Errorf("BERT config is nil")
	}
	if err := ensureBertONNXEnvironment(cfg.RuntimeLibraryPath); err != nil {
		return nil, err
	}
	if _, err := os.Stat(cfg.ModelPath); err != nil {
		return nil, fmt.Errorf("stat BERT model %s: %w", cfg.ModelPath, err)
	}
	if _, err := os.Stat(cfg.VocabPath); err != nil {
		return nil, fmt.Errorf("stat BERT vocab %s: %w", cfg.VocabPath, err)
	}
	tokenizer, err := loadBertTokenizer(cfg.VocabPath, cfg.LowerCase)
	if err != nil {
		return nil, err
	}
	inputInfos, outputInfos, err := ort.GetInputOutputInfo(cfg.ModelPath)
	if err != nil {
		return nil, fmt.Errorf("inspect BERT model IO %s: %w", cfg.ModelPath, err)
	}
	inputNames := cfg.InputNames
	if len(inputNames) == 0 {
		inputNames = make([]string, 0, len(inputInfos))
		for _, info := range inputInfos {
			inputNames = append(inputNames, info.Name)
		}
	}
	outputNames := cfg.OutputNames
	if len(outputNames) == 0 && len(outputInfos) > 0 {
		outputNames = []string{outputInfos[0].Name}
	}
	if len(inputNames) == 0 || len(outputNames) == 0 {
		return nil, fmt.Errorf("BERT model IO names are empty for %s", cfg.ModelPath)
	}
	session, err := ort.NewDynamicAdvancedSession(cfg.ModelPath, inputNames, outputNames, nil)
	if err != nil {
		return nil, fmt.Errorf("create BERT session %s: %w", cfg.ModelPath, err)
	}
	return &bertOnnxReranker{
		cfg:           cfg,
		tokenizer:     tokenizer,
		session:       session,
		inputNames:    append([]string(nil), inputNames...),
		outputNames:   append([]string(nil), outputNames...),
		maxSeqLen:     cfg.MaxSequenceLength,
		maxCandidates: cfg.MaxCandidates,
	}, nil
}

func ensureBertONNXEnvironment(runtimeLibraryPath string) error {
	bertONNXRuntimeState.mu.Lock()
	defer bertONNXRuntimeState.mu.Unlock()

	if runtimeLibraryPath == "" {
		runtimeLibraryPath = defaultONNXRuntimeLibraryPath()
	}
	if bertONNXRuntimeState.initialized {
		if bertONNXRuntimeState.runtimeLibraryPath != "" &&
			runtimeLibraryPath != "" &&
			!strings.EqualFold(bertONNXRuntimeState.runtimeLibraryPath, runtimeLibraryPath) {
			return fmt.Errorf("onnxruntime already initialized with %s, cannot switch to %s",
				bertONNXRuntimeState.runtimeLibraryPath, runtimeLibraryPath)
		}
		return nil
	}
	if runtimeLibraryPath != "" {
		ort.SetSharedLibraryPath(runtimeLibraryPath)
	}
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("initialize onnxruntime: %w", err)
	}
	bertONNXRuntimeState.initialized = true
	bertONNXRuntimeState.runtimeLibraryPath = runtimeLibraryPath
	log.Printf("BERT ONNX Runtime initialized version=%s library=%q", ort.GetVersion(), runtimeLibraryPath)
	return nil
}

func defaultONNXRuntimeLibraryPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(exePath), "input_methods", "rime", "bert", "onnxruntime.dll")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return "onnxruntime.dll"
}

func (r *bertOnnxReranker) Close() error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.session == nil {
		return nil
	}
	err := r.session.Destroy()
	r.session = nil
	return err
}

func (r *bertOnnxReranker) Rerank(ctx context.Context, input bertRerankRequest) (bertRerankResult, error) {
	if r == nil || r.session == nil || r.tokenizer == nil || r.cfg == nil {
		return bertRerankResult{}, fmt.Errorf("BERT reranker is not initialized")
	}
	input = normalizeBertRerankRequest(input, r.maxCandidates)
	if len(input.Candidates) <= 1 {
		return identityBertRerankResult(input.OriginalCandidateCount), nil
	}
	contextText := truncateRunes(buildBertContextText(input), r.cfg.LeftContextRunes)
	scores := make([]bertScore, 0, len(input.Candidates))
	for i, candidate := range input.Candidates {
		select {
		case <-ctx.Done():
			return bertRerankResult{}, ctx.Err()
		default:
		}
		score, err := r.scoreCandidate(contextText, candidate.Text)
		if err != nil {
			return bertRerankResult{}, fmt.Errorf("score candidate %q: %w", candidate.Text, err)
		}
		scores = append(scores, bertScore{
			Index: candidateOriginalIndex(input, i),
			Text:  candidate.Text,
			Score: score,
		})
	}
	if input.PromoteTopOnly {
		return promoteSingleBertCandidate(scores, input.OriginalCandidateCount, bertMinScoreLead), nil
	}
	return sortBertScores(scores, input.OriginalCandidateCount), nil
}

func buildBertContextText(input bertRerankRequest) string {
	parts := make([]string, 0, 3)
	if text := strings.TrimSpace(input.PreviousCommit); text != "" {
		parts = append(parts, text)
	}
	if raw := strings.TrimSpace(input.RawInput); raw != "" {
		parts = append(parts, raw)
	} else if composition := strings.TrimSpace(input.Composition); composition != "" {
		parts = append(parts, composition)
	}
	return strings.Join(parts, " ")
}

func candidateOriginalIndex(input bertRerankRequest, candidateIndex int) int {
	if candidateIndex >= 0 && candidateIndex < len(input.CandidateIndexes) {
		return input.CandidateIndexes[candidateIndex]
	}
	return candidateIndex
}

func (r *bertOnnxReranker) scoreCandidate(contextText, candidateText string) (float64, error) {
	inputIDs, attentionMask, tokenTypeIDs := r.tokenizer.encodePair(contextText, candidateText, r.maxSeqLen)
	inputValues := make([]ort.Value, 0, len(r.inputNames))
	destroyers := make([]ort.Value, 0, len(r.inputNames)+len(r.outputNames))
	for _, name := range r.inputNames {
		var (
			value ort.Value
			err   error
		)
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "input_ids":
			value, err = ort.NewTensor(ort.NewShape(1, int64(len(inputIDs))), inputIDs)
		case "attention_mask":
			value, err = ort.NewTensor(ort.NewShape(1, int64(len(attentionMask))), attentionMask)
		case "token_type_ids":
			value, err = ort.NewTensor(ort.NewShape(1, int64(len(tokenTypeIDs))), tokenTypeIDs)
		default:
			return 0, fmt.Errorf("unsupported BERT model input %q", name)
		}
		if err != nil {
			for _, item := range destroyers {
				_ = item.Destroy()
			}
			return 0, err
		}
		inputValues = append(inputValues, value)
		destroyers = append(destroyers, value)
	}

	outputs := make([]ort.Value, len(r.outputNames))
	r.mu.Lock()
	err := r.session.Run(inputValues, outputs)
	r.mu.Unlock()
	if err != nil {
		for _, item := range destroyers {
			_ = item.Destroy()
		}
		return 0, err
	}
	for _, output := range outputs {
		if output != nil {
			destroyers = append(destroyers, output)
		}
	}
	defer func() {
		for _, item := range destroyers {
			_ = item.Destroy()
		}
	}()
	if len(outputs) == 0 || outputs[0] == nil {
		return 0, fmt.Errorf("BERT model returned no outputs")
	}
	return extractBertScore(outputs[0], r.cfg.PositiveLabelIndex)
}

func extractBertScore(value ort.Value, positiveLabelIndex int) (float64, error) {
	switch tensor := value.(type) {
	case *ort.Tensor[float32]:
		return scoreFromFloat32Logits(tensor.GetData(), positiveLabelIndex)
	case *ort.Tensor[float64]:
		data := tensor.GetData()
		if len(data) == 1 {
			return data[0], nil
		}
		logits := make([]float64, len(data))
		copy(logits, data)
		return selectProbabilityScore(logits, positiveLabelIndex), nil
	case *ort.Tensor[int64]:
		data := tensor.GetData()
		if len(data) == 1 {
			return float64(data[0]), nil
		}
		logits := make([]float64, len(data))
		for i, item := range data {
			logits[i] = float64(item)
		}
		return selectProbabilityScore(logits, positiveLabelIndex), nil
	default:
		return 0, fmt.Errorf("unsupported BERT output tensor type %T", value)
	}
}

func scoreFromFloat32Logits(logits []float32, positiveLabelIndex int) (float64, error) {
	if len(logits) == 0 {
		return 0, fmt.Errorf("empty logits")
	}
	if len(logits) == 1 {
		return float64(logits[0]), nil
	}
	values := make([]float64, len(logits))
	for i, item := range logits {
		values[i] = float64(item)
	}
	return selectProbabilityScore(values, positiveLabelIndex), nil
}

func selectProbabilityScore(logits []float64, positiveLabelIndex int) float64 {
	if len(logits) == 0 {
		return 0
	}
	if len(logits) == 1 {
		return logits[0]
	}
	if positiveLabelIndex < 0 || positiveLabelIndex >= len(logits) {
		positiveLabelIndex = len(logits) - 1
	}
	maxLogit := logits[0]
	for _, logit := range logits[1:] {
		if logit > maxLogit {
			maxLogit = logit
		}
	}
	var sum float64
	probs := make([]float64, len(logits))
	for i, logit := range logits {
		probs[i] = math.Exp(logit - maxLogit)
		sum += probs[i]
	}
	if sum <= 0 {
		return logits[positiveLabelIndex]
	}
	return probs[positiveLabelIndex] / sum
}
