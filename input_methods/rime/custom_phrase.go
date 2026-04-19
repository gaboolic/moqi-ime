package rime

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

const (
	customPhraseFileName         = "moqi_custom_phrase.txt"
	ID_OPEN_CUSTOM_PHRASE        = 18
	customPhraseTemplateFileName = "moqi_custom_phrase.txt"
)

type customPhraseEntry struct {
	Text   string
	Code   string
	Weight int
	Order  int
}

type customPhraseCache struct {
	mu      sync.Mutex
	modTime time.Time
	size    int64
	entries []customPhraseEntry
}

var (
	sharedCustomPhraseCache    customPhraseCache
	openCustomPhraseTargetFunc = openWithDefaultApp
)

func openWithDefaultApp(target string) error {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", target).Start()
}

func customPhraseFilePath() string {
	root := moqiAppDataDir()
	if root == "" {
		return ""
	}
	return filepath.Join(root, customPhraseFileName)
}

func ensureCustomPhraseFileExists() (string, error) {
	path := customPhraseFilePath()
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
	content, err := loadDefaultTemplate(customPhraseTemplateFileName)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func loadCustomPhraseEntries() ([]customPhraseEntry, error) {
	path, err := ensureCustomPhraseFileExists()
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	sharedCustomPhraseCache.mu.Lock()
	defer sharedCustomPhraseCache.mu.Unlock()

	if !sharedCustomPhraseCache.modTime.IsZero() &&
		sharedCustomPhraseCache.modTime.Equal(info.ModTime()) &&
		sharedCustomPhraseCache.size == info.Size() {
		return append([]customPhraseEntry(nil), sharedCustomPhraseCache.entries...), nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024), 1024*1024)
	entries := make([]customPhraseEntry, 0)
	order := 0
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
		code := strings.TrimSpace(parts[1])
		if text == "" || code == "" {
			continue
		}
		weight := 0
		if len(parts) >= 3 {
			if parsed, err := strconv.Atoi(strings.TrimSpace(parts[2])); err == nil {
				weight = parsed
			}
		}
		entries = append(entries, customPhraseEntry{
			Text:   text,
			Code:   code,
			Weight: weight,
			Order:  order,
		})
		order++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sharedCustomPhraseCache.modTime = info.ModTime()
	sharedCustomPhraseCache.size = info.Size()
	sharedCustomPhraseCache.entries = append([]customPhraseEntry(nil), entries...)
	return append([]customPhraseEntry(nil), entries...), nil
}

func resetCustomPhraseCacheForTest() {
	sharedCustomPhraseCache.mu.Lock()
	sharedCustomPhraseCache.modTime = time.Time{}
	sharedCustomPhraseCache.size = 0
	sharedCustomPhraseCache.entries = nil
	sharedCustomPhraseCache.mu.Unlock()
}

func lookupCustomPhraseCandidates(composition string, limit int) []candidateItem {
	composition = strings.TrimSpace(strings.ToLower(composition))
	if composition == "" || limit <= 0 {
		return nil
	}

	entries, err := loadCustomPhraseEntries()
	if err != nil {
		log.Printf("加载置顶短语失败: %v", err)
		return nil
	}

	matches := make([]customPhraseEntry, 0)
	for _, entry := range entries {
		code := strings.ToLower(strings.TrimSpace(entry.Code))
		if code == "" || !strings.HasPrefix(code, composition) {
			continue
		}
		matches = append(matches, entry)
	}

	sort.SliceStable(matches, func(i, j int) bool {
		leftExact := strings.EqualFold(matches[i].Code, composition)
		rightExact := strings.EqualFold(matches[j].Code, composition)
		if leftExact != rightExact {
			return leftExact
		}
		if matches[i].Weight != matches[j].Weight {
			return matches[i].Weight > matches[j].Weight
		}
		if len(matches[i].Code) != len(matches[j].Code) {
			return len(matches[i].Code) < len(matches[j].Code)
		}
		return matches[i].Order < matches[j].Order
	})

	candidates := make([]candidateItem, 0, min(limit, len(matches)))
	seen := make(map[string]struct{}, len(matches))
	for _, entry := range matches {
		if _, ok := seen[entry.Text]; ok {
			continue
		}
		seen[entry.Text] = struct{}{}
		candidates = append(candidates, candidateItem{Text: entry.Text})
		if len(candidates) == limit {
			break
		}
	}
	return candidates
}

