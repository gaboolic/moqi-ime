package main

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
	gproto "google.golang.org/protobuf/proto"
)

func decodeCapturedFrame(t *testing.T, data []byte) []byte {
	t.Helper()
	if len(data) < 4 {
		t.Fatalf("expected framed payload, got %d bytes", len(data))
	}
	size := binary.LittleEndian.Uint32(data[:4])
	if int(size) != len(data[4:]) {
		t.Fatalf("expected payload size %d, got %d", size, len(data[4:]))
	}
	return data[4:]
}

func TestBuildProtoResponseIncludesClearedCompositionState(t *testing.T) {
	resp := imecore.NewResponse(1, true)
	resp.ReturnValue = 1

	got, err := imecore.BuildProtoResponse("client-1", resp)
	if err != nil {
		t.Fatalf("BuildProtoResponse failed: %v", err)
	}

	if got.GetCompositionString() != "" {
		t.Fatalf("expected empty compositionString, got %q", got.GetCompositionString())
	}
	if len(got.GetCandidateList()) != 0 {
		t.Fatalf("expected empty candidateList, got %#v", got.GetCandidateList())
	}
	if got.GetShowCandidates() {
		t.Fatalf("expected showCandidates=false, got true")
	}
	if got.GetSelStart() != 0 {
		t.Fatalf("expected selStart=0, got %d", got.GetSelStart())
	}
	if got.GetSelEnd() != 0 {
		t.Fatalf("expected selEnd=0, got %d", got.GetSelEnd())
	}
}

func TestBuildProtoResponseUsesMenuItemsWhenPresent(t *testing.T) {
	resp := imecore.NewResponse(2, true)
	resp.ReturnValue = 1
	resp.ReturnData = []map[string]interface{}{
		{"id": 1, "text": "中文 → 西文"},
	}

	got, err := imecore.BuildProtoResponse("client-2", resp)
	if err != nil {
		t.Fatalf("BuildProtoResponse failed: %v", err)
	}

	if len(got.GetMenuItems()) != 1 {
		t.Fatalf("expected menu return data, got %#v", got.GetMenuItems())
	}
	if got.GetMenuItems()[0].GetText() != "中文 → 西文" {
		t.Fatalf("unexpected menu text %#v", got.GetMenuItems()[0])
	}
}

func TestWriteFramePrefixesPayloadLength(t *testing.T) {
	payload, err := gproto.Marshal(imecoreTestResponse())
	if err != nil {
		t.Fatalf("marshal proto response: %v", err)
	}

	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- writeFrame(writer, payload)
		_ = writer.Close()
	}()

	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("writeFrame failed: %v", err)
	}
	if decoded := decodeCapturedFrame(t, raw); string(decoded) != string(payload) {
		t.Fatalf("decoded frame payload mismatch")
	}
}

func TestOpenLogFileUsesMoqiIMLogDirectoryUnderLocalAppData(t *testing.T) {
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)

	logFile, err := openLogFile()
	if err != nil {
		t.Fatalf("openLogFile failed: %v", err)
	}
	defer logFile.Close()

	want := filepath.Join(localAppData, "MoqiIM", "Log", dailyLogFileName("moqi-ime.log", time.Now()))
	if got := logFile.Name(); filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("expected log path %q, got %q", want, got)
	}
}

func TestOpenLogFileRemovesDailyLogsOlderThanRetention(t *testing.T) {
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)

	logDir := filepath.Join(localAppData, "MoqiIM", "Log")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("mkdir log dir: %v", err)
	}

	oldFile := filepath.Join(logDir, dailyLogFileName("moqi-ime.log", time.Now().AddDate(0, 0, -logRetentionDays)))
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatalf("write old log file: %v", err)
	}
	recentFile := filepath.Join(logDir, dailyLogFileName("moqi-ime.log", time.Now().AddDate(0, 0, -(logRetentionDays-2))))
	if err := os.WriteFile(recentFile, []byte("recent"), 0644); err != nil {
		t.Fatalf("write recent log file: %v", err)
	}

	logFile, err := openLogFile()
	if err != nil {
		t.Fatalf("openLogFile failed: %v", err)
	}
	defer logFile.Close()

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old log file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(recentFile); err != nil {
		t.Fatalf("expected recent log file to remain, stat err=%v", err)
	}
}

func imecoreTestResponse() gproto.Message {
	msg, _ := imecore.BuildProtoResponse("client-1", imecore.NewResponse(1, true))
	return msg
}
