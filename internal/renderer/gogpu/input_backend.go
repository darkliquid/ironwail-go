//go:build gogpu && !cgo
// +build gogpu,!cgo

package gogpu

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	gg "github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
	iinput "github.com/ironwail/ironwail-go/internal/input"
)

// InputBackend adapts gogpu input to the engine input.Backend.
type InputBackend struct {
	app             *gg.App
	sys             *iinput.System
	cursorMode      iinput.CursorMode
	callbacksInited bool
	modifiers       iinput.ModifierState

	mu              sync.Mutex
	hasMousePos     bool
	lastMouseX      float64
	lastMouseY      float64
	accumMouseDX    int32
	accumMouseDY    int32
	callbackInputOK bool
	callbackSeen    bool
	pollPrevPressed []bool
	pollCounter     uint64
	lastPollLog     time.Time
}

// NewInputBackend returns a Backend implementation wired to the renderer app.
func NewInputBackend(app *gg.App, sys *iinput.System) iinput.Backend {
	return &InputBackend{app: app, sys: sys}
}

func (b *InputBackend) Init() error {
	b.initCallbacks()
	slog.Info("gogpu input backend: init completed")
	return nil
}

func (b *InputBackend) Shutdown() {}

func (b *InputBackend) PollEvents() bool {
	if !b.callbacksInited {
		b.initCallbacks()
	}
	b.pollCounter++

	if b.app == nil {
		if time.Since(b.lastPollLog) > time.Second {
			slog.Info("INPUT poll early", "reason", "app nil", "poll_count", b.pollCounter)
			b.lastPollLog = time.Now()
		}
		return true
	}
	if b.sys == nil {
		if time.Since(b.lastPollLog) > time.Second {
			slog.Info("INPUT poll early", "reason", "sys nil", "poll_count", b.pollCounter)
			b.lastPollLog = time.Now()
		}
		return true
	}
	if b.hasCallbackInput() {
		if time.Since(b.lastPollLog) > time.Second {
			slog.Info("INPUT poll early", "reason", "callbacks active", "poll_count", b.pollCounter)
			b.lastPollLog = time.Now()
		}
		return true
	}

	state := b.app.Input()
	if state == nil || state.Keyboard() == nil {
		if time.Since(b.lastPollLog) > time.Second {
			slog.Info("INPUT poll early", "reason", "state/keyboard nil", "poll_count", b.pollCounter)
			b.lastPollLog = time.Now()
		}
		return true
	}

	state.Update()

	keyboard := state.Keyboard()
	mouse := state.Mouse()
	if time.Since(b.lastPollLog) > time.Second {
		slog.Info(
			"INPUT poll heartbeat",
			"poll_count", b.pollCounter,
			"any_pressed", keyboard.AnyPressed(),
			"mouse_x", mouse.X(),
			"mouse_y", mouse.Y(),
			"callbacks_seen", b.hasCallbackSeen(),
		)
		b.lastPollLog = time.Now()
	}
	if len(b.pollPrevPressed) != len(PollingKeyMap) {
		b.pollPrevPressed = make([]bool, len(PollingKeyMap))
	}

	for index, pair := range PollingKeyMap {
		pressed := keyboard.Pressed(pair.Src)
		prev := b.pollPrevPressed[index]
		if pressed != prev {
			slog.Info("gogpu input polling key", "src", pair.Src, "dst", pair.Dst, "down", pressed)
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: pair.Dst, Down: pressed, Device: iinput.DeviceKeyboard})
			b.pollPrevPressed[index] = pressed
		}
	}

	return true
}

func (b *InputBackend) initCallbacks() {
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
		mapped := MapGPUContextKey(key)
		b.markCallbackSeen()
		b.mu.Lock()
		b.modifiers = iinput.ModifierState{Shift: mods.HasShift(), Ctrl: mods.HasControl(), Alt: mods.HasAlt()}
		b.mu.Unlock()
		if mapped >= 0 {
			b.markCallbackInput()
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mapped, Down: true, Device: iinput.DeviceKeyboard})
		}
	})

	es.OnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		mapped := MapGPUContextKey(key)
		b.markCallbackSeen()
		b.mu.Lock()
		b.modifiers = iinput.ModifierState{Shift: mods.HasShift(), Ctrl: mods.HasControl(), Alt: mods.HasAlt()}
		b.mu.Unlock()
		if mapped >= 0 {
			b.markCallbackInput()
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mapped, Down: false, Device: iinput.DeviceKeyboard})
		}
	})

	es.OnTextInput(func(text string) {
		b.markCallbackSeen()
		for _, r := range text {
			b.sys.HandleCharEvent(r)
		}
	})

	es.OnMouseMove(func(x, y float64) {
		b.markCallbackSeen()
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
		b.markCallbackSeen()
		if key := MapGPUContextMouseButton(button); key >= 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: key, Down: true, Device: iinput.DeviceMouse})
		}
		_ = x
		_ = y
	})

	es.OnMouseRelease(func(button gpucontext.MouseButton, x, y float64) {
		b.markCallbackSeen()
		if key := MapGPUContextMouseButton(button); key >= 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: key, Down: false, Device: iinput.DeviceMouse})
		}
		_ = x
		_ = y
	})

	es.OnScroll(func(dx, dy float64) {
		b.markCallbackSeen()
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
		b.markCallbackSeen()
		if !focused {
			b.sys.ClearKeyStates()
		}
	})

	b.callbacksInited = true
	slog.Info("gogpu input backend: event source callbacks registered", "event_source", fmt.Sprintf("%T", es))
}

func (b *InputBackend) markCallbackInput() {
	b.mu.Lock()
	b.callbackInputOK = true
	b.mu.Unlock()
}

func (b *InputBackend) markCallbackSeen() {
	b.mu.Lock()
	b.callbackSeen = true
	b.mu.Unlock()
}

func (b *InputBackend) hasCallbackInput() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.callbackInputOK
}

func (b *InputBackend) hasCallbackSeen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.callbackSeen
}

func (b *InputBackend) GetMouseDelta() (dx, dy int32) {
	b.mu.Lock()
	dx, dy = b.accumMouseDX, b.accumMouseDY
	b.accumMouseDX = 0
	b.accumMouseDY = 0
	b.mu.Unlock()
	return dx, dy
}

func (b *InputBackend) GetMousePosition() (x, y int32, valid bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return int32(b.lastMouseX), int32(b.lastMouseY), b.hasMousePos
}

func (b *InputBackend) GetModifierState() iinput.ModifierState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.modifiers
}

func (b *InputBackend) SetTextMode(mode iinput.TextMode) {}

func (b *InputBackend) SetCursorMode(mode iinput.CursorMode) {
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

func (b *InputBackend) ShowKeyboard(show bool) {}

func (b *InputBackend) GetGamepadState(player int) iinput.GamepadState {
	return iinput.GamepadState{}
}

func (b *InputBackend) IsGamepadConnected(player int) bool { return false }

func (b *InputBackend) SetMouseGrab(grabbed bool) {}

func (b *InputBackend) SetWindow(win interface{}) {}
