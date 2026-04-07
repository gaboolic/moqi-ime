// 简单拼音输入法
package simplepinyin

import (
	"log"
	"unicode/utf8"

	"github.com/gaboolic/moqi-ime/imecore"
)

// IME 拼音输入法结构
type IME struct {
	*imecore.TextServiceBase
	pinyin     string
	candidates []string
	dict       map[string][]string
	asciiMode  bool
}

const (
	modeIconCommandID = 3100
	langIconCommandID = 3101
)

func normalizeLetterCharCode(keyCode, charCode int) int {
	if charCode != 0 {
		return charCode
	}
	if keyCode >= 0x41 && keyCode <= 0x5A {
		return keyCode + 32
	}
	return charCode
}

// New 创建拼音输入法实例
func New(client *imecore.Client) imecore.TextService {
	return &IME{
		TextServiceBase: imecore.NewTextServiceBase(client),
		pinyin:          "",
		candidates:      make([]string, 0),
		dict:            initDict(),
	}
}

// HandleRequest 处理请求
func (ime *IME) HandleRequest(req *imecore.Request) *imecore.Response {
	resp := imecore.NewResponse(req.SeqNum, true)

	switch req.Method {
	case "onActivate":
		log.Println("拼音输入法已激活")
		imecore.AddLangButtons(resp, ime.Client, ime.asciiMode, modeIconCommandID, langIconCommandID)
		resp.ReturnValue = 1

	case "onDeactivate":
		log.Println("拼音输入法已失活")
		imecore.RemoveLangButtons(resp, ime.Client)
		resp.ReturnValue = 1

	case "filterKeyDown":
		return ime.handleKeyDown(req, resp)

	case "filterKeyUp":
		resp.ReturnValue = 0

	case "onCompositionTerminated":
		// 清理状态
		ime.pinyin = ""
		ime.candidates = nil

	case "onCommand":
		return ime.onCommand(req, resp)
	}

	return resp
}

// handleKeyDown 处理按键按下
func (ime *IME) handleKeyDown(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	keyCode := req.KeyCode
	charCode := normalizeLetterCharCode(keyCode, req.CharCode)

	if ime.asciiMode && ime.pinyin == "" && charCode >= 0x20 {
		resp.ReturnValue = 0
		return resp
	}

	// 处理回车键 - 提交当前拼音
	if keyCode == 0x0D { // VK_RETURN
		if ime.pinyin != "" {
			// 如果有候选词，提交第一个
			if len(ime.candidates) > 0 {
				resp.CommitString = ime.candidates[0]
			} else {
				// 否则提交拼音本身
				resp.CommitString = ime.pinyin
			}
			ime.pinyin = ""
			ime.candidates = nil
			resp.ReturnValue = 1
			return resp
		}
		resp.ReturnValue = 0
		return resp
	}

	// 处理退格键
	if keyCode == 0x08 { // VK_BACK
		if ime.pinyin != "" {
			// 删除最后一个字符
			_, size := utf8.DecodeLastRuneInString(ime.pinyin)
			if len(ime.pinyin) > size {
				ime.pinyin = ime.pinyin[:len(ime.pinyin)-size]
			} else {
				ime.pinyin = ""
			}
			// 更新候选词
			ime.lookupCandidates()
			// 更新 UI
			resp.CompositionString = ime.pinyin
			resp.CursorPos = len(ime.pinyin)
			resp.CandidateList = ime.candidates
			resp.ShowCandidates = len(ime.candidates) > 0
			resp.ReturnValue = 1
			return resp
		}
		resp.ReturnValue = 0
		return resp
	}

	// 处理 Escape 键 - 取消输入
	if keyCode == 0x1B { // VK_ESCAPE
		if ime.pinyin != "" {
			ime.pinyin = ""
			ime.candidates = nil
			resp.CompositionString = ""
			resp.ShowCandidates = false
			resp.ReturnValue = 1
			return resp
		}
		resp.ReturnValue = 0
		return resp
	}

	// 处理数字键 1-9 - 选择候选词
	if keyCode >= 0x31 && keyCode <= 0x39 { // '1' - '9'
		if len(ime.candidates) > 0 {
			index := int(keyCode - 0x31) // 0-8
			if index < len(ime.candidates) {
				// 提交选中的候选词
				resp.CommitString = ime.candidates[index]
				// 清空状态
				ime.pinyin = ""
				ime.candidates = nil
				resp.CompositionString = ""
				resp.ShowCandidates = false
				resp.ReturnValue = 1
				return resp
			}
		}
	}

	// 处理字符输入（a-z 字母）
	if charCode >= 97 && charCode <= 122 { // 'a' - 'z'
		char := string(rune(charCode))
		// 添加字符到拼音
		ime.pinyin += char
		// 查找候选词
		ime.lookupCandidates()
		// 设置响应
		resp.CompositionString = ime.pinyin
		resp.CursorPos = len(ime.pinyin)
		resp.CandidateList = ime.candidates
		resp.ShowCandidates = len(ime.candidates) > 0
		resp.ReturnValue = 1
		return resp
	}

	// 不处理的按键
	resp.ReturnValue = 0
	return resp
}

func (ime *IME) onCommand(req *imecore.Request, resp *imecore.Response) *imecore.Response {
	commandID, ok := req.Data["commandId"].(float64)
	if !ok {
		resp.ReturnValue = 0
		return resp
	}

	switch int(commandID) {
	case modeIconCommandID, langIconCommandID:
		ime.asciiMode = !ime.asciiMode
		imecore.ChangeLangButtons(resp, ime.Client, ime.asciiMode)
		resp.ReturnValue = 1
	default:
		resp.ReturnValue = 0
	}
	return resp
}

// lookupCandidates 查找候选词
func (ime *IME) lookupCandidates() {
	if ime.pinyin == "" {
		ime.candidates = nil
		return
	}

	// 从词典查找
	if words, ok := ime.dict[ime.pinyin]; ok {
		ime.candidates = words
	} else {
		// 如果没有精确匹配，返回一些默认候选
		ime.candidates = []string{"测试", "输入法", "示例"}
	}

	// 限制候选词数量
	if len(ime.candidates) > 9 {
		ime.candidates = ime.candidates[:9]
	}
}

// Init 初始化
func (ime *IME) Init(req *imecore.Request) bool {
	return true
}

// Close 关闭
func (ime *IME) Close() {
	// 清理资源
}

func initDict() map[string][]string {
	return map[string][]string{
		"nihao":    {"你好", "您好", "你号"},
		"shijie":   {"世界", "视界", "时节"},
		"zhongwen": {"中文", "种文", "重文"},
		"shuru":    {"输入", "熟入", "书入"},
		"fa":       {"发", "法", "罚"},
		"pinyin":   {"拼音", "品音", "聘音"},
		"ceshi":    {"测试", "侧视", "策师"},
		"zhihuan":  {"智能", "直换", "纸换"},
		"xiexie":   {"谢谢", "蟹蟹", "歇歇"},
		"zaijian":  {"再见", "在见", "再荐"},
	}
}
