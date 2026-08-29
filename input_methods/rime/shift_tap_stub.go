//go:build !windows

package rime

// isShiftPhysicallyDown 在非 Windows 平台没有兜底实现，
// 恒返回 false 使 Shift 单击检测不触发切换，行为与未引入兜底前一致。
// 声明为变量以便测试中替换。
var isShiftPhysicallyDown = func() bool {
	return false
}
