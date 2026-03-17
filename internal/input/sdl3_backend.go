//go:build sdl3
// +build sdl3

package input

import (
	"fmt"
	"math"
	"strconv"
	"sync"

	sdl "github.com/Zyko0/go-sdl3/sdl"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
)

// SDL3 backend constants for gamepad processing.
//
// altGamepadOffset is the distance between a primary gamepad key code and its
// alternate-layer counterpart (e.g. KLThumb → KLThumbAlt). When the
// "+altmodifier" command is active, all transformable gamepad keys are shifted
// by this offset so they hit a different set of bindings.
//
// gamepadTriggerThreshold is the normalised [0,1] value above which an analog
// trigger is treated as a binary "pressed" button. This creates a crisp
// press/release transition from a continuous analog signal.
//
// radiansToDegrees converts gyroscope angular-velocity readings from the
// sensor's native rad/s units to the deg/s that the engine uses for
// sensitivity scaling and noise-threshold comparison.
//
// The gyroMode* constants define how the alt-modifier interacts with gyro
// aiming — see applyGyroMode for the full truth table.
const (
	altGamepadOffset = KLThumbAlt - KLThumb

	gamepadTriggerThreshold = 0.5
	radiansToDegrees        = 180.0 / math.Pi

	gyroModeIgnored    = 0
	gyroModeEnables    = 1
	gyroModeDisables   = 2
	gyroModeInvertsDir = 3
)

// Cvar-driven gamepad and gyroscope configuration.
//
// All of these cvars are registered with FlagArchive so their values are
// persisted to config.cfg automatically. They let the player tune deadzone
// sizes, response curves, gyro sensitivity, and gyro calibration offsets
// without recompiling.
//
// Gyro pipeline overview:
//
//  1. Raw sensor data arrives in rad/s via SDL_EVENT_GAMEPAD_SENSOR_UPDATE.
//  2. The values are converted to deg/s and have calibration offsets subtracted
//     (gyro_calibration_{x,y,z}).
//  3. A noise gate (gyro_noise_thresh) attenuates small movements to reject
//     controller vibration and hand jitter.
//  4. The user's chosen turning axis (gyro_turning_axis: pitch-X or roll-Z)
//     is selected for yaw control.
//  5. Sensitivity multipliers (gyro_yawsensitivity, gyro_pitchsensitivity)
//     are applied.
//  6. The gyro mode (gyro_mode) determines whether the alt-modifier button
//     enables, disables, inverts, or has no effect on gyro output.
//  7. The final yaw/pitch deltas are integrated over the sensor timestamp
//     interval and accumulated into gyroYawDelta / gyroPitchDelta, which the
//     game reads via GetGamepadState each frame.
//
// Stick deadzone and curve cvars follow a similar pattern: the raw ±32767
// axis value is normalised to [-1,1], the deadzone is subtracted and the
// remaining range is re-normalised, then raised to an exponent for a
// non-linear response curve (exponent > 1 = slower near centre, faster at
// extremes).
var (
	gyroEnable = cvar.Register("gyro_enable", "1", cvar.FlagArchive, "Enable gamepad gyro input")
	gyroMode   = cvar.Register("gyro_mode", "2", cvar.FlagArchive, "Gyro mode: 0=ignore modifier, 1=modifier enables, 2=modifier disables, 3=modifier inverts")

	gyroYawSensitivity   = cvar.Register("gyro_yawsensitivity", "2.5", cvar.FlagArchive, "Gyro yaw sensitivity")
	gyroPitchSensitivity = cvar.Register("gyro_pitchsensitivity", "2.5", cvar.FlagArchive, "Gyro pitch sensitivity")
	gyroNoiseThresh      = cvar.Register("gyro_noise_thresh", "1.5", cvar.FlagArchive, "Gyro noise threshold in degrees/s")
	gyroTurningAxis      = cvar.Register("gyro_turning_axis", "0", cvar.FlagArchive, "Gyro turning axis: 0=pitch(X), 1=roll(Z)")
	gyroCalibrationX     = cvar.Register("gyro_calibration_x", "0", cvar.FlagArchive, "Gyro calibration X offset")
	gyroCalibrationY     = cvar.Register("gyro_calibration_y", "0", cvar.FlagArchive, "Gyro calibration Y offset")
	gyroCalibrationZ     = cvar.Register("gyro_calibration_z", "0", cvar.FlagArchive, "Gyro calibration Z offset")

	joyDeadzoneLook    = cvar.Register("joy_deadzone_look", "0.175", cvar.FlagArchive, "Deadzone for right stick look")
	joyDeadzoneMove    = cvar.Register("joy_deadzone_move", "0.175", cvar.FlagArchive, "Deadzone for left stick movement")
	joyDeadzoneTrigger = cvar.Register("joy_deadzone_trigger", "0.05", cvar.FlagArchive, "Deadzone for analog triggers")
	joyExponent        = cvar.Register("joy_exponent", "2.0", cvar.FlagArchive, "Exponential response curve for look stick")
	joyExponentMove    = cvar.Register("joy_exponent_move", "2.0", cvar.FlagArchive, "Exponential response curve for move stick")
)