func cloneCandidateItems(items []candidateItem) []candidateItem {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]candidateItem, len(items))
	copy(cloned, items)
	return cloned
}

func equalCandidateItems(left, right []candidateItem) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func (ime *IME) resetCustomPhraseOverlay() {
	ime.customPhraseComposition = ""
	ime.customPhraseCandidates = nil
	ime.customPhraseCursor = 0
	ime.customPhraseConsumeKeyUpCode = 0
}

func (ime *IME) customPhraseMatchInput(state rimeState) string {
	rawInput := strings.TrimSpace(state.RawInput)
	if rawInput != "" {
		return rawInput
	}
	tracked := strings.TrimSpace(ime.rawInputTracked)
	if tracked != "" {
		return tracked
	}
	return strings.TrimSpace(state.Composition)
}

func shouldSuppressCustomPhraseOverlay(state rimeState) bool {
	rawInput := strings.TrimSpace(state.RawInput)
	composition := strings.TrimSpace(state.Composition)
	return strings.ContainsRune(rawInput, '`') || strings.ContainsRune(composition, '`')
}

func (ime *IME) visibleCustomPhraseCandidatesForState(state rimeState) []candidateItem {
	if state.PageNo > 0 {
		ime.resetCustomPhraseOverlay()
		return nil
	}
	if shouldSuppressCustomPhraseOverlay(state) {
		ime.resetCustomPhraseOverlay()
		return nil
	}
	return ime.visibleCustomPhraseCandidates(ime.customPhraseMatchInput(state))
}

func (ime *IME) visibleCustomPhraseCandidates(matchInput string) []candidateItem {
	matchInput = strings.TrimSpace(matchInput)
	if matchInput == "" {
		ime.resetCustomPhraseOverlay()
		return nil
	}
	candidates := lookupCustomPhraseCandidates(matchInput, ime.candidateCount())
	if len(candidates) == 0 {
		ime.customPhraseComposition = matchInput
		ime.customPhraseCandidates = nil
		ime.customPhraseCursor = 0
		return nil
	}
	if ime.customPhraseComposition != matchInput || !equalCandidateItems(ime.customPhraseCandidates, candidates) {
		ime.customPhraseComposition = matchInput
		ime.customPhraseCandidates = cloneCandidateItems(candidates)
		ime.customPhraseCursor = 0
	} else if ime.customPhraseCursor >= len(candidates) {
		ime.customPhraseCursor = len(candidates) - 1
	}
	return cloneCandidateItems(ime.customPhraseCandidates)
}

