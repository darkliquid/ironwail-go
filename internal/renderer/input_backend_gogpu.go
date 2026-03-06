//go:build gogpu
// +build gogpu

package renderer

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/gogpu/gogpu"
	ginput "github.com/gogpu/gogpu/input"
	"github.com/gogpu/gpucontext"
	iinput "github.com/ironwail/ironwail-go/internal/input"
)

// gogpuInputBackend adapts gogpu input to the engine input.Backend.
type gogpuInputBackend struct {
	app             *gogpu.App
	sys             *iinput.System
	cursorMode      iinput.CursorMode
	callbacksInited bool

	mu              sync.Mutex
	hasMousePos     bool
	lastMouseX      float64
	lastMouseY      float64
	accumMouseDX    int32
	accumMouseDY    int32
	callbackInputOK bool
}

// InputBackendForSystem returns a Backend implementation wired to this renderer's app.
func (r *Renderer) InputBackendForSystem(sys *iinput.System) iinput.Backend {
	return &gogpuInputBackend{app: r.app, sys: sys}
}

func (b *gogpuInputBackend) Init() error {
	// Defer callback setup until first poll after run loop starts.
	return nil
}

func (b *gogpuInputBackend) Shutdown() {
	// Nothing to cleanup.
}

func (b *gogpuInputBackend) PollEvents() bool {
	if !b.callbacksInited {
		b.initCallbacks()
	}

	if b.app == nil || b.sys == nil || b.hasCallbackInput() {
		return true
	}

	state := b.app.Input()
	if state == nil || state.Keyboard() == nil {
		return true
	}

	keyboard := state.Keyboard()
	for _, pair := range pollingKeyMap {
		if keyboard.JustPressed(pair.src) {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: pair.dst, Down: true, Device: iinput.DeviceKeyboard})
		}
		if keyboard.JustReleased(pair.src) {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: pair.dst, Down: false, Device: iinput.DeviceKeyboard})
		}
	}

	return true
}

func (b *gogpuInputBackend) initCallbacks() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.callbacksInited || b.app == nil || b.sys == nil {
		return
	}

	es := b.app.EventSource()
	if es == nil {
		slog.Warn("gogpu input backend: event source unavailable")
		return
	}

	es.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		mapped := mapGPUContextKey(key)
		if mapped >= 0 {
			b.markCallbackInput()
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mapped, Down: true, Device: iinput.DeviceKeyboard})
		}
		_ = mods
	})

	es.OnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		mapped := mapGPUContextKey(key)
		if mapped >= 0 {
			b.markCallbackInput()
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mapped, Down: false, Device: iinput.DeviceKeyboard})
		}
		_ = mods
	})

	es.OnTextInput(func(text string) {
		for _, r := range text {
			b.sys.HandleCharEvent(r)
		}
	})

	es.OnMouseMove(func(x, y float64) {
		b.mu.Lock()
		if b.hasMousePos {
			b.accumMouseDX += int32(x - b.lastMouseX)
			b.accumMouseDY += int32(y - b.lastMouseY)
		}
		b.lastMouseX = x
		b.lastMouseY = y
		b.hasMousePos = true
		b.mu.Unlock()
	})

	es.OnMousePress(func(button gpucontext.MouseButton, x, y float64) {
		if key := mapGPUContextMouseButton(button); key >= 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: key, Down: true, Device: iinput.DeviceMouse})
		}
		_ = x
		_ = y
	})

	es.OnMouseRelease(func(button gpucontext.MouseButton, x, y float64) {
		if key := mapGPUContextMouseButton(button); key >= 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: key, Down: false, Device: iinput.DeviceMouse})
		}
		_ = x
		_ = y
	})

	es.OnScroll(func(dx, dy float64) {
		if dy > 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelUp, Down: true, Device: iinput.DeviceMouse})
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelUp, Down: false, Device: iinput.DeviceMouse})
		} else if dy < 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelDown, Down: true, Device: iinput.DeviceMouse})
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelDown, Down: false, Device: iinput.DeviceMouse})
		}
		_ = dx
	})

	es.OnFocus(func(focused bool) {
		if !focused {
			b.sys.ClearKeyStates()
		}
	})

	b.callbacksInited = true
	slog.Info("gogpu input backend: event source callbacks registered", "event_source", fmt.Sprintf("%T", es))
}

func (b *gogpuInputBackend) markCallbackInput() {
	b.mu.Lock()
	b.callbackInputOK = true
	b.mu.Unlock()
}

func (b *gogpuInputBackend) hasCallbackInput() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.callbackInputOK
}

func (b *gogpuInputBackend) GetMouseDelta() (dx, dy int32) {
	b.mu.Lock()
	dx, dy = b.accumMouseDX, b.accumMouseDY
	b.accumMouseDX = 0
	b.accumMouseDY = 0
	b.mu.Unlock()
	return dx, dy
}

func (b *gogpuInputBackend) GetModifierState() iinput.ModifierState {
	return iinput.ModifierState{}
}

func (b *gogpuInputBackend) SetTextMode(mode iinput.TextMode) {}

