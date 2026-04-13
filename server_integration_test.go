package main

import (
	"encoding/binary"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/gaboolic/moqi-ime/imecore"
	fcitx5ime "github.com/gaboolic/moqi-ime/input_methods/fcitx5"
	simplepinyinime "github.com/gaboolic/moqi-ime/input_methods/moqi"
	rimeime "github.com/gaboolic/moqi-ime/input_methods/rime"
	moqipb "github.com/gaboolic/moqi-ime/proto"
	gproto "google.golang.org/protobuf/proto"
)

const testSimplePinyinGUID = "{5C8E1D74-2F9A-4B63-91DE-7A45C8F2B306}"
const testRimeGUID = "{3F6B5A12-8D44-4E71-9A2E-6B4F9C1D2A30}"
const testFcitx5GUID = "{D2E4A8B1-6C35-4F90-AB7D-18E2635C9F41}"

func newTestServerWithSimplePinyin() *Server {
	server := NewServer()
	server.RegisterService(testSimplePinyinGUID, func(client *imecore.Client, guid string) imecore.TextService {
		return simplepinyinime.New(client)
	})
	return server
}

func newTestServerWithRime() *Server {
	server := NewServer()
	server.RegisterService(testRimeGUID, func(client *imecore.Client, guid string) imecore.TextService {
		return rimeime.New(client)
	})
	return server
}

func newTestServerWithFcitx5() *Server {
	server := NewServer()
	server.RegisterService(testFcitx5GUID, func(client *imecore.Client, guid string) imecore.TextService {
		return fcitx5ime.New(client)
	})
	return server
}

func captureStdoutBytes(t *testing.T, fn func()) []byte {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = oldStdout
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}
	return output
}

func sendProtocolMessage(t *testing.T, server *Server, req *moqipb.ClientRequest) *moqipb.ServerResponse {
	t.Helper()

	output := captureStdoutBytes(t, func() {
		if err := server.handleMessage(req); err != nil {
			t.Fatalf("handleMessage failed: %v", err)
		}
	})
	if len(output) < 4 {
		t.Fatalf("expected framed response, got %d bytes", len(output))
	}
	size := binary.LittleEndian.Uint32(output[:4])
	payload := output[4:]
	if int(size) != len(payload) {
		t.Fatalf("expected payload size %d, got %d", size, len(payload))
	}

	resp := &moqipb.ServerResponse{}
	if err := gproto.Unmarshal(payload, resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return resp
}

func keyRequest(clientID string, method moqipb.Method, seq uint32, keyCode uint32, charCode uint32) *moqipb.ClientRequest {
	return &moqipb.ClientRequest{
		ClientId: &clientID,
		Method:   method,
		SeqNum:   seq,
		KeyEvent: &moqipb.KeyEvent{
			KeyCode:  keyCode,
			CharCode: charCode,
		},
	}
}

func TestServerHandleMessageInitUsesGuid(t *testing.T) {
	server := newTestServerWithSimplePinyin()
	clientID := "client-1"
	guid := testSimplePinyinGUID

	response := sendProtocolMessage(t, server, &moqipb.ClientRequest{
		ClientId:         &clientID,
		Method:           moqipb.Method_METHOD_INIT,
		SeqNum:           1,
		Guid:             &guid,
		IsWindows8Above:  true,
		IsMetroApp:       false,
		IsUiLess:         false,
		IsConsole:        false,
	})

	if !response.GetSuccess() {
		t.Fatalf("expected init success, got %#v", response)
	}
	if response.GetSeqNum() != 1 {
		t.Fatalf("expected seqNum 1, got %#v", response.GetSeqNum())
	}
	client := server.clients[clientID]
	if client == nil {
		t.Fatal("expected client to be registered after init")
	}
	if client.GUID != strings.ToLower(testSimplePinyinGUID) {
		t.Fatalf("expected guid %q, got %q", strings.ToLower(testSimplePinyinGUID), client.GUID)
	}
}

func TestServerHandleMessageUninitializedClientReturnsProtocolError(t *testing.T) {
	server := newTestServerWithSimplePinyin()
	clientID := "client-3"

	response := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_ON_KEY_DOWN, 9, 0x4D, 'm'))
	if response.GetSuccess() {
		t.Fatalf("expected uninitialized client to fail, got %#v", response)
	}
	if response.GetSeqNum() != 9 {
		t.Fatalf("expected seqNum 9, got %#v", response.GetSeqNum())
	}
	if response.GetError() != "客户端未初始化" {
		t.Fatalf("expected protocol error for uninitialized client, got %#v", response.GetError())
	}
}

