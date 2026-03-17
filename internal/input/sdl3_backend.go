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

const (
	altGamepadOffset = KLThumbAlt - KLThumb

	gamepadTriggerThreshold = 0.5
	radiansToDegrees        = 180.0 / math.Pi

	gyroModeIgnored    = 0
	gyroModeEnables    = 1
	gyroModeDisables   = 2
	gyroModeInvertsDir = 3
)

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

var (
	sdl3CommandOnce sync.Once
	activeSDL3Input *sdl3Backend
)

type triggerState struct {
	left  bool
	right bool
}

type gyroCalibrationState struct {
	active  bool
	samples int
	sumX    float64
	sumY    float64
	sumZ    float64
}

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

func (b *sdl3Backend) IsGamepadConnected(player int) bool {
	if player < 0 || player >= len(b.controllers) {
		return false
	}
	return b.controllers[player] != nil
}

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

func (b *sdl3Backend) Rumble(lowFreq, highFreq uint16, durationMS uint32) error {
	for _, gp := range b.controllers {
		if gp == nil {
			continue
		}
		return gp.Rumble(lowFreq, highFreq, durationMS)
	}
	return nil
}

func (b *sdl3Backend) SetLED(r, g, bl uint8) error {
	for _, gp := range b.controllers {
		if gp == nil {
			continue
		}
		return gp.SetLED(r, g, bl)
	}
	return nil
}

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