func (ime *IME) currentCustomPhraseOverlay() (rimeState, []candidateItem, []int, bool) {
	if ime.aiActive {
		return rimeState{}, nil, nil, false
	}
	state, ok := ime.currentVisibleBackendState()
	if !ok || strings.TrimSpace(state.Composition) == "" {
		ime.resetCustomPhraseOverlay()
		return rimeState{}, nil, nil, false
	}

	customCandidates := ime.visibleCustomPhraseCandidatesForState(state)
	if len(customCandidates) == 0 {
		return state, nil, nil, false
	}

	if _, overlay, ok := ime.currentSuperAbbrevOverlay(); ok {
		customCandidates = applySuperAbbrevOverlayToCandidates(customCandidates, overlay)
	}

	var backendIndexes []int
	state.Candidates, backendIndexes = filterVisibleBackendCandidatesForCustomOverlay(state.Candidates, customCandidates)

	remaining := ime.candidateCount() - len(customCandidates)
	if remaining < 0 {
		customCandidates = customCandidates[:ime.candidateCount()]
		remaining = 0
	}
	if len(state.Candidates) > remaining {
		state.Candidates = append([]candidateItem(nil), state.Candidates[:remaining]...)
		backendIndexes = append([]int(nil), backendIndexes[:remaining]...)
	}
	if state.SelectKeys != "" && len(state.SelectKeys) > remaining {
		state.SelectKeys = state.SelectKeys[:len(state.Candidates)]
	}
	total := len(customCandidates) + len(state.Candidates)
	if total <= 0 {
		ime.resetCustomPhraseOverlay()
		return state, nil, nil, false
	}
	if ime.customPhraseCursor < 0 {
		ime.customPhraseCursor = 0
	}
	if ime.customPhraseCursor >= total {
		ime.customPhraseCursor = total - 1
	}
	return state, customCandidates, backendIndexes, true
}

