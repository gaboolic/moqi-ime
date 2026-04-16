package rime

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	bertConfigFileName       = "bert_config.json"
	bertProviderCrossEncoder = "onnx_cross_encoder"
	bertModelDownloadURL     = "https://github.com/gaboolic/moqi-ime/releases"
)

type bertFileConfig struct {
	Enabled               bool     `json:"enabled"`
	Provider              string   `json:"provider"`
	ModelPath             string   `json:"model_path"`
	VocabPath             string   `json:"vocab_path"`
	RuntimeLibraryPath    string   `json:"runtime_library_path"`
	LowerCase             *bool    `json:"lower_case,omitempty"`
	MaxSequenceLength     int      `json:"max_sequence_length"`
	MaxCandidates         int      `json:"max_candidates"`
	LeftContextRunes      int      `json:"left_context_runes"`
	PositiveLabelIndex    int      `json:"positive_label_index"`
	CacheTTLSeconds       int      `json:"cache_ttl_seconds"`
	MinSentenceInputChars int      `json:"min_sentence_input_chars"`
	AsyncDebounceMS       int      `json:"async_debounce_ms"`
	InputNames            []string `json:"input_names"`
	OutputNames           []string `json:"output_names"`
}

type bertRuntimeConfig struct {
	Enabled               bool
	Provider              string
	ModelPath             string
	VocabPath             string
	RuntimeLibraryPath    string
	LowerCase             bool
	MaxSequenceLength     int
	MaxCandidates         int
	LeftContextRunes      int
	PositiveLabelIndex    int
	CacheTTL              time.Duration
	MinSentenceInputChars int
	AsyncDebounceMS       int
	InputNames            []string
	OutputNames           []string
}

func loadBertConfig() (*bertRuntimeConfig, error) {
	if err := ensureUserBertConfigCopied(); err != nil {
		return nil, err
	}
	for _, path := range bertConfigSearchPaths() {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read BERT config %s: %w", path, err)
		}
		return parseBertConfigJSON(data, filepath.Dir(path))
	}
	return nil, nil
}

func bertConfigSearchPaths() []string {
	paths := make([]string, 0, 3)
	if path := userBertConfigPath(); path != "" {
		paths = append(paths, path)
	}
	if path := legacyUserBertConfigPath(); path != "" {
		paths = append(paths, path)
	}
	if path := bundledBertConfigPath(); path != "" {
		paths = append(paths, path)
	}
	return paths
}

func userBertConfigPath() string {
	root := moqiAppDataDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, bertConfigFileName)
}

func legacyUserBertConfigPath() string {
	root := moqiAppDataDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, defaultSchemeSetName, bertConfigFileName)
}

func bundledBertConfigPath() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exePath), "input_methods", "rime", bertConfigFileName)
}

func bundledBertAssetsDir() string {
	path := bundledBertConfigPath()
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}

func installedBertAssetsDir() string {
	root := os.Getenv("PROGRAMFILES(X86)")
	if strings.TrimSpace(root) == "" {
		root = `C:\Program Files (x86)`
	}
	return filepath.Join(root, "MoqiIM", "bert")
}

func ensureUserBertConfigCopied() error {
	return copyBertConfigIfMissing(userBertConfigPath(), bundledBertConfigPath())
}

func copyBertConfigIfMissing(dstPath, srcPath string) error {
	if dstPath == "" || srcPath == "" {
		return nil
	}
	if _, err := os.Stat(dstPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat BERT config %s: %w", dstPath, err)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open bundled BERT config %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("create BERT config directory %s: %w", filepath.Dir(dstPath), err)
	}

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return fmt.Errorf("create user BERT config %s: %w", dstPath, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy BERT config to %s: %w", dstPath, err)
	}
	return nil
}

