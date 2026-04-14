//go:build windows

// RIME Windows DLL 动态加载封装
// 参考 python/librime.py
package rime

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	RIME_MAX_NUM_CANDIDATES = 10
)

type RimeSessionId uintptr

type RimeTraits struct {
	SharedDataDir        string
	UserDataDir          string
	DistributionName     string
	DistributionCodeName string
	DistributionVersion  string
	AppName              string
	Modules              []string
	MinLogLevel          int
	LogDir               string
	PrebuiltDataDir      string
	StagingDir           string
}

type RimeComposition struct {
	Length    int
	CursorPos int
	SelStart  int
	SelEnd    int
	Preedit   string
}

type RimeCandidate struct {
	Text    string
	Comment string
}

type RimeMenu struct {
	PageSize                  int
	PageNo                    int
	IsLastPage                bool
	HighlightedCandidateIndex int
	NumCandidates             int
	Candidates                []RimeCandidate
	SelectKeys                string
}

type RimeCommit struct {
	Text string
}

type RimeSchema struct {
	ID   string
	Name string
}

type RimeSwitch struct {
	Name   string
	States []string
}

type NotificationHandler func(session RimeSessionId, messageType, messageValue string)

type rimeTraitsC struct {
	DataSize             int32
	SharedDataDir        *byte
	UserDataDir          *byte
	DistributionName     *byte
	DistributionCodeName *byte
	DistributionVersion  *byte
	AppName              *byte
	Modules              **byte
	MinLogLevel          int32
	LogDir               *byte
	PrebuiltDataDir      *byte
	StagingDir           *byte
}

type rimeCompositionC struct {
	Length    int32
	CursorPos int32
	SelStart  int32
	SelEnd    int32
	Preedit   *byte
}

type rimeCandidateC struct {
	Text     *byte
	Comment  *byte
	Reserved uintptr
}

type rimeMenuC struct {
	PageSize                  int32
	PageNo                    int32
	IsLastPage                int32
	HighlightedCandidateIndex int32
	NumCandidates             int32
	Candidates                *rimeCandidateC
	SelectKeys                *byte
}

type rimeCommitC struct {
	DataSize int32
	Text     *byte
}

type rimeContextC struct {
	DataSize          int32
	Composition       rimeCompositionC
	Menu              rimeMenuC
	CommitTextPreview *byte
	SelectLabels      **byte
}

type rimeSchemaListItemC struct {
	SchemaID *byte
	Name     *byte
	Reserved uintptr
}

type rimeSchemaListC struct {
	Size uint64
	List *rimeSchemaListItemC
}

type rimeConfigC struct {
	Ptr uintptr
}

type rimeConfigIteratorC struct {
	List  uintptr
	Map   uintptr
	Index int32
	Key   *byte
	Path  *byte
}

var (
	rimeDLLMu sync.Mutex
	rimeDLL   *syscall.LazyDLL
	rimeProcs struct {
		setup                 *syscall.LazyProc
		initialize            *syscall.LazyProc
		finalize              *syscall.LazyProc
		startMaintenance      *syscall.LazyProc
		joinMaintenanceThread *syscall.LazyProc
		deployConfigFile      *syscall.LazyProc
		syncUserData          *syscall.LazyProc
		createSession         *syscall.LazyProc
		findSession           *syscall.LazyProc
		destroySession        *syscall.LazyProc
		processKey            *syscall.LazyProc
		clearComposition      *syscall.LazyProc
		getCommit             *syscall.LazyProc
		freeCommit            *syscall.LazyProc
		getContext            *syscall.LazyProc
		freeContext           *syscall.LazyProc
		setOption             *syscall.LazyProc
		getOption             *syscall.LazyProc
		getSchemaList         *syscall.LazyProc
		freeSchemaList        *syscall.LazyProc
		getCurrentSchema      *syscall.LazyProc
		selectSchema          *syscall.LazyProc
		schemaOpen            *syscall.LazyProc
		configOpen            *syscall.LazyProc
		configClose           *syscall.LazyProc
		configGetCString      *syscall.LazyProc
		configGetItem         *syscall.LazyProc
		configBeginMap        *syscall.LazyProc
		configBeginList       *syscall.LazyProc
		configNext            *syscall.LazyProc
		configEnd             *syscall.LazyProc
		configListSize        *syscall.LazyProc
		configSetInt          *syscall.LazyProc
		getStateLabel         *syscall.LazyProc
		getStateLabelAbbrev   *syscall.LazyProc
		getVersion            *syscall.LazyProc
	}
)