func (b *gogpuInputBackend) SetCursorMode(mode iinput.CursorMode) {
	b.cursorMode = mode
	if b.app == nil {
		return
	}
	switch mode {
	case iinput.CursorModeNormal:
		b.app.SetCursor(gpucontext.CursorDefault)
	case iinput.CursorModeHidden, iinput.CursorModeGrabbed:
		b.app.SetCursor(gpucontext.CursorNone)
	default:
		b.app.SetCursor(gpucontext.CursorDefault)
	}
}

func (b *gogpuInputBackend) ShowKeyboard(show bool) {}

func (b *gogpuInputBackend) GetGamepadState(player int) iinput.GamepadState {
	return iinput.GamepadState{}
}

func (b *gogpuInputBackend) IsGamepadConnected(player int) bool { return false }

func (b *gogpuInputBackend) SetMouseGrab(grabbed bool) {}

func (b *gogpuInputBackend) SetWindow(win interface{}) {}

type pollingKeyPair struct {
	src ginput.Key
	dst int
}

var pollingKeyMap = func() []pollingKeyPair {
	pairs := []pollingKeyPair{
		{src: ginput.KeyEscape, dst: iinput.KEscape},
		{src: ginput.KeyEnter, dst: iinput.KEnter},
		{src: ginput.KeyNumpadEnter, dst: iinput.KEnter},
		{src: ginput.KeyTab, dst: iinput.KTab},
		{src: ginput.KeySpace, dst: iinput.KSpace},
		{src: ginput.KeyBackspace, dst: iinput.KBackspace},
		{src: ginput.KeyUp, dst: iinput.KUpArrow},
		{src: ginput.KeyDown, dst: iinput.KDownArrow},
		{src: ginput.KeyLeft, dst: iinput.KLeftArrow},
		{src: ginput.KeyRight, dst: iinput.KRightArrow},
		{src: ginput.KeyShiftLeft, dst: iinput.KShift},
		{src: ginput.KeyShiftRight, dst: iinput.KShift},
		{src: ginput.KeyControlLeft, dst: iinput.KCtrl},
		{src: ginput.KeyControlRight, dst: iinput.KCtrl},
		{src: ginput.KeyAltLeft, dst: iinput.KAlt},
		{src: ginput.KeyAltRight, dst: iinput.KAlt},
	}

	letterKeys := []ginput.Key{
		ginput.KeyA, ginput.KeyB, ginput.KeyC, ginput.KeyD, ginput.KeyE, ginput.KeyF,
		ginput.KeyG, ginput.KeyH, ginput.KeyI, ginput.KeyJ, ginput.KeyK, ginput.KeyL,
		ginput.KeyM, ginput.KeyN, ginput.KeyO, ginput.KeyP, ginput.KeyQ, ginput.KeyR,
		ginput.KeyS, ginput.KeyT, ginput.KeyU, ginput.KeyV, ginput.KeyW, ginput.KeyX,
		ginput.KeyY, ginput.KeyZ,
	}
	for index, key := range letterKeys {
		pairs = append(pairs, pollingKeyPair{src: key, dst: int('a' + index)})
	}

	numberKeys := []ginput.Key{
		ginput.Key0, ginput.Key1, ginput.Key2, ginput.Key3, ginput.Key4,
		ginput.Key5, ginput.Key6, ginput.Key7, ginput.Key8, ginput.Key9,
	}
	for index, key := range numberKeys {
		pairs = append(pairs, pollingKeyPair{src: key, dst: int('0' + index)})
	}

	return pairs
}()

func mapGPUContextMouseButton(button gpucontext.MouseButton) int {
	switch button {
	case gpucontext.MouseButtonLeft:
		return iinput.KMouse1
	case gpucontext.MouseButtonRight:
		return iinput.KMouse2
	case gpucontext.MouseButtonMiddle:
		return iinput.KMouse3
	case gpucontext.MouseButton4:
		return iinput.KMouse4
	case gpucontext.MouseButton5:
		return iinput.KMouse5
	default:
		return -1
	}
}

func mapGPUContextKey(key gpucontext.Key) int {
	switch key {
	case gpucontext.KeyEscape:
		return iinput.KEscape
	case gpucontext.KeyEnter, gpucontext.KeyNumpadEnter:
		return iinput.KEnter
	case gpucontext.KeyTab:
		return iinput.KTab
	case gpucontext.KeySpace:
		return iinput.KSpace
	case gpucontext.KeyBackspace:
		return iinput.KBackspace
	case gpucontext.KeyUp:
		return iinput.KUpArrow
	case gpucontext.KeyDown:
		return iinput.KDownArrow
	case gpucontext.KeyLeft:
		return iinput.KLeftArrow
	case gpucontext.KeyRight:
		return iinput.KRightArrow
	case gpucontext.KeyLeftShift, gpucontext.KeyRightShift:
		return iinput.KShift
	case gpucontext.KeyLeftControl, gpucontext.KeyRightControl:
		return iinput.KCtrl
	case gpucontext.KeyLeftAlt, gpucontext.KeyRightAlt:
		return iinput.KAlt
	}

	if key >= gpucontext.KeyA && key <= gpucontext.KeyZ {
		return int('a' + (key - gpucontext.KeyA))
	}
	if key >= gpucontext.Key0 && key <= gpucontext.Key9 {
		return int('0' + (key - gpucontext.Key0))
	}

	return -1
}
