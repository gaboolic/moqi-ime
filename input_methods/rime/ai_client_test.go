package rime

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewAIClientFromEnvRequiresAllFields(t *testing.T) {
	t.Setenv("MOQI_AI_BASE_URL", "")
	t.Setenv("MOQI_AI_API_KEY", "")
	t.Setenv("MOQI_AI_MODEL", "")

	if client := newAIClientFromEnv(); client != nil {
		t.Fatalf("expected nil client when env vars are missing, got %#v", client)
	}
}

func TestNewAIClientFromEnvTrimsValues(t *testing.T) {
	t.Setenv("MOQI_AI_BASE_URL", " https://example.test/v1/ ")
	t.Setenv("MOQI_AI_API_KEY", " secret ")
	t.Setenv("MOQI_AI_MODEL", " demo-model ")

	client := newAIClientFromEnv()
	if client == nil {
		t.Fatal("expected client to be created from env")
	}
	if client.baseURL != "https://example.test/v1" {
		t.Fatalf("expected trimmed base URL, got %q", client.baseURL)
	}
	if client.apiKey != "secret" {
		t.Fatalf("expected trimmed API key, got %q", client.apiKey)
	}
	if client.model != "demo-model" {
		t.Fatalf("expected trimmed model, got %q", client.model)
	}
	if client.httpClient == nil || client.httpClient.Timeout != aiRequestTimeout {
		t.Fatalf("expected default timeout %v, got %#v", aiRequestTimeout, client.httpClient)
	}
}

