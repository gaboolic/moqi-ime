//go:build windows

package rime

import (
	"log"
	"sync"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

type nativeBackend struct {
	sessionID RimeSessionId
}

var (
	rimeInitOnce sync.Once
	rimeInitOK   bool
	rimeRuntime  nativeRuntimeState
	nativeFindSessionFunc        = FindSession
	nativeStartSessionFunc       = StartSession
	nativeEndSessionFunc         = EndSession
	nativeClearCompositionFunc   = ClearComposition
	nativeProcessKeyFunc         = ProcessKey
	nativeGetMenuFunc            = GetMenu
	nativeSelectSchemaFunc       = SelectSchema
	nativeSetOptionFunc          = SetOption
)

type nativeRuntimeState struct {
	mu                  sync.Mutex
	opMu                sync.RWMutex
	redeploying         bool
	pendingNotification *imecore.TrayNotification
}

var rimeRedeployFunc = RimeRedeploy

func (s *nativeRuntimeState) tryBeginOperation() bool {
	s.mu.Lock()
	redeploying := s.redeploying
	s.mu.Unlock()
	if redeploying {
		return false
	}

	s.opMu.RLock()

	s.mu.Lock()
	redeploying = s.redeploying
	s.mu.Unlock()
	if redeploying {
		s.opMu.RUnlock()
		return false
	}
	return true
}

func (s *nativeRuntimeState) endOperation() {
	s.opMu.RUnlock()
}

func (s *nativeRuntimeState) tryBeginExclusiveOperation() bool {
	s.mu.Lock()
	redeploying := s.redeploying
	s.mu.Unlock()
	if redeploying {
		return false
	}

	if !s.opMu.TryLock() {
		return false
	}

	s.mu.Lock()
	redeploying = s.redeploying
	s.mu.Unlock()
	if redeploying {
		s.opMu.Unlock()
		return false
	}
	return true
}

func (s *nativeRuntimeState) endExclusiveOperation() {
	s.opMu.Unlock()
}

func (s *nativeRuntimeState) startRedeploy(sharedDir, userDir string) bool {
	s.mu.Lock()
	if s.redeploying {
		s.mu.Unlock()
		return false
	}
	s.redeploying = true
	s.pendingNotification = nil
	s.mu.Unlock()

	go func() {
		s.opMu.Lock()
		success := rimeRedeployFunc(sharedDir, userDir, APP, APP_VERSION)
		s.opMu.Unlock()

		s.mu.Lock()
		s.redeploying = false
		s.pendingNotification = deployTrayNotification(success)
		s.mu.Unlock()
	}()

	return true
}

func (s *nativeRuntimeState) consumeNotification() *imecore.TrayNotification {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pendingNotification == nil {
		return nil
	}
	notification := s.pendingNotification
	s.pendingNotification = nil
	return notification
}

func (s *nativeRuntimeState) isRedeploying() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.redeploying
}

func resetNativeRuntimeStateForTest() {
	rimeRuntime.mu.Lock()
	rimeRuntime.redeploying = false
	rimeRuntime.pendingNotification = nil
	rimeRuntime.mu.Unlock()
}

func newNativeBackend() rimeBackend {
	return &nativeBackend{}
}

func (b *nativeBackend) Initialize(sharedDir, userDir string, firstRun bool) bool {
	initStart := time.Now()
	executedOnce := false
	rimeInitOnce.Do(func() {
		executedOnce = true
		onceStart := time.Now()
		debugLogf("nativeBackend.Initialize 一次性初始化开始 firstRun=%t sharedDir=%q userDir=%q", firstRun, sharedDir, userDir)
		rimeInitOK = RimeInit(sharedDir, userDir, APP, APP_VERSION, firstRun)
		debugLogf("nativeBackend.Initialize 一次性初始化完成 elapsed=%s success=%t", time.Since(onceStart), rimeInitOK)
		if !rimeInitOK {
			log.Println("RIME 初始化失败，原生后端不可用")
		}
	})
	debugLogf("nativeBackend.Initialize 返回 elapsed=%s success=%t executedOnce=%t", time.Since(initStart), rimeInitOK, executedOnce)
	return rimeInitOK
}

func (b *nativeBackend) Redeploy(sharedDir, userDir string) bool {
	b.DestroySession()
	if !rimeRuntime.startRedeploy(sharedDir, userDir) {
		log.Println("RIME 已在重新部署中")
		return true
	}
	return true
}

func (b *nativeBackend) Available() bool {
	return rimeInitOK && !rimeRuntime.isRedeploying()
}

func (b *nativeBackend) ConsumeNotification() *imecore.TrayNotification {
	return rimeRuntime.consumeNotification()
}

func (b *nativeBackend) SyncUserData() bool {
	if !rimeRuntime.tryBeginOperation() {
		return false
	}
	defer rimeRuntime.endOperation()
	if !SyncUserData() {
		log.Println("RIME 同步用户数据失败")
		return false
	}
	return true
}

func (b *nativeBackend) ensureSessionLocked() bool {
	return b.ensureSessionHandleLocked(&b.sessionID)
}

func (b *nativeBackend) ensureSessionHandleLocked(handle *RimeSessionId) bool {
	if handle == nil {
		return false
	}
	if *handle != 0 && nativeFindSessionFunc(*handle) {
		return true
	}
	sessionID, ok := nativeStartSessionFunc()
	if ok {
		*handle = sessionID
	}
	return ok
}

func (b *nativeBackend) destroySessionHandleLocked(handle *RimeSessionId) {
	if handle == nil || *handle == 0 {
		return
	}
	nativeEndSessionFunc(*handle)
	*handle = 0
}

