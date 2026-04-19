// 通信协议定义
package imecore

import (
	"fmt"

	moqipb "github.com/gaboolic/moqi-ime/proto"
)

type FlexibleID struct {
	String string
	Int    int
	IsInt  bool
}

type KeyStates []int

func (k KeyStates) IsKeyDown(code int) bool {
	return code >= 0 && code < len(k) && (k[code]&(1<<7)) != 0
}

func (k KeyStates) IsKeyToggled(code int) bool {
	return code >= 0 && code < len(k) && (k[code]&1) != 0
}

func (id FlexibleID) StringValue() string {
	if id.IsInt {
		return ""
	}
	return id.String
}

func (id FlexibleID) IntValue() int {
	if id.IsInt {
		return id.Int
	}
	return 0
}

// Request Moqi请求结构
type Request struct {
	Method            string
	SeqNum            int
	ID                FlexibleID
	IsWindows8Above   bool
	IsMetroApp        bool
	IsUiLess          bool
	IsConsole         bool
	IsKeyboardOpen    bool
	Opened            bool
	Forced            bool
	CommandType       int
	CharCode          int
	KeyCode           int
	RepeatCount       int
	ScanCode          int
	IsExtended        bool
	KeyStates         KeyStates
	CompositionString string
	CandidateList     []string
	ShowCandidates    bool
	CursorPos         int
	SelStart          int
	SelEnd            int
	Data              map[string]interface{}
}

// ButtonInfo 按钮信息
type ButtonInfo struct {
	ID        string
	Icon      string
	Text      string
	Tooltip   string
	CommandID int
	Type      string
	Enable    bool
	Toggled   bool
	Style     uint32
}

type MenuItem struct {
	ID        int
	Text      string
	Checked   bool
	Enabled   bool
	Submenu   []MenuItem
	Separator bool
}

type MessageWindow struct {
	Message  string
	Duration int
}

type TrayNotificationIcon string

const (
	TrayNotificationIconInfo    TrayNotificationIcon = "info"
	TrayNotificationIconWarning TrayNotificationIcon = "warning"
	TrayNotificationIconError   TrayNotificationIcon = "error"
)

type TrayNotification struct {
	Title   string
	Message string
	Icon    TrayNotificationIcon
}

type PreservedKeyInfo struct {
	KeyCode   uint32
	Modifiers uint32
	GUID      string
}

type CandidateEntry struct {
	Text    string
	Comment string
}

type AutoPairRule struct {
	Open  string
	Close string
}

// Response Moqi响应结构
type Response struct {
	SeqNum             int
	Success            bool
	ReturnValue        int
	ReturnData         interface{}
	CompositionString  string
	CommitString       string
	CandidateList      []string
	CandidateEntries   []CandidateEntry
	ShowCandidates     bool
	CursorPos          int
	CompositionCursor  int
	CandidateCursor    int
	SelStart           int
	SelEnd             int
	SetSelKeys         string
	Message            string
	Error              string
	CustomizeUI        map[string]interface{}
	AddButton          []ButtonInfo
	RemoveButton       []string
	ChangeButton       []ButtonInfo
	ShowMessage        *MessageWindow
	TrayNotification   *TrayNotification
	HideMessage        bool
	OpenKeyboard       bool
	AddPreservedKey    []PreservedKeyInfo
	RemovePreservedKey []string
}

func NewResponse(seqNum int, success bool) *Response {
	return &Response{
		SeqNum:            seqNum,
		Success:           success,
		CandidateList:     []string{},
		CandidateEntries:  []CandidateEntry{},
		CompositionString: "",
	}
}

