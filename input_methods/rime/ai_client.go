package rime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const aiRequestTimeout = 20 * time.Second

type aiClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

type chatCompletionsRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

func newConfiguredAIReviewGenerator(cfg *aiRuntimeConfig) func(aiGenerateRequest) ([]string, error) {
	client := newAIClient(cfg)
	if client == nil {
		return nil
	}
	return client.GenerateReviewCandidates
}

func newAIClient(cfg *aiRuntimeConfig) *aiClient {
	if cfg == nil {
		return nil
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.API.BaseURL), "/")
	apiKey := strings.TrimSpace(cfg.API.APIKey)
	model := strings.TrimSpace(cfg.API.Model)
	if baseURL == "" || apiKey == "" || model == "" {
		return nil
	}
	return &aiClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: aiRequestTimeout,
		},
	}
}

func newAIClientFromEnv() *aiClient {
	return newAIClient(envAIConfig())
}

func (c *aiClient) GenerateReviewCandidates(input aiGenerateRequest) ([]string, error) {
	if c == nil {
		return nil, fmt.Errorf("AI client is not configured")
	}
	input = normalizeAIGenerateRequest(input)
	if input.PreviousCommit == "" && input.Composition == "" && len(input.Candidates) == 0 {
		return nil, fmt.Errorf("AI input is empty")
	}

	payload := chatCompletionsRequest{
		Model: c.model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "你是一个中文输入法助手。请严格按用户要求输出候选文案，每条单独一行，不要编号，不要项目符号，不要解释，不要输出思考过程。",
			},
			{
				Role:    "user",
				Content: buildAIUserPrompt(input.Prompt, input),
			},
		},
		Temperature: 0.8,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal AI request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create AI request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call AI API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read AI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("AI API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode AI response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("AI API returned no choices")
	}

	candidates := parseReviewCandidates(parsed.Choices[0].Message.Content)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("AI API returned empty review content")
	}
	return candidates, nil
}

func normalizeAIGenerateRequest(input aiGenerateRequest) aiGenerateRequest {
	input.PreviousCommit = strings.TrimSpace(input.PreviousCommit)
	input.Composition = strings.TrimSpace(input.Composition)
	normalized := make([]string, 0, len(input.Candidates))
	for _, candidate := range input.Candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		normalized = append(normalized, candidate)
		if len(normalized) == 3 {
			break
		}
	}
	input.Candidates = normalized
	return input
}

func buildAIUserPrompt(promptTemplate string, input aiGenerateRequest) string {
	input = normalizeAIGenerateRequest(input)
	promptTemplate = strings.TrimSpace(promptTemplate)
	if promptTemplate == "" {
		promptTemplate = defaultAIUserPromptTemplate()
	}
	prompt, replaced := applyAIPromptPlaceholders(promptTemplate, input)
	if replaced {
		return prompt
	}
	return promptTemplate + "\n\n" + buildAIContextText(input)
}

func applyAIPromptPlaceholders(prompt string, input aiGenerateRequest) (string, bool) {
	candidate1 := aiCandidateAt(input.Candidates, 0)
	candidate2 := aiCandidateAt(input.Candidates, 1)
	candidate3 := aiCandidateAt(input.Candidates, 2)
	top3 := buildAICandidatesTop3Text(input.Candidates)
	previousCommit := input.PreviousCommit
	if previousCommit == "" {
		previousCommit = "无"
	}
	replaced := false

	replacements := []struct {
		old string
		new string
	}{
		{old: "{{previous_commit}}", new: previousCommit},
		{old: "{{composition}}", new: input.Composition},
		{old: "{{raw_input}}", new: input.Composition},
		{old: "{{candidate_1}}", new: candidate1},
		{old: "{{candidate_2}}", new: candidate2},
		{old: "{{candidate_3}}", new: candidate3},
		{old: "{{first_candidate}}", new: candidate1},
		{old: "{{second_candidate}}", new: candidate2},
		{old: "{{third_candidate}}", new: candidate3},
		{old: "{{candidates_top3}}", new: top3},
	}

	for _, item := range replacements {
		if strings.Contains(prompt, item.old) {
			prompt = strings.ReplaceAll(prompt, item.old, item.new)
			replaced = true
		}
	}
	return prompt, replaced
}

func aiCandidateAt(candidates []string, index int) string {
	if index < 0 || index >= len(candidates) {
		return ""
	}
	return candidates[index]
}

func buildAICandidatesTop3Text(candidates []string) string {
	if len(candidates) == 0 {
		return "无"
	}
	lines := make([]string, 0, len(candidates))
	for i, candidate := range candidates {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, candidate))
	}
	return strings.Join(lines, "\n")
}

func buildAIContextText(input aiGenerateRequest) string {
	previousCommit := input.PreviousCommit
	if previousCommit == "" {
		previousCommit = "无"
	}
	composition := input.Composition
	if composition == "" {
		composition = "无"
	}
	return "上一句：" + previousCommit + "\n原始输入：" + composition + "\n前三个候选词：\n" + buildAICandidatesTop3Text(input.Candidates)
}

func parseReviewCandidates(content string) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	var jsonArray []string
	if strings.HasPrefix(content, "[") && json.Unmarshal([]byte(content), &jsonArray) == nil {
		return normalizeAICandidates(jsonArray)
	}

	lines := strings.Split(content, "\n")
	candidates := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		candidates = append(candidates, line)
	}
	if len(candidates) == 0 {
		candidates = append(candidates, content)
	}
	return normalizeAICandidates(candidates)
}