func filterVisibleBackendCandidatesForCustomOverlay(candidates []candidateItem, existing []candidateItem) ([]candidateItem, []int) {
	if len(candidates) == 0 {
		return nil, nil
	}
	seen := make(map[string]struct{}, len(existing))
	for _, candidate := range existing {
		key := candidateTextKey(candidate)
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	filtered := make([]candidateItem, 0, len(candidates))
	indexes := make([]int, 0, len(candidates))
	for i, candidate := range candidates {
		key := candidateTextKey(candidate)
		if key != "" {
			if _, ok := seen[key]; ok {
				continue
			}
		}
		filtered = append(filtered, candidate)
		indexes = append(indexes, i)
	}
	return filtered, indexes
}

func (ime *IME) isCustomPhraseHandledKey(req *imecore.Request, totalCandidates int) bool {
	if totalCandidates <= 0 {
		return false
	}
	if index, ok := ime.selectionKeyIndex(req); ok {
		return index < totalCandidates
	}
	if req == nil {
		return false
	}
	switch req.KeyCode {
	case vkUp, vkDown, vkSpace:
		return true
	}
	return false
}

func (ime *IME) handleCustomPhraseKeyDownFilter(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil {
		return false
	}
	state, customCandidates, _, ok := ime.currentCustomPhraseOverlay()
	if !ok {
		return false
	}
	total := len(customCandidates) + len(state.Candidates)
	if !ime.isCustomPhraseHandledKey(req, total) {
		return false
	}
	ime.customPhraseConsumeKeyUpCode = selectionShortcutConsumeCode(req)
	if isSemicolonDebugEvent(req) {
		debugLogf("semicolon filter custom handled consume=%d composition=%q custom=%v backend=%v",
			ime.customPhraseConsumeKeyUpCode,
			state.Composition,
			summarizeCandidateTexts(customCandidates, 6),
			summarizeCandidateTexts(state.Candidates, 6),
		)
	}
	resp.ReturnValue = 1
	return true
}

func (ime *IME) handleCustomPhraseKeyUpFilter(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil {
		return false
	}
	if ime.customPhraseConsumeKeyUpCode != 0 && selectionShortcutConsumeCode(req) == ime.customPhraseConsumeKeyUpCode {
		resp.ReturnValue = 1
		return true
	}
	return false
}

func (ime *IME) commitCustomPhraseCandidate(resp *imecore.Response, candidate candidateItem) {
	if resp == nil || strings.TrimSpace(candidate.Text) == "" {
		return
	}
	resp.CommitString = candidate.Text
	ime.rememberAICommit(candidate.Text)
	ime.clearResponse(resp)
	ime.resetTrackedRawInput()
	if ime.backend != nil {
		ime.backend.ClearComposition()
	}
	ime.resetCustomPhraseOverlay()
	ime.keyComposing = false
	ime.selectKeys = ""
}

func (ime *IME) handleCustomPhraseKeyDown(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || ime.customPhraseConsumeKeyUpCode == 0 || selectionShortcutConsumeCode(req) != ime.customPhraseConsumeKeyUpCode {
		return false
	}
	state, customCandidates, backendIndexes, ok := ime.currentCustomPhraseOverlay()
	if !ok {
		ime.fillResponseFromCurrentState(resp)
		resp.ReturnValue = 1
		return true
	}
	total := len(customCandidates) + len(state.Candidates)
	switch req.KeyCode {
	case vkUp:
		if ime.customPhraseCursor > 0 {
			ime.customPhraseCursor--
		}
		ime.fillResponseFromCurrentState(resp)
		resp.ReturnValue = 1
		return true
	case vkDown:
		if ime.customPhraseCursor < total-1 {
			ime.customPhraseCursor++
		}
		ime.fillResponseFromCurrentState(resp)
		resp.ReturnValue = 1
		return true
	case vkSpace:
		if ime.customPhraseCursor < len(customCandidates) {
			ime.commitCustomPhraseCandidate(resp, customCandidates[ime.customPhraseCursor])
			resp.ReturnValue = 1
			return true
		}
		backendListIndex := ime.customPhraseCursor - len(customCandidates)
		if backendListIndex >= 0 && backendListIndex < len(backendIndexes) &&
			ime.commitBackendOverlayCandidate(resp, backendIndexes[backendListIndex]) {
			resp.ReturnValue = 1
			return true
		}
		ime.fillResponseFromCurrentState(resp)
		resp.ReturnValue = 1
		return true
	default:
		if index, ok := ime.selectionKeyIndex(req); ok {
			if index < len(customCandidates) {
				if isSemicolonDebugEvent(req) {
					debugLogf("semicolon onKeyDown custom selecting customIndex=%d text=%q composition=%q custom=%v backend=%v",
						index,
						customCandidates[index].Text,
						state.Composition,
						summarizeCandidateTexts(customCandidates, 6),
						summarizeCandidateTexts(state.Candidates, 6),
					)
				}
				ime.customPhraseCursor = index
				ime.commitCustomPhraseCandidate(resp, customCandidates[index])
				resp.ReturnValue = 1
				return true
			}
			backendListIndex := index - len(customCandidates)
			if index < total && backendListIndex >= 0 && backendListIndex < len(backendIndexes) &&
				ime.commitBackendOverlayCandidate(resp, backendIndexes[backendListIndex]) {
				if isSemicolonDebugEvent(req) {
					debugLogf("semicolon onKeyDown custom selecting backendIndex=%d visibleBackendIndex=%d composition=%q custom=%v backend=%v commit=%q",
						backendIndexes[backendListIndex],
						backendListIndex,
						state.Composition,
						summarizeCandidateTexts(customCandidates, 6),
						summarizeCandidateTexts(state.Candidates, 6),
						resp.CommitString,
					)
				}
				resp.ReturnValue = 1
				return true
			}
		}
	}
	return false
}

func (ime *IME) handleCustomPhraseKeyUp(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || ime.customPhraseConsumeKeyUpCode == 0 || selectionShortcutConsumeCode(req) != ime.customPhraseConsumeKeyUpCode {
		return false
	}
	ime.customPhraseConsumeKeyUpCode = 0
	ime.fillResponseFromCurrentState(resp)
	resp.ReturnValue = 1
	return true
}

func (ime *IME) openCustomPhraseFile(resp *imecore.Response) bool {
	path, err := ensureCustomPhraseFileExists()
	if err != nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("创建置顶短语文件失败", imecore.TrayNotificationIconError)
		}
		return false
	}
	if err := openCustomPhraseTargetFunc(path); err != nil {
		if resp != nil {
			resp.TrayNotification = trayNotification("打开置顶短语失败", imecore.TrayNotificationIconError)
		}
		return false
	}
	return true
}
