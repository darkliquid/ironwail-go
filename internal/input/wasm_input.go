//go:build js && wasm

package input

import (
	"log/slog"
	"strings"
	"sync"
	"syscall/js"
)

type wasmListenerPair struct {
	eventType string
	fn        js.Func
}

// WASMBackend implements input.Backend for WebAssembly browser DOM environments.
type WASMBackend struct {
	mu           sync.Mutex
	sys          *System
	textMode     TextMode
	cursorMode   CursorMode
	mouseGrabbed bool
	pendingKeys  []KeyEvent
	pendingChars []rune
	mouseDeltaX  float64
	mouseDeltaY  float64
	mouseX       float64
	mouseY       float64
	listeners    []wasmListenerPair
}

// NewWASMBackend creates an input backend connected to the browser DOM.
func NewWASMBackend() *WASMBackend {
	return &WASMBackend{}
}

func (b *WASMBackend) Init() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	window := js.Global().Get("window")
	if window.IsUndefined() || window.IsNull() {
		slog.Warn("WASM input: window object unavailable")
		return nil
	}

	// Browser autoplay policy: the Web Audio AudioContext is created
	// suspended until a user gesture. Resume it here so the first gesture
	// (keydown/click on the canvas) unlocks audio.
	resumeAudio := js.Global().Get("__ironwailAudioResume")
	if !resumeAudio.IsUndefined() && !resumeAudio.IsNull() {
		_ = resumeAudio.Invoke()
	}

	keydownFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		resumeAudio := js.Global().Get("__ironwailAudioResume")
		if !resumeAudio.IsUndefined() && !resumeAudio.IsNull() {
			_ = resumeAudio.Invoke()
		}

		event := args[0]
		code := event.Get("code").String()
		key := mapDOMCodeToQuakeKey(code)
		if key != 0 {
			b.mu.Lock()
			b.pendingKeys = append(b.pendingKeys, KeyEvent{Key: key, Down: true})
			b.mu.Unlock()
		}

		keyChar := event.Get("key").String()
		if len(keyChar) == 1 && b.textMode != TextModeOff {
			b.mu.Lock()
			b.pendingChars = append(b.pendingChars, []rune(keyChar)[0])
			b.mu.Unlock()
		}

		// Prevent browser scrolling on arrow keys and space
		if code == "Space" || strings.HasPrefix(code, "Arrow") {
			event.Call("preventDefault")
		}
		return nil
	})

	keyupFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		code := event.Get("code").String()
		key := mapDOMCodeToQuakeKey(code)
		if key != 0 {
			b.mu.Lock()
			b.pendingKeys = append(b.pendingKeys, KeyEvent{Key: key, Down: false})
			b.mu.Unlock()
		}
		return nil
	})

	mousemoveFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		dx := event.Get("movementX").Float()
		dy := event.Get("movementY").Float()
		b.mu.Lock()
		b.mouseDeltaX += dx
		b.mouseDeltaY += dy
		b.mu.Unlock()
		return nil
	})

	mousedownFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		resumeAudio := js.Global().Get("__ironwailAudioResume")
		if !resumeAudio.IsUndefined() && !resumeAudio.IsNull() {
			_ = resumeAudio.Invoke()
		}

		event := args[0]
		button := event.Get("button").Int()
		key := mapDOMMouseButton(button)
		if key != 0 {
			b.mu.Lock()
			b.pendingKeys = append(b.pendingKeys, KeyEvent{Key: key, Down: true})
			b.mu.Unlock()
		}
		return nil
	})

	mouseupFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		button := event.Get("button").Int()
		key := mapDOMMouseButton(button)
		if key != 0 {
			b.mu.Lock()
			b.pendingKeys = append(b.pendingKeys, KeyEvent{Key: key, Down: false})
			b.mu.Unlock()
		}
		return nil
	})

	listeners := []wasmListenerPair{
		{"keydown", keydownFunc},
		{"keyup", keyupFunc},
		{"mousemove", mousemoveFunc},
		{"mousedown", mousedownFunc},
		{"mouseup", mouseupFunc},
	}

	for _, l := range listeners {
		window.Call("addEventListener", l.eventType, l.fn)
	}

	b.listeners = listeners
	slog.Info("WASM DOM input backend registered")
	return nil
}

func (b *WASMBackend) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()
	window := js.Global().Get("window")
	if !window.IsUndefined() && !window.IsNull() {
		for _, l := range b.listeners {
			window.Call("removeEventListener", l.eventType, l.fn)
			l.fn.Release()
		}
	}
	b.listeners = nil
}

// PollEvents implements the Backend interface. The engine also drains DOM
// events via PollEventsSys each frame; this keeps the interface satisfied
// for callers that only use the standard cycle.
func (b *WASMBackend) PollEvents() bool { return true }

