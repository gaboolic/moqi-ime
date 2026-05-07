package mobilebridge

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
	"github.com/gaboolic/moqi-ime/input_methods/fcitx5"
	"github.com/gaboolic/moqi-ime/input_methods/moqi"
	"github.com/gaboolic/moqi-ime/input_methods/rime"
)

const (
	GUIDMoqi   = "{5C8E1D74-2F9A-4B63-91DE-7A45C8F2B306}"
	GUIDRime   = "{3F6B5A12-8D44-4E71-9A2E-6B4F9C1D2A30}"
	GUIDFcitx5 = "{D2E4A8B1-6C35-4F90-AB7D-18E2635C9F41}"

	mobileSlowLogThreshold = 30 * time.Millisecond
)

type StringList struct {
	values []string
}

func newStringList(values []string) *StringList {
	return &StringList{values: append([]string{}, values...)}
}

func (l *StringList) Len() int {
	if l == nil {
		return 0
	}
	return len(l.values)
}

func (l *StringList) Get(index int) string {
	if l == nil || index < 0 || index >= len(l.values) {
		return ""
	}
	return l.values[index]
}

func SetAndroidDataDir(path string) {
	rime.SetAndroidDataDir(path)
}

type MobileResponse struct {
	Success            bool
	ReturnValue        int
	CompositionString  string
	CommitString       string
	CandidateList      *StringList
	CandidateEntries   *StringList
	ShowCandidates     bool
	CursorPos          int
	CompositionCursor  int
	CandidateCursor    int
	HasCandidateCursor bool
	SetSelKeys         string
	Message            string
	Error              string
}

type Session struct {
	mu          sync.Mutex
	clientID    string
	guid        string
	seqNum      int
	service     imecore.TextService
	composition string
	candidates  []string
	show        bool
	cursorPos   int
	closed      bool
}

func NewSession(guid string) *Session {
	guid = strings.TrimSpace(guid)
	if guid == "" {
		guid = GUIDMoqi
	}
	return &Session{
		clientID: "android",
		guid:     strings.ToLower(guid),
	}
}

func (s *Session) Init() *MobileResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seqNum++
	client := &imecore.Client{
		ID:   s.clientID,
		GUID: s.guid,
	}
	service, err := newService(client, s.guid)
	if err != nil {
		return errorResponse(s.seqNum, err)
	}
	s.service = service
	s.closed = false
	req := s.baseRequest("init")
	req.ID = imecore.FlexibleID{String: s.guid}
	ok := s.service.Init(req)
	if !ok {
		return errorResponse(s.seqNum, fmt.Errorf("init failed"))
	}
	return s.applyResponse(imecore.NewResponse(s.seqNum, true))
}

func (s *Session) Activate() *MobileResponse {
	return s.handle("onActivate", 0, 0, -1, false, 0)
}

func (s *Session) Deactivate() *MobileResponse {
	return s.handle("onDeactivate", 0, 0, -1, false, 0)
}

func (s *Session) KeyDown(keyCode int, charCode int) *MobileResponse {
	totalStart := time.Now()
	filterStart := time.Now()
	resp := s.handle("filterKeyDown", keyCode, charCode, -1, false, 0)
	filterElapsed := time.Since(filterStart)
	if resp != nil && resp.ReturnValue != 0 {
		onStart := time.Now()
		onResp := s.handle("onKeyDown", keyCode, charCode, -1, false, 0)
		onElapsed := time.Since(onStart)
		logMobileSlow("KeyDown", totalStart, "key=%d char=%d filter=%s on=%s ret=%d", keyCode, charCode, filterElapsed, onElapsed, onRespReturnValue(onResp))
		if responseHasPayload(onResp) || onResp.ReturnValue != 0 {
			return onResp
		}
	}
	logMobileSlow("KeyDown", totalStart, "key=%d char=%d filter=%s ret=%d", keyCode, charCode, filterElapsed, onRespReturnValue(resp))
	return resp
}