func ParseProtoRequest(msg *moqipb.ClientRequest) *Request {
	req := &Request{
		Method:            methodFromProto(msg.GetMethod()),
		SeqNum:            int(msg.GetSeqNum()),
		IsWindows8Above:   msg.GetIsWindows8Above(),
		IsMetroApp:        msg.GetIsMetroApp(),
		IsUiLess:          msg.GetIsUiLess(),
		IsConsole:         msg.GetIsConsole(),
		IsKeyboardOpen:    msg.GetIsKeyboardOpen(),
		Opened:            msg.GetOpened(),
		Forced:            msg.GetForced(),
		CommandType:       int(msg.GetCommandType()),
		CompositionString: msg.GetCompositionString(),
		CandidateList:     append([]string{}, msg.GetCandidateList()...),
		ShowCandidates:    msg.GetShowCandidates(),
		CursorPos:         int(msg.GetCursorPos()),
		SelStart:          int(msg.GetSelStart()),
		SelEnd:            int(msg.GetSelEnd()),
		Data:              map[string]interface{}{},
	}

	if keyEvent := msg.GetKeyEvent(); keyEvent != nil {
		req.CharCode = int(keyEvent.GetCharCode())
		req.KeyCode = int(keyEvent.GetKeyCode())
		req.RepeatCount = int(keyEvent.GetRepeatCount())
		req.ScanCode = int(keyEvent.GetScanCode())
		req.IsExtended = keyEvent.GetIsExtended()
		req.KeyStates = make(KeyStates, len(keyEvent.GetKeyStates()))
		for i, state := range keyEvent.GetKeyStates() {
			req.KeyStates[i] = int(state)
		}
	}

	if guid := msg.GetGuid(); guid != "" {
		req.ID = FlexibleID{String: guid}
		req.Data["guid"] = guid
	}
	if buttonID := msg.GetButtonId(); buttonID != "" {
		req.ID = FlexibleID{String: buttonID}
		req.Data["buttonId"] = buttonID
		req.Data["id"] = buttonID
	}
	if commandID := msg.GetCommandId(); commandID != 0 {
		req.ID = FlexibleID{Int: int(commandID), IsInt: true}
		req.Data["commandId"] = float64(commandID)
	}
	if guid := msg.GetPreservedKeyGuid(); guid != "" {
		req.Data["guid"] = guid
	}
	if guid := msg.GetCompartmentGuid(); guid != "" {
		req.Data["guid"] = guid
	}
	if len(req.Data) == 0 {
		req.Data = nil
	}

	return req
}

func BuildProtoResponse(clientID string, resp *Response) (*moqipb.ServerResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("response is nil")
	}

	msg := &moqipb.ServerResponse{
		ClientId:          stringPtrOrNil(clientID),
		SeqNum:            uint32(resp.SeqNum),
		Success:           resp.Success,
		ReturnValue:       int32(resp.ReturnValue),
		CompositionString: resp.CompositionString,
		CommitString:      resp.CommitString,
		CandidateList:     append([]string{}, resp.CandidateList...),
		ShowCandidates:    resp.ShowCandidates,
		CursorPos:         int32(resp.CursorPos),
		CompositionCursor: int32(resp.CompositionCursor),
		CandidateCursor:   int32(resp.CandidateCursor),
		SelStart:          int32(resp.SelStart),
		SelEnd:            int32(resp.SelEnd),
		SetSelKeys:        resp.SetSelKeys,
		HideMessage:       resp.HideMessage,
		OpenKeyboard:      resp.OpenKeyboard,
		RemoveButton:      append([]string{}, resp.RemoveButton...),
		RemovePreservedKey: append([]string{},
			resp.RemovePreservedKey...),
		Error: resp.Error,
	}

	if ui := customizeUiToProto(resp.CustomizeUI); ui != nil {
		msg.CustomizeUi = ui
	}
	for _, candidate := range resp.CandidateEntries {
		msg.CandidateEntries = append(msg.CandidateEntries, &moqipb.CandidateEntry{
			Text:    candidate.Text,
			Comment: candidate.Comment,
		})
	}
	if resp.ShowMessage != nil {
		msg.ShowMessage = &moqipb.MessageWindow{
			Message:  resp.ShowMessage.Message,
			Duration: int32(resp.ShowMessage.Duration),
		}
	}
	if resp.TrayNotification != nil {
		msg.TrayNotification = &moqipb.TrayNotification{
			Title:   resp.TrayNotification.Title,
			Message: resp.TrayNotification.Message,
			Icon:    trayNotificationIconToProto(resp.TrayNotification.Icon),
		}
	}
	for _, btn := range resp.AddButton {
		msg.AddButton = append(msg.AddButton, buttonInfoToProto(btn))
	}
	for _, btn := range resp.ChangeButton {
		msg.ChangeButton = append(msg.ChangeButton, buttonInfoToProto(btn))
	}
	for _, key := range resp.AddPreservedKey {
		msg.AddPreservedKey = append(msg.AddPreservedKey, &moqipb.PreservedKey{
			KeyCode:   key.KeyCode,
			Modifiers: key.Modifiers,
			Guid:      key.GUID,
		})
	}

	menuItems, err := menuItemsFromAny(resp.ReturnData)
	if err != nil {
		return nil, err
	}
	msg.MenuItems = menuItems
	return msg, nil
}