// sdlButtonToKey maps SDL3 gamepad button constants to engine key codes.
//
// The mapping follows the Xbox-style "ABXY" layout that SDL3's gamepad API
// uses as its canonical model. SDL internally handles controller-specific
// remapping (e.g. DualSense ×/○ → A/B) via its game controller database, so
// this table only needs to express the logical mapping once. Unmapped buttons
// (GUIDE, MISC2–6) are mapped to 0 and silently dropped by emitGamepadKeyEvent.
var sdlButtonToKey = map[sdl.GamepadButton]int{
	sdl.GAMEPAD_BUTTON_SOUTH:          KAButton,
	sdl.GAMEPAD_BUTTON_EAST:           KBButton,
	sdl.GAMEPAD_BUTTON_WEST:           KXButton,
	sdl.GAMEPAD_BUTTON_NORTH:          KYButton,
	sdl.GAMEPAD_BUTTON_BACK:           KBack,
	sdl.GAMEPAD_BUTTON_GUIDE:          0,
	sdl.GAMEPAD_BUTTON_START:          KStart,
	sdl.GAMEPAD_BUTTON_LEFT_STICK:     KLThumb,
	sdl.GAMEPAD_BUTTON_RIGHT_STICK:    KRThumb,
	sdl.GAMEPAD_BUTTON_LEFT_SHOULDER:  KLShoulder,
	sdl.GAMEPAD_BUTTON_RIGHT_SHOULDER: KRShoulder,
	sdl.GAMEPAD_BUTTON_DPAD_UP:        KDpadUp,
	sdl.GAMEPAD_BUTTON_DPAD_DOWN:      KDpadDown,
	sdl.GAMEPAD_BUTTON_DPAD_LEFT:      KDpadLeft,
	sdl.GAMEPAD_BUTTON_DPAD_RIGHT:     KDpadRight,
	sdl.GAMEPAD_BUTTON_MISC1:          KMisc1,
	sdl.GAMEPAD_BUTTON_RIGHT_PADDLE1:  KPaddle1,
	sdl.GAMEPAD_BUTTON_LEFT_PADDLE1:   KPaddle2,
	sdl.GAMEPAD_BUTTON_RIGHT_PADDLE2:  KPaddle3,
	sdl.GAMEPAD_BUTTON_LEFT_PADDLE2:   KPaddle4,
	sdl.GAMEPAD_BUTTON_TOUCHPAD:       KTouchpad,
	sdl.GAMEPAD_BUTTON_MISC2:          0,
	sdl.GAMEPAD_BUTTON_MISC3:          0,
	sdl.GAMEPAD_BUTTON_MISC4:          0,
	sdl.GAMEPAD_BUTTON_MISC5:          0,
	sdl.GAMEPAD_BUTTON_MISC6:          0,
}