func (s *Session) KeyUp(keyCode int, charCode int) *MobileResponse {
	resp := s.handle("filterKeyUp", keyCode, charCode, -1, false, 0)
	if resp != nil && resp.ReturnValue != 0 {
		onResp := s.handle("onKeyUp", keyCode, charCode, -1, false, 0)
		if responseHasPayload(onResp) || onResp.ReturnValue != 0 {
			return onResp
		}
	}
	return resp
}

func (s *Session) SelectCandidate(index int) *MobileResponse {
	return s.handle("selectCandidate", 0, 0, index, false, 0)
}

func (s *Session) ChangePage(backward bool) *MobileResponse {
	return s.handle("changePage", 0, 0, -1, backward, 0)
}

func (s *Session) Command(commandID int) *MobileResponse {
	return s.handle("onCommand", 0, 0, -1, false, commandID)
}

func (s *Session) SchemeSets() *StringList {
	return newStringList(rime.AvailableSchemeSetNames())
}

func (s *Session) CurrentSchemeSet() string {
	return rime.CurrentSchemeSetName()
}

func (s *Session) SelectSchemeSet(name string) *MobileResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seqNum++
	if s.service == nil || s.closed {
		return errorResponse(s.seqNum, fmt.Errorf("session is not initialized"))
	}
	if ime, ok := s.service.(*rime.IME); ok {
		return s.applyResponse(ime.MobileSelectSchemeSet(name, s.seqNum))
	}
	if !rime.SelectSchemeSetName(name) {
		return errorResponse(s.seqNum, fmt.Errorf("failed to select scheme set: %s", name))
	}
	return &MobileResponse{Success: true, ReturnValue: 1, CandidateList: newStringList(nil)}
}

func (s *Session) SchemaEntries() *StringList {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ime, ok := s.service.(*rime.IME); ok {
		return newStringList(ime.MobileSchemaEntries())
	}
	return newStringList(nil)
}

func (s *Session) MenuEntries() *StringList {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ime, ok := s.service.(*rime.IME); ok {
		return newStringList(ime.MobileMenuEntries())
	}
	return newStringList(nil)
}

func (s *Session) CurrentSchemaID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ime, ok := s.service.(*rime.IME); ok {
		return ime.MobileCurrentSchemaID()
	}
	return ""
}

func (s *Session) SelectSchema(schemaID string) *MobileResponse {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seqNum++
	if s.service == nil || s.closed {
		return errorResponse(s.seqNum, fmt.Errorf("session is not initialized"))
	}
	if ime, ok := s.service.(*rime.IME); ok && ime.MobileSelectSchema(schemaID) {
		return s.applyResponse(imecore.NewResponse(s.seqNum, true))
	}
	return errorResponse(s.seqNum, fmt.Errorf("failed to select schema: %s", schemaID))
}

func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.service != nil && !s.closed {
		s.seqNum++
		s.service.HandleRequest(s.baseRequest("close"))
		s.service.Close()
	}
	s.closed = true
	s.service = nil
}

func (s *Session) handle(method string, keyCode int, charCode int, candidateIndex int, pageBackward bool, commandID int) *MobileResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seqNum++
	if s.service == nil || s.closed {
		return errorResponse(s.seqNum, fmt.Errorf("session is not initialized"))
	}
	req := s.baseRequest(method)
	req.KeyCode = keyCode
	req.CharCode = charCode
	if candidateIndex >= 0 {
		req.CandidateIndex = candidateIndex
		req.HasCandidateIndex = true
	}
	req.PageBackward = pageBackward
	if commandID != 0 {
		req.ID = imecore.FlexibleID{Int: commandID, IsInt: true}
		req.CommandType = commandID
		req.Data = map[string]interface{}{
			"commandId": float64(commandID),
		}
	}

	start := time.Now()
	resp := s.service.HandleRequest(req)
	handleElapsed := time.Since(start)
	applyStart := time.Now()
	mobileResp := s.applyResponse(resp)
	applyElapsed := time.Since(applyStart)
	if elapsed := time.Since(start); elapsed >= mobileSlowLogThreshold {
		log.Printf("mobilebridge slow handle method=%s key=%d char=%d total=%s handle=%s apply=%s ret=%d candidates=%d", method, keyCode, charCode, elapsed, handleElapsed, applyElapsed, mobileResp.ReturnValue, mobileResp.CandidateList.Len())
	}
	return mobileResp
}