func methodFromProto(method moqipb.Method) string {
	switch method {
	case moqipb.Method_METHOD_INIT:
		return "init"
	case moqipb.Method_METHOD_CLOSE:
		return "close"
	case moqipb.Method_METHOD_ON_ACTIVATE:
		return "onActivate"
	case moqipb.Method_METHOD_ON_DEACTIVATE:
		return "onDeactivate"
	case moqipb.Method_METHOD_FILTER_KEY_DOWN:
		return "filterKeyDown"
	case moqipb.Method_METHOD_ON_KEY_DOWN:
		return "onKeyDown"
	case moqipb.Method_METHOD_FILTER_KEY_UP:
		return "filterKeyUp"
	case moqipb.Method_METHOD_ON_KEY_UP:
		return "onKeyUp"
	case moqipb.Method_METHOD_ON_PRESERVED_KEY:
		return "onPreservedKey"
	case moqipb.Method_METHOD_ON_COMMAND:
		return "onCommand"
	case moqipb.Method_METHOD_ON_MENU:
		return "onMenu"
	case moqipb.Method_METHOD_ON_COMPARTMENT_CHANGED:
		return "onCompartmentChanged"
	case moqipb.Method_METHOD_ON_KEYBOARD_STATUS_CHANGED:
		return "onKeyboardStatusChanged"
	case moqipb.Method_METHOD_ON_COMPOSITION_TERMINATED:
		return "onCompositionTerminated"
	case moqipb.Method_METHOD_ON_LANG_PROFILE_ACTIVATED:
		return "onLangProfileActivated"
	default:
		return ""
	}
}

func MethodToProto(method string) moqipb.Method {
	switch method {
	case "init":
		return moqipb.Method_METHOD_INIT
	case "close":
		return moqipb.Method_METHOD_CLOSE
	case "onActivate":
		return moqipb.Method_METHOD_ON_ACTIVATE
	case "onDeactivate":
		return moqipb.Method_METHOD_ON_DEACTIVATE
	case "filterKeyDown":
		return moqipb.Method_METHOD_FILTER_KEY_DOWN
	case "onKeyDown":
		return moqipb.Method_METHOD_ON_KEY_DOWN
	case "filterKeyUp":
		return moqipb.Method_METHOD_FILTER_KEY_UP
	case "onKeyUp":
		return moqipb.Method_METHOD_ON_KEY_UP
	case "onPreservedKey":
		return moqipb.Method_METHOD_ON_PRESERVED_KEY
	case "onCommand":
		return moqipb.Method_METHOD_ON_COMMAND
	case "onMenu":
		return moqipb.Method_METHOD_ON_MENU
	case "onCompartmentChanged":
		return moqipb.Method_METHOD_ON_COMPARTMENT_CHANGED
	case "onKeyboardStatusChanged":
		return moqipb.Method_METHOD_ON_KEYBOARD_STATUS_CHANGED
	case "onCompositionTerminated":
		return moqipb.Method_METHOD_ON_COMPOSITION_TERMINATED
	case "onLangProfileActivated":
		return moqipb.Method_METHOD_ON_LANG_PROFILE_ACTIVATED
	default:
		return moqipb.Method_METHOD_UNSPECIFIED
	}
}