func loadRimeDLL(dllPath string) error {
	rimeDLLMu.Lock()
	defer rimeDLLMu.Unlock()

	if rimeDLL != nil {
		return nil
	}

	if dllPath == "" {
		dllPath = "rime.dll"
	}
	dll := syscall.NewLazyDLL(dllPath)
	procs := struct {
		setup                 *syscall.LazyProc
		initialize            *syscall.LazyProc
		finalize              *syscall.LazyProc
		startMaintenance      *syscall.LazyProc
		joinMaintenanceThread *syscall.LazyProc
		deployConfigFile      *syscall.LazyProc
		syncUserData          *syscall.LazyProc
		createSession         *syscall.LazyProc
		findSession           *syscall.LazyProc
		destroySession        *syscall.LazyProc
		processKey            *syscall.LazyProc
		clearComposition      *syscall.LazyProc
		getCommit             *syscall.LazyProc
		freeCommit            *syscall.LazyProc
		getContext            *syscall.LazyProc
		freeContext           *syscall.LazyProc
		setOption             *syscall.LazyProc
		getOption             *syscall.LazyProc
		getSchemaList         *syscall.LazyProc
		freeSchemaList        *syscall.LazyProc
		getCurrentSchema      *syscall.LazyProc
		selectSchema          *syscall.LazyProc
		schemaOpen            *syscall.LazyProc
		configOpen            *syscall.LazyProc
		configClose           *syscall.LazyProc
		configGetCString      *syscall.LazyProc
		configGetItem         *syscall.LazyProc
		configBeginMap        *syscall.LazyProc
		configBeginList       *syscall.LazyProc
		configNext            *syscall.LazyProc
		configEnd             *syscall.LazyProc
		configListSize        *syscall.LazyProc
		configSetInt          *syscall.LazyProc
		getStateLabel         *syscall.LazyProc
		getStateLabelAbbrev   *syscall.LazyProc
		getVersion            *syscall.LazyProc
	}{
		setup:                 dll.NewProc("RimeSetup"),
		initialize:            dll.NewProc("RimeInitialize"),
		finalize:              dll.NewProc("RimeFinalize"),
		startMaintenance:      dll.NewProc("RimeStartMaintenance"),
		joinMaintenanceThread: dll.NewProc("RimeJoinMaintenanceThread"),
		deployConfigFile:      dll.NewProc("RimeDeployConfigFile"),
		syncUserData:          dll.NewProc("RimeSyncUserData"),
		createSession:         dll.NewProc("RimeCreateSession"),
		findSession:           dll.NewProc("RimeFindSession"),
		destroySession:        dll.NewProc("RimeDestroySession"),
		processKey:            dll.NewProc("RimeProcessKey"),
		clearComposition:      dll.NewProc("RimeClearComposition"),
		getCommit:             dll.NewProc("RimeGetCommit"),
		freeCommit:            dll.NewProc("RimeFreeCommit"),
		getContext:            dll.NewProc("RimeGetContext"),
		freeContext:           dll.NewProc("RimeFreeContext"),
		setOption:             dll.NewProc("RimeSetOption"),
		getOption:             dll.NewProc("RimeGetOption"),
		getSchemaList:         dll.NewProc("RimeGetSchemaList"),
		freeSchemaList:        dll.NewProc("RimeFreeSchemaList"),
		getCurrentSchema:      dll.NewProc("RimeGetCurrentSchema"),
		selectSchema:          dll.NewProc("RimeSelectSchema"),
		schemaOpen:            dll.NewProc("RimeSchemaOpen"),
		configOpen:            dll.NewProc("RimeConfigOpen"),
		configClose:           dll.NewProc("RimeConfigClose"),
		configGetCString:      dll.NewProc("RimeConfigGetCString"),
		configGetItem:         dll.NewProc("RimeConfigGetItem"),
		configBeginMap:        dll.NewProc("RimeConfigBeginMap"),
		configBeginList:       dll.NewProc("RimeConfigBeginList"),
		configNext:            dll.NewProc("RimeConfigNext"),
		configEnd:             dll.NewProc("RimeConfigEnd"),
		configListSize:        dll.NewProc("RimeConfigListSize"),
		configSetInt:          dll.NewProc("RimeConfigSetInt"),
		getStateLabel:         dll.NewProc("RimeGetStateLabel"),
		getStateLabelAbbrev:   dll.NewProc("RimeGetStateLabelAbbreviated"),
		getVersion:            dll.NewProc("RimeGetVersion"),
	}

	for _, proc := range []*syscall.LazyProc{
		procs.setup, procs.initialize, procs.finalize, procs.startMaintenance, procs.joinMaintenanceThread,
		procs.deployConfigFile, procs.createSession, procs.findSession, procs.destroySession, procs.processKey,
		procs.clearComposition, procs.getCommit, procs.freeCommit, procs.getContext, procs.freeContext,
		procs.setOption, procs.getOption,
	} {
		if err := proc.Find(); err != nil {
			return err
		}
	}

	rimeDLL = dll
	rimeProcs = procs
	return nil
}