func TestGenerateReviewCandidatesSendsChatCompletionRequest(t *testing.T) {
	type capturedRequest struct {
		Model       string        `json:"model"`
		Messages    []chatMessage `json:"messages"`
		Temperature float64       `json:"temperature"`
		Thinking    thinkingMode  `json:"thinking"`
	}

	var captured capturedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions path, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret-key" {
			t.Fatalf("unexpected authorization header: %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("unexpected content type: %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [
				{
					"message": {
						"role": "assistant",
						"content": "1. 质量很好，细节做得很到位。\n2. 使用体验顺手，整体很满意。"
					}
				}
			]
		}`))
	}))
	defer server.Close()

	client := &aiClient{
		baseURL:    server.URL,
		apiKey:     "secret-key",
		model:      "kimi-k2.5",
		httpClient: server.Client(),
	}

	candidates, err := client.GenerateReviewCandidates(aiGenerateRequest{
		PreviousCommit: "上一句内容",
		Composition:    "咖啡机",
		Candidates:     []string{"咖啡机", "咖啡壶", "咖啡杯"},
		Prompt:         defaultAIUserPromptTemplate(),
	})
	if err != nil {
		t.Fatalf("expected request to succeed, got error: %v", err)
	}
	if captured.Model != "kimi-k2.5" {
		t.Fatalf("expected model to be sent, got %q", captured.Model)
	}
	if len(captured.Messages) != 2 {
		t.Fatalf("expected 2 chat messages, got %#v", captured.Messages)
	}
	if captured.Messages[0].Role != "system" {
		t.Fatalf("expected system prompt first, got %#v", captured.Messages)
	}
	if !strings.Contains(captured.Messages[0].Content, "不要输出思考过程") {
		t.Fatalf("expected system prompt to forbid reasoning output, got %q", captured.Messages[0].Content)
	}
	if !strings.Contains(captured.Messages[1].Content, "咖啡机") {
		t.Fatalf("expected composition in user prompt, got %q", captured.Messages[1].Content)
	}
	if !strings.Contains(captured.Messages[1].Content, "最多 3 条") {
		t.Fatalf("expected user prompt to limit candidate count, got %q", captured.Messages[1].Content)
	}
	if !strings.Contains(captured.Messages[1].Content, "20 字左右") {
		t.Fatalf("expected user prompt to limit candidate length, got %q", captured.Messages[1].Content)
	}
	if captured.Temperature != 0.8 {
		t.Fatalf("expected temperature 0.8, got %v", captured.Temperature)
	}
	if captured.Thinking.Type != "disabled" {
		t.Fatalf("expected thinking disabled, got %#v", captured.Thinking)
	}
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %#v", candidates)
	}
	if candidates[0] != "质量很好，细节做得很到位。" {
		t.Fatalf("unexpected first candidate: %#v", candidates[0])
	}
	if candidates[1] != "使用体验顺手，整体很满意。" {
		t.Fatalf("unexpected second candidate: %#v", candidates[1])
	}
}

func TestGenerateReviewCandidatesReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer server.Close()

	client := &aiClient{
		baseURL:    server.URL,
		apiKey:     "secret-key",
		model:      "kimi-k2.5",
		httpClient: server.Client(),
	}

	_, err := client.GenerateReviewCandidates(aiGenerateRequest{
		Composition: "咖啡机",
		Candidates:  []string{"咖啡机"},
		Prompt:      defaultAIUserPromptTemplate(),
	})
	if err == nil {
		t.Fatal("expected API error")
	}
	if !strings.Contains(err.Error(), "AI API returned 502") {
		t.Fatalf("expected status code in error, got %v", err)
	}
}

func TestGenerateReviewCandidatesReturnsTimeoutError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer server.Close()

	client := &aiClient{
		baseURL: server.URL,
		apiKey:  "secret-key",
		model:   "kimi-k2.5",
		httpClient: &http.Client{
			Timeout: 20 * time.Millisecond,
		},
	}

	_, err := client.GenerateReviewCandidates(aiGenerateRequest{
		Composition: "咖啡机",
		Candidates:  []string{"咖啡机"},
		Prompt:      defaultAIUserPromptTemplate(),
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "call AI API") {
		t.Fatalf("expected wrapped timeout error, got %v", err)
	}
}

func TestParseReviewCandidatesSupportsJSONArrayAndText(t *testing.T) {
	jsonCandidates := parseReviewCandidates(`["1. 质感不错","2. 很好用","很好用"]`)
	if len(jsonCandidates) != 2 {
		t.Fatalf("expected deduplicated JSON candidates, got %#v", jsonCandidates)
	}
	if jsonCandidates[0] != "质感不错" {
		t.Fatalf("unexpected first JSON candidate: %#v", jsonCandidates[0])
	}

	textCandidates := parseReviewCandidates("1. 外观漂亮。\n2. 上手简单。\n\n")
	if len(textCandidates) != 2 {
		t.Fatalf("expected parsed text candidates, got %#v", textCandidates)
	}
	if textCandidates[1] != "上手简单。" {
		t.Fatalf("unexpected second text candidate: %#v", textCandidates[1])
	}
}

func TestParseReviewCandidatesLimitsCandidateCount(t *testing.T) {
	candidates := parseReviewCandidates("1. 第一条。\n2. 第二条。\n3. 第三条。\n4. 第四条。")
	if len(candidates) != 3 {
		t.Fatalf("expected candidate count to be capped at 3, got %#v", candidates)
	}
	if candidates[2] != "第三条。" {
		t.Fatalf("unexpected third candidate after cap: %#v", candidates[2])
	}
}

func TestBuildAIUserPromptSupportsCompositionAndCandidatesPlaceholders(t *testing.T) {
	prompt := buildAIUserPrompt("上一句：{{previous_commit}}\n原始输入：{{composition}}\n第一候选：{{candidate_1}}\n候选列表：\n{{candidates_top3}}", aiGenerateRequest{
		PreviousCommit: "人们在春节和元宵节期间也制做冰灯摆在门前",
		Composition:    "咖啡机",
		Candidates:     []string{"咖啡机", "咖啡壶", "咖啡杯"},
	})
	if !strings.Contains(prompt, "上一句：人们在春节和元宵节期间也制做冰灯摆在门前") {
		t.Fatalf("expected previous commit placeholder to be replaced, got %q", prompt)
	}
	if !strings.Contains(prompt, "原始输入：咖啡机") {
		t.Fatalf("expected composition placeholder to be replaced, got %q", prompt)
	}
	if !strings.Contains(prompt, "第一候选：咖啡机") {
		t.Fatalf("expected first candidate placeholder to be replaced, got %q", prompt)
	}
	if !strings.Contains(prompt, "2. 咖啡壶") || !strings.Contains(prompt, "3. 咖啡杯") {
		t.Fatalf("expected top candidates placeholder to be replaced, got %q", prompt)
	}
}

func TestBuildAIUserPromptAppendsIMContextWithoutPlaceholders(t *testing.T) {
	prompt := buildAIUserPrompt("请生成 1 条短评。", aiGenerateRequest{
		PreviousCommit: "上一句内容",
		Composition:    "咖啡机",
		Candidates:     []string{"咖啡机", "咖啡壶"},
	})
	if !strings.Contains(prompt, "请生成 1 条短评。") || !strings.Contains(prompt, "上一句：上一句内容") || !strings.Contains(prompt, "原始输入：咖啡机") {
		t.Fatalf("unexpected prompt without placeholders: %q", prompt)
	}
	if !strings.Contains(prompt, "1. 咖啡机") || !strings.Contains(prompt, "2. 咖啡壶") {
		t.Fatalf("expected candidate context to be appended, got %q", prompt)
	}
}

func TestGenerateReviewCandidatesWithRealAPI(t *testing.T) {
	if os.Getenv("MOQI_AI_REAL_TEST") != "1" {
		t.Skip("set MOQI_AI_REAL_TEST=1 to run real API test")
	}

	client := newAIClientFromEnv()
	if client == nil {
		t.Fatal("expected real AI client from environment variables")
	}

	candidates, err := client.GenerateReviewCandidates(aiGenerateRequest{
		PreviousCommit: "上一句内容",
		Composition:    "咖啡机",
		Candidates:     []string{"咖啡机", "咖啡壶", "咖啡杯"},
		Prompt:         defaultAIUserPromptTemplate(),
	})
	if err != nil {
		t.Fatalf("real AI request failed: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one real AI candidate")
	}
	t.Logf("real AI candidates: %q", candidates)
}