func buttonInfoToProto(btn ButtonInfo) *moqipb.ButtonInfo {
	msg := &moqipb.ButtonInfo{
		Id:        btn.ID,
		Icon:      btn.Icon,
		Text:      btn.Text,
		Tooltip:   btn.Tooltip,
		CommandId: uint32(btn.CommandID),
		Type:      buttonTypeToProto(btn.Type),
		// Legacy JSON omitted "enable" for normal buttons, which the Windows side
		// treated as enabled. Preserve that behavior for protobuf payloads.
		Enable:  true,
		Toggled: btn.Toggled,
	}
	if btn.Style != 0 {
		msg.Style = uint32Ptr(btn.Style)
	}
	return msg
}

func buttonTypeToProto(value string) moqipb.ButtonType {
	switch value {
	case "button":
		return moqipb.ButtonType_BUTTON_TYPE_BUTTON
	case "toggle":
		return moqipb.ButtonType_BUTTON_TYPE_TOGGLE
	case "menu":
		return moqipb.ButtonType_BUTTON_TYPE_MENU
	default:
		return moqipb.ButtonType_BUTTON_TYPE_UNSPECIFIED
	}
}

func customizeUiToProto(data map[string]interface{}) *moqipb.CustomizeUi {
	if len(data) == 0 {
		return nil
	}
	ui := &moqipb.CustomizeUi{}
	if value, ok := data["candFontName"].(string); ok {
		ui.CandFontName = stringPtrOrNil(value)
	}
	if value, ok := data["candCommentFontName"].(string); ok {
		ui.CandCommentFontName = stringPtrOrNil(value)
	}
	if value, ok := numericToUint32(data["candFontSize"]); ok {
		ui.CandFontSize = uint32Ptr(value)
	}
	if value, ok := numericToUint32(data["candPerRow"]); ok {
		ui.CandPerRow = uint32Ptr(value)
	}
	if value, ok := data["candUseCursor"].(bool); ok {
		ui.CandUseCursor = boolPtr(value)
	}
	if value, ok := data["inlinePreedit"].(bool); ok {
		ui.InlinePreedit = boolPtr(value)
	}
	if value, ok := data["candBackgroundColor"].(string); ok {
		ui.CandBackgroundColor = stringPtrOrNil(value)
	}
	if value, ok := data["candHighlightColor"].(string); ok {
		ui.CandHighlightColor = stringPtrOrNil(value)
	}
	if value, ok := data["candTextColor"].(string); ok {
		ui.CandTextColor = stringPtrOrNil(value)
	}
	if value, ok := data["candHighlightTextColor"].(string); ok {
		ui.CandHighlightTextColor = stringPtrOrNil(value)
	}
	if value, ok := data["candCommentColor"].(string); ok {
		ui.CandCommentColor = stringPtrOrNil(value)
	}
	if value, ok := data["candCommentHighlightColor"].(string); ok {
		ui.CandCommentHighlightColor = stringPtrOrNil(value)
	}
	if value, ok := numericToUint32(data["candCommentFontSize"]); ok {
		ui.CandCommentFontSize = uint32Ptr(value)
	}
	if value, ok := data["autoPairQuotes"].(bool); ok {
		ui.AutoPairQuotes = boolPtr(value)
	}
	if value, ok := data["semicolonSelectSecond"].(bool); ok {
		ui.SemicolonSelectSecond = boolPtr(value)
	}
	if rules, ok := data["autoPairRules"].([]AutoPairRule); ok {
		ui.AutoPairRules = make([]*moqipb.AutoPairRule, 0, len(rules))
		for _, rule := range rules {
			if rule.Open == "" || rule.Close == "" {
				continue
			}
			ui.AutoPairRules = append(ui.AutoPairRules, &moqipb.AutoPairRule{
				Open:  rule.Open,
				Close: rule.Close,
			})
		}
	}
	return ui
}

