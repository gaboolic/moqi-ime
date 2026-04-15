package rime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const defaultTemplatesDirName = "templates"

func defaultTemplateSearchRoots() []string {
	roots := make([]string, 0, 2)
	if exePath, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Join(filepath.Dir(exePath), "input_methods", "rime"))
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		roots = append(roots, filepath.Dir(file))
	}
	return roots
}

func loadDefaultTemplate(templateName string) ([]byte, error) {
	for _, root := range defaultTemplateSearchRoots() {
		path := filepath.Join(root, defaultTemplatesDirName, templateName)
		if data, err := os.ReadFile(path); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("default template not found: %s", templateName)
}
