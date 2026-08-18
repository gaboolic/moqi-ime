package rime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
	"github.com/gaboolic/moqi-ime/input_methods/rime/cloudclipboard"
)

type cloudClipboardEntry struct {
	Name     string
	Preview  string
	Comment  string
	FullText string
}

type cloudClipboardAsyncResult struct {
	RequestSeq uint64
	Entries    []cloudClipboardEntry
	Err        error
}

const (
	cloudClipboardCandidatePreviewMaxLength = 60
	cloudClipboardPageSize                  = 9
)

func cloudClipboardTrayNotification(message string, icon imecore.TrayNotificationIcon) *imecore.TrayNotification {
	return &imecore.TrayNotification{
		Title:   "云剪贴板",
		Message: message,
		Icon:    icon,
	}
}

func (ime *IME) cloudClipboardMenuSection() map[string]interface{} {
	cfg := loadCloudClipboardConfig()
	return map[string]interface{}{
		"text": "同步设置",
		"submenu": []map[string]interface{}{
			{"id": ID_WEBDAV_SETTINGS, "text": "WebDAV 配置..."},
			{"id": ID_CLOUD_CLIPBOARD_TEST, "text": "测试 WebDAV"},
			{"id": ID_CLOUD_CLIPBOARD_ENABLED, "text": "启用云剪贴板", "checked": cfg.Enabled},
			{"id": ID_CLOUD_CLIPBOARD_SETTINGS, "text": "云剪贴板设置..."},
			{"id": ID_SYNC, "text": "同步用户词库"},
		},
	}
}

func (ime *IME) onCloudClipboardUpload(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	if req == nil || resp == nil {
		return resp
	}
	text := strings.TrimSpace(req.CloudClipboardText)
	if text == "" {
		resp.ReturnValue = 0
		return resp
	}
	globalCloudClipboardService().UploadFromRequest(text)
	resp.ReturnValue = 1
	return resp
}

func (ime *IME) openCloudClipboardSettingsAsync(resp *imecore.Response) bool {
	go func() {
		current := loadCloudClipboardConfig()
		result, err := promptCloudClipboardSettings(context.Background(), current)
		var tray *imecore.TrayNotification
		if err != nil {
			if err.Error() == "已取消" {
				return
			}
			tray = cloudClipboardTrayNotification("打开设置失败: "+shortErrorMessage(err), imecore.TrayNotificationIconError)
		} else {
			cfg := dialogResultToConfig(result, current)
			if err := saveCloudClipboardConfig(cfg, "", true); err != nil {
				tray = cloudClipboardTrayNotification("保存失败: "+shortErrorMessage(err), imecore.TrayNotificationIconError)
			} else {
				tray = cloudClipboardTrayNotification("云剪贴板设置已保存", imecore.TrayNotificationIconInfo)
			}
		}
		if tray != nil {
			ime.sendAsyncTrayNotification(tray)
		}
	}()
	if resp != nil {
		resp.ReturnValue = 1
	}
	return true
}

func (ime *IME) openWebDAVSettingsAsync(resp *imecore.Response) bool {
	go func() {
		current := loadCloudClipboardConfig()
		hasPassword := readCloudClipboardPassword() != ""
		result, err := promptWebDAVSettings(context.Background(), current, hasPassword)
		var tray *imecore.TrayNotification
		if err != nil {
			if err.Error() == "已取消" {
				return
			}
			tray = cloudClipboardTrayNotification("打开 WebDAV 配置失败: "+shortErrorMessage(err), imecore.TrayNotificationIconError)
		} else {
			cfg := webDAVDialogResultToConfig(result, current)
			keepPassword := result.KeepPassword && result.Password == ""
			newPassword := result.Password
			if err := saveCloudClipboardConfig(cfg, newPassword, keepPassword); err != nil {
				tray = cloudClipboardTrayNotification("保存失败: "+shortErrorMessage(err), imecore.TrayNotificationIconError)
			} else {
				tray = cloudClipboardTrayNotification("WebDAV 配置已保存", imecore.TrayNotificationIconInfo)
			}
		}
		if tray != nil {
			ime.sendAsyncTrayNotification(tray)
		}
	}()
	if resp != nil {
		resp.ReturnValue = 1
	}
	return true
}

