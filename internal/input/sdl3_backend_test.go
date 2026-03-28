//go:build sdl3
// +build sdl3

package input

import (
	"math"
	"strconv"
	"testing"

	sdl "github.com/Zyko0/go-sdl3/sdl"
	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func withSDLTextHooks(
	t *testing.T,
	start func(*sdl.Window) error,
	stop func(*sdl.Window) error,
	hasSupport func() bool,
) {
	t.Helper()
	oldStart := startTextInput
	oldStop := stopTextInput
	oldSupport := hasScreenKeyboardSupport
	startTextInput = start
	stopTextInput = stop
	hasScreenKeyboardSupport = hasSupport
	t.Cleanup(func() {
		startTextInput = oldStart
		stopTextInput = oldStop
		hasScreenKeyboardSupport = oldSupport
	})
}

// TestTransformKey tests gamepad key transformation.
// It ensures that certain keys are correctly shifted when an "alt" modifier is pressed, allowing for expanded gamepad bindings.
// Where in C: N/A (Modern engine extension)
func TestTransformKey(t *testing.T) {
	b := &sdl3Backend{}

	if got := b.transformKey(KLThumb); got != KLThumb {
		t.Fatalf("without alt modifier: got %d want %d", got, KLThumb)
	}

	b.altModifierPressed = true
	if got := b.transformKey(KLThumb); got != KLThumbAlt {
		t.Fatalf("with alt modifier KLThumb: got %d want %d", got, KLThumbAlt)
	}
	if got := b.transformKey(KRTrigger); got != KRTriggerAlt {
		t.Fatalf("with alt modifier KRTrigger: got %d want %d", got, KRTriggerAlt)
	}
	// Start/back are outside the transformable range and should not shift.
	if got := b.transformKey(KStart); got != KStart {
		t.Fatalf("with alt modifier KStart should not shift: got %d want %d", got, KStart)
	}
}

// TestTransformKeyRangeMatchesOffset verifies the range of transformable gamepad keys.
// It ensures that all keys between KLThumb and KTouchpad are correctly shifted by the defined offset.
// Where in C: N/A
func TestTransformKeyRangeMatchesOffset(t *testing.T) {
	b := &sdl3Backend{altModifierPressed: true}
	for key := KLThumb; key <= KTouchpad; key++ {
		got := b.transformKey(key)
		want := key + altGamepadOffset
		if got != want {
			t.Fatalf("key %d transformed to %d want %d", key, got, want)
		}
	}
}

// TestSDLButtonToKeyCoverage tests the mapping between SDL gamepad buttons and internal key codes.
// It ensures that all standard SDL buttons have a corresponding internal representation.
// Where in C: IN_SDL_Runtime_KeyEvent in in_sdl.c (modernized)
func TestSDLButtonToKeyCoverage(t *testing.T) {
	if got, want := len(sdlButtonToKey), int(sdl.GAMEPAD_BUTTON_COUNT); got != want {
		t.Fatalf("mapping entries = %d, want %d", got, want)
	}

	cases := []struct {
		button sdl.GamepadButton
		want   int
	}{
		{sdl.GAMEPAD_BUTTON_SOUTH, KAButton},
		{sdl.GAMEPAD_BUTTON_EAST, KBButton},
		{sdl.GAMEPAD_BUTTON_START, KStart},
		{sdl.GAMEPAD_BUTTON_DPAD_LEFT, KDpadLeft},
		{sdl.GAMEPAD_BUTTON_LEFT_SHOULDER, KLShoulder},
		{sdl.GAMEPAD_BUTTON_TOUCHPAD, KTouchpad},
		{sdl.GAMEPAD_BUTTON_GUIDE, 0},
		{sdl.GAMEPAD_BUTTON_MISC2, 0},
	}
	for _, tc := range cases {
		if got := sdlButtonToKey[tc.button]; got != tc.want {
			t.Fatalf("button %v mapped to %d, want %d", tc.button, got, tc.want)
		}
	}
}

// TestApplyGyroMode tests different gyroscope operational modes.
// It verifies that gyro input can be ignored, enabled/disabled by modifiers, or inverted.
// Where in C: N/A (Modern engine extension)
func TestApplyGyroMode(t *testing.T) {
	b := &sdl3Backend{}
	oldMode := gyroMode.String
	defer cvar.Set(gyroMode.Name, oldMode)

	cvar.Set(gyroMode.Name, strconv.Itoa(gyroModeIgnored))
	if yaw, pitch, active := b.applyGyroMode(1, 2); !active || yaw != 1 || pitch != 2 {
		t.Fatalf("ignored mode: yaw=%f pitch=%f active=%v", yaw, pitch, active)
	}

	cvar.Set(gyroMode.Name, strconv.Itoa(gyroModeEnables))
	if _, _, active := b.applyGyroMode(1, 2); active {
		t.Fatal("enables mode should be inactive when modifier is not held")
	}
	b.altModifierPressed = true
	if yaw, pitch, active := b.applyGyroMode(1, 2); !active || yaw != 1 || pitch != 2 {
		t.Fatalf("enables mode with modifier: yaw=%f pitch=%f active=%v", yaw, pitch, active)
	}

	cvar.Set(gyroMode.Name, strconv.Itoa(gyroModeDisables))
	if _, _, active := b.applyGyroMode(1, 2); active {
		t.Fatal("disables mode should be inactive when modifier is held")
	}
	b.altModifierPressed = false
	if _, _, active := b.applyGyroMode(1, 2); !active {
		t.Fatal("disables mode should be active when modifier is not held")
	}

	cvar.Set(gyroMode.Name, strconv.Itoa(gyroModeInvertsDir))
	b.altModifierPressed = true
	if yaw, pitch, active := b.applyGyroMode(1, -2); !active || yaw != -1 || pitch != 2 {
		t.Fatalf("invert mode with modifier: yaw=%f pitch=%f active=%v", yaw, pitch, active)
	}
}

// TestFilterGyroValue tests gyroscope noise filtering.
// It ensures that small gyro movements are filtered out while larger movements pass through.
// Where in C: N/A
func TestFilterGyroValue(t *testing.T) {
	if got := filterGyroValue(1, 2); math.Abs(float64(got-0.5)) > 1e-6 {
		t.Fatalf("filtered value = %f, want 0.5", got)
	}
	if got := filterGyroValue(-1, 2); math.Abs(float64(got+0.5)) > 1e-6 {
		t.Fatalf("filtered negative value = %f, want -0.5", got)
	}
	if got := filterGyroValue(3, 2); got != 3 {
		t.Fatalf("above threshold should pass through: got %f", got)
	}
}

// TestUpdateGyroAccumulatesDeltas tests gyroscope input integration.
// It providing smooth motion-based aiming by correctly accumulating angular velocity over time.
// Where in C: N/A (Modern engine extension)
func TestUpdateGyroAccumulatesDeltas(t *testing.T) {
	oldEnable := gyroEnable.String
	oldMode := gyroMode.String
	oldNoise := gyroNoiseThresh.String
	oldYaw := gyroYawSensitivity.String
	oldPitch := gyroPitchSensitivity.String
	oldTurn := gyroTurningAxis.String
	oldCalX := gyroCalibrationX.String
	oldCalY := gyroCalibrationY.String
	oldCalZ := gyroCalibrationZ.String
	defer func() {
		cvar.Set(gyroEnable.Name, oldEnable)
		cvar.Set(gyroMode.Name, oldMode)
		cvar.Set(gyroNoiseThresh.Name, oldNoise)
		cvar.Set(gyroYawSensitivity.Name, oldYaw)
		cvar.Set(gyroPitchSensitivity.Name, oldPitch)
		cvar.Set(gyroTurningAxis.Name, oldTurn)
		cvar.Set(gyroCalibrationX.Name, oldCalX)
		cvar.Set(gyroCalibrationY.Name, oldCalY)
		cvar.Set(gyroCalibrationZ.Name, oldCalZ)
	}()

	cvar.Set(gyroEnable.Name, "1")
	cvar.Set(gyroMode.Name, strconv.Itoa(gyroModeIgnored))
	cvar.Set(gyroNoiseThresh.Name, "0")
	cvar.Set(gyroYawSensitivity.Name, "1")
	cvar.Set(gyroPitchSensitivity.Name, "1")
	cvar.Set(gyroTurningAxis.Name, "0")
	cvar.Set(gyroCalibrationX.Name, "0")
	cvar.Set(gyroCalibrationY.Name, "0")
	cvar.Set(gyroCalibrationZ.Name, "0")

	b := &sdl3Backend{
		gyroLastTimestamp: make(map[sdl.JoystickID]uint64),
	}
	id := sdl.JoystickID(1)
	raw := [3]float32{1, 2, 3} // rad/s

	// First sample seeds timestamp, second sample integrates one second.
	b.updateGyro(id, raw, 1_000_000_000)
	b.updateGyro(id, raw, 2_000_000_000)

	if math.Abs(float64(b.gyroYawDelta-57.29578)) > 0.001 {
		t.Fatalf("gyro yaw delta = %f, want ~57.29578", b.gyroYawDelta)
	}
	if math.Abs(float64(b.gyroPitchDelta-114.59156)) > 0.001 {
		t.Fatalf("gyro pitch delta = %f, want ~114.59156", b.gyroPitchDelta)
	}
}

// TestApplyDeadzoneAndCurve tests stick deadzone and response curve logic.
// It improving controller feel by filtering out stick drift and providing non-linear sensitivity.
// Where in C: IN_Move or similar in in_sdl.c (extended)
func TestApplyDeadzoneAndCurve(t *testing.T) {
	if got := applyDeadzoneAndCurve(0.1, 0.2, 2.0); got != 0 {
		t.Fatalf("deadzone expected 0, got %f", got)
	}
	got := applyDeadzoneAndCurve(0.6, 0.2, 2.0)
	// ((0.6 - 0.2) / 0.8)^2 = 0.25
	if math.Abs(float64(got-0.25)) > 1e-6 {
		t.Fatalf("curve value = %f, want 0.25", got)
	}
	got = applyDeadzoneAndCurve(-0.6, 0.2, 2.0)
	if math.Abs(float64(got+0.25)) > 1e-6 {
		t.Fatalf("negative curve value = %f, want -0.25", got)
	}
}

// TestApplyTriggerDeadzone tests trigger deadzone logic.
// It ensuring triggers don't activate accidentally at low values.
// Where in C: N/A (Modern engine extension)
func TestApplyTriggerDeadzone(t *testing.T) {
	if got := applyTriggerDeadzone(0.02, 0.05); got != 0 {
		t.Fatalf("trigger below deadzone should be 0, got %f", got)
	}
	got := applyTriggerDeadzone(0.525, 0.05)
	// (0.525 - 0.05) / 0.95 = 0.5
	if math.Abs(float64(got-0.5)) > 1e-6 {
		t.Fatalf("trigger scaled value = %f, want 0.5", got)
	}
}

func TestMapSDLKeyboardKeyMapsPunctuationAndNavigation(t *testing.T) {
	tests := []struct {
		name     string
		scancode sdl.Scancode
		keycode  sdl.Keycode
		want     int
	}{
		{name: "grave", scancode: sdl.SCANCODE_GRAVE, keycode: sdl.K_GRAVE, want: int('`')},
		{name: "backslash", scancode: sdl.SCANCODE_BACKSLASH, keycode: sdl.K_BACKSLASH, want: int('\\')},
		{name: "apostrophe", scancode: sdl.SCANCODE_APOSTROPHE, keycode: sdl.K_APOSTROPHE, want: int('\'')},
		{name: "semicolon", scancode: sdl.SCANCODE_SEMICOLON, keycode: sdl.K_SEMICOLON, want: int(';')},
		{name: "minus", scancode: sdl.SCANCODE_MINUS, keycode: sdl.K_MINUS, want: int('-')},
		{name: "equals", scancode: sdl.SCANCODE_EQUALS, keycode: sdl.K_EQUALS, want: int('=')},
		{name: "left bracket", scancode: sdl.SCANCODE_LEFTBRACKET, keycode: sdl.K_LEFTBRACKET, want: int('[')},
		{name: "right bracket", scancode: sdl.SCANCODE_RIGHTBRACKET, keycode: sdl.K_RIGHTBRACKET, want: int(']')},
		{name: "slash", scancode: sdl.SCANCODE_SLASH, keycode: sdl.K_SLASH, want: int('/')},
		{name: "comma", scancode: sdl.SCANCODE_COMMA, keycode: sdl.K_COMMA, want: int(',')},
		{name: "period", scancode: sdl.SCANCODE_PERIOD, keycode: sdl.K_PERIOD, want: int('.')},
		{name: "insert", scancode: sdl.SCANCODE_INSERT, keycode: sdl.K_INSERT, want: KIns},
		{name: "home", scancode: sdl.SCANCODE_HOME, keycode: sdl.K_HOME, want: KHome},
		{name: "page up", scancode: sdl.SCANCODE_PAGEUP, keycode: sdl.K_PAGEUP, want: KPgUp},
		{name: "delete", scancode: sdl.SCANCODE_DELETE, keycode: sdl.K_DELETE, want: KDel},
		{name: "end", scancode: sdl.SCANCODE_END, keycode: sdl.K_END, want: KEnd},
		{name: "page down", scancode: sdl.SCANCODE_PAGEDOWN, keycode: sdl.K_PAGEDOWN, want: KPgDn},
		{name: "printscreen", scancode: sdl.SCANCODE_PRINTSCREEN, keycode: sdl.K_PRINTSCREEN, want: KPrintScreen},
		{name: "pause", scancode: sdl.SCANCODE_PAUSE, keycode: sdl.K_PAUSE, want: KPause},
	}

	for _, tc := range tests {
		if got := mapSDLKeyboardKey(tc.scancode, tc.keycode); got != tc.want {
			t.Fatalf("%s mapped to %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestSetTextModeWithoutWindowIsNoop(t *testing.T) {
	var starts, stops int
	withSDLTextHooks(t,
		func(*sdl.Window) error {
			starts++
			return nil
		},
		func(*sdl.Window) error {
			stops++
			return nil
		},
		func() bool { return true },
	)

	b := &sdl3Backend{}
	b.SetTextMode(TextModeOn)
	b.SetTextMode(TextModeNoPopup)
	b.SetTextMode(TextModeOff)

	if starts != 0 || stops != 0 {
		t.Fatalf("expected no text input calls without window, got starts=%d stops=%d", starts, stops)
	}
}

func TestSetTextModeTogglesSDLTextInput(t *testing.T) {
	var starts, stops int
	withSDLTextHooks(t,
		func(*sdl.Window) error {
			starts++
			return nil
		},
		func(*sdl.Window) error {
			stops++
			return nil
		},
		func() bool { return true },
	)

	b := &sdl3Backend{window: &sdl.Window{}}
	b.SetTextMode(TextModeOn)
	b.SetTextMode(TextModeNoPopup)
	b.SetTextMode(TextModeOff)

	if starts != 2 {
		t.Fatalf("start calls = %d, want 2", starts)
	}
	if stops != 1 {
		t.Fatalf("stop calls = %d, want 1", stops)
	}
}

func TestShowKeyboardUsesPlatformSupport(t *testing.T) {
	var starts, stops int
	withSDLTextHooks(t,
		func(*sdl.Window) error {
			starts++
			return nil
		},
		func(*sdl.Window) error {
			stops++
			return nil
		},
		func() bool { return false },
	)

	b := &sdl3Backend{window: &sdl.Window{}}
	b.ShowKeyboard(true)
	b.ShowKeyboard(false)
	if starts != 0 || stops != 0 {
		t.Fatalf("expected no calls without support, got starts=%d stops=%d", starts, stops)
	}

	withSDLTextHooks(t,
		func(*sdl.Window) error {
			starts++
			return nil
		},
		func(*sdl.Window) error {
			stops++
			return nil
		},
		func() bool { return true },
	)

	b.ShowKeyboard(true)
	b.ShowKeyboard(false)

	if starts != 1 {
		t.Fatalf("start calls with support = %d, want 1", starts)
	}
	if stops != 1 {
		t.Fatalf("stop calls with support = %d, want 1", stops)
	}
}
