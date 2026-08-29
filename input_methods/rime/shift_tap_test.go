package rime

import (
	"testing"
	"time"

	"github.com/gaboolic/moqi-ime/imecore"
)

type shiftTapBackend struct {
	fakeBackend
	options map[string]bool
}

func newShiftTapBackend() *shiftTapBackend {
	return &shiftTapBackend{options: map[string]bool{"ascii_mode": false}}
}

func (b *shiftTapBackend) SetOption(name string, value bool) { b.options[name] = value }
func (b *shiftTapBackend) GetOption(name string) bool        { return b.options[name] }

// withPhysicalShiftDown 控制测试中 Shift 的物理状态。
func withPhysicalShiftDown(t *testing.T, down bool) {
	t.Helper()
	orig := isShiftPhysicallyDown
	isShiftPhysicallyDown = func() bool { return down }
	t.Cleanup(func() { isShiftPhysicallyDown = orig })
}

func shiftTapState(t *testing.T, ime *IME, backend *shiftTapBackend) (toggled, active bool) {
	t.Helper()
	ime.mu.Lock()
	defer ime.mu.Unlock()
	return backend.options["ascii_mode"], ime.shiftTapActive
}

func TestShiftTapFallbackTogglesAsciiMode(t *testing.T) {
	backend := newShiftTapBackend()
	ime := newTestIMEWithBackend(backend)
	withPhysicalShiftDown(t, false)

	// Shift keydown（无组合键）启动单击检测。
	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   vkShift,
		KeyStates: keyStatesWithDown(vkShift),
	})

	// 不发送 keyup，也不按其他键：模拟微信、部分浏览器吞掉 Shift keyup
	// 且用户单击后停顿的场景，兜底检测应在 shiftTapDelay 后翻转 ascii_mode。
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		toggled, active := shiftTapState(t, ime, backend)
		if !active && toggled {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("expected shift tap fallback to toggle ascii_mode")
}

func TestShiftTapTogglesImmediatelyOnNextKey(t *testing.T) {
	backend := newShiftTapBackend()
	ime := newTestIMEWithBackend(backend)
	withPhysicalShiftDown(t, false)

	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   vkShift,
		KeyStates: keyStatesWithDown(vkShift),
	})

	// Shift keyup 被吞掉后用户立即打字：下一个 keydown 到达时
	// Shift 物理上已抬起，应在处理该键之前立即完成切换。
	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    2,
		KeyCode:   'N',
		CharCode:  'n',
		KeyStates: keyStatesWithDown(),
	})

	toggled, active := shiftTapState(t, ime, backend)
	if active {
		t.Fatal("detection should be finished after next keydown")
	}
	if !toggled {
		t.Fatal("expected ascii_mode to toggle before the next key is processed")
	}
}

func TestShiftTapCancelledByRealKeyUp(t *testing.T) {
	backend := newShiftTapBackend()
	ime := newTestIMEWithBackend(backend)
	withPhysicalShiftDown(t, false)

	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   vkShift,
		KeyStates: keyStatesWithDown(vkShift),
	})
	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyUp",
		SeqNum:    2,
		KeyCode:   vkShift,
		KeyStates: keyStatesWithDown(),
	})

	time.Sleep(shiftTapDelay + 200*time.Millisecond)
	toggled, active := shiftTapState(t, ime, backend)
	if active {
		t.Fatal("detection should be cancelled by real keyup")
	}
	if toggled {
		t.Fatal("fallback must not fire after a real keyup arrived")
	}
}

func TestShiftTapCancelledWhileShiftStillDown(t *testing.T) {
	backend := newShiftTapBackend()
	ime := newTestIMEWithBackend(backend)
	// Shift 物理上仍按着：下一个 keydown 属于 Shift 组合（如输入大写字母）。
	withPhysicalShiftDown(t, true)

	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   vkShift,
		KeyStates: keyStatesWithDown(vkShift),
	})
	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    2,
		KeyCode:   'A',
		CharCode:  'A',
		KeyStates: keyStatesWithDown(vkShift),
	})

	time.Sleep(shiftTapDelay + 200*time.Millisecond)
	toggled, active := shiftTapState(t, ime, backend)
	if active {
		t.Fatal("detection should be cancelled while shift is held down")
	}
	if toggled {
		t.Fatal("fallback must not fire when shift is still held down")
	}
}

func TestShiftTapNotStartedWhenModifiersDown(t *testing.T) {
	backend := newShiftTapBackend()
	ime := newTestIMEWithBackend(backend)
	withPhysicalShiftDown(t, false)

	// Ctrl+Shift 组合（如 Ctrl+Shift+E 快捷键）不应触发单击检测。
	ime.HandleRequest(&imecore.Request{
		Method:    "filterKeyDown",
		SeqNum:    1,
		KeyCode:   vkShift,
		KeyStates: keyStatesWithDown(vkControl, vkShift),
	})

	time.Sleep(shiftTapDelay + 200*time.Millisecond)
	toggled, active := shiftTapState(t, ime, backend)
	if active {
		t.Fatal("detection must not start for modifier combos")
	}
	if toggled {
		t.Fatal("fallback must not fire for modifier combos")
	}
}

func TestTranslateModifiersShiftReleaseIncludesSelfMask(t *testing.T) {
	// Shift keydown：不包含自身修饰位（X11 keypress 语义）。
	down := &imecore.Request{KeyCode: vkShift, KeyStates: keyStatesWithDown(vkShift)}
	if got := translateModifiers(down, false); got != 0 {
		t.Fatalf("shift keydown modifiers = %#x, want 0", got)
	}
	// Shift keyup：包含自身修饰位（X11 keyrelease 语义）。
	up := &imecore.Request{KeyCode: vkShift, KeyStates: keyStatesWithDown()}
	if got := translateModifiers(up, true); got != shiftMask|releaseMask {
		t.Fatalf("shift keyup modifiers = %#x, want shiftMask|releaseMask", got)
	}
	// 普通字符键不受影响。
	letter := &imecore.Request{KeyCode: 'A', CharCode: 'A', KeyStates: keyStatesWithDown(vkShift)}
	if got := translateModifiers(letter, false); got != shiftMask {
		t.Fatalf("letter keydown with shift modifiers = %#x, want shiftMask", got)
	}
}