func testCloudClipboardConnection(cfg cloudclipboard.Config) error {
	if !cloudclipboard.IsAllowedBaseURL(cfg.BaseURL) {
		return fmt.Errorf("%s", cloudclipboard.RejectionMessage())
	}
	if cfg.Username == "" || cfg.Password == "" {
		return fmt.Errorf("请填写用户名和密码")
	}
	// A connection test only verifies that the WebDAV endpoint is reachable and
	// the credentials are valid; it must not depend on whether the cloud
	// clipboard is currently enabled. Force Enabled so the sync client is
	// actually created, otherwise reloadClient() would leave client==nil and
	// TestConnection() would just report "云剪贴板未配置完整".
	testCfg := cfg
	testCfg.Enabled = true
	sync := cloudclipboard.NewSync(testCfg)
	return sync.TestConnection()
}

func (ime *IME) toggleCloudClipboardEnabled(resp *imecore.Response) {
	cfg := loadCloudClipboardConfig()
	cfg.Enabled = !cfg.Enabled
	cfg.Password = readCloudClipboardPassword()
	_ = saveCloudClipboardConfig(cfg, "", true)
	state := "已关闭"
	if cfg.Enabled {
		state = "已开启"
	}
	resp.TrayNotification = cloudClipboardTrayNotification(state, imecore.TrayNotificationIconInfo)
	resp.ReturnValue = 1
}

func (ime *IME) testCloudClipboardConnectionCommand(resp *imecore.Response) {
	cfg := loadCloudClipboardConfig()
	if err := testCloudClipboardConnection(cfg); err != nil {
		resp.TrayNotification = cloudClipboardTrayNotification("连接失败: "+shortErrorMessage(err), imecore.TrayNotificationIconError)
	} else {
		resp.TrayNotification = cloudClipboardTrayNotification("WebDAV 连接成功", imecore.TrayNotificationIconInfo)
	}
	resp.ReturnValue = 1
}

func (ime *IME) handleCloudClipboardListHotkey(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil {
		return false
	}
	action := loadCloudClipboardListHotkey()
	if !action.matches(req) {
		return false
	}
	debugLogf("云剪贴板快捷键触发 event=down key=%d char=%d hotkey=%s", req.KeyCode, req.CharCode, action.Hotkey)
	return ime.showCloudClipboardList(resp)
}

func (ime *IME) handleCloudClipboardPreservedKey(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || req.Data == nil {
		return false
	}
	guid, _ := req.Data["guid"].(string)
	if !strings.EqualFold(guid, cloudClipboardListPreservedKeyGUID) {
		return false
	}
	debugLogf("云剪贴板 preserved key 触发 guid=%s", guid)
	return ime.showCloudClipboardList(resp)
}

func (ime *IME) showCloudClipboardList(resp *imecore.Response) bool {
	cfg := loadCloudClipboardConfig()
	if !cfg.Enabled {
		debugLogf("云剪贴板列表未显示: disabled")
		resp.ShowMessage = &imecore.MessageWindow{Message: "请先在设置中开启云剪贴板", Duration: 2500}
		resp.ReturnValue = 1
		return true
	}
	if !cfg.IsComplete() || !cloudclipboard.IsAllowedBaseURL(cfg.BaseURL) {
		debugLogf("云剪贴板列表未显示: incomplete base=%q user=%t root=%q", cfg.BaseURL, cfg.Username != "", cfg.SettingsRoot)
		resp.ShowMessage = &imecore.MessageWindow{Message: "请先完成 WebDAV 配置", Duration: 2500}
		resp.ReturnValue = 1
		return true
	}
	debugLogf("云剪贴板开始加载列表 hotkey=%s", cfg.ListHotkey)
	ime.resetCloudClipboardState()
	ime.resetAIState()
	ime.cloudClipboardRequestSeq++
	ime.cloudClipboardPending = true
	resp.ReturnValue = 1

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	items, err := globalCloudClipboardService().ListClipsForDisplay(ctx)
	result := cloudClipboardAsyncResult{RequestSeq: ime.cloudClipboardRequestSeq, Err: err}
	if err == nil {
		result.Entries = cloudClipboardEntriesFromItems(items)
	}
	if !ime.applyCloudClipboardAsyncResult(result) {
		if err != nil {
			debugLogf("云剪贴板列表加载失败: %v", err)
			resp.ShowMessage = &imecore.MessageWindow{Message: "加载失败: " + shortErrorMessage(err), Duration: 3000}
		} else {
			debugLogf("云剪贴板列表为空")
			resp.ShowMessage = &imecore.MessageWindow{Message: "云剪贴板为空", Duration: 2500}
		}
		return true
	}
	debugLogf("云剪贴板列表加载成功 count=%d", len(ime.cloudClipboardEntries))
	ime.fillCloudClipboardResponse(resp)
	return true
}

