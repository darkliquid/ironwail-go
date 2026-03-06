//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"sync"

	"github.com/go-gl/glfw/v3.3/glfw"
	iinput "github.com/ironwail/ironwail-go/internal/input"
)

type glfwInputBackend struct {
	window *glfw.Window
	sys    *iinput.System

	mu            sync.Mutex
	quitRequested bool
	accumDX       float64
	accumDY       float64
	lastX         float64
	lastY         float64
	hasLastPos    bool
	textMode      iinput.TextMode
	modifiers     iinput.ModifierState
}

// InputBackendForSystem returns a GLFW-based input backend for the OpenGL/CGO renderer.
func (r *Renderer) InputBackendForSystem(sys *iinput.System) iinput.Backend {
	if r.window == nil {
		return nil
	}
	return &glfwInputBackend{
		window: r.window,
		sys:    sys,
	}
}

func (b *glfwInputBackend) Init() error {
	b.window.SetKeyCallback(b.keyCallback)
	b.window.SetCharCallback(b.charCallback)
	b.window.SetMouseButtonCallback(b.mouseButtonCallback)
	b.window.SetScrollCallback(b.scrollCallback)
	b.window.SetCursorPosCallback(b.cursorPosCallback)
	b.window.SetCloseCallback(func(w *glfw.Window) {
		b.mu.Lock()
		b.quitRequested = true
		b.mu.Unlock()
	})
	return nil
}

func (b *glfwInputBackend) Shutdown() {
	b.window.SetKeyCallback(nil)
	b.window.SetCharCallback(nil)
	b.window.SetMouseButtonCallback(nil)
	b.window.SetScrollCallback(nil)
	b.window.SetCursorPosCallback(nil)
	b.window.SetCloseCallback(nil)
}

// PollEvents returns false when the window close has been requested.
// Actual GLFW event pumping is done by the renderer's run loop via glfw.PollEvents().
func (b *glfwInputBackend) PollEvents() bool {
	b.mu.Lock()
	quit := b.quitRequested
	b.mu.Unlock()
	return !quit
}

func (b *glfwInputBackend) GetMouseDelta() (dx, dy int32) {
	b.mu.Lock()
	x := int32(b.accumDX)
	y := int32(b.accumDY)
	b.accumDX -= float64(x)
	b.accumDY -= float64(y)
	b.mu.Unlock()
	return x, y
}

func (b *glfwInputBackend) GetModifierState() iinput.ModifierState {
	b.mu.Lock()
	m := b.modifiers
	b.mu.Unlock()
	return m
}

func (b *glfwInputBackend) SetTextMode(mode iinput.TextMode) {
	b.mu.Lock()
	b.textMode = mode
	b.mu.Unlock()
}

func (b *glfwInputBackend) SetCursorMode(mode iinput.CursorMode) {
	switch mode {
	case iinput.CursorModeNormal:
		b.window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
	case iinput.CursorModeHidden:
		b.window.SetInputMode(glfw.CursorMode, glfw.CursorHidden)
	case iinput.CursorModeGrabbed:
		b.window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
	}
}

func (b *glfwInputBackend) ShowKeyboard(_ bool) {}

func (b *glfwInputBackend) GetGamepadState(_ int) iinput.GamepadState {
	return iinput.GamepadState{}
}

func (b *glfwInputBackend) IsGamepadConnected(_ int) bool {
	return false
}

func (b *glfwInputBackend) SetMouseGrab(grabbed bool) {
	if grabbed {
		b.window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
	} else {
		b.window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
	}
}

func (b *glfwInputBackend) SetWindow(_ interface{}) {}

// GLFW callbacks — all called from the main thread during glfw.PollEvents().