// Module-level singleton state for the SDL3 backend.
//
// sdl3CommandOnce ensures that console commands (+altmodifier, gyro_calibrate,
// etc.) are registered exactly once, even if the backend is re-initialised.
// activeSDL3Input points to the currently active backend instance so that the
// global console-command callbacks can reach it.
var (
	sdl3CommandOnce sync.Once
	activeSDL3Input *sdl3Backend
)

// triggerState tracks the digital (pressed/released) interpretation of each
// controller's analog triggers. Because triggers are continuous [0,1] axes
// but the engine's binding system expects discrete button events, we must
// detect threshold crossings and emit synthetic press/release key events.
// Each controller (identified by JoystickID) has its own triggerState to
// avoid cross-talk in multi-controller setups.
type triggerState struct {
	left  bool
	right bool
}

// gyroCalibrationState accumulates gyroscope sensor samples during an active
// calibration run. The "gyro_calibrate" console command sets active=true and
// the backend then collects 300 samples (≈5 seconds at 60 Hz). Once enough
// samples are gathered, the mean offsets are stored in the gyro_calibration_*
// cvars and the struct is reset. Calibration should be performed with the
// controller resting motionless on a flat surface.
type gyroCalibrationState struct {
	active  bool
	samples int
	sumX    float64
	sumY    float64
	sumZ    float64
}

// sdl3Backend implements the Backend interface using go-sdl3 (pure-Go SDL3
// bindings — no CGO required for the input layer itself).
//
// It is responsible for the Platform and Translation layers of the input
// pipeline: it polls SDL for raw events (keyboard, mouse, gamepad, gyro) and
// translates them into engine key codes before forwarding them to the System.
//
// State tracking:
//   - mx, my:               Accumulated mouse-motion deltas for this frame.
//   - modifiers:            Current Shift/Ctrl/Alt state from SDL_GetModState.
//   - altModifierPressed:   True when "+altmodifier" is active (gamepad layer).
//   - triggerStates:        Per-controller digital trigger state.
//   - gyroLastTimestamp:    Per-controller last gyro sensor timestamp (ns).
//   - gyroYawDelta/PitchDelta: Accumulated gyro rotation for this frame.
//   - calibration:          Active gyro calibration run (if any).
type sdl3Backend struct {
	sys         *System
	controllers []*sdl.Gamepad
	window      *sdl.Window
	mx, my      int32
	modifiers   ModifierState

	altModifierPressed bool
	triggerStates      map[sdl.JoystickID]triggerState
	gyroLastTimestamp  map[sdl.JoystickID]uint64
	gyroYawDelta       float32
	gyroPitchDelta     float32
	calibration        gyroCalibrationState
}

// NewSDL3Backend returns a Backend that uses go-sdl3 for input.
func NewSDL3Backend(sys *System) Backend {
	return &sdl3Backend{sys: sys}
}

// Init initialises the SDL3 joystick subsystem and prepares internal state
// maps. A deferred recover is used to catch panics from the SDL bindings
// (which can occur if the native library is missing or ABI-incompatible)
// and convert them to a regular error.
//
// Actual gamepad devices are not opened here — they are discovered and opened
// dynamically when SDL fires EVENT_JOYSTICK_ADDED or EVENT_GAMEPAD_ADDED
// events in PollEvents, which handles hot-plug correctly.
func (b *sdl3Backend) Init() (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("sdl init panic: %v", recovered)
		}
	}()

	if err := sdl.Init(sdl.INIT_JOYSTICK); err != nil {
		return fmt.Errorf("sdl init: %w", err)
	}
	if b.triggerStates == nil {
		b.triggerStates = make(map[sdl.JoystickID]triggerState)
	}
	if b.gyroLastTimestamp == nil {
		b.gyroLastTimestamp = make(map[sdl.JoystickID]uint64)
	}
	activeSDL3Input = b
	b.registerCommands()
	// Joysticks are opened on SDL_JOYSTICK_ADDED events in PollEvents.
	return nil
}