func cloudClipboardEntriesFromItems(items []cloudclipboard.DisplayItem) []cloudClipboardEntry {
	entries := make([]cloudClipboardEntry, 0, len(items))
	for _, item := range items {
		comment := formatCloudClipboardTime(item.LastModified)
		entries = append(entries, cloudClipboardEntry{
			Name:     item.Name,
			Preview:  item.Preview,
			Comment:  comment,
			FullText: item.FullText,
		})
	}
	return entries
}

func formatCloudClipboardTime(ms int64) string {
	if ms <= 0 {
		return ""
	}
	t := time.UnixMilli(ms)
	return t.Format("01-02 15:04")
}

func (ime *IME) applyCloudClipboardAsyncResult(result cloudClipboardAsyncResult) bool {
	if result.RequestSeq != ime.cloudClipboardRequestSeq {
		return false
	}
	ime.cloudClipboardPending = false
	if result.Err != nil {
		ime.resetCloudClipboardState()
		return false
	}
	ime.cloudClipboardEntries = append([]cloudClipboardEntry(nil), result.Entries...)
	ime.cloudClipboardCursor = 0
	ime.cloudClipboardPage = 0
	ime.cloudClipboardActive = len(ime.cloudClipboardEntries) > 0
	return ime.cloudClipboardActive
}

func (ime *IME) resetCloudClipboardState() {
	ime.cloudClipboardActive = false
	ime.cloudClipboardPending = false
	ime.cloudClipboardEntries = nil
	ime.cloudClipboardCursor = 0
	ime.cloudClipboardPage = 0
}

func (ime *IME) fillCloudClipboardResponse(resp *imecore.Response) {
	if resp == nil || !ime.cloudClipboardActive {
		return
	}
	total := len(ime.cloudClipboardEntries)
	if total == 0 {
		resp.ShowCandidates = false
		resp.HasCandidateCursor = false
		return
	}
	pageCount := cloudClipboardPageCount(total)
	if ime.cloudClipboardPage < 0 {
		ime.cloudClipboardPage = 0
	} else if ime.cloudClipboardPage >= pageCount {
		ime.cloudClipboardPage = pageCount - 1
	}
	start, end := ime.cloudClipboardPageBounds()
	if ime.cloudClipboardCursor < start || ime.cloudClipboardCursor >= end {
		ime.cloudClipboardCursor = start
	}

	resp.CompositionString = fmt.Sprintf("云剪贴板 %d/%d", ime.cloudClipboardPage+1, pageCount)
	resp.CursorPos = len([]rune(resp.CompositionString))
	resp.CompositionCursor = resp.CursorPos
	resp.CandidateList = make([]string, 0, end-start)
	resp.CandidateEntries = make([]imecore.CandidateEntry, 0, end-start)
	for _, entry := range ime.cloudClipboardEntries[start:end] {
		displayText := cloudclipboard.FormatPreview(entry.Preview, cloudClipboardCandidatePreviewMaxLength)
		resp.CandidateList = append(resp.CandidateList, displayText)
		resp.CandidateEntries = append(resp.CandidateEntries, imecore.CandidateEntry{
			Text:    displayText,
			Comment: entry.Comment,
		})
	}
	if len(resp.CandidateList) == 0 {
		resp.ShowCandidates = false
		resp.HasCandidateCursor = false
		return
	}
	resp.CandidateCursor = ime.cloudClipboardCursor - start
	resp.HasCandidateCursor = true
	resp.ShowCandidates = true
	resp.SetSelKeys = defaultSelectKeys[:len(resp.CandidateList)]
	ime.keyComposing = true
}

func cloudClipboardPageCount(total int) int {
	if total <= 0 {
		return 0
	}
	return (total + cloudClipboardPageSize - 1) / cloudClipboardPageSize
}

func (ime *IME) cloudClipboardPageBounds() (int, int) {
	total := len(ime.cloudClipboardEntries)
	start := ime.cloudClipboardPage * cloudClipboardPageSize
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := start + cloudClipboardPageSize
	if end > total {
		end = total
	}
	return start, end
}

func (ime *IME) cloudClipboardVisibleIndex(index int) (int, bool) {
	start, end := ime.cloudClipboardPageBounds()
	absolute := start + index
	if absolute < start || absolute >= end || absolute >= len(ime.cloudClipboardEntries) {
		return 0, false
	}
	return absolute, true
}

