package rime

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gaboolic/moqi-ime/imecore"
)

const (
	superAbbrevFileName         = "moqi_super_abbrev.txt"
	superAbbrevTemplateFileName = "moqi_super_abbrev.txt"
	ID_OPEN_SUPER_ABBREV        = 19
	superAbbrevCommitMark       = "⚡"
	superAbbrevMaxCodeLen       = 3
)

type superAbbrevCache struct {
	mu      sync.Mutex
	modTime time.Time
	size    int64
	entries map[string]string
}

type superAbbrevOverlay struct {
	Text      string
	Synthetic bool
}

var (
	sharedSuperAbbrevCache    superAbbrevCache
	openSuperAbbrevTargetFunc = openWithDefaultApp
)

func superAbbrevFilePath() string {
	root := moqiAppDataDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, superAbbrevFileName)
}

func ensureSuperAbbrevFileExists() (string, error) {
	path := superAbbrevFilePath()
	if path == "" {
		return "", os.ErrNotExist
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	content, err := loadDefaultTemplate(superAbbrevTemplateFileName)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func loadSuperAbbrevEntries() (map[string]string, error) {
	path := superAbbrevFilePath()
	if path == "" {
		return nil, os.ErrNotExist
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	sharedSuperAbbrevCache.mu.Lock()
	defer sharedSuperAbbrevCache.mu.Unlock()

	if !sharedSuperAbbrevCache.modTime.IsZero() &&
		sharedSuperAbbrevCache.modTime.Equal(info.ModTime()) &&
		sharedSuperAbbrevCache.size == info.Size() {
		return cloneStringMap(sharedSuperAbbrevCache.entries), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)
	entries := make(map[string]string)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		text := strings.TrimSpace(parts[0])
		code := strings.ToLower(strings.TrimSpace(parts[1]))
		if text == "" || code == "" {
			continue
		}
		if _, exists := entries[code]; !exists {
			entries[code] = text
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sharedSuperAbbrevCache.modTime = info.ModTime()
	sharedSuperAbbrevCache.size = info.Size()
	sharedSuperAbbrevCache.entries = cloneStringMap(entries)
	return cloneStringMap(entries), nil
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(src))
	for key, value := range src {
		cloned[key] = value
	}
	return cloned
}

func resetSuperAbbrevCacheForTest() {
	sharedSuperAbbrevCache.mu.Lock()
	sharedSuperAbbrevCache.modTime = time.Time{}
	sharedSuperAbbrevCache.size = 0
	sharedSuperAbbrevCache.entries = nil
	sharedSuperAbbrevCache.mu.Unlock()
}

func lookupSuperAbbrev(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return ""
	}
	entries, err := loadSuperAbbrevEntries()
	if err != nil {
		log.Printf("加载超级简拼失败: %v", err)
		return ""
	}
	return strings.TrimSpace(entries[code])
}

func isASCIIAlphaLower(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func (ime *IME) superAbbrevMatchInput(state rimeState) string {
	if shouldSuppressCustomPhraseOverlay(state) {
		return ""
	}
	matchInput := strings.TrimSpace(state.RawInput)
	if matchInput == "" {
		matchInput = strings.TrimSpace(ime.rawInputTracked)
	}
	if matchInput == "" {
		matchInput = strings.TrimSpace(state.Composition)
	}
	matchInput = strings.ToLower(strings.TrimSpace(matchInput))
	if matchInput == "" {
		return ""
	}
	if utf8.RuneCountInString(matchInput) > superAbbrevMaxCodeLen {
		return ""
	}
	if !isASCIIAlphaLower(matchInput) {
		return ""
	}
	return matchInput
}

func (ime *IME) currentSuperAbbrevOverlay() (rimeState, superAbbrevOverlay, bool) {
	if ime.aiActive {
		return rimeState{}, superAbbrevOverlay{}, false
	}
	state, ok := ime.currentVisibleBackendState()
	if !ok {
		return rimeState{}, superAbbrevOverlay{}, false
	}

	matchInput := ime.superAbbrevMatchInput(state)
	if matchInput == "" {
		return state, superAbbrevOverlay{}, false
	}
	text := lookupSuperAbbrev(matchInput)
	if text == "" {
		return state, superAbbrevOverlay{}, false
	}
	if len(state.Candidates) > 0 {
		if strings.TrimSpace(state.Candidates[0].Text) == text {
			return state, superAbbrevOverlay{}, false
		}
		return state, superAbbrevOverlay{Text: text}, true
	}
	return state, superAbbrevOverlay{Text: text, Synthetic: true}, true
}

func applySuperAbbrevOverlay(state rimeState, overlay superAbbrevOverlay) rimeState {
	if strings.TrimSpace(overlay.Text) == "" {
		return state
	}
	if len(state.Candidates) > 0 {
		state.Candidates = append([]candidateItem(nil), state.Candidates...)
		state.Candidates[0].Comment = overlay.Text
		return state
	}
	state.Candidates = []candidateItem{{
		Text:    overlay.Text,
		Comment: superAbbrevCommitMark,
	}}
	state.CandidateCursor = 0
	if state.SelectKeys == "" {
		state.SelectKeys = "1"
	}
	return state
}

func applySuperAbbrevOverlayToCandidates(candidates []candidateItem, overlay superAbbrevOverlay) []candidateItem {
	if len(candidates) == 0 || strings.TrimSpace(overlay.Text) == "" {
		return candidates
	}
	if strings.TrimSpace(candidates[0].Text) == overlay.Text {
		return candidates
	}
	cloned := append([]candidateItem(nil), candidates...)
	cloned[0].Comment = overlay.Text
	return cloned
}

func isSuperAbbrevCommitKey(req *imecore.Request) bool {
	if req == nil {
		return false
	}
	if req.KeyStates.IsKeyDown(vkShift) || req.KeyStates.IsKeyDown(vkControl) || req.KeyStates.IsKeyDown(vkMenu) {
		return false
	}
	if req.KeyCode == vkTab || req.KeyCode == vkOemPeriod {
		return true
	}
	return req.CharCode == int('.')
}

func superAbbrevConsumeCode(req *imecore.Request) int {
	if req == nil {
		return 0
	}
	if req.KeyCode == vkTab {
		return vkTab
	}
	if req.KeyCode == vkOemPeriod || req.CharCode == int('.') {
		return int('.')
	}
	return 0
}

func (ime *IME) resetSuperAbbrevOverlay() {
	ime.superAbbrevConsumeKeyUpCode = 0
}

func (ime *IME) handleSuperAbbrevKeyDownFilter(req *imecore.Request, resp *imecore.Response) bool {
	if !isSuperAbbrevCommitKey(req) {
		return false
	}
	_, overlay, ok := ime.currentSuperAbbrevOverlay()
	if !ok || strings.TrimSpace(overlay.Text) == "" {
		return false
	}
	ime.superAbbrevConsumeKeyUpCode = superAbbrevConsumeCode(req)
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleSuperAbbrevKeyUpFilter(req *imecore.Request, resp *imecore.Response) bool {
	if ime.superAbbrevConsumeKeyUpCode == 0 || superAbbrevConsumeCode(req) != ime.superAbbrevConsumeKeyUpCode {
		return false
	}
	resp.ReturnValue = 1
	return true
}

func (ime *IME) commitSuperAbbrev(resp *imecore.Response, text string) {
	if resp == nil || strings.TrimSpace(text) == "" {
		return
	}
	resp.CommitString = text
	ime.rememberAICommit(text)
	ime.clearResponse(resp)
	ime.resetTrackedRawInput()
	if ime.backend != nil {
		ime.backend.ClearComposition()
	}
	ime.resetCustomPhraseOverlay()
	ime.resetSuperAbbrevOverlay()
	ime.keyComposing = false
	ime.selectKeys = ""
}

func (ime *IME) handleSuperAbbrevKeyDown(req *imecore.Request, resp *imecore.Response) bool {
	if ime.superAbbrevConsumeKeyUpCode == 0 || superAbbrevConsumeCode(req) != ime.superAbbrevConsumeKeyUpCode {
		return false
	}
	_, overlay, ok := ime.currentSuperAbbrevOverlay()
	if !ok || strings.TrimSpace(overlay.Text) == "" {
		ime.fillResponseFromCurrentState(resp)
		resp.ReturnValue = 1
		return true
	}
	ime.commitSuperAbbrev(resp, overlay.Text)
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleSuperAbbrevKeyUp(req *imecore.Request, resp *imecore.Response) bool {
	if ime.superAbbrevConsumeKeyUpCode == 0 || superAbbrevConsumeCode(req) != ime.superAbbrevConsumeKeyUpCode {
		return false
	}
	ime.resetSuperAbbrevOverlay()
	ime.fillResponseFromCurrentState(resp)
	resp.ReturnValue = 1
	return true
}

func (ime *IME) openSuperAbbrevFile(resp *imecore.Response) bool {
	path, err := ensureSuperAbbrevFileExists()
	if err != nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("创建超级简拼文件失败", imecore.TrayNotificationIconError)
		}
		return false
	}
	if err := openSuperAbbrevTargetFunc(path); err != nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("打开超级简拼失败", imecore.TrayNotificationIconError)
		}
		return false
	}
	return true
}