// Shutdown closes all open gamepad handles, clears the singleton pointer, and
// tears down the SDL joystick subsystem. After Shutdown the backend must not
// be used.
func (b *sdl3Backend) Shutdown() {
	for _, c := range b.controllers {
		if c != nil {
			c.Close()
		}
	}
	if activeSDL3Input == b {
		activeSDL3Input = nil
	}
	sdl.QuitSubSystem(sdl.INIT_JOYSTICK)
}

// PollEvents drains the SDL event queue and translates each event into engine
// input. This is the heart of the Platform → Translation layer. The method
// handles the following event types:
//
//   - EVENT_QUIT: Returns false to signal application exit.
//   - EVENT_KEY_DOWN / EVENT_KEY_UP: Maps SDL keycodes to engine K_* codes.
//     Letters are folded to lowercase ASCII; common navigation and modifier
//     keys are mapped explicitly. Unmapped keys are dropped (key == 0).
//   - EVENT_MOUSE_MOTION: Accumulates relative pixel deltas into (mx, my).
//   - EVENT_MOUSE_BUTTON_DOWN/UP: Maps SDL button indices (1=left, 2=middle,
//     3=right, 4/5=side) to KMouse1–KMouse5.
//   - EVENT_MOUSE_WHEEL: Emits an instantaneous press+release pair for
//     KMWheelUp or KMWheelDown so scroll can be bound like a button.
//   - EVENT_TEXT_INPUT: Forwards composed Unicode runes to HandleCharEvent for
//     console/chat text entry.
//   - EVENT_JOYSTICK_ADDED / EVENT_GAMEPAD_ADDED: Opens the new controller
//     via the Gamepad API and enables the gyroscope sensor if available.
//   - EVENT_GAMEPAD_BUTTON_DOWN/UP: Looks up the engine key in sdlButtonToKey
//     and emits via emitGamepadKeyEvent (which applies the alt-modifier layer).
//   - EVENT_GAMEPAD_AXIS_MOTION: Converts analog trigger axes to digital
//     press/release events when they cross gamepadTriggerThreshold. Stick
//     axes are read on-demand in GetGamepadState instead.
//   - EVENT_GAMEPAD_SENSOR_UPDATE: Routes gyroscope data to updateGyro.
//   - EVENT_GAMEPAD_REMOVED / EVENT_JOYSTICK_REMOVED: Cleans up controller
//     state and closes the Gamepad handle.
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
					if gp.HasSensor(sdl.SENSOR_GYRO) {
						_ = gp.SetSensorEnabled(sdl.SENSOR_GYRO, true)
					}
					b.controllers = append(b.controllers, gp)
				}
			}
		case sdl.EVENT_GAMEPAD_ADDED:
			// GamepadDeviceEvent provides Which ID; attempt to open gamepad
			gde := event.GamepadDeviceEvent()
			if gde.Which.IsGamepad() {
				if gp, err := gde.Which.OpenGamepad(); err == nil {
					if gp.HasSensor(sdl.SENSOR_GYRO) {
						_ = gp.SetSensorEnabled(sdl.SENSOR_GYRO, true)
					}
					b.controllers = append(b.controllers, gp)
				}
			}
		case sdl.EVENT_GAMEPAD_BUTTON_DOWN, sdl.EVENT_GAMEPAD_BUTTON_UP:
			gbe := event.GamepadButtonEvent()
			key := sdlButtonToKey[sdl.GamepadButton(gbe.Button)]
			if key != 0 {
				b.emitGamepadKeyEvent(key, gbe.Down)
			}
		case sdl.EVENT_GAMEPAD_AXIS_MOTION:
			gae := event.GamepadAxisEvent()
			state := b.triggerStates[gae.Which]
			value := float32(gae.Value) / 32767.0
			switch sdl.GamepadAxis(gae.Axis) {
			case sdl.GAMEPAD_AXIS_LEFT_TRIGGER:
				pressed := value >= gamepadTriggerThreshold
				if pressed != state.left {
					state.left = pressed
					b.triggerStates[gae.Which] = state
					b.emitGamepadKeyEvent(KLTrigger, pressed)
				}
			case sdl.GAMEPAD_AXIS_RIGHT_TRIGGER:
				pressed := value >= gamepadTriggerThreshold
				if pressed != state.right {
					state.right = pressed
					b.triggerStates[gae.Which] = state
					b.emitGamepadKeyEvent(KRTrigger, pressed)
				}
			}
		case sdl.EVENT_GAMEPAD_SENSOR_UPDATE:
			gse := event.GamepadSensorEvent()
			if sdl.SensorType(gse.Sensor) == sdl.SENSOR_GYRO {
				b.updateGyro(gse.Which, gse.Data, gse.SensorTimestamp)
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
				delete(b.triggerStates, sdl.JoystickID(rid))
				delete(b.gyroLastTimestamp, sdl.JoystickID(rid))
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

// GetMouseDelta returns the accumulated mouse movement since the last call and
// resets the accumulators to zero. The engine calls this exactly once per frame
// via System.GetState to ensure no motion is lost or double-counted.
func (b *sdl3Backend) GetMouseDelta() (int32, int32) {
	dx, dy := b.mx, b.my
	b.mx, b.my = 0, 0
	return dx, dy
}

// GetModifierState returns the cached modifier key state that was last updated
// during PollEvents from SDL_GetModState. This is more reliable than tracking
// individual KShift/KCtrl/KAlt events because it handles edge cases where a
// modifier is pressed or released while the window is unfocused.
func (b *sdl3Backend) GetModifierState() ModifierState { return b.modifiers }

// SetTextMode is a no-op in the current SDL3 backend. SDL's text input is
// implicitly enabled when the event loop receives TEXT_INPUT events.
// A full implementation would call sdl.StartTextInput / sdl.StopTextInput.
func (b *sdl3Backend) SetTextMode(mode TextMode) {}

// SetCursorMode is a no-op stub. Cursor visibility is managed through
// SetMouseGrab and the renderer's own cursor handling.
func (b *sdl3Backend) SetCursorMode(mode CursorMode) {}

// ShowKeyboard is a no-op on desktop platforms. On mobile it would call
// SDL_StartTextInput to raise the virtual keyboard.
func (b *sdl3Backend) ShowKeyboard(show bool) {}

// SetMouseGrab enables or disables SDL relative mouse mode on the attached
// window. In relative mode the cursor is hidden and SDL reports motion as
// deltas from the window centre, which is what first-person look control
// needs. If no window has been attached yet (SetWindow not called), this is
// a no-op — the renderer is expected to call SetWindow once its window is
// ready.
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

// GetGamepadState returns the fully-processed state of the gamepad at the
// given player index. This method performs the following processing pipeline:
//
//  1. Reads raw axis values from the SDL Gamepad API (int16 range ±32767).
//  2. Normalises each axis to [-1,1] (sticks) or [0,1] (triggers).
//  3. Applies cvar-driven deadzone removal and exponential response curves
//     via applyDeadzoneAndCurve / applyTriggerDeadzone. The move stick and
//     look stick use separate deadzone and exponent cvars so the player can
//     tune them independently.
//  4. Reads button state into a bitmask (bits 0–4 = South/East/West/North/Start).
//  5. Copies the accumulated gyro yaw/pitch deltas and resets the accumulators
//     so the game receives exactly one frame's worth of gyro rotation.
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
	gs.LeftX = applyDeadzoneAndCurve(norm(lx), joyDeadzoneMove.Float32(), joyExponentMove.Float32())
	gs.LeftY = applyDeadzoneAndCurve(norm(ly), joyDeadzoneMove.Float32(), joyExponentMove.Float32())
	gs.RightX = applyDeadzoneAndCurve(norm(rx), joyDeadzoneLook.Float32(), joyExponent.Float32())
	gs.RightY = applyDeadzoneAndCurve(norm(ry), joyDeadzoneLook.Float32(), joyExponent.Float32())
	gs.LeftTrigger = applyTriggerDeadzone((float32(lt)+32768.0)/65535.0, joyDeadzoneTrigger.Float32())
	gs.RightTrigger = applyTriggerDeadzone((float32(rt)+32768.0)/65535.0, joyDeadzoneTrigger.Float32())

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
	gs.GyroYawDelta = b.gyroYawDelta
	gs.GyroPitchDelta = b.gyroPitchDelta
	b.gyroYawDelta = 0
	b.gyroPitchDelta = 0
	return gs
}

