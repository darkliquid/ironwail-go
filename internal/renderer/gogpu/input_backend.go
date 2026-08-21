package gogpu

import (
	"log/slog"
	"sync"
	"time"

	iinput "github.com/darkliquid/ironwail-go/internal/input"
	gg "github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

// InputBackend adapts gogpu input to the engine input.Backend.
type InputBackend struct {
	app             *gg.App
	sys             *iinput.System
	cursorMode      iinput.CursorMode
	callbacksInited bool
	pollOnly        bool // never registers EventSource callbacks (ADR-0012).
	modifiers       iinput.ModifierState

	mu                     sync.Mutex
	hasMousePos            bool
	lastMouseX             float64
	lastMouseY             float64
	accumMouseDX           int32
	accumMouseDY           int32
	keyCallbackSeen        bool
	mouseButtonCallbackSeen bool
	mouseMoveCallbackSeen  bool
	pollPrevPressed        []bool
	pollPrevMouse          []bool
	pollCounter            uint64
	lastPollLog            time.Time
	eventLogCount          uint32
}

// NewInputBackend returns a Backend implementation wired to the renderer app.
func NewInputBackend(app *gg.App, sys *iinput.System) iinput.Backend {
	return &InputBackend{app: app, sys: sys}
}

// NewPollOnlyInputBackend returns a Backend that drives all input from
// gogpu's polled app.Input() state (ADR-0012 §5.2: engine gameplay input
// migrates from the callback backend to polling). It never registers
// EventSource callbacks — the UI owns the EventSource on the gogpu/ui path,
// so callbacks would double-deliver. The polling loop below synthesizes
// press/release edges from held-state transitions exactly as the callback
// path would.
func NewPollOnlyInputBackend(app *gg.App, sys *iinput.System) iinput.Backend {
	return &InputBackend{app: app, sys: sys, pollOnly: true}
}

func (b *InputBackend) Init() error {
	if !b.pollOnly {
		b.initCallbacks()
	}
	slog.Debug("gogpu input backend initialized")
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
			slog.Debug("INPUT poll early", "reason", "app nil", "poll_count", b.pollCounter)
			b.lastPollLog = time.Now()
		}
		return true
	}
	if b.sys == nil {
		if time.Since(b.lastPollLog) > time.Second {
			slog.Debug("INPUT poll early", "reason", "sys nil", "poll_count", b.pollCounter)
			b.lastPollLog = time.Now()
		}
		return true
	}

	state := b.app.Input()
	if state == nil || state.Keyboard() == nil || state.Mouse() == nil {
		if time.Since(b.lastPollLog) > time.Second {
			slog.Debug("INPUT poll early", "reason", "state/keyboard nil", "poll_count", b.pollCounter)
			b.lastPollLog = time.Now()
		}
		return true
	}

	keyboard := state.Keyboard()
	mouse := state.Mouse()
	if time.Since(b.lastPollLog) > time.Second {
		slog.Debug(
			"INPUT poll heartbeat",
			"poll_count", b.pollCounter,
			"any_pressed", keyboard.AnyPressed(),
			"mouse_x", mouse.X(),
			"mouse_y", mouse.Y(),
			"key_callbacks_seen", b.hasKeyCallbackSeen(),
			"mouse_btn_callbacks_seen", b.hasMouseButtonCallbackSeen(),
			"mouse_move_callbacks_seen", b.hasMouseMoveCallbackSeen(),
		)
		b.lastPollLog = time.Now()
	}

	if !b.hasKeyCallbackSeen() || b.pollOnly {
		if len(b.pollPrevPressed) != len(PollingKeyMap) {
			b.pollPrevPressed = make([]bool, len(PollingKeyMap))
		}
		for index, pair := range PollingKeyMap {
			pressed := keyboard.Pressed(pair.Src)
			prev := b.pollPrevPressed[index]
			if pressed != prev {
				b.logInputEvent("poll-key", pair.Dst, pressed)
				b.sys.HandleKeyEvent(iinput.KeyEvent{Key: pair.Dst, Down: pressed, Device: iinput.DeviceKeyboard})
				b.pollPrevPressed[index] = pressed
			}
		}
	}

	if !b.hasMouseButtonCallbackSeen() || b.pollOnly {
		if len(b.pollPrevMouse) != len(PollingMouseButtonMap) {
			b.pollPrevMouse = make([]bool, len(PollingMouseButtonMap))
		}
		for index, pair := range PollingMouseButtonMap {
			pressed := mouse.Pressed(pair.Src)
			prev := b.pollPrevMouse[index]
			if pressed != prev {
				b.logInputEvent("poll-mouse", pair.Dst, pressed)
				b.sys.HandleKeyEvent(iinput.KeyEvent{Key: pair.Dst, Down: pressed, Device: iinput.DeviceMouse})
				b.pollPrevMouse[index] = pressed
			}
		}

		scrollX, scrollY := mouse.Scroll()
		if scrollY > 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelUp, Down: true, Device: iinput.DeviceMouse})
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelUp, Down: false, Device: iinput.DeviceMouse})
		} else if scrollY < 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelDown, Down: true, Device: iinput.DeviceMouse})
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelDown, Down: false, Device: iinput.DeviceMouse})
		}
		_ = scrollX
	}

	if !b.hasMouseMoveCallbackSeen() || b.pollOnly {
		dx, dy := mouse.Delta()
		x, y := mouse.Position()
		b.accumulateMousePosition(int32(dx), int32(dy), float64(x), float64(y))
	}

	return true
}

