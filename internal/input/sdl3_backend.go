//go:build sdl3
// +build sdl3

package input

import (
	"fmt"

	sdl "github.com/Zyko0/go-sdl3/sdl"
)

type sdl3Backend struct {
	sys         *System
	controllers []*sdl.Gamepad
	window      *sdl.Window
	mx, my      int32
	modifiers   ModifierState
}

// NewSDL3Backend returns a Backend that uses go-sdl3 for input.
func NewSDL3Backend(sys *System) Backend {
	return &sdl3Backend{sys: sys}
}

func (b *sdl3Backend) Init() (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("sdl init panic: %v", recovered)
		}
	}()

	if err := sdl.Init(sdl.INIT_JOYSTICK); err != nil {
		return fmt.Errorf("sdl init: %w", err)
	}
	// Joysticks are opened on SDL_JOYSTICK_ADDED events in PollEvents.
	return nil
}

func (b *sdl3Backend) Shutdown() {
	for _, c := range b.controllers {
		if c != nil {
			c.Close()
		}
	}
	sdl.QuitSubSystem(sdl.INIT_JOYSTICK)
}

func (b *sdl3Backend) PollEvents() bool {
	var event sdl.Event
	for sdl.PollEvent(&event) {
		switch event.Type {
		case sdl.EVENT_QUIT:
			return false
		case sdl.EVENT_KEY_DOWN, sdl.EVENT_KEY_UP:
			ke := event.KeyboardEvent()
			down := ke.Down
			sym := ke.Key
			var key int
			switch sym {
			case sdl.K_W:
				key = int('w')
			case sdl.K_A:
				key = int('a')
			case sdl.K_S:
				key = int('s')
			case sdl.K_D:
				key = int('d')
			case sdl.K_SPACE:
				key = KSpace
			case sdl.K_ESCAPE:
				key = KEscape
			case sdl.K_RETURN, sdl.K_KP_ENTER:
				key = KEnter
			case sdl.K_TAB:
				key = KTab
			case sdl.K_UP:
				key = KUpArrow
			case sdl.K_DOWN:
				key = KDownArrow
			case sdl.K_LEFT:
				key = KLeftArrow
			case sdl.K_RIGHT:
				key = KRightArrow
			case sdl.K_LSHIFT, sdl.K_RSHIFT:
				key = KShift
			case sdl.K_LCTRL, sdl.K_RCTRL:
				key = KCtrl
			case sdl.K_LALT, sdl.K_RALT:
				key = KAlt
			default:
				if sym >= sdl.K_A && sym <= sdl.K_Z {
					// map to lowercase ASCII
					key = int('a' + (sym - sdl.K_A))
				} else if sym >= sdl.K_0 && sym <= sdl.K_9 {
					key = int(sym)
				} else {
					key = 0
				}
			}
			if key != 0 {
				b.sys.HandleKeyEvent(KeyEvent{Key: key, Down: down, Device: DeviceKeyboard})
			}
			mods := sdl.GetModState()
			b.modifiers.Shift = (mods&sdl.KMOD_SHIFT != 0)
			b.modifiers.Ctrl = (mods&sdl.KMOD_CTRL != 0)
			b.modifiers.Alt = (mods&sdl.KMOD_ALT != 0)

		case sdl.EVENT_MOUSE_MOTION:
			me := event.MouseMotionEvent()
			b.mx += int32(me.Xrel)
			b.my += int32(me.Yrel)
		case sdl.EVENT_MOUSE_BUTTON_DOWN, sdl.EVENT_MOUSE_BUTTON_UP:
			mbe := event.MouseButtonEvent()
			down := mbe.Down
			var k int
			switch mbe.Button {
			case 1:
				k = KMouse1
			case 3:
				k = KMouse2
			case 2:
				k = KMouse3
			case 4:
				k = KMouse4
			case 5:
				k = KMouse5
			}
			if k != 0 {
				b.sys.HandleKeyEvent(KeyEvent{Key: k, Down: down, Device: DeviceMouse})
			}
		case sdl.EVENT_MOUSE_WHEEL:
			we := event.MouseWheelEvent()
			if we.Y > 0 {
				b.sys.HandleKeyEvent(KeyEvent{Key: KMWheelUp, Down: true, Device: DeviceMouse})
				b.sys.HandleKeyEvent(KeyEvent{Key: KMWheelUp, Down: false, Device: DeviceMouse})
			} else if we.Y < 0 {
				b.sys.HandleKeyEvent(KeyEvent{Key: KMWheelDown, Down: true, Device: DeviceMouse})
				b.sys.HandleKeyEvent(KeyEvent{Key: KMWheelDown, Down: false, Device: DeviceMouse})
			}
		case sdl.EVENT_TEXT_INPUT:
			te := event.TextInputEvent()
			for _, r := range te.Text {
				b.sys.HandleCharEvent(r)
			}
		case sdl.EVENT_JOYSTICK_ADDED:
			evt := event.JoyDeviceEvent()
			// Prefer the gamepad interface when available
			if evt.Which.IsGamepad() {
				if gp, err := evt.Which.OpenGamepad(); err == nil {
					b.controllers = append(b.controllers, gp)
				}
			}
		case sdl.EVENT_GAMEPAD_ADDED:
			// GamepadDeviceEvent provides Which ID; attempt to open gamepad
			gde := event.GamepadDeviceEvent()
			if gde.Which.IsGamepad() {
				if gp, err := gde.Which.OpenGamepad(); err == nil {
					b.controllers = append(b.controllers, gp)
				}
			}
		case sdl.EVENT_GAMEPAD_REMOVED, sdl.EVENT_JOYSTICK_REMOVED:
			// Remove any controllers matching the instance id
			// Event types differ; try to obtain an id from known event helpers
			rid := int32(-1)
			if event.Type == sdl.EVENT_GAMEPAD_REMOVED {
				gde := event.GamepadDeviceEvent()
				rid = int32(gde.Which)
			} else if event.Type == sdl.EVENT_JOYSTICK_REMOVED {
				jde := event.JoyDeviceEvent()
				rid = int32(jde.Which)
			}
			if rid >= 0 {
				// Close and nil out matching controllers (best-effort)
				for i, gp := range b.controllers {
					if gp == nil {
						continue
					}
					// There's no direct instance ID accessor on Gamepad, so best-effort: close all nil/unknown
					// Keep simple approach: if index matches instance id, close it.
					if int32(i) == rid {
						gp.Close()
						b.controllers[i] = nil
					}
				}
			}
		}
	}
	return true
}