// IsGamepadConnected returns true if a gamepad is present and open at the
// given player index.
func (b *sdl3Backend) IsGamepadConnected(player int) bool {
	if player < 0 || player >= len(b.controllers) {
		return false
	}
	return b.controllers[player] != nil
}

// emitGamepadKeyEvent applies the alt-modifier layer transformation (via
// transformKey) and then forwards the resulting key event to the System's
// HandleKeyEvent. If the transformed key is 0 (unmapped) the event is dropped.
func (b *sdl3Backend) emitGamepadKeyEvent(key int, down bool) {
	if b.sys == nil {
		return
	}
	key = b.transformKey(key)
	if key == 0 {
		return
	}
	b.sys.HandleKeyEvent(KeyEvent{Key: key, Down: down, Device: DeviceGamepad})
}

// transformKey applies Ironwail's alternate gamepad layer.
// When +altmodifier is held, gamepad keys in the transformable range are
// shifted by a fixed offset to their *_ALT variants.
func (b *sdl3Backend) transformKey(key int) int {
	if !b.altModifierPressed {
		return key
	}
	if key >= KLThumb && key < KLThumbAlt {
		return key + altGamepadOffset
	}
	return key
}

// applyDeadzoneAndCurve removes stick deadzone and applies an exponential
// response curve to a normalised stick axis value in [-1,1].
//
// Processing steps:
//  1. If |value| ≤ deadzone, return 0 (ignore tiny movements near centre).
//  2. Re-normalise the remaining range [deadzone,1] → [0,1].
//  3. Raise to the power of exponent. An exponent > 1 makes small deflections
//     more precise (good for aiming) while preserving full range at the
//     extremes. An exponent of 1 gives a linear response.
//  4. Restore the original sign.
func applyDeadzoneAndCurve(value, deadzone, exponent float32) float32 {
	absValue := float32(math.Abs(float64(value)))
	if absValue <= deadzone {
		return 0
	}

	rangeNorm := (absValue - deadzone) / (1 - deadzone)
	if rangeNorm < 0 {
		rangeNorm = 0
	}
	if rangeNorm > 1 {
		rangeNorm = 1
	}
	if exponent <= 0 {
		exponent = 1
	}
	curved := float32(math.Pow(float64(rangeNorm), float64(exponent)))
	if value < 0 {
		return -curved
	}
	return curved
}