func (b *glfwInputBackend) keyCallback(_ *glfw.Window, key glfw.Key, _ int, action glfw.Action, mods glfw.ModifierKey) {
	if action == glfw.Repeat {
		return
	}
	down := action == glfw.Press
	b.mu.Lock()
	b.modifiers = iinput.ModifierState{
		Shift: mods&glfw.ModShift != 0,
		Ctrl:  mods&glfw.ModControl != 0,
		Alt:   mods&glfw.ModAlt != 0,
	}
	b.mu.Unlock()
	if mapped := mapGLFWKey(key); mapped >= 0 {
		b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mapped, Down: down, Device: iinput.DeviceKeyboard})
	}
}

func (b *glfwInputBackend) charCallback(_ *glfw.Window, r rune) {
	b.mu.Lock()
	mode := b.textMode
	b.mu.Unlock()
	if mode != iinput.TextModeOff {
		b.sys.HandleCharEvent(r)
	}
}

func (b *glfwInputBackend) mouseButtonCallback(_ *glfw.Window, button glfw.MouseButton, action glfw.Action, _ glfw.ModifierKey) {
	down := action == glfw.Press
	if key := mapGLFWMouseButton(button); key >= 0 {
		b.sys.HandleKeyEvent(iinput.KeyEvent{Key: key, Down: down, Device: iinput.DeviceMouse})
	}
}

func (b *glfwInputBackend) scrollCallback(_ *glfw.Window, _, yoff float64) {
	if yoff > 0 {
		b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelUp, Down: true, Device: iinput.DeviceMouse})
		b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelUp, Down: false, Device: iinput.DeviceMouse})
	} else if yoff < 0 {
		b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelDown, Down: true, Device: iinput.DeviceMouse})
		b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelDown, Down: false, Device: iinput.DeviceMouse})
	}
}

func (b *glfwInputBackend) cursorPosCallback(_ *glfw.Window, x, y float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.hasLastPos {
		b.accumDX += x - b.lastX
		b.accumDY += y - b.lastY
	}
	b.lastX = x
	b.lastY = y
	b.hasLastPos = true
}

