package rime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseBertConfigJSONAppliesDefaultsAndResolvesRelativePaths(t *testing.T) {
	t.Setenv("PROGRAMFILES(X86)", filepath.Join(t.TempDir(), "ProgramFilesX86"))
	baseDir := filepath.Join(t.TempDir(), "bert")
	cfg, err := parseBertConfigJSON([]byte(`{
		"enabled": true,
		"provider": "onnx_cross_encoder",
		"model_path": "model.onnx",
		"vocab_path": "vocab.txt",
		"runtime_library_path": "onnxruntime.dll"
	}`), baseDir)
	if err != nil {
		t.Fatalf("expected config to parse, got %v", err)
	}
	if !cfg.Enabled {
		t.Fatal("expected config enabled")
	}
	if cfg.ModelPath != filepath.Join(baseDir, "model.onnx") {
		t.Fatalf("unexpected model path %q", cfg.ModelPath)
	}
	if cfg.VocabPath != filepath.Join(baseDir, "vocab.txt") {
		t.Fatalf("unexpected vocab path %q", cfg.VocabPath)
	}
	if cfg.RuntimeLibraryPath != filepath.Join(baseDir, "onnxruntime.dll") {
		t.Fatalf("unexpected runtime path %q", cfg.RuntimeLibraryPath)
	}
	if cfg.MaxSequenceLength != 96 || cfg.MaxCandidates != 5 || cfg.LeftContextRunes != 48 {
		t.Fatalf("expected defaults applied, got %#v", cfg)
	}
	if cfg.PositiveLabelIndex != 0 {
		t.Fatalf("expected zero-value positive label index by default, got %d", cfg.PositiveLabelIndex)
	}
}

func TestParseBertConfigJSONFallsBackToInstalledBertDir(t *testing.T) {
	programFiles := filepath.Join(t.TempDir(), "ProgramFilesX86")
	t.Setenv("PROGRAMFILES(X86)", programFiles)
	installDir := filepath.Join(programFiles, "MoqiIM", "bert")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		t.Fatalf("create installed bert dir: %v", err)
	}
	for _, name := range []string{"model.onnx", "vocab.txt", "onnxruntime.dll"} {
		if err := os.WriteFile(filepath.Join(installDir, name), []byte(name), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	cfg, err := parseBertConfigJSON([]byte(`{
		"enabled": true,
		"provider": "onnx_cross_encoder",
		"model_path": "bert/model.onnx",
		"vocab_path": "bert/vocab.txt",
		"runtime_library_path": "bert/onnxruntime.dll"
	}`), filepath.Join(t.TempDir(), "usercfg"))
	if err != nil {
		t.Fatalf("expected config to parse, got %v", err)
	}
	if cfg.ModelPath != filepath.Join(installDir, "model.onnx") {
		t.Fatalf("unexpected installed model path %q", cfg.ModelPath)
	}
	if cfg.VocabPath != filepath.Join(installDir, "vocab.txt") {
		t.Fatalf("unexpected installed vocab path %q", cfg.VocabPath)
	}
	if cfg.RuntimeLibraryPath != filepath.Join(installDir, "onnxruntime.dll") {
		t.Fatalf("unexpected installed runtime path %q", cfg.RuntimeLibraryPath)
	}
}

func TestParseBertConfigJSONAllowsDisabledConfigWithoutModel(t *testing.T) {
	cfg, err := parseBertConfigJSON([]byte(`{"enabled": false}`), t.TempDir())
	if err != nil {
		t.Fatalf("expected disabled config to parse, got %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Enabled {
		t.Fatal("expected disabled config")
	}
}

func TestCopyBertConfigIfMissingCopiesBundledTemplate(t *testing.T) {
	root := t.TempDir()
	srcPath := filepath.Join(root, "src.json")
	dstPath := filepath.Join(root, "nested", "dst.json")
	content := []byte(`{"enabled": false}`)
	if err := os.WriteFile(srcPath, content, 0o644); err != nil {
		t.Fatalf("write source config: %v", err)
	}
	if err := copyBertConfigIfMissing(dstPath, srcPath); err != nil {
		t.Fatalf("copy BERT config: %v", err)
	}
	data, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read copied config: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("unexpected copied content %q", data)
	}
}