// applyTriggerDeadzone removes the deadzone from a trigger value in [0,1] and
// re-normalises the result so that the trigger reports 0.0 until it passes
// the deadzone threshold, then linearly ramps to 1.0 at full depression.
// Unlike sticks, triggers are unipolar so no sign handling is needed.
func applyTriggerDeadzone(value, deadzone float32) float32 {
	if value <= deadzone {
		return 0
	}
	scaled := (value - deadzone) / (1 - deadzone)
	if scaled < 0 {
		scaled = 0
	}
	if scaled > 1 {
		scaled = 1
	}
	return scaled
}

// filterGyroValue implements a soft noise gate for gyroscope readings. Values
// below the threshold are attenuated proportionally to their magnitude
// (linear fade-out) rather than hard-clamped to zero. This avoids the
// "quantisation" feel of a hard threshold while still suppressing jitter when
// the controller is nearly stationary.
//
//	If |value| ≥ threshold: output = value (unmodified).
//	If |value| <  threshold: output = value × (|value| / threshold).
//
// A threshold of 0 disables filtering entirely.
func filterGyroValue(value, threshold float32) float32 {
	if threshold <= 0 {
		return value
	}
	absValue := float32(math.Abs(float64(value)))
	if absValue >= threshold {
		return value
	}
	return value * (absValue / threshold)
}

