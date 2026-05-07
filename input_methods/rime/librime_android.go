//go:build android

package rime

/*
#cgo android,arm64 CFLAGS: -I${SRCDIR}/../../../librime/src
#cgo android,arm64 LDFLAGS: -L${SRCDIR}/android/arm64-v8a -lrime -lc++_shared
#include <stdlib.h>
#include "rime_api.h"

static RimeApi* moqi_rime_api() {
	return rime_get_api();
}

static int moqi_rime_setup(RimeTraits* traits) {
	RimeApi* api = moqi_rime_api();
	if (!api || !api->setup) return 0;
	api->setup(traits);
	return 1;
}

static void moqi_rime_initialize() {
	RimeApi* api = moqi_rime_api();
	if (api && api->initialize) api->initialize(NULL);
}

static void moqi_rime_finalize() {
	RimeApi* api = moqi_rime_api();
	if (api && api->finalize) api->finalize();
}

static RimeSessionId moqi_rime_create_session() {
	RimeApi* api = moqi_rime_api();
	return (api && api->create_session) ? api->create_session() : 0;
}

static Bool moqi_rime_find_session(RimeSessionId session_id) {
	RimeApi* api = moqi_rime_api();
	return (api && api->find_session) ? api->find_session(session_id) : False;
}

static Bool moqi_rime_destroy_session(RimeSessionId session_id) {
	RimeApi* api = moqi_rime_api();
	return (api && api->destroy_session) ? api->destroy_session(session_id) : False;
}

static Bool moqi_rime_process_key(RimeSessionId session_id, int keycode, int mask) {
	RimeApi* api = moqi_rime_api();
	return (api && api->process_key) ? api->process_key(session_id, keycode, mask) : False;
}

static void moqi_rime_clear_composition(RimeSessionId session_id) {
	RimeApi* api = moqi_rime_api();
	if (api && api->clear_composition) api->clear_composition(session_id);
}

static const char* moqi_rime_get_input(RimeSessionId session_id) {
	RimeApi* api = moqi_rime_api();
	return (api && api->get_input) ? api->get_input(session_id) : NULL;
}

static Bool moqi_rime_get_commit(RimeSessionId session_id, RimeCommit* commit) {
	RimeApi* api = moqi_rime_api();
	return (api && api->get_commit) ? api->get_commit(session_id, commit) : False;
}

static Bool moqi_rime_free_commit(RimeCommit* commit) {
	RimeApi* api = moqi_rime_api();
	return (api && api->free_commit) ? api->free_commit(commit) : False;
}

static Bool moqi_rime_get_context(RimeSessionId session_id, RimeContext* context) {
	RimeApi* api = moqi_rime_api();
	return (api && api->get_context) ? api->get_context(session_id, context) : False;
}

static Bool moqi_rime_free_context(RimeContext* context) {
	RimeApi* api = moqi_rime_api();
	return (api && api->free_context) ? api->free_context(context) : False;
}

static void moqi_rime_set_option(RimeSessionId session_id, const char* option, Bool value) {
	RimeApi* api = moqi_rime_api();
	if (api && api->set_option) api->set_option(session_id, option, value);
}

static Bool moqi_rime_get_option(RimeSessionId session_id, const char* option) {
	RimeApi* api = moqi_rime_api();
	return (api && api->get_option) ? api->get_option(session_id, option) : False;
}

static Bool moqi_rime_get_schema_list(RimeSchemaList* list) {
	RimeApi* api = moqi_rime_api();
	return (api && api->get_schema_list) ? api->get_schema_list(list) : False;
}

static void moqi_rime_free_schema_list(RimeSchemaList* list) {
	RimeApi* api = moqi_rime_api();
	if (api && api->free_schema_list) api->free_schema_list(list);
}

static Bool moqi_rime_get_current_schema(RimeSessionId session_id, char* schema_id, size_t buffer_size) {
	RimeApi* api = moqi_rime_api();
	return (api && api->get_current_schema) ? api->get_current_schema(session_id, schema_id, buffer_size) : False;
}

static Bool moqi_rime_select_schema(RimeSessionId session_id, const char* schema_id) {
	RimeApi* api = moqi_rime_api();
	return (api && api->select_schema) ? api->select_schema(session_id, schema_id) : False;
}

static Bool moqi_rime_schema_open(const char* schema_id, RimeConfig* config) {
	RimeApi* api = moqi_rime_api();
	return (api && api->schema_open) ? api->schema_open(schema_id, config) : False;
}

static Bool moqi_rime_config_open(const char* config_id, RimeConfig* config) {
	RimeApi* api = moqi_rime_api();
	return (api && api->config_open) ? api->config_open(config_id, config) : False;
}

static Bool moqi_rime_config_close(RimeConfig* config) {
	RimeApi* api = moqi_rime_api();
	return (api && api->config_close) ? api->config_close(config) : False;
}

static const char* moqi_rime_config_get_cstring(RimeConfig* config, const char* key) {
	RimeApi* api = moqi_rime_api();
	return (api && api->config_get_cstring) ? api->config_get_cstring(config, key) : NULL;
}

static size_t moqi_rime_config_list_size(RimeConfig* config, const char* key) {
	RimeApi* api = moqi_rime_api();
	return (api && api->config_list_size) ? api->config_list_size(config, key) : 0;
}

static Bool moqi_rime_config_begin_list(RimeConfigIterator* iterator, RimeConfig* config, const char* key) {
	RimeApi* api = moqi_rime_api();
	return (api && api->config_begin_list) ? api->config_begin_list(iterator, config, key) : False;
}

static Bool moqi_rime_config_next(RimeConfigIterator* iterator) {
	RimeApi* api = moqi_rime_api();
	return (api && api->config_next) ? api->config_next(iterator) : False;
}

static void moqi_rime_config_end(RimeConfigIterator* iterator) {
	RimeApi* api = moqi_rime_api();
	if (api && api->config_end) api->config_end(iterator);
}

static Bool moqi_rime_config_set_int(RimeConfig* config, const char* key, int value) {
	RimeApi* api = moqi_rime_api();
	return (api && api->config_set_int) ? api->config_set_int(config, key, value) : False;
}

static Bool moqi_rime_select_candidate_on_current_page(RimeSessionId session_id, size_t index) {
	RimeApi* api = moqi_rime_api();
	return (api && api->select_candidate_on_current_page) ? api->select_candidate_on_current_page(session_id, index) : False;
}

static Bool moqi_rime_highlight_candidate_on_current_page(RimeSessionId session_id, size_t index) {
	RimeApi* api = moqi_rime_api();
	return (api && api->highlight_candidate_on_current_page) ? api->highlight_candidate_on_current_page(session_id, index) : False;
}

static Bool moqi_rime_change_page(RimeSessionId session_id, Bool backward) {
	RimeApi* api = moqi_rime_api();
	return (api && api->change_page) ? api->change_page(session_id, backward) : False;
}

static Bool moqi_rime_deploy_config_file(const char* file_name, const char* version_key) {
	RimeApi* api = moqi_rime_api();
	return (api && api->deploy_config_file) ? api->deploy_config_file(file_name, version_key) : False;
}

static Bool moqi_rime_start_maintenance(Bool full_check) {
	RimeApi* api = moqi_rime_api();
	return (api && api->start_maintenance) ? api->start_maintenance(full_check) : False;
}

static void moqi_rime_join_maintenance_thread() {
	RimeApi* api = moqi_rime_api();
	if (api && api->join_maintenance_thread) api->join_maintenance_thread();
}

static Bool moqi_rime_sync_user_data() {
	RimeApi* api = moqi_rime_api();
	return (api && api->sync_user_data) ? api->sync_user_data() : False;
}

static const char* moqi_rime_get_version() {
	RimeApi* api = moqi_rime_api();
	return (api && api->get_version) ? api->get_version() : NULL;
}

static int moqi_rime_struct_data_size_RimeTraits() {
	return sizeof(RimeTraits) - sizeof(int);
}

static int moqi_rime_struct_data_size_RimeCommit() {
	return sizeof(RimeCommit) - sizeof(int);
}

static int moqi_rime_struct_data_size_RimeContext() {
	return sizeof(RimeContext) - sizeof(int);
}
*/
import "C"

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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

