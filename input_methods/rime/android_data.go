//go:build android

package rime

import (
	"embed"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed data/**
var androidRimeData embed.FS

var (
	androidDirsOnce  sync.Once
	androidSharedDir string
	androidDirsOK    bool

	androidDataRootMu sync.Mutex
	androidBaseDir    string
)

func SetAndroidDataDir(path string) {
	path = strings.TrimSpace(path)
	androidDataRootMu.Lock()
	defer androidDataRootMu.Unlock()
	if androidBaseDir == path {
		return
	}
	androidBaseDir = path
	androidDirsOnce = sync.Once{}
	androidSharedDir = ""
	androidDirsOK = false
}

func androidAppDataRoot() string {
	androidDataRootMu.Lock()
	baseDir := androidBaseDir
	androidDataRootMu.Unlock()

	if baseDir == "" {
		if configDir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(configDir) != "" {
			baseDir = configDir
		} else {
			baseDir = os.TempDir()
		}
	}
	if strings.TrimSpace(baseDir) == "" {
		return ""
	}
	return filepath.Join(baseDir, APP)
}

func androidRimeDirs() (sharedDir string, userDir string, ok bool) {
	androidDirsOnce.Do(func() {
		root := androidAppDataRoot()
		if root == "" {
			log.Printf("Android RIME 数据目录不可用")
			return
		}

		androidSharedDir = filepath.Join(root, "rime_shared")
		if err := extractAndroidRimeData(androidSharedDir); err != nil {
			log.Printf("解压 Android RIME 数据失败: %v", err)
			return
		}
		androidDirsOK = true
		log.Printf("Android RIME shared 数据目录已就绪 shared=%s", androidSharedDir)
	})
	if !androidDirsOK {
		return androidSharedDir, "", false
	}
	root := androidAppDataRoot()
	if root == "" {
		return androidSharedDir, "", false
	}
	userDir = filepath.Join(root, currentSchemeSetName())
	if err := os.MkdirAll(userDir, 0o700); err != nil {
		log.Printf("创建 Android RIME 用户目录失败: %v", err)
		return androidSharedDir, userDir, false
	}
	return androidSharedDir, userDir, true
}

func extractAndroidRimeData(dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return fs.WalkDir(androidRimeData, "data", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("data", path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		content, err := androidRimeData.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
}