// applyGyroMode determines whether gyro input is active this frame and
// optionally modifies the yaw/pitch values based on the gyro_mode cvar and
// the alt-modifier button state. The four modes are:
//
//   - 0 (Ignored):    Alt-modifier has no effect on gyro — always active.
//   - 1 (Enables):    Gyro is only active while alt-modifier is held.
//   - 2 (Disables):   Gyro is active by default but disabled while alt-modifier
//     is held (e.g. for a "gyro pause" button).
//   - 3 (Inverts):    Gyro is always active but yaw/pitch are negated while
//     alt-modifier is held (useful for flick-stick combos).
//
// Returns the (possibly modified) yaw/pitch and whether the gyro is active.
func (b *sdl3Backend) applyGyroMode(yaw, pitch float32) (outYaw, outPitch float32, active bool) {
	switch gyroMode.Int {
	case gyroModeEnables:
		if !b.altModifierPressed {
			return 0, 0, false
		}
	case gyroModeDisables:
		if b.altModifierPressed {
			return 0, 0, false
		}
	case gyroModeInvertsDir:
		if b.altModifierPressed {
			yaw = -yaw
			pitch = -pitch
		}
	case gyroModeIgnored:
		// Always active.
	default:
		// Unknown mode: treat as ignored to avoid silently losing gyro.
	}
	return yaw, pitch, true
}

// updateGyro processes a single gyroscope sensor event from SDL3.
//
// The full pipeline for each event:
//
//  1. Convert raw sensor data from rad/s to deg/s and subtract calibration
//     offsets (gyro_calibration_{x,y,z} cvars).
//  2. If a calibration run is active, accumulate the sample and, once 300
//     samples are collected, compute and store the mean offsets.
//  3. If gyro_enable is false, discard the data entirely.
//  4. Apply the soft noise gate (filterGyroValue) with gyro_noise_thresh.
//  5. Select the yaw axis: by default X (pitch axis of the physical sensor),
//     but gyro_turning_axis=1 uses Z (roll axis) instead.
//  6. Multiply by sensitivity cvars.
//  7. Apply the gyro mode filter (applyGyroMode) — may zero or invert.
//  8. Integrate over the time delta (sensorTimestamp difference in nanoseconds)
//     and accumulate into gyroYawDelta / gyroPitchDelta.
func (b *sdl3Backend) updateGyro(id sdl.JoystickID, raw [3]float32, sensorTimestamp uint64) {
	// Gyro event data is in rad/s. Convert to deg/s for thresholding and scaling.
	x := raw[0]*radiansToDegrees - gyroCalibrationX.Float32()
	y := raw[1]*radiansToDegrees - gyroCalibrationY.Float32()
	z := raw[2]*radiansToDegrees - gyroCalibrationZ.Float32()

	if b.calibration.active {
		b.calibration.samples++
		b.calibration.sumX += float64(raw[0] * radiansToDegrees)
		b.calibration.sumY += float64(raw[1] * radiansToDegrees)
		b.calibration.sumZ += float64(raw[2] * radiansToDegrees)
		if b.calibration.samples >= 300 {
			n := float64(b.calibration.samples)
			cvar.SetFloat(gyroCalibrationX.Name, b.calibration.sumX/n)
			cvar.SetFloat(gyroCalibrationY.Name, b.calibration.sumY/n)
			cvar.SetFloat(gyroCalibrationZ.Name, b.calibration.sumZ/n)
			b.calibration = gyroCalibrationState{}
		}
	}

	if !gyroEnable.Bool() {
		return
	}

	threshold := gyroNoiseThresh.Float32()
	x = filterGyroValue(x, threshold)
	y = filterGyroValue(y, threshold)
	z = filterGyroValue(z, threshold)

	// SDL axis mapping:
	// X = pitch axis, Y = yaw axis, Z = roll axis.
	// Ironwail exposes turning axis selection:
	// 0 -> yaw from pitch axis (X), 1 -> yaw from roll axis (Z).
	yawRate := x
	if gyroTurningAxis.Int != 0 {
		yawRate = z
	}
	pitchRate := y

	yawRate *= gyroYawSensitivity.Float32()
	pitchRate *= gyroPitchSensitivity.Float32()

	yawRate, pitchRate, active := b.applyGyroMode(yawRate, pitchRate)
	if !active {
		return
	}

	last, ok := b.gyroLastTimestamp[id]
	b.gyroLastTimestamp[id] = sensorTimestamp
	if !ok || sensorTimestamp <= last {
		return
	}
	dt := float32(sensorTimestamp-last) / 1_000_000_000.0
	b.gyroYawDelta += yawRate * dt
	b.gyroPitchDelta += pitchRate * dt
}