// PollEventsSys drains the DOM listener queues into the System each frame.
func (b *WASMBackend) PollEventsSys(sys *System) bool {
	b.mu.Lock()
	keys := b.pendingKeys
	chars := b.pendingChars
	dx := b.mouseDeltaX
	dy := b.mouseDeltaY
	b.pendingKeys = nil
	b.pendingChars = nil
	b.mouseDeltaX = 0
	b.mouseDeltaY = 0
	b.mu.Unlock()

	for _, k := range keys {
		sys.HandleKeyEvent(k)
	}
	for _, c := range chars {
		sys.HandleCharEvent(c)
	}
	// Apply the DOM mouse deltas directly so mouse-look works even when the
	// backend is polled out-of-band from System.State (wasm walkthrough loop).
	sys.ApplyMouseDelta(int32(dx), int32(dy))
	return true
}

func (b *WASMBackend) MouseDelta() (dx, dy int32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	dx = int32(b.mouseDeltaX)
	dy = int32(b.mouseDeltaY)
	b.mouseDeltaX = 0
	b.mouseDeltaY = 0
	return dx, dy
}

func (b *WASMBackend) MousePosition() (x, y int32, valid bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return int32(b.mouseX), int32(b.mouseY), true
}

func (b *WASMBackend) ModifierState() ModifierState {
	return ModifierState{}
}

func (b *WASMBackend) SetTextMode(mode TextMode) {
	b.mu.Lock()
	b.textMode = mode
	b.mu.Unlock()
}

func (b *WASMBackend) SetCursorMode(mode CursorMode) {
	b.mu.Lock()
	b.cursorMode = mode
	b.mu.Unlock()
}

func (b *WASMBackend) ShowKeyboard(show bool) {}

func (b *WASMBackend) GamepadState(player int) GamepadState {
	return GamepadState{}
}

func (b *WASMBackend) IsGamepadConnected(player int) bool {
	return false
}

func (b *WASMBackend) SetMouseGrab(grabbed bool) {
	b.mu.Lock()
	b.mouseGrabbed = grabbed
	b.mu.Unlock()

	// Browser pointer lock: the walkthrough viewport needs relative mouse
	// deltas (movementX/Y) which only arrive while the canvas has pointer
	// lock. Engines grab on click; browsers require the request from a user
	// gesture, so the DOM backend applies it here directly.
	doc := js.Global().Get("document")
	if doc.IsUndefined() || doc.IsNull() {
		return
	}
	canvas := doc.Call("querySelector", "canvas")
	if canvas.IsNull() || canvas.IsUndefined() {
		return
	}
	if grabbed {
		canvas.Call("requestPointerLock")
	} else {
		_ = doc.Call("exitPointerLock")
	}
}

func (b *WASMBackend) SetWindow(win any) {}

func mapDOMCodeToQuakeKey(code string) int {
	switch code {
	case "KeyA":
		return 'a'
	case "KeyB":
		return 'b'
	case "KeyC":
		return 'c'
	case "KeyD":
		return 'd'
	case "KeyE":
		return 'e'
	case "KeyF":
		return 'f'
	case "KeyG":
		return 'g'
	case "KeyH":
		return 'h'
	case "KeyI":
		return 'i'
	case "KeyJ":
		return 'j'
	case "KeyK":
		return 'k'
	case "KeyL":
		return 'l'
	case "KeyM":
		return 'm'
	case "KeyN":
		return 'n'
	case "KeyO":
		return 'o'
	case "KeyP":
		return 'p'
	case "KeyQ":
		return 'q'
	case "KeyR":
		return 'r'
	case "KeyS":
		return 's'
	case "KeyT":
		return 't'
	case "KeyU":
		return 'u'
	case "KeyV":
		return 'v'
	case "KeyW":
		return 'w'
	case "KeyX":
		return 'x'
	case "KeyY":
		return 'y'
	case "KeyZ":
		return 'z'
	case "Digit0":
		return '0'
	case "Digit1":
		return '1'
	case "Digit2":
		return '2'
	case "Digit3":
		return '3'
	case "Digit4":
		return '4'
	case "Digit5":
		return '5'
	case "Digit6":
		return '6'
	case "Digit7":
		return '7'
	case "Digit8":
		return '8'
	case "Digit9":
		return '9'
	case "Space":
		return KSpace
	case "Enter":
		return KEnter
	case "Escape":
		return KEscape
	case "Tab":
		return KTab
	case "Backspace":
		return KBackspace
	case "ArrowUp":
		return KUpArrow
	case "ArrowDown":
		return KDownArrow
	case "ArrowLeft":
		return KLeftArrow
	case "ArrowRight":
		return KRightArrow
	case "ControlLeft", "ControlRight":
		return KCtrl
	case "ShiftLeft", "ShiftRight":
		return KShift
	case "AltLeft", "AltRight":
		return KAlt
	case "F1":
		return KF1
	case "F2":
		return KF2
	case "F3":
		return KF3
	case "F4":
		return KF4
	case "F5":
		return KF5
	case "F6":
		return KF6
	case "F7":
		return KF7
	case "F8":
		return KF8
	case "F9":
		return KF9
	case "F10":
		return KF10
	case "F11":
		return KF11
	case "F12":
		return KF12
	default:
		return 0
	}
}

func mapDOMMouseButton(button int) int {
	switch button {
	case 0:
		return KMouse1
	case 1:
		return KMouse3
	case 2:
		return KMouse2
	default:
		return 0
	}
}
