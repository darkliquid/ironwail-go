//go:build gogpu
// +build gogpu

package renderer

import (
	"github.com/gogpu/gogpu"
	ginput "github.com/gogpu/gogpu/input"
	"github.com/gogpu/gpucontext"
	iinput "github.com/ironwail/ironwail-go/internal/input"
)

// gogpuInputBackend adapts gogpu's input state to the engine's input.Backend
type gogpuInputBackend struct {
	app         *gogpu.App
	sys         *iinput.System
	prevKeys    map[ginput.Key]bool
	prevButtons map[int]bool
	cursorMode  iinput.CursorMode
}

// InputBackendForSystem returns a Backend implementation wired to this renderer's app
func (r *Renderer) InputBackendForSystem(sys *iinput.System) iinput.Backend {
	return &gogpuInputBackend{
		app:         r.app,
		sys:         sys,
		prevKeys:    make(map[ginput.Key]bool),
		prevButtons: make(map[int]bool),
	}
}

func (b *gogpuInputBackend) Init() error {
	// gogpu app already created by renderer; nothing to init here
	return nil
}

func (b *gogpuInputBackend) Shutdown() {
	// nothing to cleanup
}

func (b *gogpuInputBackend) PollEvents() bool {
	if b.app == nil || b.sys == nil {
		return true
	}

	state := b.app.Input()
	kbd := state.Keyboard()

	// Minimal key mapping: map common keys to engine keycodes.
	mappings := []struct {
		gk  ginput.Key
		our int
	}{
		{ginput.KeyW, int('w')},
		{ginput.KeyA, int('a')},
		{ginput.KeyS, int('s')},
		{ginput.KeyD, int('d')},
		{ginput.KeySpace, iinput.KSpace},
		{ginput.KeyEscape, iinput.KEscape},
		{ginput.KeyEnter, iinput.KEnter},
		{ginput.KeyTab, iinput.KTab},
		{ginput.KeyUp, iinput.KUpArrow},
		{ginput.KeyDown, iinput.KDownArrow},
		{ginput.KeyLeft, iinput.KLeftArrow},
		{ginput.KeyRight, iinput.KRightArrow},
	}

	for _, m := range mappings {
		pressed := kbd.Pressed(m.gk)
		prev := b.prevKeys[m.gk]
		if pressed != prev {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: m.our, Down: pressed, Device: iinput.DeviceKeyboard})
			b.prevKeys[m.gk] = pressed
		}
	}

	// Forward character events for printable keys
	shift := kbd.Modifier(ginput.ModShift)
	// Letters A-Z
	for k := ginput.KeyA; k <= ginput.KeyZ; k++ {
		if kbd.JustPressed(k) {
			ch := rune('a' + (k - ginput.KeyA))
			if shift {
				ch = rune('A' + (k - ginput.KeyA))
			}
			b.sys.HandleCharEvent(ch)
		}
	}
	// Numbers 0-9
	nums := []struct {
		key         ginput.Key
		ch, shiftch rune
	}{
		{ginput.Key0, '0', ')'}, {ginput.Key1, '1', '!'}, {ginput.Key2, '2', '@'}, {ginput.Key3, '3', '#'}, {ginput.Key4, '4', '$'},
		{ginput.Key5, '5', '%'}, {ginput.Key6, '6', '^'}, {ginput.Key7, '7', '&'}, {ginput.Key8, '8', '*'}, {ginput.Key9, '9', '('},
	}
	for _, n := range nums {
		if kbd.JustPressed(n.key) {
			if shift {
				b.sys.HandleCharEvent(n.shiftch)
			} else {
				b.sys.HandleCharEvent(n.ch)
			}
		}
	}
	// Common punctuation mapping (minimal)
	punct := []struct {
		key         ginput.Key
		ch, shiftch rune
	}{
		{ginput.KeyComma, ',', '<'}, {ginput.KeyPeriod, '.', '>'}, {ginput.KeySlash, '/', '?'},
		{ginput.KeySemicolon, ';', ':'}, {ginput.KeyApostrophe, '\'', '"'}, {ginput.KeyMinus, '-', '_'},
		{ginput.KeyEqual, '=', '+'}, {ginput.KeyLeftBracket, '[', '{'}, {ginput.KeyRightBracket, ']', '}'},
		{ginput.KeyBackslash, '\\', '|'}, {ginput.KeyGrave, '`', '~'},
	}
	for _, p := range punct {
		if kbd.JustPressed(p.key) {
			if shift {
				b.sys.HandleCharEvent(p.shiftch)
			} else {
				b.sys.HandleCharEvent(p.ch)
			}
		}
	}

	// Mouse buttons and wheel
	m := state.Mouse()
	if m != nil {
		// Map common mouse buttons
		mouseMap := []struct {
			gbtn ginput.MouseButton
			our  int
		}{
			{ginput.MouseButtonLeft, iinput.KMouse1},
			{ginput.MouseButtonRight, iinput.KMouse2},
			{ginput.MouseButtonMiddle, iinput.KMouse3},
		}
		for _, mm := range mouseMap {
			if m.JustPressed(mm.gbtn) {
				b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mm.our, Down: true, Device: iinput.DeviceMouse})
				b.prevButtons[int(mm.gbtn)] = true
			} else if m.JustReleased(mm.gbtn) {
				b.sys.HandleKeyEvent(iinput.KeyEvent{Key: mm.our, Down: false, Device: iinput.DeviceMouse})
				b.prevButtons[int(mm.gbtn)] = false
			}
		}

		// Scroll wheel
		_, sy := m.Scroll()
		if sy > 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelUp, Down: true, Device: iinput.DeviceMouse})
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelUp, Down: false, Device: iinput.DeviceMouse})
		} else if sy < 0 {
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelDown, Down: true, Device: iinput.DeviceMouse})
			b.sys.HandleKeyEvent(iinput.KeyEvent{Key: iinput.KMWheelDown, Down: false, Device: iinput.DeviceMouse})
		}
	}

	// Note: text input and gamepad handling can be added later.
	return true
}

func (b *gogpuInputBackend) GetMouseDelta() (dx, dy int32) {
	if b.app == nil {
		return 0, 0
	}
	m := b.app.Input().Mouse()
	if m == nil {
		return 0, 0
	}
	fx, fy := m.Delta()
	return int32(fx), int32(fy)
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