// Rumble sends a haptic rumble command to the first connected gamepad. The
// low-frequency and high-frequency motor intensities are in the range 0–65535
// and the duration is in milliseconds. Only the first non-nil controller
// receives the command (Quake is single-player so only one gamepad matters).
func (b *sdl3Backend) Rumble(lowFreq, highFreq uint16, durationMS uint32) error {
	for _, gp := range b.controllers {
		if gp == nil {
			continue
		}
		return gp.Rumble(lowFreq, highFreq, durationMS)
	}
	return nil
}

// SetLED sets the RGB colour of the first connected gamepad's LED (e.g. the
// DualSense light bar). Values are 0–255 per channel.
func (b *sdl3Backend) SetLED(r, g, bl uint8) error {
	for _, gp := range b.controllers {
		if gp == nil {
			continue
		}
		return gp.SetLED(r, g, bl)
	}
	return nil
}

// registerCommands registers console commands for gamepad features. This is
// called once (guarded by sync.Once) during Init. The commands use the
// module-level activeSDL3Input pointer so they work even if the backend
// instance is replaced.
//
// Registered commands:
//   - +altmodifier / -altmodifier: Toggles the alternate gamepad binding layer.
//     Bound to a gamepad button, this lets one physical button double the
//     number of available bindings (see altGamepadOffset).
//   - gyro_calibrate: Starts a 300-sample calibration run to determine gyro
//     zero-point offsets.
//   - joy_rumble <low> <high> <duration_ms>: Triggers haptic feedback.
//   - joy_led <r> <g> <b>: Sets the controller's LED colour.
func (b *sdl3Backend) registerCommands() {
	sdl3CommandOnce.Do(func() {
		cmdsys.AddCommand("+altmodifier", func(args []string) {
			if activeSDL3Input != nil {
				activeSDL3Input.altModifierPressed = true
			}
		}, "Enable alternate gamepad button layer")
		cmdsys.AddCommand("-altmodifier", func(args []string) {
			if activeSDL3Input != nil {
				activeSDL3Input.altModifierPressed = false
			}
		}, "Disable alternate gamepad button layer")
		cmdsys.AddCommand("gyro_calibrate", func(args []string) {
			if activeSDL3Input != nil {
				activeSDL3Input.calibration = gyroCalibrationState{active: true}
			}
		}, "Capture 300 gyro samples and set calibration offsets")
		cmdsys.AddCommand("joy_rumble", func(args []string) {
			if activeSDL3Input == nil || len(args) < 4 {
				return
			}
			low, err1 := strconv.ParseUint(args[1], 10, 16)
			high, err2 := strconv.ParseUint(args[2], 10, 16)
			dur, err3 := strconv.ParseUint(args[3], 10, 32)
			if err1 != nil || err2 != nil || err3 != nil {
				return
			}
			_ = activeSDL3Input.Rumble(uint16(low), uint16(high), uint32(dur))
		}, "joy_rumble <low> <high> <duration_ms>")
		cmdsys.AddCommand("joy_led", func(args []string) {
			if activeSDL3Input == nil || len(args) < 4 {
				return
			}
			r, err1 := strconv.ParseUint(args[1], 10, 8)
			g, err2 := strconv.ParseUint(args[2], 10, 8)
			bl, err3 := strconv.ParseUint(args[3], 10, 8)
			if err1 != nil || err2 != nil || err3 != nil {
				return
			}
			_ = activeSDL3Input.SetLED(uint8(r), uint8(g), uint8(bl))
		}, "joy_led <r> <g> <b>")
	})
}