// accumulateMousePosition folds a new mouse delta/position sample into the
// backend's accumulator, shared by the polling and callback paths so mouse
// look is fed exactly once regardless of whether key/button events are
// delivered via EventSource callbacks or raw polling.
func (b *InputBackend) accumulateMousePosition(dx, dy int32, x, y float64) {
	b.mu.Lock()
	b.accumMouseDX += dx
	b.accumMouseDY += dy
	b.lastMouseX = x
	b.lastMouseY = y
	b.hasMousePos = true
	b.mu.Unlock()
}

func (b *InputBackend) initCallbacks() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.callbacksInited || b.app == nil || b.sys == nil {
		return
	}

	es := b.app.EventSource()

	es.OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		mapped := MapGPUContextKey(key)
		b.markKeyCallbackSeen()
		b.mu.Lock()
		b.modifiers = iinput.ModifierState{Shift: mods.HasShift(), Ctrl: mods.HasControl(), Alt: mods.HasAlt()}
		b.mu.Unlock()
		if mapped >= 0 {
			b.logInputEvent("callback-key", mapped, true)
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mapped, Down: true, Device: iinput.DeviceKeyboard})
		}
	})

	es.OnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		mapped := MapGPUContextKey(key)
		b.markKeyCallbackSeen()
		b.mu.Lock()
		b.modifiers = iinput.ModifierState{Shift: mods.HasShift(), Ctrl: mods.HasControl(), Alt: mods.HasAlt()}
		b.mu.Unlock()
		if mapped >= 0 {
			b.logInputEvent("callback-key", mapped, false)
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mapped, Down: false, Device: iinput.DeviceKeyboard})
		}
	})

	es.OnTextInput(func(text string) {
		b.markKeyCallbackSeen()
		for _, r := range text {
			b.sys.HandleCharEvent(r)
		}
	})

	if pes, ok := es.(gpucontext.PointerEventSource); ok {
		pes.OnPointer(func(ev gpucontext.PointerEvent) {
			b.markMouseMoveCallbackSeen()
			if ev.PointerType != gpucontext.PointerTypeMouse || ev.Type != gpucontext.PointerMove {
				return
			}
			b.mu.Lock()
			locked := b.app != nil && b.app.CursorMode() == gpucontext.CursorModeLocked
			if b.cursorMode == iinput.CursorModeGrabbed && locked {
				b.accumMouseDX += int32(ev.DeltaX)
				b.accumMouseDY += int32(ev.DeltaY)
			} else if b.hasMousePos {
				b.accumMouseDX += int32(ev.X - b.lastMouseX)
				b.accumMouseDY += int32(ev.Y - b.lastMouseY)
			}
			b.lastMouseX = ev.X
			b.lastMouseY = ev.Y
			b.hasMousePos = true
			b.mu.Unlock()
		})
	} else {
		es.OnMouseMove(func(x, y float64) {
			b.markMouseMoveCallbackSeen()
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
	}

	es.OnMousePress(func(button gpucontext.MouseButton, x, y float64) {
		b.markMouseButtonCallbackSeen()
		if key := MapGPUContextMouseButton(button); key >= 0 {
			b.logInputEvent("callback-mouse", key, true)
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: key, Down: true, Device: iinput.DeviceMouse})
		}
		_ = x
		_ = y
	})

	es.OnMouseRelease(func(button gpucontext.MouseButton, x, y float64) {
		b.markMouseButtonCallbackSeen()
		if key := MapGPUContextMouseButton(button); key >= 0 {
			b.logInputEvent("callback-mouse", key, false)
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: key, Down: false, Device: iinput.DeviceMouse})
		}
		_ = x
		_ = y
	})

	es.OnScroll(func(dx, dy float64) {
		b.markMouseButtonCallbackSeen()
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
	slog.Debug("gogpu input callbacks registered")
}

func (b *InputBackend) logInputEvent(source string, key int, down bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.eventLogCount >= 32 {
		return
	}
	b.eventLogCount++
	keyName := iinput.KeyToString(key)
	if keyName == "" {
		keyName = "UNKNOWN"
	}
	slog.Debug("input event observed", "source", source, "key", keyName, "key_code", key, "down", down, "event_index", b.eventLogCount)
}

func (b *InputBackend) markKeyCallbackSeen() {
	b.mu.Lock()
	b.keyCallbackSeen = true
	b.mu.Unlock()
}

func (b *InputBackend) hasKeyCallbackSeen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.keyCallbackSeen
}