func (s *Session) baseRequest(method string) *imecore.Request {
	return &imecore.Request{
		Method:            method,
		SeqNum:            s.seqNum,
		ID:                imecore.FlexibleID{String: s.guid},
		CompositionString: s.composition,
		CandidateList:     append([]string{}, s.candidates...),
		ShowCandidates:    s.show,
		CursorPos:         s.cursorPos,
		Data: map[string]interface{}{
			"source": "android",
		},
	}
}

func (s *Session) applyResponse(resp *imecore.Response) *MobileResponse {
	if resp == nil {
		return errorResponse(s.seqNum, fmt.Errorf("response is nil"))
	}

	if resp.CompositionString != "" || resp.CommitString != "" || resp.ReturnValue != 0 || !resp.ShowCandidates {
		s.composition = resp.CompositionString
		s.cursorPos = resp.CursorPos
	}
	if resp.CandidateList != nil {
		s.candidates = append([]string{}, resp.CandidateList...)
	}
	s.show = resp.ShowCandidates
	if resp.CommitString != "" {
		s.composition = ""
		s.candidates = nil
		s.show = false
		s.cursorPos = 0
	}

	return &MobileResponse{
		Success:            resp.Success,
		ReturnValue:        resp.ReturnValue,
		CompositionString:  resp.CompositionString,
		CommitString:       resp.CommitString,
		CandidateList:      newStringList(resp.CandidateList),
		CandidateEntries:   newStringList(mobileCandidateEntries(resp)),
		ShowCandidates:     resp.ShowCandidates,
		CursorPos:          resp.CursorPos,
		CompositionCursor:  resp.CompositionCursor,
		CandidateCursor:    resp.CandidateCursor,
		HasCandidateCursor: resp.HasCandidateCursor,
		SetSelKeys:         resp.SetSelKeys,
		Message:            resp.Message,
		Error:              resp.Error,
	}
}

func mobileCandidateEntries(resp *imecore.Response) []string {
	if resp == nil || len(resp.CandidateEntries) == 0 {
		return nil
	}
	entries := make([]string, 0, len(resp.CandidateEntries))
	for _, candidate := range resp.CandidateEntries {
		entries = append(entries, sanitizeMobileCandidateField(candidate.Text)+"\t"+sanitizeMobileCandidateField(candidate.Comment))
	}
	return entries
}

func sanitizeMobileCandidateField(value string) string {
	value = strings.ReplaceAll(value, "\t", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func newService(client *imecore.Client, guid string) (imecore.TextService, error) {
	switch strings.ToLower(guid) {
	case strings.ToLower(GUIDMoqi):
		return moqi.New(client), nil
	case strings.ToLower(GUIDRime):
		return rime.New(client), nil
	case strings.ToLower(GUIDFcitx5):
		return fcitx5.New(client), nil
	default:
		return nil, fmt.Errorf("unknown input method guid: %s", guid)
	}
}

func errorResponse(seq int, err error) *MobileResponse {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &MobileResponse{
		Success:       false,
		ReturnValue:   0,
		CandidateList: newStringList(nil),
		Error:         msg,
	}
}

func responseHasPayload(resp *MobileResponse) bool {
	if resp == nil {
		return false
	}
	return resp.CompositionString != "" ||
		resp.CommitString != "" ||
		resp.ShowCandidates ||
		resp.CandidateList.Len() > 0 ||
		resp.Error != "" ||
		resp.Message != ""
}

func onRespReturnValue(resp *MobileResponse) int {
	if resp == nil {
		return 0
	}
	return resp.ReturnValue
}

func logMobileSlow(operation string, start time.Time, format string, args ...interface{}) {
	elapsed := time.Since(start)
	if elapsed < mobileSlowLogThreshold {
		return
	}
	log.Printf("mobilebridge slow %s total=%s %s", operation, elapsed, fmt.Sprintf(format, args...))
}