func (ime *IME) handleCloudClipboardKeyDownFilter(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil {
		return false
	}
	if !ime.cloudClipboardActive {
		if ime.handleCloudClipboardListHotkey(req, resp) {
			return true
		}
		return false
	}
	if loadCloudClipboardListHotkey().matches(req) {
		ime.fillCloudClipboardResponse(resp)
		resp.ReturnValue = 1
		return true
	}
	if ime.isCloudClipboardHandledKey(req) {
		resp.ReturnValue = 1
		return true
	}
	ime.resetCloudClipboardState()
	return false
}

func (ime *IME) handleCloudClipboardKeyUpFilter(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil {
		return false
	}
	if !ime.cloudClipboardActive {
		return false
	}
	action := loadCloudClipboardListHotkey()
	if req.KeyCode == action.KeyCode || isModifierKeyCode(req.KeyCode) {
		ime.fillCloudClipboardResponse(resp)
		resp.ReturnValue = 1
		return true
	}
	return false
}

func (ime *IME) handleCloudClipboardKeyDown(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || !ime.cloudClipboardActive {
		return false
	}
	if loadCloudClipboardListHotkey().matches(req) {
		ime.fillCloudClipboardResponse(resp)
		resp.ReturnValue = 1
		return true
	}
	if isCloudClipboardPreviousPageKey(req) {
		ime.changeCloudClipboardPage(true, resp)
		resp.ReturnValue = 1
		return true
	}
	if isCloudClipboardNextPageKey(req) {
		ime.changeCloudClipboardPage(false, resp)
		resp.ReturnValue = 1
		return true
	}
	switch req.KeyCode {
	case vkEscape:
		ime.resetCloudClipboardState()
		ime.clearResponse(resp)
		ime.keyComposing = false
		resp.ReturnValue = 1
		return true
	case vkReturn, vkSpace:
		if ime.cloudClipboardCursor >= 0 && ime.cloudClipboardCursor < len(ime.cloudClipboardEntries) {
			ime.commitCloudClipboardEntry(resp, ime.cloudClipboardEntries[ime.cloudClipboardCursor])
			resp.ReturnValue = 1
			return true
		}
	case vkDelete:
		ime.deleteCurrentCloudClipboardEntry(resp)
		resp.ReturnValue = 1
		return true
	case vkUp:
		if ime.cloudClipboardCursor > 0 {
			ime.cloudClipboardCursor--
			ime.cloudClipboardPage = ime.cloudClipboardCursor / cloudClipboardPageSize
		}
		ime.fillCloudClipboardResponse(resp)
		resp.ReturnValue = 1
		return true
	case vkDown:
		if ime.cloudClipboardCursor+1 < len(ime.cloudClipboardEntries) {
			ime.cloudClipboardCursor++
			ime.cloudClipboardPage = ime.cloudClipboardCursor / cloudClipboardPageSize
		}
		ime.fillCloudClipboardResponse(resp)
		resp.ReturnValue = 1
		return true
	}
	if index, ok := ime.selectionKeyIndex(req); ok {
		if absolute, ok := ime.cloudClipboardVisibleIndex(index); ok {
			ime.commitCloudClipboardEntry(resp, ime.cloudClipboardEntries[absolute])
			resp.ReturnValue = 1
			return true
		}
	}
	return false
}

func (ime *IME) changeCloudClipboardPage(backward bool, resp *imecore.Response) {
	pageCount := cloudClipboardPageCount(len(ime.cloudClipboardEntries))
	if pageCount <= 0 {
		ime.fillCloudClipboardResponse(resp)
		return
	}
	if backward {
		if ime.cloudClipboardPage > 0 {
			ime.cloudClipboardPage--
		}
	} else if ime.cloudClipboardPage+1 < pageCount {
		ime.cloudClipboardPage++
	}
	start, _ := ime.cloudClipboardPageBounds()
	ime.cloudClipboardCursor = start
	ime.fillCloudClipboardResponse(resp)
}

func (ime *IME) deleteCurrentCloudClipboardEntry(resp *imecore.Response) {
	if resp == nil {
		return
	}
	if ime.cloudClipboardCursor < 0 || ime.cloudClipboardCursor >= len(ime.cloudClipboardEntries) {
		ime.fillCloudClipboardResponse(resp)
		return
	}
	entry := ime.cloudClipboardEntries[ime.cloudClipboardCursor]
	if err := globalCloudClipboardService().DeleteClip(entry.Name); err != nil {
		ime.fillCloudClipboardResponse(resp)
		resp.ShowMessage = &imecore.MessageWindow{Message: "删除失败: " + shortErrorMessage(err), Duration: 3000}
		return
	}
	ime.cloudClipboardEntries = append(ime.cloudClipboardEntries[:ime.cloudClipboardCursor], ime.cloudClipboardEntries[ime.cloudClipboardCursor+1:]...)
	if len(ime.cloudClipboardEntries) == 0 {
		ime.resetCloudClipboardState()
		ime.clearResponse(resp)
		ime.keyComposing = false
		resp.ShowMessage = &imecore.MessageWindow{Message: "云剪贴板已清空", Duration: 2500}
		return
	}
	if ime.cloudClipboardCursor >= len(ime.cloudClipboardEntries) {
		ime.cloudClipboardCursor = len(ime.cloudClipboardEntries) - 1
	}
	ime.cloudClipboardPage = ime.cloudClipboardCursor / cloudClipboardPageSize
	ime.fillCloudClipboardResponse(resp)
}