func parseBertConfigJSON(data []byte, baseDir string) (*bertRuntimeConfig, error) {
	var raw bertFileConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode BERT config JSON: %w", err)
	}

	cfg := &bertRuntimeConfig{
		Enabled:               raw.Enabled,
		Provider:              strings.TrimSpace(raw.Provider),
		ModelPath:             resolveBertConfigPath(baseDir, bundledBertAssetsDir(), installedBertAssetsDir(), raw.ModelPath),
		VocabPath:             resolveBertConfigPath(baseDir, bundledBertAssetsDir(), installedBertAssetsDir(), raw.VocabPath),
		RuntimeLibraryPath:    resolveBertConfigPath(baseDir, bundledBertAssetsDir(), installedBertAssetsDir(), raw.RuntimeLibraryPath),
		LowerCase:             true,
		MaxSequenceLength:     raw.MaxSequenceLength,
		MaxCandidates:         raw.MaxCandidates,
		LeftContextRunes:      raw.LeftContextRunes,
		PositiveLabelIndex:    raw.PositiveLabelIndex,
		MinSentenceInputChars: raw.MinSentenceInputChars,
		AsyncDebounceMS:       raw.AsyncDebounceMS,
		InputNames:            normalizeStringList(raw.InputNames),
		OutputNames:           normalizeStringList(raw.OutputNames),
	}
	if raw.LowerCase != nil {
		cfg.LowerCase = *raw.LowerCase
	}
	if cfg.Provider == "" {
		cfg.Provider = bertProviderCrossEncoder
	}
	if cfg.MaxSequenceLength <= 0 {
		cfg.MaxSequenceLength = 96
	}
	if cfg.MaxCandidates <= 0 {
		cfg.MaxCandidates = 5
	}
	if cfg.LeftContextRunes <= 0 {
		cfg.LeftContextRunes = 48
	}
	if raw.CacheTTLSeconds <= 0 {
		cfg.CacheTTL = defaultBertCacheTTL
	} else {
		cfg.CacheTTL = time.Duration(raw.CacheTTLSeconds) * time.Second
	}
	if cfg.MinSentenceInputChars <= 0 {
		cfg.MinSentenceInputChars = defaultBertMinSentenceInputChars
	}
	if cfg.AsyncDebounceMS <= 0 {
		cfg.AsyncDebounceMS = defaultBertAsyncDebounceDelayMS
	}
	if !cfg.Enabled {
		return cfg, nil
	}
	if cfg.Provider != bertProviderCrossEncoder {
		return nil, fmt.Errorf("unsupported BERT provider %q", cfg.Provider)
	}
	if cfg.ModelPath == "" {
		return nil, fmt.Errorf("BERT config requires model_path when enabled")
	}
	if cfg.VocabPath == "" {
		return nil, fmt.Errorf("BERT config requires vocab_path when enabled")
	}
	return cfg, nil
}

func resolveBertConfigPath(baseDir, bundledBaseDir, installedBaseDir, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if filepath.IsAbs(value) {
		return value
	}
	if baseDir == "" {
		return value
	}
	resolved := filepath.Join(baseDir, value)
	if _, err := os.Stat(resolved); err == nil {
		return resolved
	}
	if bundledBaseDir != "" {
		bundled := filepath.Join(bundledBaseDir, value)
		if _, err := os.Stat(bundled); err == nil {
			return bundled
		}
	}
	if installedBaseDir != "" {
		installed := filepath.Join(installedBaseDir, filepath.Base(value))
		if _, err := os.Stat(installed); err == nil {
			return installed
		}
	}
	return resolved
}

func bertExternalDataPath(modelPath string) string {
	modelPath = strings.TrimSpace(modelPath)
	if modelPath == "" {
		return ""
	}
	return modelPath + ".data"
}

func bertInstallDir(cfg *bertRuntimeConfig) string {
	if dir := installedBertAssetsDir(); dir != "" {
		return dir
	}
	if cfg == nil {
		return ""
	}
	if dir := filepath.Dir(strings.TrimSpace(cfg.ModelPath)); dir != "" && dir != "." {
		return dir
	}
	if dir := filepath.Dir(strings.TrimSpace(cfg.VocabPath)); dir != "" && dir != "." {
		return dir
	}
	return ""
}

func bertMissingAssetPaths(cfg *bertRuntimeConfig) []string {
	if cfg == nil {
		return nil
	}
	required := []string{
		strings.TrimSpace(cfg.ModelPath),
		strings.TrimSpace(bertExternalDataPath(cfg.ModelPath)),
		strings.TrimSpace(cfg.VocabPath),
	}
	missing := make([]string, 0, len(required))
	for _, path := range required {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			continue
		}
		missing = append(missing, path)
	}
	return missing
}

func bertMissingAssetsMessage(cfg *bertRuntimeConfig) string {
	installDir := bertInstallDir(cfg)
	if installDir == "" {
		installDir = `C:\Program Files (x86)\MoqiIM\bert`
	}
	return fmt.Sprintf(
		"BERT 模型文件缺失。\n请从 %s 下载模型包，并将 model.onnx、model.onnx.data、vocab.txt 放到：\n%s",
		bertModelDownloadURL,
		installDir,
	)
}

func normalizeStringList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