func TestServerHandleMessageCloseSucceeds(t *testing.T) {
	server := newTestServerWithSimplePinyin()
	clientID := "client-close"
	guid := testSimplePinyinGUID

	sendProtocolMessage(t, server, &moqipb.ClientRequest{
		ClientId:        &clientID,
		Method:          moqipb.Method_METHOD_INIT,
		SeqNum:          1,
		Guid:            &guid,
		IsWindows8Above: true,
	})

	response := sendProtocolMessage(t, server, &moqipb.ClientRequest{
		ClientId: &clientID,
		Method:   moqipb.Method_METHOD_CLOSE,
		SeqNum:   2,
	})
	if !response.GetSuccess() {
		t.Fatalf("expected close success, got %#v", response)
	}
	if _, ok := server.clients[clientID]; ok {
		t.Fatal("expected client to be removed after close")
	}
}

func TestServerHandleMessageSimplePinyinRequestResponseFlow(t *testing.T) {
	server := newTestServerWithSimplePinyin()
	clientID := "client-4"
	guid := testSimplePinyinGUID

	sendProtocolMessage(t, server, &moqipb.ClientRequest{
		ClientId:        &clientID,
		Method:          moqipb.Method_METHOD_INIT,
		SeqNum:          1,
		Guid:            &guid,
		IsWindows8Above: true,
	})

	firstResp := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_FILTER_KEY_DOWN, 2, 0x4E, 'n'))
	if firstResp.GetCompositionString() != "n" {
		t.Fatalf("expected first key to build composition n, got %#v", firstResp)
	}
	if firstResp.GetReturnValue() != 1 {
		t.Fatalf("expected first key return 1, got %#v", firstResp)
	}

	secondResp := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_FILTER_KEY_DOWN, 3, 0x49, 'i'))
	if secondResp.GetCompositionString() != "ni" {
		t.Fatalf("expected second key to build composition ni, got %#v", secondResp)
	}
	if len(secondResp.GetCandidateList()) != 3 {
		t.Fatalf("expected fallback candidate count 3, got %d", len(secondResp.GetCandidateList()))
	}
	if secondResp.GetCandidateList()[0] != "测试" {
		t.Fatalf("expected fallback candidate 测试, got %#v", secondResp.GetCandidateList()[0])
	}

	selectResp := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_FILTER_KEY_DOWN, 4, 0x31, 0))
	if selectResp.GetCommitString() != "测试" {
		t.Fatalf("expected number key to commit first fallback candidate, got %#v", selectResp)
	}
	if selectResp.GetReturnValue() != 1 {
		t.Fatalf("expected candidate selection return 1, got %#v", selectResp)
	}
	if selectResp.GetShowCandidates() {
		t.Fatalf("expected candidate window to close, got %#v", selectResp)
	}
}

