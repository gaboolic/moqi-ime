//go:build !windows || !cgo

package rime

import "fmt"

func newConfiguredBertReranker(cfg *bertRuntimeConfig) (bertReranker, error) {
	if cfg == nil {
		return nil, nil
	}
	return nil, fmt.Errorf("BERT ONNX reranker requires Windows with CGO enabled")
}
