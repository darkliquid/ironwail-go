//go:build gogpu
// +build gogpu

package renderer

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
	iinput "github.com/ironwail/ironwail-go/internal/input"
)

// gogpuInputBackend adapts gogpu's input state to the engine's input.Backend
type gogpuInputBackend struct {
	app             *gogpu.App
	sys             *iinput.System
	cursorMode      iinput.CursorMode
	callbacksInited bool

	mu            sync.Mutex
	hasMousePos   bool
	lastMouseX    float64
	lastMouseY    float64
	accumMouseDX  int32
	accumMouseDY  int32
	lastKeyStates [256]bool // Track previous frame's key states for edge detection
}

// InputBackendForSystem returns a Backend implementation wired to this renderer's app
func (r *Renderer) InputBackendForSystem(sys *iinput.System) iinput.Backend {
	return &gogpuInputBackend{
		app: r.app,
		sys: sys,
	}
}

func (b *gogpuInputBackend) Init() error {
	// Callbacks will be registered on first PollEvents() call
	// This defers initialization until after gogpu's event loop is running
	return nil
}

func (b *gogpuInputBackend) Shutdown() {
	// nothing to cleanup
}

func (b *gogpuInputBackend) PollEvents() bool {
	// Initialize callbacks on first call (after Run() has started)
	if !b.callbacksInited {
		b.initCallbacks()
	}
	return true
}

func (b *gogpuInputBackend) initCallbacks() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.callbacksInited || b.app == nil {
		return
	}

	es := b.app.EventSource()
	if es == nil {
		slog.Warn("gogpu input backend: event source unavailable")
		return
	}

	es.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		mapped := mapGPUContextKey(key)
		slog.Info("gogpu input: key press", "gpucontext_key", key, "mapped_key", mapped)
		if mapped >= 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mapped, Down: true, Device: iinput.DeviceKeyboard})
		}
		_ = mods
	})

	es.OnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		mapped := mapGPUContextKey(key)
		slog.Info("gogpu input: key release", "gpucontext_key", key, "mapped_key", mapped)
		if mapped >= 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mapped, Down: false, Device: iinput.DeviceKeyboard})
		}
		_ = mods
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
		slog.Info("gogpu input: window focus changed", "focused", focused)
		if !focused {
			b.sys.ClearKeyStates()
		}
	})

	b.callbacksInited = true
	slog.Info("gogpu input backend: event source callbacks registered", "event_source", fmt.Sprintf("%T", es))
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
	// Minimal: return empty modifier state
	return iinput.ModifierState{}
}

func (b *gogpuInputBackend) SetTextMode(mode iinput.TextMode) {
	// gogpu provides text input handling via the app; no-op for now
}

func (b *gogpuInputBackend) SetCursorMode(mode iinput.CursorMode) {
	b.cursorMode = mode
	// Best-effort: hide cursor for hidden/grabbed modes
	if b.app == nil {
		return
	}
	switch mode {
	case iinput.CursorModeNormal:
		b.app.SetCursor(gpucontext.CursorDefault)
	case iinput.CursorModeHidden:
		b.app.SetCursor(gpucontext.CursorNone)
	case iinput.CursorModeGrabbed:
		// No pointer-lock API exposed; hide cursor as best-effort and rely on relative deltas
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