func TestServerHandleMessageRimeRequestResponseFlow(t *testing.T) {
	server := newTestServerWithRime()
	clientID := "client-6"
	guid := testRimeGUID

	sendProtocolMessage(t, server, &moqipb.ClientRequest{
		ClientId:        &clientID,
		Method:          moqipb.Method_METHOD_INIT,
		SeqNum:          1,
		Guid:            &guid,
		IsWindows8Above: true,
	})

	service, ok := server.clients[clientID].Service.(*rimeime.IME)
	if !ok {
		t.Fatal("expected concrete Rime IME service")
	}
	if !service.BackendAvailable() {
		t.Skip("native Rime backend unavailable in test environment")
	}

	firstResp := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_FILTER_KEY_DOWN, 2, 0x4E, 'n'))
	if firstResp.GetReturnValue() != 1 {
		t.Fatalf("expected first key return 1, got %#v", firstResp)
	}

	firstKeyState := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_ON_KEY_DOWN, 3, 0x4E, 'n'))
	if firstKeyState.GetCompositionString() != "n" {
		t.Fatalf("expected onKeyDown to expose n, got %#v", firstKeyState)
	}
	if len(firstKeyState.GetCandidateList()) == 0 {
		t.Fatalf("expected prefix candidates, got %#v", firstKeyState.GetCandidateList())
	}
	if firstKeyState.GetCandidateList()[0] != "你" {
		t.Fatalf("expected first candidate 你, got %#v", firstKeyState.GetCandidateList()[0])
	}

	secondResp := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_FILTER_KEY_DOWN, 4, 0x49, 'i'))
	if secondResp.GetReturnValue() != 1 {
		t.Fatalf("expected second key return 1, got %#v", secondResp)
	}

	secondKeyState := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_ON_KEY_DOWN, 5, 0x49, 'i'))
	if secondKeyState.GetCompositionString() != "ni" {
		t.Fatalf("expected second key to build ni, got %#v", secondKeyState)
	}
	if len(secondKeyState.GetCandidateList()) < 2 {
		t.Fatalf("expected exact candidates after ni, got %#v", secondKeyState.GetCandidateList())
	}
	if secondKeyState.GetCandidateList()[1] != "呢" {
		t.Fatalf("expected second candidate 呢, got %#v", secondKeyState.GetCandidateList()[1])
	}

	selectFilterResp := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_FILTER_KEY_DOWN, 6, 0x32, 0))
	if selectFilterResp.GetReturnValue() != 1 {
		t.Fatalf("expected number filter to be handled, got %#v", selectFilterResp)
	}

	selectResp := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_ON_KEY_DOWN, 7, 0x32, 0))
	if selectResp.GetCommitString() != "呢" {
		t.Fatalf("expected number key to commit 呢, got %#v", selectResp)
	}
	if selectResp.GetReturnValue() != 1 {
		t.Fatalf("expected candidate selection return 1, got %#v", selectResp)
	}
}

func TestServerHandleMessageFcitx5RequestResponseFlow(t *testing.T) {
	server := newTestServerWithFcitx5()
	clientID := "client-7"
	guid := testFcitx5GUID

	sendProtocolMessage(t, server, &moqipb.ClientRequest{
		ClientId:        &clientID,
		Method:          moqipb.Method_METHOD_INIT,
		SeqNum:          1,
		Guid:            &guid,
		IsWindows8Above: true,
	})

	firstResp := sendProtocolMessage(t, server, keyRequest(clientID, moqipb.Method_METHOD_FILTER_KEY_DOWN, 2, 0x48, 'h'))
	if firstResp.GetCompositionString() != "ha" {
		t.Fatalf("expected first key to build ha, got %#v", firstResp)
	}
	if firstResp.GetReturnValue() != 1 {
		t.Fatalf("expected first key return 1, got %#v", firstResp)
	}
	if len(firstResp.GetCandidateList()) != 5 {
		t.Fatalf("expected 5 candidates, got %d", len(firstResp.GetCandidateList()))
	}
	if firstResp.GetCandidateList()[2] != "喝" {
		t.Fatalf("expected third candidate 喝, got %#v", firstResp.GetCandidateList()[2])
	}

	commandID := uint32(0x33)
	selectResp := sendProtocolMessage(t, server, &moqipb.ClientRequest{
		ClientId:      &clientID,
		Method:        moqipb.Method_METHOD_ON_KEY_DOWN,
		SeqNum:        3,
		CommandId:     &commandID,
		CandidateList: []string{"哈", "呵", "喝", "和", "河"},
		KeyEvent:      &moqipb.KeyEvent{KeyCode: 0x33},
	})
	if selectResp.GetCommitString() != "喝" {
		t.Fatalf("expected number key to commit 喝, got %#v", selectResp)
	}
	if selectResp.GetReturnValue() != 1 {
		t.Fatalf("expected candidate selection return 1, got %#v", selectResp)
	}
}
