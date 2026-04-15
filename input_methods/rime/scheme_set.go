package rime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultSchemeSetName    = "Rime"
	schemeSetConfigFileName = "scheme_set.json"
)

type schemeSetConfig struct {
	Current string `json:"current"`
}

func moqiAppDataDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, APP)
}

func normalizeSchemeSetName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return defaultSchemeSetName
	}
	if name == "." || name == ".." || strings.ContainsAny(name, `\/`) {
		return defaultSchemeSetName
	}
	return name
}

func schemeSetConfigPath() string {
	root := moqiAppDataDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, schemeSetConfigFileName)
}

func availableSchemeSets() []string {
	root := moqiAppDataDir()
	if root == "" {
		return []string{defaultSchemeSetName}
	}

	entries, err := os.ReadDir(root)
	names := make([]string, 0, len(entries)+1)
	seen := map[string]struct{}{defaultSchemeSetName: {}}
	names = append(names, defaultSchemeSetName)
	if err != nil {
		return names
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := normalizeSchemeSetName(entry.Name())
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	sort.Slice(names[1:], func(i, j int) bool {
		left := strings.ToLower(names[i+1])
		right := strings.ToLower(names[j+1])
		if left == right {
			return names[i+1] < names[j+1]
		}
		return left < right
	})
	return names
}

func currentSchemeSetName() string {
	names := availableSchemeSets()
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}

	path := schemeSetConfigPath()
	if path == "" {
		return defaultSchemeSetName
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return defaultSchemeSetName
	}

	var cfg schemeSetConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultSchemeSetName
	}
	name := normalizeSchemeSetName(cfg.Current)
	if _, ok := allowed[name]; !ok {
		return defaultSchemeSetName
	}
	return name
}

func saveCurrentSchemeSetName(name string) bool {
	root := moqiAppDataDir()
	if root == "" {
		return false
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return false
	}

	name = normalizeSchemeSetName(name)
	data, err := json.MarshalIndent(schemeSetConfig{Current: name}, "", "  ")
	if err != nil {
		return false
	}
	return os.WriteFile(schemeSetConfigPath(), data, 0o644) == nil
}

func schemeSetMenuItems() []map[string]interface{} {
	names := availableSchemeSets()
	current := currentSchemeSetName()
	items := make([]map[string]interface{}, 0, len(names))
	for i, name := range names {
		items = append(items, map[string]interface{}{
			"id":      schemeSetCommandID(i),
			"text":    name,
			"checked": name == current,
		})
	}
	return items
}