func utf8Ptr(s string) *byte {
	if s == "" {
		return nil
	}
	ptr, _ := syscall.BytePtrFromString(s)
	return ptr
}

func cString(ptr *byte) string {
	if ptr == nil {
		return ""
	}
	bytes := make([]byte, 0, 32)
	for p := uintptr(unsafe.Pointer(ptr)); ; p++ {
		b := *(*byte)(unsafe.Pointer(p))
		if b == 0 {
			break
		}
		bytes = append(bytes, b)
	}
	return string(bytes)
}

func boolResult(r1 uintptr) bool {
	return r1 != 0
}

func procAvailable(proc *syscall.LazyProc) bool {
	if proc == nil {
		return false
	}
	return proc.Find() == nil
}

func cStringFromBytes(buf []byte) string {
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

func Init(traits RimeTraits) bool {
	cTraits := rimeTraitsC{
		DataSize:             int32(unsafe.Sizeof(rimeTraitsC{})) - 4,
		SharedDataDir:        utf8Ptr(traits.SharedDataDir),
		UserDataDir:          utf8Ptr(traits.UserDataDir),
		DistributionName:     utf8Ptr(traits.DistributionName),
		DistributionCodeName: utf8Ptr(traits.DistributionCodeName),
		DistributionVersion:  utf8Ptr(traits.DistributionVersion),
		AppName:              utf8Ptr(traits.AppName),
		MinLogLevel:          int32(traits.MinLogLevel),
		LogDir:               utf8Ptr(traits.LogDir),
		PrebuiltDataDir:      utf8Ptr(traits.PrebuiltDataDir),
		StagingDir:           utf8Ptr(traits.StagingDir),
	}

	r1, _, _ := rimeProcs.setup.Call(uintptr(unsafe.Pointer(&cTraits)))
	runtime.KeepAlive(cTraits)
	return boolResult(r1) || true
}

func Finalize() {
	rimeProcs.finalize.Call()
}

func StartSession() (RimeSessionId, bool) {
	r1, _, _ := rimeProcs.createSession.Call()
	return RimeSessionId(r1), r1 != 0
}

func FindSession(sessionId RimeSessionId) bool {
	if sessionId == 0 {
		return false
	}
	r1, _, _ := rimeProcs.findSession.Call(uintptr(sessionId))
	return boolResult(r1)
}

func EndSession(sessionId RimeSessionId) {
	if sessionId != 0 {
		rimeProcs.destroySession.Call(uintptr(sessionId))
	}
}

func ProcessKey(sessionId RimeSessionId, keyCode, modifiers int) bool {
	r1, _, _ := rimeProcs.processKey.Call(uintptr(sessionId), uintptr(keyCode), uintptr(modifiers))
	return boolResult(r1)
}

func ClearComposition(sessionId RimeSessionId) {
	rimeProcs.clearComposition.Call(uintptr(sessionId))
}

func GetComposition(sessionId RimeSessionId) (RimeComposition, bool) {
	context, ok := getContext(sessionId)
	if !ok {
		return RimeComposition{}, false
	}
	defer freeContext(&context)

	return RimeComposition{
		Length:    int(context.Composition.Length),
		CursorPos: int(context.Composition.CursorPos),
		SelStart:  int(context.Composition.SelStart),
		SelEnd:    int(context.Composition.SelEnd),
		Preedit:   cString(context.Composition.Preedit),
	}, true
}

func GetMenu(sessionId RimeSessionId) (RimeMenu, bool) {
	context, ok := getContext(sessionId)
	if !ok {
		return RimeMenu{}, false
	}
	defer freeContext(&context)

	menu := RimeMenu{
		PageSize:                  int(context.Menu.PageSize),
		PageNo:                    int(context.Menu.PageNo),
		IsLastPage:                context.Menu.IsLastPage != 0,
		HighlightedCandidateIndex: int(context.Menu.HighlightedCandidateIndex),
		NumCandidates:             int(context.Menu.NumCandidates),
		SelectKeys:                cString(context.Menu.SelectKeys),
	}

	if context.Menu.NumCandidates > 0 && context.Menu.Candidates != nil {
		candidates := unsafe.Slice(context.Menu.Candidates, int(context.Menu.NumCandidates))
		menu.Candidates = make([]RimeCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			menu.Candidates = append(menu.Candidates, RimeCandidate{
				Text:    cString(candidate.Text),
				Comment: cString(candidate.Comment),
			})
		}
	}
	return menu, true
}

func GetCommit(sessionId RimeSessionId) (RimeCommit, bool) {
	commit := rimeCommitC{DataSize: int32(unsafe.Sizeof(rimeCommitC{})) - 4}
	r1, _, _ := rimeProcs.getCommit.Call(uintptr(sessionId), uintptr(unsafe.Pointer(&commit)))
	if !boolResult(r1) {
		return RimeCommit{}, false
	}
	defer rimeProcs.freeCommit.Call(uintptr(unsafe.Pointer(&commit)))
	return RimeCommit{Text: cString(commit.Text)}, true
}

func SetOption(sessionId RimeSessionId, option string, value bool) {
	name := utf8Ptr(option)
	var v uintptr
	if value {
		v = 1
	}
	rimeProcs.setOption.Call(uintptr(sessionId), uintptr(unsafe.Pointer(name)), v)
	runtime.KeepAlive(name)
}

func GetOption(sessionId RimeSessionId, option string) bool {
	name := utf8Ptr(option)
	r1, _, _ := rimeProcs.getOption.Call(uintptr(sessionId), uintptr(unsafe.Pointer(name)))
	runtime.KeepAlive(name)
	return boolResult(r1)
}

func GetSchemaList() []RimeSchema {
	if !procAvailable(rimeProcs.getSchemaList) || !procAvailable(rimeProcs.freeSchemaList) {
		return nil
	}
	var schemaList rimeSchemaListC
	r1, _, _ := rimeProcs.getSchemaList.Call(uintptr(unsafe.Pointer(&schemaList)))
	if !boolResult(r1) || schemaList.Size == 0 || schemaList.List == nil {
		return nil
	}
	defer rimeProcs.freeSchemaList.Call(uintptr(unsafe.Pointer(&schemaList)))

	items := unsafe.Slice(schemaList.List, int(schemaList.Size))
	schemas := make([]RimeSchema, 0, len(items))
	for _, item := range items {
		schemaID := cString(item.SchemaID)
		name := cString(item.Name)
		if schemaID == "" {
			continue
		}
		if name == "" {
			name = schemaID
		}
		schemas = append(schemas, RimeSchema{
			ID:   schemaID,
			Name: name,
		})
	}
	return schemas
}

func GetCurrentSchema(sessionId RimeSessionId) string {
	if sessionId == 0 || !procAvailable(rimeProcs.getCurrentSchema) {
		return ""
	}
	buf := make([]byte, 256)
	r1, _, _ := rimeProcs.getCurrentSchema.Call(
		uintptr(sessionId),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	runtime.KeepAlive(buf)
	if !boolResult(r1) {
		return ""
	}
	return cStringFromBytes(buf)
}

func SelectSchema(sessionId RimeSessionId, schemaID string) bool {
	if sessionId == 0 || schemaID == "" || !procAvailable(rimeProcs.selectSchema) {
		return false
	}
	cSchemaID := utf8Ptr(schemaID)
	r1, _, _ := rimeProcs.selectSchema.Call(uintptr(sessionId), uintptr(unsafe.Pointer(cSchemaID)))
	runtime.KeepAlive(cSchemaID)
	return boolResult(r1)
}

func openConfig(configID string) (rimeConfigC, bool) {
	if configID == "" || !procAvailable(rimeProcs.configOpen) || !procAvailable(rimeProcs.configClose) {
		return rimeConfigC{}, false
	}
	var config rimeConfigC
	cConfigID := utf8Ptr(configID)
	r1, _, _ := rimeProcs.configOpen.Call(uintptr(unsafe.Pointer(cConfigID)), uintptr(unsafe.Pointer(&config)))
	runtime.KeepAlive(cConfigID)
	if !boolResult(r1) {
		return rimeConfigC{}, false
	}
	return config, true
}

func closeConfig(config *rimeConfigC) {
	if config == nil || !procAvailable(rimeProcs.configClose) {
		return
	}
	rimeProcs.configClose.Call(uintptr(unsafe.Pointer(config)))
}

func configGetCString(config *rimeConfigC, key string) string {
	if config == nil || !procAvailable(rimeProcs.configGetCString) {
		return ""
	}
	cKey := utf8Ptr(key)
	r1, _, _ := rimeProcs.configGetCString.Call(uintptr(unsafe.Pointer(config)), uintptr(unsafe.Pointer(cKey)))
	runtime.KeepAlive(cKey)
	return cString((*byte)(unsafe.Pointer(r1)))
}

func configListSize(config *rimeConfigC, key string) int {
	if config == nil || !procAvailable(rimeProcs.configListSize) {
		return 0
	}
	cKey := utf8Ptr(key)
	r1, _, _ := rimeProcs.configListSize.Call(uintptr(unsafe.Pointer(config)), uintptr(unsafe.Pointer(cKey)))
	runtime.KeepAlive(cKey)
	return int(r1)
}

func configListStrings(config *rimeConfigC, key string) []string {
	if config == nil || !procAvailable(rimeProcs.configBeginList) || !procAvailable(rimeProcs.configNext) || !procAvailable(rimeProcs.configEnd) {
		return nil
	}
	cKey := utf8Ptr(key)
	var iterator rimeConfigIteratorC
	r1, _, _ := rimeProcs.configBeginList.Call(
		uintptr(unsafe.Pointer(&iterator)),
		uintptr(unsafe.Pointer(config)),
		uintptr(unsafe.Pointer(cKey)),
	)
	runtime.KeepAlive(cKey)
	if !boolResult(r1) {
		return nil
	}
	defer rimeProcs.configEnd.Call(uintptr(unsafe.Pointer(&iterator)))

	var items []string
	for {
		r1, _, _ = rimeProcs.configNext.Call(uintptr(unsafe.Pointer(&iterator)))
		if !boolResult(r1) {
			break
		}
		value := strings.TrimSpace(configGetCString(config, cString(iterator.Path)))
		if value == "" {
			continue
		}
		items = append(items, value)
	}
	return items
}

func configPathJoin(base, key string) string {
	if base == "" {
		return key
	}
	if key == "" {
		return base
	}
	return base + "/" + key
}

func getSchemaSwitchesFromConfig(config *rimeConfigC) []RimeSwitch {
	if config == nil || !procAvailable(rimeProcs.configBeginList) || !procAvailable(rimeProcs.configNext) || !procAvailable(rimeProcs.configEnd) {
		return nil
	}
	var iterator rimeConfigIteratorC
	cKey := utf8Ptr("switches")
	r1, _, _ := rimeProcs.configBeginList.Call(
		uintptr(unsafe.Pointer(&iterator)),
		uintptr(unsafe.Pointer(config)),
		uintptr(unsafe.Pointer(cKey)),
	)
	runtime.KeepAlive(cKey)
	if !boolResult(r1) {
		return nil
	}
	defer rimeProcs.configEnd.Call(uintptr(unsafe.Pointer(&iterator)))

	switches := make([]RimeSwitch, 0)
	for {
		r1, _, _ = rimeProcs.configNext.Call(uintptr(unsafe.Pointer(&iterator)))
		if !boolResult(r1) {
			break
		}
		basePath := cString(iterator.Path)
		name := strings.TrimSpace(configGetCString(config, configPathJoin(basePath, "name")))
		if name == "" {
			continue
		}
		statesPath := configPathJoin(basePath, "states")
		stateCount := configListSize(config, statesPath)
		states := make([]string, 0, stateCount)
		for _, state := range configListStrings(config, statesPath) {
			state = strings.TrimSpace(state)
			if state == "" {
				continue
			}
			states = append(states, state)
		}
		switches = append(switches, RimeSwitch{
			Name:   name,
			States: states,
		})
	}
	return switches
}

func GetConfigStringList(configID, key string) []string {
	if configID == "" {
		return nil
	}
	config, ok := openConfig(configID)
	if !ok {
		return nil
	}
	defer closeConfig(&config)
	return configListStrings(&config, key)
}

func GetSchemaConfigStringList(schemaID, key string) []string {
	if schemaID == "" || !procAvailable(rimeProcs.schemaOpen) || !procAvailable(rimeProcs.configClose) {
		return nil
	}
	var config rimeConfigC
	cSchemaID := utf8Ptr(schemaID)
	r1, _, _ := rimeProcs.schemaOpen.Call(uintptr(unsafe.Pointer(cSchemaID)), uintptr(unsafe.Pointer(&config)))
	runtime.KeepAlive(cSchemaID)
	if !boolResult(r1) {
		return nil
	}
	defer closeConfig(&config)
	return configListStrings(&config, key)
}

func GetSchemaSwitches(schemaID string) []RimeSwitch {
	if schemaID == "" || !procAvailable(rimeProcs.schemaOpen) || !procAvailable(rimeProcs.configClose) {
		return nil
	}
	var config rimeConfigC
	cSchemaID := utf8Ptr(schemaID)
	r1, _, _ := rimeProcs.schemaOpen.Call(uintptr(unsafe.Pointer(cSchemaID)), uintptr(unsafe.Pointer(&config)))
	runtime.KeepAlive(cSchemaID)
	if !boolResult(r1) {
		return nil
	}
	defer closeConfig(&config)
	return getSchemaSwitchesFromConfig(&config)
}

func SetSchemaPageSize(schemaID string, pageSize int) bool {
	if schemaID == "" || pageSize <= 0 {
		return false
	}
	if !procAvailable(rimeProcs.schemaOpen) || !procAvailable(rimeProcs.configClose) || !procAvailable(rimeProcs.configSetInt) {
		return false
	}
	var config rimeConfigC
	cSchemaID := utf8Ptr(schemaID)
	r1, _, _ := rimeProcs.schemaOpen.Call(uintptr(unsafe.Pointer(cSchemaID)), uintptr(unsafe.Pointer(&config)))
	runtime.KeepAlive(cSchemaID)
	if !boolResult(r1) {
		return false
	}
	defer rimeProcs.configClose.Call(uintptr(unsafe.Pointer(&config)))

	cPath := utf8Ptr("menu/page_size")
	r1, _, _ = rimeProcs.configSetInt.Call(
		uintptr(unsafe.Pointer(&config)),
		uintptr(unsafe.Pointer(cPath)),
		uintptr(pageSize),
	)
	runtime.KeepAlive(cPath)
	return boolResult(r1)
}

func SelectCandidate(sessionId RimeSessionId, index int) {
	_ = sessionId
	_ = index
}

func SelectPage(sessionId RimeSessionId, pageNo int) {
	_ = sessionId
	_ = pageNo
}

func DeployConfigFile(filePath, key string) bool {
	cFile := utf8Ptr(filePath)
	cKey := utf8Ptr(key)
	r1, _, _ := rimeProcs.deployConfigFile.Call(uintptr(unsafe.Pointer(cFile)), uintptr(unsafe.Pointer(cKey)))
	runtime.KeepAlive(cFile)
	runtime.KeepAlive(cKey)
	return boolResult(r1)
}

func StartMaintenance(fullcheck bool) bool {
	if !procAvailable(rimeProcs.startMaintenance) {
		return false
	}
	var fullcheckArg uintptr
	if fullcheck {
		fullcheckArg = 1
	}
	r1, _, _ := rimeProcs.startMaintenance.Call(fullcheckArg)
	return boolResult(r1)
}

func JoinMaintenanceThread() {
	if !procAvailable(rimeProcs.joinMaintenanceThread) {
		return
	}
	rimeProcs.joinMaintenanceThread.Call()
}

func SyncUserData() bool {
	if rimeProcs.syncUserData == nil {
		return false
	}
	if err := rimeProcs.syncUserData.Find(); err != nil {
		log.Printf("RIME SyncUserData 不可用: %v", err)
		return false
	}
	r1, _, _ := rimeProcs.syncUserData.Call()
	return boolResult(r1)
}

func SetNotificationHandler(handler NotificationHandler) {
	_ = handler
}

func APIVersion() string {
	return ""
}

func GetName() string {
	return ""
}

func GetVersion() string {
	if rimeProcs.getVersion == nil {
		return ""
	}
	if err := rimeProcs.getVersion.Find(); err != nil {
		return ""
	}
	r1, _, _ := rimeProcs.getVersion.Call()
	return cString((*byte)(unsafe.Pointer(r1)))
}

func initializeEngine(traits RimeTraits, fullcheck bool) bool {
	start := time.Now()
	success := false
	defer func() {
		log.Printf("RIME initializeEngine 完成 elapsed=%s success=%t fullcheck=%t", time.Since(start), success, fullcheck)
	}()

	log.Printf("RIME initializeEngine 开始 fullcheck=%t sharedDir=%q userDir=%q prebuiltDir=%q stagingDir=%q", fullcheck, traits.SharedDataDir, traits.UserDataDir, traits.PrebuiltDataDir, traits.StagingDir)
	setupStart := time.Now()
	if !Init(traits) {
		log.Println("RIME setup 失败")
		return false
	}
	log.Printf("RIME initializeEngine setup 完成 elapsed=%s", time.Since(setupStart))

	initializeStart := time.Now()
	rimeProcs.initialize.Call(0)
	log.Printf("RIME initializeEngine initialize 完成 elapsed=%s", time.Since(initializeStart))
	var fullcheckArg uintptr
	if fullcheck {
		fullcheckArg = 1
	}
	maintenanceStart := time.Now()
	r1, _, _ := rimeProcs.startMaintenance.Call(fullcheckArg)
	maintenanceStarted := boolResult(r1)
	log.Printf("RIME initializeEngine startMaintenance 完成 elapsed=%s started=%t fullcheck=%t", time.Since(maintenanceStart), maintenanceStarted, fullcheck)
	if maintenanceStarted {
		joinStart := time.Now()
		rimeProcs.joinMaintenanceThread.Call()
		log.Printf("RIME initializeEngine joinMaintenanceThread 完成 elapsed=%s", time.Since(joinStart))
	}
	success = true
	return true
}

func RimeInit(datadir, userdir, appname, appver string, fullcheck bool) bool {
	start := time.Now()
	success := false
	defer func() {
		log.Printf("RIME RimeInit 完成 elapsed=%s success=%t fullcheck=%t datadir=%q userdir=%q", time.Since(start), success, fullcheck, datadir, userdir)
	}()

	log.Printf("RIME RimeInit 开始 fullcheck=%t datadir=%q userdir=%q appname=%s", fullcheck, datadir, userdir, appname)
	mkdirStart := time.Now()
	if err := os.MkdirAll(userdir, 0700); err != nil {
		log.Printf("创建用户目录失败: %v", err)
		return false
	}
	log.Printf("RIME RimeInit 创建用户目录完成 elapsed=%s", time.Since(mkdirStart))

	dllStart := time.Now()
	dllPath := filepath.Join(filepath.Dir(datadir), "rime.dll")
	if _, err := os.Stat(dllPath); err != nil {
		dllPath = "rime.dll"
	}
	if err := loadRimeDLL(dllPath); err != nil {
		log.Printf("加载 RIME DLL 失败: %v", err)
		return false
	}
	log.Printf("RIME RimeInit 加载DLL完成 elapsed=%s dllPath=%q", time.Since(dllStart), dllPath)

	logDir := rimeLogDir()
	if logDir != "" {
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			log.Printf("创建 RIME 日志目录失败: %v", err)
			logDir = ""
		}
	}

	traits := RimeTraits{
		SharedDataDir:        datadir,
		UserDataDir:          userdir,
		DistributionName:     "Rime",
		DistributionCodeName: appname,
		DistributionVersion:  appver,
		AppName:              fmt.Sprintf("Rime.%s", appname),
		LogDir:               logDir,
		PrebuiltDataDir:      filepath.Join(datadir, "build"),
		StagingDir:           filepath.Join(userdir, "build"),
	}
	engineStart := time.Now()
	if !initializeEngine(traits, fullcheck) {
		return false
	}
	log.Printf("RIME RimeInit initializeEngine 完成 elapsed=%s", time.Since(engineStart))
	success = true
	return true
}

func RimeRedeploy(datadir, userdir, appname, appver string) bool {
	logDir := rimeLogDir()
	if logDir != "" {
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			log.Printf("创建 RIME 日志目录失败: %v", err)
			logDir = ""
		}
	}

	traits := RimeTraits{
		SharedDataDir:        datadir,
		UserDataDir:          userdir,
		DistributionName:     "Rime",
		DistributionCodeName: appname,
		DistributionVersion:  appver,
		AppName:              fmt.Sprintf("Rime.%s", appname),
		LogDir:               logDir,
		PrebuiltDataDir:      filepath.Join(datadir, "build"),
		StagingDir:           filepath.Join(userdir, "build"),
	}

	Finalize()
	if !initializeEngine(traits, true) {
		return false
	}
	return true
}

func getContext(sessionId RimeSessionId) (rimeContextC, bool) {
	context := rimeContextC{DataSize: int32(unsafe.Sizeof(rimeContextC{})) - 4}
	r1, _, _ := rimeProcs.getContext.Call(uintptr(sessionId), uintptr(unsafe.Pointer(&context)))
	if !boolResult(r1) {
		return rimeContextC{}, false
	}
	return context, true
}

func freeContext(context *rimeContextC) {
	rimeProcs.freeContext.Call(uintptr(unsafe.Pointer(context)))
}