func (b *InputBackend) markMouseButtonCallbackSeen() {
	b.mu.Lock()
	b.mouseButtonCallbackSeen = true
	b.mu.Unlock()
}

func (b *InputBackend) hasMouseButtonCallbackSeen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mouseButtonCallbackSeen
}

func (b *InputBackend) markMouseMoveCallbackSeen() {
	b.mu.Lock()
	b.mouseMoveCallbackSeen = true
	b.mu.Unlock()
}

func (b *InputBackend) hasMouseMoveCallbackSeen() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mouseMoveCallbackSeen
}

func (b *InputBackend) MouseDelta() (dx, dy int32) {
	b.mu.Lock()
	dx, dy = b.accumMouseDX, b.accumMouseDY
	b.accumMouseDX = 0
	b.accumMouseDY = 0
	b.mu.Unlock()
	return dx, dy
}

func (b *InputBackend) MousePosition() (x, y int32, valid bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return int32(b.lastMouseX), int32(b.lastMouseY), b.hasMousePos
}

func (b *InputBackend) ModifierState() iinput.ModifierState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.modifiers
}

func (b *InputBackend) SetTextMode(mode iinput.TextMode) {}

type cursorModeState struct {
	cursor     gpucontext.CursorShape
	cursorMode gpucontext.CursorMode
	setCursor  bool
}

func cursorModeAdapter(mode iinput.CursorMode) cursorModeState {
	switch mode {
	case iinput.CursorModeNormal:
		return cursorModeState{
			cursor:     gpucontext.CursorDefault,
			cursorMode: gpucontext.CursorModeNormal,
			setCursor:  true,
		}
	case iinput.CursorModeHidden:
		return cursorModeState{
			cursor:     gpucontext.CursorNone,
			cursorMode: gpucontext.CursorModeNormal,
			setCursor:  true,
		}
	case iinput.CursorModeGrabbed:
		return cursorModeState{
			cursorMode: gpucontext.CursorModeLocked,
		}
	default:
		return cursorModeState{
			cursor:     gpucontext.CursorDefault,
			cursorMode: gpucontext.CursorModeNormal,
			setCursor:  true,
		}
	}
}

func (b *InputBackend) SetCursorMode(mode iinput.CursorMode) {
	b.mu.Lock()
	b.cursorMode = mode
	b.mu.Unlock()
	if b.app == nil {
		return
	}
	state := cursorModeAdapter(mode)
	b.app.SetCursorMode(state.cursorMode)
	if state.setCursor {
		b.app.SetCursor(state.cursor)
	}
}

func (b *InputBackend) ShowKeyboard(show bool) {}

func (b *InputBackend) GamepadState(player int) iinput.GamepadState {
	return iinput.GamepadState{}
}

func (b *InputBackend) IsGamepadConnected(player int) bool { return false }

func (b *InputBackend) SetMouseGrab(grabbed bool) {
	if grabbed {
		b.SetCursorMode(iinput.CursorModeGrabbed)
		return
	}
	b.SetCursorMode(iinput.CursorModeNormal)
}

func (b *InputBackend) SetWindow(win any) {}