func boolResult(value C.Bool) bool {
	return value != 0
}

func cBool(value bool) C.Bool {
	if value {
		return 1
	}
	return 0
}

func cStringFromBytes(buf []byte) string {
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

func withCString(value string, fn func(*C.char)) {
	if value == "" {
		fn(nil)
		return
	}
	cValue := C.CString(value)
	defer C.free(unsafe.Pointer(cValue))
	fn(cValue)
}

func Init(traits RimeTraits) bool {
	cTraits := C.RimeTraits{
		data_size: C.int(C.moqi_rime_struct_data_size_RimeTraits()),
	}
	cStrings := make([]*C.char, 0, 8)
	addString := func(value string) *C.char {
		if value == "" {
			return nil
		}
		cValue := C.CString(value)
		cStrings = append(cStrings, cValue)
		return cValue
	}
	cModules := make([]*C.char, 0, len(traits.Modules)+1)
	var cModuleArray unsafe.Pointer
	defer func() {
		for _, value := range cStrings {
			C.free(unsafe.Pointer(value))
		}
		for _, value := range cModules {
			C.free(unsafe.Pointer(value))
		}
		if cModuleArray != nil {
			C.free(cModuleArray)
		}
	}()

	cTraits.shared_data_dir = addString(traits.SharedDataDir)
	cTraits.user_data_dir = addString(traits.UserDataDir)
	cTraits.distribution_name = addString(traits.DistributionName)
	cTraits.distribution_code_name = addString(traits.DistributionCodeName)
	cTraits.distribution_version = addString(traits.DistributionVersion)
	cTraits.app_name = addString(traits.AppName)
	cTraits.min_log_level = C.int(traits.MinLogLevel)
	cTraits.log_dir = addString(traits.LogDir)
	cTraits.prebuilt_data_dir = addString(traits.PrebuiltDataDir)
	cTraits.staging_dir = addString(traits.StagingDir)
	if len(traits.Modules) > 0 {
		for _, module := range traits.Modules {
			module = strings.TrimSpace(module)
			if module == "" {
				continue
			}
			cModules = append(cModules, C.CString(module))
		}
		cModules = append(cModules, nil)
		if len(cModules) > 1 {
			cModuleArray = C.malloc(C.size_t(len(cModules)) * C.size_t(unsafe.Sizeof(uintptr(0))))
			cModuleSlice := unsafe.Slice((**C.char)(cModuleArray), len(cModules))
			copy(cModuleSlice, cModules)
			cTraits.modules = (**C.char)(cModuleArray)
		}
	}

	return C.moqi_rime_setup(&cTraits) != 0
}

func Finalize() {
	C.moqi_rime_finalize()
}

func StartSession() (RimeSessionId, bool) {
	sessionID := C.moqi_rime_create_session()
	return RimeSessionId(sessionID), sessionID != 0
}

func FindSession(sessionId RimeSessionId) bool {
	if sessionId == 0 {
		return false
	}
	return boolResult(C.moqi_rime_find_session(C.RimeSessionId(sessionId)))
}

func EndSession(sessionId RimeSessionId) {
	if sessionId == 0 {
		return
	}
	C.moqi_rime_destroy_session(C.RimeSessionId(sessionId))
}

func ProcessKey(sessionId RimeSessionId, keyCode, modifiers int) bool {
	if sessionId == 0 {
		return false
	}
	return boolResult(C.moqi_rime_process_key(C.RimeSessionId(sessionId), C.int(keyCode), C.int(modifiers)))
}

func ClearComposition(sessionId RimeSessionId) {
	if sessionId == 0 {
		return
	}
	C.moqi_rime_clear_composition(C.RimeSessionId(sessionId))
}

func GetInput(sessionId RimeSessionId) string {
	if sessionId == 0 {
		return ""
	}
	return C.GoString(C.moqi_rime_get_input(C.RimeSessionId(sessionId)))
}

func GetComposition(sessionId RimeSessionId) (RimeComposition, bool) {
	context, ok := getContext(sessionId)
	if !ok {
		return RimeComposition{}, false
	}
	defer freeContext(&context)

	return RimeComposition{
		Length:    int(context.composition.length),
		CursorPos: int(context.composition.cursor_pos),
		SelStart:  int(context.composition.sel_start),
		SelEnd:    int(context.composition.sel_end),
		Preedit:   C.GoString(context.composition.preedit),
	}, true
}

func GetMenu(sessionId RimeSessionId) (RimeMenu, bool) {
	context, ok := getContext(sessionId)
	if !ok {
		return RimeMenu{}, false
	}
	defer freeContext(&context)

	menu := RimeMenu{
		PageSize:                  int(context.menu.page_size),
		PageNo:                    int(context.menu.page_no),
		IsLastPage:                context.menu.is_last_page != 0,
		HighlightedCandidateIndex: int(context.menu.highlighted_candidate_index),
		NumCandidates:             int(context.menu.num_candidates),
		SelectKeys:                C.GoString(context.menu.select_keys),
	}
	if context.menu.num_candidates > 0 && context.menu.candidates != nil {
		candidates := unsafe.Slice(context.menu.candidates, int(context.menu.num_candidates))
		menu.Candidates = make([]RimeCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			menu.Candidates = append(menu.Candidates, RimeCandidate{
				Text:    C.GoString(candidate.text),
				Comment: C.GoString(candidate.comment),
			})
		}
	}
	return menu, true
}

func GetCommit(sessionId RimeSessionId) (RimeCommit, bool) {
	if sessionId == 0 {
		return RimeCommit{}, false
	}
	commit := C.RimeCommit{data_size: C.int(C.moqi_rime_struct_data_size_RimeCommit())}
	if !boolResult(C.moqi_rime_get_commit(C.RimeSessionId(sessionId), &commit)) {
		return RimeCommit{}, false
	}
	defer C.moqi_rime_free_commit(&commit)
	return RimeCommit{Text: C.GoString(commit.text)}, true
}

func SetOption(sessionId RimeSessionId, option string, value bool) {
	if sessionId == 0 || option == "" {
		return
	}
	withCString(option, func(cOption *C.char) {
		C.moqi_rime_set_option(C.RimeSessionId(sessionId), cOption, cBool(value))
	})
}

func GetOption(sessionId RimeSessionId, option string) bool {
	if sessionId == 0 || option == "" {
		return false
	}
	result := false
	withCString(option, func(cOption *C.char) {
		result = boolResult(C.moqi_rime_get_option(C.RimeSessionId(sessionId), cOption))
	})
	return result
}

func GetSchemaList() []RimeSchema {
	var schemaList C.RimeSchemaList
	if !boolResult(C.moqi_rime_get_schema_list(&schemaList)) || schemaList.size == 0 || schemaList.list == nil {
		return nil
	}
	defer C.moqi_rime_free_schema_list(&schemaList)

	items := unsafe.Slice(schemaList.list, int(schemaList.size))
	schemas := make([]RimeSchema, 0, len(items))
	for _, item := range items {
		schemaID := C.GoString(item.schema_id)
		name := C.GoString(item.name)
		if schemaID == "" {
			continue
		}
		if name == "" {
			name = schemaID
		}
		schemas = append(schemas, RimeSchema{ID: schemaID, Name: name})
	}
	return schemas
}

func GetCurrentSchema(sessionId RimeSessionId) string {
	if sessionId == 0 {
		return ""
	}
	buf := make([]byte, 256)
	if !boolResult(C.moqi_rime_get_current_schema(C.RimeSessionId(sessionId), (*C.char)(unsafe.Pointer(&buf[0])), C.size_t(len(buf)))) {
		return ""
	}
	return cStringFromBytes(buf)
}

func SelectSchema(sessionId RimeSessionId, schemaID string) bool {
	if sessionId == 0 || schemaID == "" {
		return false
	}
	result := false
	withCString(schemaID, func(cSchemaID *C.char) {
		result = boolResult(C.moqi_rime_select_schema(C.RimeSessionId(sessionId), cSchemaID))
	})
	return result
}

func openConfig(configID string) (C.RimeConfig, bool) {
	if configID == "" {
		return C.RimeConfig{}, false
	}
	var config C.RimeConfig
	ok := false
	withCString(configID, func(cConfigID *C.char) {
		ok = boolResult(C.moqi_rime_config_open(cConfigID, &config))
	})
	if !ok {
		return C.RimeConfig{}, false
	}
	return config, true
}

func closeConfig(config *C.RimeConfig) {
	if config == nil {
		return
	}
	C.moqi_rime_config_close(config)
}

func configGetCString(config *C.RimeConfig, key string) string {
	if config == nil || key == "" {
		return ""
	}
	var result string
	withCString(key, func(cKey *C.char) {
		result = C.GoString(C.moqi_rime_config_get_cstring(config, cKey))
	})
	return result
}

func configListSize(config *C.RimeConfig, key string) int {
	if config == nil || key == "" {
		return 0
	}
	size := 0
	withCString(key, func(cKey *C.char) {
		size = int(C.moqi_rime_config_list_size(config, cKey))
	})
	return size
}

func configListStrings(config *C.RimeConfig, key string) []string {
	if config == nil || key == "" {
		return nil
	}
	var iterator C.RimeConfigIterator
	ok := false
	withCString(key, func(cKey *C.char) {
		ok = boolResult(C.moqi_rime_config_begin_list(&iterator, config, cKey))
	})
	if !ok {
		return nil
	}
	defer C.moqi_rime_config_end(&iterator)

	var items []string
	for boolResult(C.moqi_rime_config_next(&iterator)) {
		value := strings.TrimSpace(configGetCString(config, C.GoString(iterator.path)))
		if value != "" {
			items = append(items, value)
		}
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

func getSchemaSwitchesFromConfig(config *C.RimeConfig) []RimeSwitch {
	if config == nil {
		return nil
	}
	var iterator C.RimeConfigIterator
	ok := false
	withCString("switches", func(cKey *C.char) {
		ok = boolResult(C.moqi_rime_config_begin_list(&iterator, config, cKey))
	})
	if !ok {
		return nil
	}
	defer C.moqi_rime_config_end(&iterator)

	switches := make([]RimeSwitch, 0)
	for boolResult(C.moqi_rime_config_next(&iterator)) {
		basePath := C.GoString(iterator.path)
		name := strings.TrimSpace(configGetCString(config, configPathJoin(basePath, "name")))
		if name == "" {
			continue
		}
		statesPath := configPathJoin(basePath, "states")
		stateCount := configListSize(config, statesPath)
		states := make([]string, 0, stateCount)
		for _, state := range configListStrings(config, statesPath) {
			state = strings.TrimSpace(state)
			if state != "" {
				states = append(states, state)
			}
		}
		switches = append(switches, RimeSwitch{Name: name, States: states})
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
	if schemaID == "" {
		return nil
	}
	var config C.RimeConfig
	ok := false
	withCString(schemaID, func(cSchemaID *C.char) {
		ok = boolResult(C.moqi_rime_schema_open(cSchemaID, &config))
	})
	if !ok {
		return nil
	}
	defer closeConfig(&config)
	return configListStrings(&config, key)
}

func GetSchemaSwitches(schemaID string) []RimeSwitch {
	if schemaID == "" {
		return nil
	}
	var config C.RimeConfig
	ok := false
	withCString(schemaID, func(cSchemaID *C.char) {
		ok = boolResult(C.moqi_rime_schema_open(cSchemaID, &config))
	})
	if !ok {
		return nil
	}
	defer closeConfig(&config)
	return getSchemaSwitchesFromConfig(&config)
}

func SetSchemaPageSize(schemaID string, pageSize int) bool {
	if schemaID == "" || pageSize <= 0 {
		return false
	}
	var config C.RimeConfig
	ok := false
	withCString(schemaID, func(cSchemaID *C.char) {
		ok = boolResult(C.moqi_rime_schema_open(cSchemaID, &config))
	})
	if !ok {
		return false
	}
	defer closeConfig(&config)

	result := false
	withCString("menu/page_size", func(cPath *C.char) {
		result = boolResult(C.moqi_rime_config_set_int(&config, cPath, C.int(pageSize)))
	})
	return result
}

func SelectCandidate(sessionId RimeSessionId, index int) bool {
	if sessionId == 0 || index < 0 {
		return false
	}
	return boolResult(C.moqi_rime_select_candidate_on_current_page(C.RimeSessionId(sessionId), C.size_t(index)))
}

func HighlightCandidate(sessionId RimeSessionId, index int) bool {
	if sessionId == 0 || index < 0 {
		return false
	}
	return boolResult(C.moqi_rime_highlight_candidate_on_current_page(C.RimeSessionId(sessionId), C.size_t(index)))
}

func ChangePage(sessionId RimeSessionId, backward bool) bool {
	if sessionId == 0 {
		return false
	}
	return boolResult(C.moqi_rime_change_page(C.RimeSessionId(sessionId), cBool(backward)))
}

func DeployConfigFile(filePath, key string) bool {
	if filePath == "" {
		return false
	}
	result := false
	withCString(filePath, func(cFile *C.char) {
		withCString(key, func(cKey *C.char) {
			result = boolResult(C.moqi_rime_deploy_config_file(cFile, cKey))
		})
	})
	return result
}

func StartMaintenance(fullcheck bool) bool {
	return boolResult(C.moqi_rime_start_maintenance(cBool(fullcheck)))
}

func JoinMaintenanceThread() {
	C.moqi_rime_join_maintenance_thread()
}

func SyncUserData() bool {
	return boolResult(C.moqi_rime_sync_user_data())
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
	return C.GoString(C.moqi_rime_get_version())
}

func initializeEngine(traits RimeTraits, fullcheck bool) bool {
	start := time.Now()
	success := false
	defer func() {
		debugLogf("RIME initializeEngine 完成 elapsed=%s success=%t fullcheck=%t", time.Since(start), success, fullcheck)
	}()

	debugLogf("RIME initializeEngine 开始 fullcheck=%t sharedDir=%q userDir=%q prebuiltDir=%q stagingDir=%q", fullcheck, traits.SharedDataDir, traits.UserDataDir, traits.PrebuiltDataDir, traits.StagingDir)
	if !Init(traits) {
		log.Println("RIME setup 失败")
		return false
	}

	C.moqi_rime_initialize()

	if StartMaintenance(fullcheck) {
		JoinMaintenanceThread()
	}
	success = true
	return true
}

func RimeInit(datadir, userdir, appname, appver string, fullcheck bool) bool {
	start := time.Now()
	success := false
	defer func() {
		debugLogf("RIME RimeInit 完成 elapsed=%s success=%t fullcheck=%t datadir=%q userdir=%q", time.Since(start), success, fullcheck, datadir, userdir)
	}()

	if err := os.MkdirAll(userdir, 0700); err != nil {
		log.Printf("创建用户目录失败: %v", err)
		return false
	}

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
		Modules:              []string{"default", "lua"},
		LogDir:               logDir,
		PrebuiltDataDir:      filepath.Join(datadir, "build"),
		StagingDir:           filepath.Join(userdir, "build"),
	}
	if !initializeEngine(traits, fullcheck) {
		return false
	}
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
		Modules:              []string{"default", "lua"},
		LogDir:               logDir,
		PrebuiltDataDir:      filepath.Join(datadir, "build"),
		StagingDir:           filepath.Join(userdir, "build"),
	}

	Finalize()
	return initializeEngine(traits, true)
}

func getContext(sessionId RimeSessionId) (C.RimeContext, bool) {
	if sessionId == 0 {
		return C.RimeContext{}, false
	}
	context := C.RimeContext{data_size: C.int(C.moqi_rime_struct_data_size_RimeContext())}
	if !boolResult(C.moqi_rime_get_context(C.RimeSessionId(sessionId), &context)) {
		return C.RimeContext{}, false
	}
	return context, true
}

func freeContext(context *C.RimeContext) {
	if context == nil {
		return
	}
	C.moqi_rime_free_context(context)
}