func trayNotificationIconToProto(icon TrayNotificationIcon) moqipb.TrayNotificationIcon {
	switch icon {
	case TrayNotificationIconWarning:
		return moqipb.TrayNotificationIcon_TRAY_NOTIFICATION_ICON_WARNING
	case TrayNotificationIconError:
		return moqipb.TrayNotificationIcon_TRAY_NOTIFICATION_ICON_ERROR
	default:
		return moqipb.TrayNotificationIcon_TRAY_NOTIFICATION_ICON_INFO
	}
}

func menuItemsFromAny(value interface{}) ([]*moqipb.MenuItem, error) {
	if value == nil {
		return nil, nil
	}
	switch items := value.(type) {
	case []MenuItem:
		result := make([]*moqipb.MenuItem, 0, len(items))
		for _, item := range items {
			result = append(result, menuItemToProto(item))
		}
		return result, nil
	case []map[string]interface{}:
		result := make([]*moqipb.MenuItem, 0, len(items))
		for _, item := range items {
			menuItem, err := menuItemFromMap(item)
			if err != nil {
				return nil, err
			}
			result = append(result, menuItemToProto(menuItem))
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unsupported return data type %T", value)
	}
}

func menuItemFromMap(item map[string]interface{}) (MenuItem, error) {
	menuItem := MenuItem{Enabled: true}
	if value, ok := item["id"]; ok {
		id, ok := numericToInt(value)
		if !ok {
			return MenuItem{}, fmt.Errorf("invalid menu id type %T", value)
		}
		menuItem.ID = id
	}
	if value, ok := item["text"].(string); ok {
		menuItem.Text = value
		if value == "" {
			menuItem.Separator = true
		}
	}
	if value, ok := item["checked"].(bool); ok {
		menuItem.Checked = value
	}
	if value, ok := item["enabled"].(bool); ok {
		menuItem.Enabled = value
	}
	if value, ok := item["submenu"]; ok {
		submenu, err := submenuItemsFromAny(value)
		if err != nil {
			return MenuItem{}, err
		}
		menuItem.Submenu = submenu
	}
	return menuItem, nil
}

func submenuItemsFromAny(value interface{}) ([]MenuItem, error) {
	switch items := value.(type) {
	case []MenuItem:
		return items, nil
	case []map[string]interface{}:
		result := make([]MenuItem, 0, len(items))
		for _, item := range items {
			menuItem, err := menuItemFromMap(item)
			if err != nil {
				return nil, err
			}
			result = append(result, menuItem)
		}
		return result, nil
	case []interface{}:
		result := make([]MenuItem, 0, len(items))
		for _, raw := range items {
			item, ok := raw.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("invalid submenu item type %T", raw)
			}
			menuItem, err := menuItemFromMap(item)
			if err != nil {
				return nil, err
			}
			result = append(result, menuItem)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("invalid submenu type %T", value)
	}
}

func menuItemToProto(item MenuItem) *moqipb.MenuItem {
	msg := &moqipb.MenuItem{
		Id:        uint32(item.ID),
		Text:      item.Text,
		Checked:   item.Checked,
		Enabled:   item.Enabled,
		Separator: item.Separator,
	}
	for _, child := range item.Submenu {
		msg.Submenu = append(msg.Submenu, menuItemToProto(child))
	}
	return msg
}

func numericToInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func numericToUint32(value interface{}) (uint32, bool) {
	switch v := value.(type) {
	case int:
		return uint32(v), true
	case int32:
		return uint32(v), true
	case int64:
		return uint32(v), true
	case uint32:
		return v, true
	case uint64:
		return uint32(v), true
	case float64:
		return uint32(v), true
	default:
		return 0, false
	}
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func uint32Ptr(value uint32) *uint32 {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}
