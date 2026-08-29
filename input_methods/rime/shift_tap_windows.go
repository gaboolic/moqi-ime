//go:build windows

package rime

import "syscall"

var (
	user32Mod            = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = user32Mod.NewProc("GetAsyncKeyState")
)

// isShiftPhysicallyDown 查询物理键盘上 Shift 的实时状态，
// 不依赖宿主应用是否把 keyup 事件转发给输入法。
// 声明为变量以便测试中替换。
var isShiftPhysicallyDown = func() bool {
	for _, vk := range []uintptr{vkShift, vkLShift, vkRShift} {
		ret, _, _ := procGetAsyncKeyState.Call(vk)
		if ret&0x8000 != 0 {
			return true
		}
	}
	return false
}
