//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package opengl

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

// NewInputBackend returns a GLFW-based input backend for the OpenGL/CGO renderer.
func NewInputBackend(window *glfw.Window, sys *iinput.System) iinput.Backend {
	if window == nil {
		return nil
	}
	return &glfwInputBackend{window: window, sys: sys}
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
	b.mu.Lock()
	b.accumDX = 0
	b.accumDY = 0
	b.hasLastPos = false
	b.mu.Unlock()

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
	b.mu.Lock()
	b.accumDX = 0
	b.accumDY = 0
	b.hasLastPos = false
	b.mu.Unlock()

	if grabbed {
		b.window.SetInputMode(glfw.CursorMode, glfw.CursorDisabled)
	} else {
		b.window.SetInputMode(glfw.CursorMode, glfw.CursorNormal)
	}
}

func (b *glfwInputBackend) SetWindow(_ interface{}) {}

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
	if mapped := MapGLFWKey(key); mapped >= 0 {
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
	if key := MapGLFWMouseButton(button); key >= 0 {
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

func (b *glfwInputBackend) GetMousePosition() (x, y int32, valid bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return int32(b.lastX), int32(b.lastY), b.hasLastPos
}
