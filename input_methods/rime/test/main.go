package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const requestTimeout = 20 * time.Second

type fileConfig struct {
	API struct {
		BaseURL string `json:"base_url"`
		APIKey  string `json:"api_key"`
		Model   string `json:"model"`
	} `json:"api"`
	Actions []struct {
		Name   string `json:"name"`
		Hotkey string `json:"hotkey"`
		Prompt string `json:"prompt"`
	} `json:"actions"`
}

type chatCompletionsRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Thinking    thinkingMode  `json:"thinking"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type thinkingMode struct {
	Type string `json:"type"`
}

type chatCompletionsResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type aiInput struct {
	PreviousCommit string
	Composition    string
	Candidates     []string
}

func main() {
	cfgPath := siblingPath("../ai_config.json")

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		fail("load config", err)
	}

	input := aiInput{
		PreviousCommit: "人们在春节和元宵节期间也制做冰灯摆在门前",
		Composition:    "tangkongchuanshengranghaizitizhewan",
		Candidates:     []string{"糖空传声让孩子提着万"},
	}
	fmt.Printf("config: %s\n", cfgPath)
	fmt.Println("input: built-in sample")
	fmt.Printf("previous_commit: %s\n", input.PreviousCommit)
	fmt.Printf("composition: %s\n", input.Composition)
	fmt.Printf("candidates: %v\n", input.Candidates)
	fmt.Println()

	for _, actionName := range []string{"优化整句", "中译英"} {
		if err := runAction(cfg, actionName, input); err != nil {
			fail("run action "+actionName, err)
		}
	}
}

func runAction(cfg *fileConfig, actionName string, input aiInput) error {
	actionPrompt, err := findActionPrompt(cfg, actionName)
	if err != nil {
		return err
	}

	userPrompt := buildPrompt(actionPrompt, input)
	fmt.Printf("=== %s ===\n", actionName)
	fmt.Println("prompt:")
	fmt.Println(userPrompt)
	fmt.Println()

	results, err := callAI(cfg, userPrompt)
	if err != nil {
		return err
	}

	fmt.Println("AI result:")
	for i, item := range results {
		fmt.Printf("%d. %s\n", i+1, item)
	}
	fmt.Println()
	return nil
}

func siblingPath(relative string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), relative))
}

func loadConfig(path string) (*fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg fileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.API.BaseURL = strings.TrimRight(strings.TrimSpace(firstNonEmpty(cfg.API.BaseURL, os.Getenv("MOQI_AI_BASE_URL"))), "/")
	cfg.API.APIKey = strings.TrimSpace(firstNonEmpty(cfg.API.APIKey, os.Getenv("MOQI_AI_API_KEY")))
	cfg.API.Model = strings.TrimSpace(firstNonEmpty(cfg.API.Model, os.Getenv("MOQI_AI_MODEL")))
	fmt.Printf("cfg.API.BaseURL: %s\n", cfg.API.BaseURL)
	fmt.Printf("cfg.API.APIKey: %s\n", cfg.API.APIKey)
	fmt.Printf("cfg.API.Model: %s\n", cfg.API.Model)
	if cfg.API.BaseURL == "" || cfg.API.APIKey == "" || cfg.API.Model == "" {
		return nil, fmt.Errorf("missing api config: base_url/api_key/model")
	}
	return &cfg, nil
}

func findActionPrompt(cfg *fileConfig, actionName string) (string, error) {
	for _, action := range cfg.Actions {
		if strings.TrimSpace(action.Name) == actionName {
			return strings.TrimSpace(action.Prompt), nil
		}
	}
	return "", fmt.Errorf("action %q not found", actionName)
}

func loadInput(path string) (aiInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return aiInput{}, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return aiInput{
			PreviousCommit: "人们在春节和元宵节期间也制做冰灯摆在门前",
			Composition:    "tangkongchuanshengranghaizitizhewan",
			Candidates:     []string{"糖空传声让孩子提着万"},
		}, nil
	}

	lines := make([]string, 0)
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return aiInput{}, fmt.Errorf("input file is empty")
	}

	// First line is previous commit, last line is the raw IM input, middle lines are candidate texts.
	input := aiInput{
		PreviousCommit: lines[0],
		Composition:    lines[len(lines)-1],
	}
	if len(lines) > 2 {
		input.Candidates = append(input.Candidates, lines[1:len(lines)-1]...)
	} else if len(lines) == 2 {
		input.Candidates = append(input.Candidates, lines[0])
	}
	if len(input.Candidates) > 3 {
		input.Candidates = input.Candidates[:3]
	}
	return input, nil
}

func buildPrompt(prompt string, input aiInput) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		prompt = "上一句是：{{previous_commit}}\n原始输入“{{composition}}”和前三个候选词：\n{{candidates_top3}}\n只输出 1 条最通顺、最自然的整句候选，不要解释。"
	}

	replacements := map[string]string{
		"{{previous_commit}}":  previousCommitText(input.PreviousCommit),
		"{{composition}}":      input.Composition,
		"{{raw_input}}":        input.Composition,
		"{{candidate_1}}":      candidateAt(input.Candidates, 0),
		"{{candidate_2}}":      candidateAt(input.Candidates, 1),
		"{{candidate_3}}":      candidateAt(input.Candidates, 2),
		"{{first_candidate}}":  candidateAt(input.Candidates, 0),
		"{{second_candidate}}": candidateAt(input.Candidates, 1),
		"{{third_candidate}}":  candidateAt(input.Candidates, 2),
		"{{candidates_top3}}":  joinCandidates(input.Candidates),
	}

	replaced := false
	for old, newValue := range replacements {
		if strings.Contains(prompt, old) {
			prompt = strings.ReplaceAll(prompt, old, newValue)
			replaced = true
		}
	}
	if replaced {
		return prompt
	}
	return prompt + "\n\n上一句：" + previousCommitText(input.PreviousCommit) + "\n原始输入：" + input.Composition + "\n前三个候选词：\n" + joinCandidates(input.Candidates)
}

func candidateAt(candidates []string, index int) string {
	if index < 0 || index >= len(candidates) {
		return ""
	}
	return candidates[index]
}

func joinCandidates(candidates []string) string {
	if len(candidates) == 0 {
		return "无"
	}
	lines := make([]string, 0, len(candidates))
	for i, candidate := range candidates {
		lines = append(lines, fmt.Sprintf("%d. %s", i+1, candidate))
	}
	return strings.Join(lines, "\n")
}

func previousCommitText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "无"
	}
	return value
}

func callAI(cfg *fileConfig, userPrompt string) ([]string, error) {
	payload := chatCompletionsRequest{
		Model: cfg.API.Model,
		Messages: []chatMessage{
			{
				Role:    "system",
				Content: "你是一个中文输入法候选优化助手。请严格按用户要求输出候选文案，每条单独一行，不要编号，不要项目符号，不要解释，不要输出思考过程。",
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Temperature: 0.4,
		Thinking: thinkingMode{
			Type: "disabled",
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, cfg.API.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.API.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("api returned %d (%s), body preview: %s", resp.StatusCode, resp.Status, previewResponseBody(respBody, 200))
	}

	var parsed chatCompletionsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("decode response JSON failed, status=%s, body preview: %s, err: %w", resp.Status, previewResponseBody(respBody, 200), err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("api returned no choices")
	}
	return parseCandidates(parsed.Choices[0].Message.Content), nil
}

func parseCandidates(content string) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	results := make([]string, 0, len(lines))
	seen := map[string]struct{}{}
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimLeft(line, "-*0123456789.、)） \t"))
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		results = append(results, line)
	}
	if len(results) == 0 && strings.TrimSpace(content) != "" {
		results = append(results, strings.TrimSpace(content))
	}
	return results
}

func previewResponseBody(body []byte, limit int) string {
	text := strings.TrimSpace(strings.ReplaceAll(string(body), "\r\n", "\n"))
	if text == "" {
		return "<empty>"
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func fail(step string, err error) {
	fmt.Fprintf(os.Stderr, "%s failed: %v\n", step, err)
	os.Exit(1)
}