func (b *nativeBackend) HasSession() bool {
	if b.sessionID == 0 {
		return false
	}
	if !rimeRuntime.tryBeginOperation() {
		return false
	}
	defer rimeRuntime.endOperation()
	return b.sessionID != 0 && nativeFindSessionFunc(b.sessionID)
}

func (b *nativeBackend) EnsureSession() bool {
	if !rimeRuntime.tryBeginOperation() {
		return false
	}
	defer rimeRuntime.endOperation()
	return b.ensureSessionLocked()
}

func (b *nativeBackend) DestroySession() {
	if !rimeRuntime.tryBeginOperation() {
		b.sessionID = 0
		return
	}
	defer rimeRuntime.endOperation()
	b.destroySessionHandleLocked(&b.sessionID)
}

func (b *nativeBackend) ClearComposition() {
	if !rimeRuntime.tryBeginOperation() {
		return
	}
	defer rimeRuntime.endOperation()
	if b.sessionID != 0 {
		nativeClearCompositionFunc(b.sessionID)
	}
}

func (b *nativeBackend) ProcessKey(req *imecore.Request, translatedKeyCode, modifiers int) bool {
	if !rimeRuntime.tryBeginOperation() {
		return false
	}
	defer rimeRuntime.endOperation()
	if !b.ensureSessionLocked() {
		return false
	}
	return ProcessKey(b.sessionID, translatedKeyCode, modifiers)
}

func (b *nativeBackend) State() rimeState {
	state := rimeState{}
	if !rimeRuntime.tryBeginOperation() {
		return state
	}
	defer rimeRuntime.endOperation()
	if b.sessionID == 0 {
		return state
	}
	if commit, ok := GetCommit(b.sessionID); ok {
		state.CommitString = commit.Text
	}
	if composition, ok := GetComposition(b.sessionID); ok {
		state.Composition = composition.Preedit
		state.CursorPos = composition.CursorPos
		state.SelStart = composition.SelStart
		state.SelEnd = composition.SelEnd
	}
	state.RawInput = GetInput(b.sessionID)
	if menu, ok := GetMenu(b.sessionID); ok {
		candidates := make([]candidateItem, 0, len(menu.Candidates))
		for _, candidate := range menu.Candidates {
			candidates = append(candidates, candidateItem{
				Text:    candidate.Text,
				Comment: candidate.Comment,
			})
		}
		state.Candidates = candidates
		state.PageNo = menu.PageNo
		state.CandidateCursor = menu.HighlightedCandidateIndex
		state.SelectKeys = menu.SelectKeys
	}
	state.AsciiMode = b.GetOption("ascii_mode")
	state.FullShape = b.GetOption("full_shape")
	return state
}

func (b *nativeBackend) SetOption(name string, value bool) {
	if !rimeRuntime.tryBeginOperation() {
		return
	}
	defer rimeRuntime.endOperation()
	if b.ensureSessionLocked() {
		SetOption(b.sessionID, name, value)
	}
}

func (b *nativeBackend) GetOption(name string) bool {
	if !rimeRuntime.tryBeginOperation() {
		return false
	}
	defer rimeRuntime.endOperation()
	if !b.ensureSessionLocked() {
		return false
	}
	return GetOption(b.sessionID, name)
}

func (b *nativeBackend) SaveOptions() []string {
	if !rimeRuntime.tryBeginOperation() {
		return nil
	}
	defer rimeRuntime.endOperation()

	if b.ensureSessionLocked() {
		if schemaID := GetCurrentSchema(b.sessionID); schemaID != "" {
			if options := GetSchemaConfigStringList(schemaID, "switcher/save_options"); len(options) > 0 {
				return options
			}
		}
	}
	return GetConfigStringList("default", "switcher/save_options")
}

func (b *nativeBackend) SchemaSwitches() []RimeSwitch {
	if !rimeRuntime.tryBeginOperation() {
		return nil
	}
	defer rimeRuntime.endOperation()
	if !b.ensureSessionLocked() {
		return nil
	}
	schemaID := GetCurrentSchema(b.sessionID)
	if schemaID == "" {
		return nil
	}
	return GetSchemaSwitches(schemaID)
}

func (b *nativeBackend) SchemaList() []RimeSchema {
	if !rimeRuntime.tryBeginOperation() {
		return nil
	}
	defer rimeRuntime.endOperation()
	return GetSchemaList()
}

func (b *nativeBackend) CurrentSchemaID() string {
	if !rimeRuntime.tryBeginOperation() {
		return ""
	}
	defer rimeRuntime.endOperation()
	if !b.ensureSessionLocked() {
		return ""
	}
	return GetCurrentSchema(b.sessionID)
}

func (b *nativeBackend) SelectSchema(schemaID string) bool {
	if !rimeRuntime.tryBeginOperation() {
		return false
	}
	defer rimeRuntime.endOperation()
	if !b.ensureSessionLocked() {
		return false
	}
	return SelectSchema(b.sessionID, schemaID)
}

func (b *nativeBackend) SetCandidatePageSize(pageSize int) bool {
	if !rimeRuntime.tryBeginOperation() {
		return false
	}
	defer rimeRuntime.endOperation()
	if !b.ensureSessionLocked() {
		return false
	}
	schemaID := GetCurrentSchema(b.sessionID)
	if schemaID == "" {
		return false
	}
	return SetSchemaPageSize(schemaID, pageSize)
}

func (b *nativeBackend) SelectCandidate(index int) bool {
	if !rimeRuntime.tryBeginOperation() {
		return false
	}
	defer rimeRuntime.endOperation()
	if !b.ensureSessionLocked() {
		return false
	}
	return SelectCandidate(b.sessionID, index)
}
