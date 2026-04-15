package rime

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type debugLogLevel int

const (
	debugLogLevelOff debugLogLevel = iota
	debugLogLevelDebug
	debugLogLevelTrace
)

var debugLogState struct {
	mu          sync.Mutex
	lastRefresh time.Time
	level       debugLogLevel
}

type launcherDebugConfig struct {
	LogLevel string `json:"logLevel"`
}

func debugConfigPath() string {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		return ""
	}
	return filepath.Join(localAppData, "MoqiIM", "MoqiLauncher.json")
}

func parseDebugLogLevel(raw string) debugLogLevel {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "trace":
		return debugLogLevelTrace
	case "debug":
		return debugLogLevelDebug
	default:
		return debugLogLevelOff
	}
}

func currentDebugLogLevel() debugLogLevel {
	debugLogState.mu.Lock()
	defer debugLogState.mu.Unlock()

	now := time.Now()
	if !debugLogState.lastRefresh.IsZero() && now.Sub(debugLogState.lastRefresh) < time.Second {
		return debugLogState.level
	}

	debugLogState.level = debugLogLevelOff
	path := debugConfigPath()
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			var cfg launcherDebugConfig
			if json.Unmarshal(data, &cfg) == nil {
				debugLogState.level = parseDebugLogLevel(cfg.LogLevel)
			}
		}
	}

	debugLogState.lastRefresh = now
	return debugLogState.level
}

func isDebugLoggingEnabled() bool {
	return currentDebugLogLevel() >= debugLogLevelDebug
}

func isTraceLoggingEnabled() bool {
	return currentDebugLogLevel() >= debugLogLevelTrace
}

func debugLogf(format string, args ...any) {
	if isDebugLoggingEnabled() {
		log.Printf(format, args...)
	}
}

func traceLogf(format string, args ...any) {
	if isTraceLoggingEnabled() {
		log.Printf(format, args...)
	}
}