func (b *sdl3Backend) GetMouseDelta() (int32, int32) {
	dx, dy := b.mx, b.my
	b.mx, b.my = 0, 0
	return dx, dy
}

func (b *sdl3Backend) GetModifierState() ModifierState { return b.modifiers }

func (b *sdl3Backend) SetTextMode(mode TextMode) {}

func (b *sdl3Backend) SetCursorMode(mode CursorMode) {}

func (b *sdl3Backend) ShowKeyboard(show bool) {}

func (b *sdl3Backend) SetMouseGrab(grabbed bool) {
	if b.window != nil {
		_ = b.window.SetRelativeMouseMode(grabbed)
		return
	}
	// No window available: no-op. The renderer may call SetWindow when it
	// has created a window to enable proper relative mouse mode.
}

// SetWindow attaches an SDL Window to the backend so mouse grabbing
// (relative mode) can be enabled/disabled. The renderer should call
// this if it creates an SDL window.
func (b *sdl3Backend) SetWindow(win interface{}) {
	if win == nil {
		b.window = nil
		return
	}
	if w, ok := win.(*sdl.Window); ok {
		b.window = w
	}
}

func (b *sdl3Backend) GetGamepadState(player int) GamepadState {
	var gs GamepadState
	if player < 0 || player >= len(b.controllers) {
		return gs
	}
	c := b.controllers[player]
	if c == nil {
		return gs
	}
	// Use the Gamepad API provided by go-sdl3.
	// Axes
	lx := c.Axis(sdl.GAMEPAD_AXIS_LEFTX)
	ly := c.Axis(sdl.GAMEPAD_AXIS_LEFTY)
	rx := c.Axis(sdl.GAMEPAD_AXIS_RIGHTX)
	ry := c.Axis(sdl.GAMEPAD_AXIS_RIGHTY)
	lt := c.Axis(sdl.GAMEPAD_AXIS_LEFT_TRIGGER)
	rt := c.Axis(sdl.GAMEPAD_AXIS_RIGHT_TRIGGER)

	norm := func(v int16) float32 { return float32(v) / 32767.0 }
	gs.LeftX = norm(lx)
	gs.LeftY = norm(ly)
	gs.RightX = norm(rx)
	gs.RightY = norm(ry)
	gs.LeftTrigger = (float32(lt) + 32768.0) / 65535.0
	gs.RightTrigger = (float32(rt) + 32768.0) / 65535.0

	var buttons uint32
	if c.Button(sdl.GAMEPAD_BUTTON_SOUTH) {
		buttons |= 1 << 0
	}
	if c.Button(sdl.GAMEPAD_BUTTON_EAST) {
		buttons |= 1 << 1
	}
	if c.Button(sdl.GAMEPAD_BUTTON_WEST) {
		buttons |= 1 << 2
	}
	if c.Button(sdl.GAMEPAD_BUTTON_NORTH) {
		buttons |= 1 << 3
	}
	if c.Button(sdl.GAMEPAD_BUTTON_START) {
		buttons |= 1 << 4
	}
	gs.Buttons = buttons
	return gs
}

func (b *sdl3Backend) IsGamepadConnected(player int) bool {
	if player < 0 || player >= len(b.controllers) {
		return false
	}
	return b.controllers[player] != nil
}
