package rime

import (
	"log"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

// shiftTapDelay 是 Shift keydown 之后等待物理抬起的兜底时长。
// 必须小于 librime ascii_composer 的 toggle_expired_（keydown + 500ms），
// 同时给宿主应用转发的真实 keyup 留出先到达的机会。
const shiftTapDelay = 350 * time.Millisecond

// noteKeyDownForShiftTap 在每次 keydown 转发给 RIME 之前调用（ime.mu 持有中）：
// Shift keydown 启动单击检测；其余按键时若 Shift 已物理抬起，
// 说明上一次 Shift 单击的 keyup 被宿主吞掉且单击已完成，立即兜底切换；
// 若 Shift 仍按着（Shift+字母等组合），取消检测。
func (ime *IME) noteKeyDownForShiftTap(req *imecore.Request, resp *imecore.Response) {
	if req == nil {
		return
	}
	if req.KeyCode == vkShift {
		if ime.canStartShiftTap(req) {
			ime.beginShiftTapDetection()
		} else {
			ime.cancelShiftTapDetection("keydown")
		}
		return
	}
	if !ime.shiftTapActive {
		return
	}
	if isShiftPhysicallyDown() {
		ime.cancelShiftTapDetection("combo")
		return
	}
	ime.finishShiftTapDetection(resp)
}

func (ime *IME) canStartShiftTap(req *imecore.Request) bool {
	if ime.backend == nil || ime.isComposing() {
		return false
	}
	// Ctrl/Alt/Win + Shift 属于组合键而非单击切换。
	if req.KeyStates.IsKeyDown(vkControl) || req.KeyStates.IsKeyDown(vkMenu) ||
		req.KeyStates.IsKeyDown(vkLWin) || req.KeyStates.IsKeyDown(vkRWin) {
		return false
	}
	return true
}

func (ime *IME) beginShiftTapDetection() {
	if ime.shiftTapActive {
		return
	}
	ime.shiftTapActive = true
	ime.shiftTapCancel = make(chan struct{})
	cancel := ime.shiftTapCancel
	go func() {
		select {
		case <-cancel:
			return
		case <-time.After(shiftTapDelay):
		}
		// 检测期间没有其他按键：构造异步响应刷新语言栏图标。
		ime.mu.Lock()
		var updateResp *imecore.Response
		sender := ime.asyncResponseSender
		if ime.finishShiftTapDetection(nil) && sender != nil {
			updateResp = imecore.NewResponse(0, true)
			ime.updateLangStatus(nil, updateResp)
		}
		ime.mu.Unlock()
		if updateResp != nil && sender != nil {
			sender(updateResp)
		}
	}()
}

// finishShiftTapDetection 翻转 ascii_mode 并返回是否发生了切换。
// 微信、部分 UWP/浏览器等宿主不调用 TSF 的 TestKeyUp/KeyUp，
// librime 的 ascii_composer 等不到 Shift 释放事件便无法判定"单击"，
// 这里用物理键盘状态判定单击完成，直接翻转 ascii_mode 作为兜底。
// 调用方必须持有 ime.mu。
func (ime *IME) finishShiftTapDetection(resp *imecore.Response) bool {
	ime.shiftTapActive = false
	if ime.shiftTapCancel != nil {
		close(ime.shiftTapCancel)
		ime.shiftTapCancel = nil
	}
	if ime.backend == nil || ime.isComposing() || isShiftPhysicallyDown() {
		return false
	}
	beforeASCII := ime.backend.GetOption("ascii_mode")
	ime.toggleOption("ascii_mode")
	afterASCII := ime.backend.GetOption("ascii_mode")
	log.Printf("Shift 单击兜底切换: ascii_mode %v -> %v", beforeASCII, afterASCII)
	if afterASCII != beforeASCII && resp != nil {
		ime.updateLangStatus(nil, resp)
	}
	return afterASCII != beforeASCII
}

func (ime *IME) cancelShiftTapDetection(reason string) {
	if !ime.shiftTapActive {
		return
	}
	ime.shiftTapActive = false
	if ime.shiftTapCancel != nil {
		close(ime.shiftTapCancel)
		ime.shiftTapCancel = nil
	}
	debugLogf("Shift 单击检测取消: %s", reason)
}