func (ime *IME) handleCloudClipboardKeyUp(req *imecore.Request, resp *imecore.Response) bool {
	if req == nil || !ime.cloudClipboardActive {
		return false
	}
	action := loadCloudClipboardListHotkey()
	if req.KeyCode == action.KeyCode || isModifierKeyCode(req.KeyCode) || ime.isCloudClipboardHandledKey(req) {
		ime.fillCloudClipboardResponse(resp)
		resp.ReturnValue = 1
		return true
	}
	return false
}

func isCloudClipboardPageKey(req *imecore.Request) bool {
	return isCloudClipboardPreviousPageKey(req) || isCloudClipboardNextPageKey(req)
}

func isCloudClipboardPreviousPageKey(req *imecore.Request) bool {
	if req == nil {
		return false
	}
	return req.KeyCode == vkOemMinus || req.CharCode == int('-')
}

func isCloudClipboardNextPageKey(req *imecore.Request) bool {
	if req == nil {
		return false
	}
	return req.KeyCode == vkOemPlus || req.CharCode == int('=')
}

func (ime *IME) isCloudClipboardHandledKey(req *imecore.Request) bool {
	if req == nil || !ime.cloudClipboardActive {
		return false
	}
	if req.KeyCode == vkEscape || req.KeyCode == vkReturn || req.KeyCode == vkSpace || req.KeyCode == vkDelete || req.KeyCode == vkUp || req.KeyCode == vkDown || isCloudClipboardPageKey(req) || isModifierKeyCode(req.KeyCode) {
		return true
	}
	if _, ok := ime.selectionKeyIndex(req); ok {
		return true
	}
	return false
}

func isModifierKeyCode(keyCode int) bool {
	switch keyCode {
	case vkControl, vkLControl, vkRControl, vkShift, vkLShift, vkRShift, vkMenu, vkLMenu, vkRMenu:
		return true
	default:
		return false
	}
}

func (ime *IME) applyCloudClipboardCandidateHighlight(req *imecore.Request, resp *imecore.Response) bool {
	if !ime.cloudClipboardActive || req == nil || !req.HasCandidateIndex {
		return false
	}
	index := req.CandidateIndex
	absolute, ok := ime.cloudClipboardVisibleIndex(index)
	if !ok {
		return false
	}
	ime.cloudClipboardCursor = absolute
	ime.fillCloudClipboardResponse(resp)
	return true
}

func (ime *IME) applyCloudClipboardCandidateSelection(req *imecore.Request, resp *imecore.Response) bool {
	if !ime.cloudClipboardActive || req == nil || !req.HasCandidateIndex {
		return false
	}
	index := req.CandidateIndex
	absolute, ok := ime.cloudClipboardVisibleIndex(index)
	if !ok {
		return false
	}
	ime.cloudClipboardCursor = absolute
	ime.commitCloudClipboardEntry(resp, ime.cloudClipboardEntries[absolute])
	return true
}

func (ime *IME) commitCloudClipboardEntry(resp *imecore.Response, entry cloudClipboardEntry) {
	text := strings.TrimSpace(entry.FullText)
	if text == "" && entry.Name != "" {
		if body, err := globalCloudClipboardService().DownloadClip(entry.Name); err == nil {
			text = strings.TrimSpace(body)
		}
	}
	ime.resetCloudClipboardState()
	if text == "" {
		ime.clearResponse(resp)
		ime.keyComposing = false
		return
	}
	globalCloudClipboardService().SetApplyingRemote(true)
	go func() {
		time.Sleep(cloudclipboard.RemoteClipClearDuration())
		globalCloudClipboardService().SetApplyingRemote(false)
	}()
	resp.CommitString = text
	resp.CompositionString = ""
	resp.ShowCandidates = false
	resp.CandidateList = nil
	resp.CandidateEntries = nil
	ime.keyComposing = false
}