// mapGLFWKey maps a GLFW key to a Quake engine key code. Returns -1 if unmapped.
func mapGLFWKey(k glfw.Key) int {
	switch k {
	case glfw.KeyEscape:
		return iinput.KEscape
	case glfw.KeyEnter, glfw.KeyKPEnter:
		return iinput.KEnter
	case glfw.KeyTab:
		return iinput.KTab
	case glfw.KeyBackspace:
		return iinput.KBackspace
	case glfw.KeySpace:
		return iinput.KSpace
	case glfw.KeyUp:
		return iinput.KUpArrow
	case glfw.KeyDown:
		return iinput.KDownArrow
	case glfw.KeyLeft:
		return iinput.KLeftArrow
	case glfw.KeyRight:
		return iinput.KRightArrow
	case glfw.KeyLeftShift, glfw.KeyRightShift:
		return iinput.KShift
	case glfw.KeyLeftControl, glfw.KeyRightControl:
		return iinput.KCtrl
	case glfw.KeyLeftAlt, glfw.KeyRightAlt:
		return iinput.KAlt
	case glfw.KeyCapsLock:
		return iinput.KCapsLock
	case glfw.KeyPause:
		return iinput.KPause
	case glfw.KeyInsert:
		return iinput.KIns
	case glfw.KeyDelete:
		return iinput.KDel
	case glfw.KeyHome:
		return iinput.KHome
	case glfw.KeyEnd:
		return iinput.KEnd
	case glfw.KeyPageUp:
		return iinput.KPgUp
	case glfw.KeyPageDown:
		return iinput.KPgDn
	case glfw.KeyF1:
		return iinput.KF1
	case glfw.KeyF2:
		return iinput.KF2
	case glfw.KeyF3:
		return iinput.KF3
	case glfw.KeyF4:
		return iinput.KF4
	case glfw.KeyF5:
		return iinput.KF5
	case glfw.KeyF6:
		return iinput.KF6
	case glfw.KeyF7:
		return iinput.KF7
	case glfw.KeyF8:
		return iinput.KF8
	case glfw.KeyF9:
		return iinput.KF9
	case glfw.KeyF10:
		return iinput.KF10
	case glfw.KeyF11:
		return iinput.KF11
	case glfw.KeyF12:
		return iinput.KF12
	// Keypad — mapped to Quake's KKp* codes
	case glfw.KeyKP0:
		return iinput.KKpIns
	case glfw.KeyKP1:
		return iinput.KKpEnd
	case glfw.KeyKP2:
		return iinput.KKpDownArrow
	case glfw.KeyKP3:
		return iinput.KKpPgDn
	case glfw.KeyKP4:
		return iinput.KKpLeftArrow
	case glfw.KeyKP5:
		return iinput.KKp5
	case glfw.KeyKP6:
		return iinput.KKpRightArrow
	case glfw.KeyKP7:
		return iinput.KKpHome
	case glfw.KeyKP8:
		return iinput.KKpUpArrow
	case glfw.KeyKP9:
		return iinput.KKpPgUp
	case glfw.KeyKPDivide:
		return iinput.KKpSlash
	case glfw.KeyKPMultiply:
		return iinput.KKpStar
	case glfw.KeyKPSubtract:
		return iinput.KKpMinus
	case glfw.KeyKPAdd:
		return iinput.KKpPlus
	case glfw.KeyKPDecimal:
		return iinput.KKpDel
	case glfw.KeyNumLock:
		return iinput.KKpNumLock
	// Letter keys — lower-case ASCII
	case glfw.KeyA:
		return int('a')
	case glfw.KeyB:
		return int('b')
	case glfw.KeyC:
		return int('c')
	case glfw.KeyD:
		return int('d')
	case glfw.KeyE:
		return int('e')
	case glfw.KeyF:
		return int('f')
	case glfw.KeyG:
		return int('g')
	case glfw.KeyH:
		return int('h')
	case glfw.KeyI:
		return int('i')
	case glfw.KeyJ:
		return int('j')
	case glfw.KeyK:
		return int('k')
	case glfw.KeyL:
		return int('l')
	case glfw.KeyM:
		return int('m')
	case glfw.KeyN:
		return int('n')
	case glfw.KeyO:
		return int('o')
	case glfw.KeyP:
		return int('p')
	case glfw.KeyQ:
		return int('q')
	case glfw.KeyR:
		return int('r')
	case glfw.KeyS:
		return int('s')
	case glfw.KeyT:
		return int('t')
	case glfw.KeyU:
		return int('u')
	case glfw.KeyV:
		return int('v')
	case glfw.KeyW:
		return int('w')
	case glfw.KeyX:
		return int('x')
	case glfw.KeyY:
		return int('y')
	case glfw.KeyZ:
		return int('z')
	// Number keys
	case glfw.Key0:
		return int('0')
	case glfw.Key1:
		return int('1')
	case glfw.Key2:
		return int('2')
	case glfw.Key3:
		return int('3')
	case glfw.Key4:
		return int('4')
	case glfw.Key5:
		return int('5')
	case glfw.Key6:
		return int('6')
	case glfw.Key7:
		return int('7')
	case glfw.Key8:
		return int('8')
	case glfw.Key9:
		return int('9')
	}
	return -1
}

// mapGLFWMouseButton maps a GLFW mouse button to a Quake engine key code. Returns -1 if unmapped.
func mapGLFWMouseButton(b glfw.MouseButton) int {
	switch b {
	case glfw.MouseButtonLeft:
		return iinput.KMouse1
	case glfw.MouseButtonRight:
		return iinput.KMouse2
	case glfw.MouseButtonMiddle:
		return iinput.KMouse3
	case glfw.MouseButton4:
		return iinput.KMouse4
	case glfw.MouseButton5:
		return iinput.KMouse5
	}
	return -1
}
