// Package input provides cross-platform input handling for Quake.
//
// This file implements key binding and key-state management methods.
package input

import (
	"strconv"
	"strings"
)

type namedKey struct {
	name string
	key  int
}

var namedKeys = []namedKey{
	{name: "TAB", key: KTab},
	{name: "ENTER", key: KEnter},
	{name: "ESCAPE", key: KEscape},
	{name: "SPACE", key: KSpace},
	{name: "BACKSPACE", key: KBackspace},
	{name: "UPARROW", key: KUpArrow},
	{name: "DOWNARROW", key: KDownArrow},
	{name: "LEFTARROW", key: KLeftArrow},
	{name: "RIGHTARROW", key: KRightArrow},
	{name: "ALT", key: KAlt},
	{name: "CTRL", key: KCtrl},
	{name: "SHIFT", key: KShift},
	{name: "INS", key: KIns},
	{name: "DEL", key: KDel},
	{name: "PGDN", key: KPgDn},
	{name: "PGUP", key: KPgUp},
	{name: "HOME", key: KHome},
	{name: "END", key: KEnd},
	{name: "KP_NUMLOCK", key: KKpNumLock},
	{name: "KP_SLASH", key: KKpSlash},
	{name: "KP_STAR", key: KKpStar},
	{name: "KP_MINUS", key: KKpMinus},
	{name: "KP_HOME", key: KKpHome},
	{name: "KP_UPARROW", key: KKpUpArrow},
	{name: "KP_PGUP", key: KKpPgUp},
	{name: "KP_PLUS", key: KKpPlus},
	{name: "KP_LEFTARROW", key: KKpLeftArrow},
	{name: "KP_5", key: KKp5},
	{name: "KP_RIGHTARROW", key: KKpRightArrow},
	{name: "KP_END", key: KKpEnd},
	{name: "KP_DOWNARROW", key: KKpDownArrow},
	{name: "KP_PGDN", key: KKpPgDn},
	{name: "KP_ENTER", key: KKpEnter},
	{name: "KP_INS", key: KKpIns},
	{name: "KP_DEL", key: KKpDel},
	{name: "COMMAND", key: KCommand},
	{name: "CAPSLOCK", key: KCapsLock},
	{name: "SCROLLLOCK", key: KScrollLock},
	{name: "NUMLOCK", key: KKpNumLock},
	{name: "PRINTSCREEN", key: KPrintScreen},
	{name: "MOUSE1", key: KMouse1},
	{name: "MOUSE2", key: KMouse2},
	{name: "MOUSE3", key: KMouse3},
	{name: "MOUSE4", key: KMouse4},
	{name: "MOUSE5", key: KMouse5},
	{name: "PAUSE", key: KPause},
	{name: "MWHEELUP", key: KMWheelUp},
	{name: "MWHEELDOWN", key: KMWheelDown},
	{name: "SEMICOLON", key: int(';')},
	{name: "BACKQUOTE", key: int('`')},
	{name: "TILDE", key: int('~')},
	{name: "LTHUMB", key: KLThumb},
	{name: "RTHUMB", key: KRThumb},
	{name: "LSHOULDER", key: KLShoulder},
	{name: "RSHOULDER", key: KRShoulder},
	{name: "DPAD_UP", key: KDpadUp},
	{name: "DPAD_DOWN", key: KDpadDown},
	{name: "DPAD_LEFT", key: KDpadLeft},
	{name: "DPAD_RIGHT", key: KDpadRight},
	{name: "ABUTTON", key: KAButton},
	{name: "BBUTTON", key: KBButton},
	{name: "XBUTTON", key: KXButton},
	{name: "YBUTTON", key: KYButton},
	{name: "LTRIGGER", key: KLTrigger},
	{name: "RTRIGGER", key: KRTrigger},
	{name: "MISC1", key: KMisc1},
	{name: "PADDLE1", key: KPaddle1},
	{name: "PADDLE2", key: KPaddle2},
	{name: "PADDLE3", key: KPaddle3},
	{name: "PADDLE4", key: KPaddle4},
	{name: "TOUCHPAD", key: KTouchpad},
	{name: "LTHUMB_ALT", key: KLThumbAlt},
	{name: "RTHUMB_ALT", key: KRThumbAlt},
	{name: "LSHOULDER_ALT", key: KLShoulderAlt},
	{name: "RSHOULDER_ALT", key: KRShoulderAlt},
	{name: "DPAD_UP_ALT", key: KDpadUpAlt},
	{name: "DPAD_DOWN_ALT", key: KDpadDownAlt},
	{name: "DPAD_LEFT_ALT", key: KDpadLeftAlt},
	{name: "DPAD_RIGHT_ALT", key: KDpadRightAlt},
	{name: "ABUTTON_ALT", key: KAButtonAlt},
	{name: "BBUTTON_ALT", key: KBButtonAlt},
	{name: "XBUTTON_ALT", key: KXButtonAlt},
	{name: "YBUTTON_ALT", key: KYButtonAlt},
	{name: "LTRIGGER_ALT", key: KLTriggerAlt},
	{name: "RTRIGGER_ALT", key: KRTriggerAlt},
	{name: "MISC1_ALT", key: KMisc1Alt},
	{name: "PADDLE1_ALT", key: KPaddle1Alt},
	{name: "PADDLE2_ALT", key: KPaddle2Alt},
	{name: "PADDLE3_ALT", key: KPaddle3Alt},
	{name: "PADDLE4_ALT", key: KPaddle4Alt},
	{name: "TOUCHPAD_ALT", key: KTouchpadAlt},
}

var keyToName = func() map[int]string {
	names := make(map[int]string, len(namedKeys))
	for _, entry := range namedKeys {
		names[entry.key] = entry.name
	}
	return names
}()

var nameToKey = func() map[string]int {
	names := make(map[string]int, len(namedKeys))
	for _, entry := range namedKeys {
		names[entry.name] = entry.key
	}
	return names
}()

// SetBinding associates a console command string with an engine key code.
// When the key is pressed in KeyGame mode, the command is submitted to the
// console system. Out-of-range keys are silently ignored.
func (s *System) SetBinding(key int, binding string) {
	if key >= 0 && key < NumKeycode {
		s.bindings[key] = binding
	}
}

// GetBinding returns the console command string bound to the given key code,
// or "" if the key has no binding.
func (s *System) GetBinding(key int) string {
	if key >= 0 && key < NumKeycode {
		return s.bindings[key]
	}
	return ""
}

// IsKeyDown returns true if the given engine key code is currently held down.
// This queries the real-time state array, not a one-shot event.
func (s *System) IsKeyDown(key int) bool {
	if key >= 0 && key < NumKeycode {
		return s.state.Keys[key]
	}
	return false
}

// AnyKeyDown returns true if at least one key (of any device) is currently
// pressed. Used by "press any key to continue" prompts.
func (s *System) AnyKeyDown() bool {
	for _, down := range s.state.Keys {
		if down {
			return true
		}
	}
	return false
}

// ClearKeyStates resets every key to the "up" state. This is called when
// changing video modes or loading a new level to prevent stuck keys caused
// by a release event being missed during the transition.
func (s *System) ClearKeyStates() {
	for i := range s.state.Keys {
		s.state.Keys[i] = false
	}
}

// HandleKeyEvent is the central routing function for key events. It:
//
//  1. Deduplicates: suppresses repeated key-down events in KeyGame mode
//     (Quake only cares about the initial press, not OS key-repeat).
//  2. Ignores spurious key-up events for keys that aren't currently down
//     (can happen after focus changes).
//  3. Updates the key-state array (state.Keys).
//  4. Tracks modifier flags (Shift, Ctrl, Alt).
//  5. Dispatches to the appropriate callback based on KeyDest — in menu mode
//     the event goes to OnMenuKey first and then OnKey only if the menu
//     handler kept the destination in KeyMenu; in all other modes it goes only
//     to OnKey.
func (s *System) HandleKeyEvent(event KeyEvent) {
	wasDown := false
	if event.Key >= 0 && event.Key < NumKeycode {
		wasDown = s.state.Keys[event.Key]
	}

	if event.Down {
		if wasDown && s.keyDest == KeyGame {
			return
		}
	} else if !wasDown {
		return
	}

	if event.Key >= 0 && event.Key < NumKeycode {
		s.state.Keys[event.Key] = event.Down
	}

	// Update modifier tracking
	switch event.Key {
	case KShift:
		s.modifiers.Shift = event.Down
	case KCtrl:
		s.modifiers.Ctrl = event.Down
	case KAlt:
		s.modifiers.Alt = event.Down
	}

	// Forward to appropriate callback based on key destination
	if s.keyDest == KeyMenu {
		// In menu mode, route to menu callback
		if s.OnMenuKey != nil {
			s.OnMenuKey(event)
		}
		// Still forward to the general callback if the menu handler kept the
		// event in menu mode.
		if s.keyDest == KeyMenu && s.OnKey != nil {
			s.OnKey(event)
		}
	} else {
		// Forward to general callback for game/console
		if s.OnKey != nil {
			s.OnKey(event)
		}
	}

}

// HandleCharEvent processes a text-input character. The rune is appended to
// the frame's character buffer (state.Chars) and dispatched to the
// appropriate callback. In menu mode both OnMenuChar and OnChar are called.
func (s *System) HandleCharEvent(char rune) {
	s.state.Chars = append(s.state.Chars, char)

	if s.keyDest == KeyMenu && s.OnMenuChar != nil {
		s.OnMenuChar(char)
	}

	if s.OnChar != nil {
		s.OnChar(char)
	}
}

// GetModifierState returns the current modifier-key state by querying the
// backend's platform-level modifier bitmask (e.g. SDL_GetModState). This is
// more reliable than tracking individual KShift/KCtrl/KAlt press events
// because it handles focus-loss edge cases.
func (s *System) GetModifierState() ModifierState {
	if s.backend == nil {
		return ModifierState{}
	}
	return s.backend.GetModifierState()
}

// SetCursorMode delegates cursor visibility/grab mode to the backend. During
// gameplay the cursor is grabbed (locked to window centre for relative mouse
// input); in menus/console it is released.
func (s *System) SetCursorMode(mode CursorMode) {
	if s.backend != nil {
		s.backend.SetCursorMode(mode)
	}
}

// ShowKeyboard shows or hides the platform's on-screen keyboard. This is
// primarily useful on mobile/touchscreen platforms where no physical keyboard
// is available.
func (s *System) ShowKeyboard(show bool) {
	if s.backend != nil {
		s.backend.ShowKeyboard(show)
	}
}

// GetGamepadState returns the fully processed state of the gamepad at the
// given player index. Analog values have deadzone and response-curve
// processing already applied. Returns a zero-value GamepadState if no
// backend is set or no gamepad is connected at that index.
func (s *System) GetGamepadState(player int) GamepadState {
	if s.backend == nil {
		return GamepadState{}
	}
	return s.backend.GetGamepadState(player)
}

// IsGamepadConnected returns true if a gamepad is present at the given player
// index. The engine uses this to decide whether to show gamepad-style
// button prompts in the UI.
func (s *System) IsGamepadConnected(player int) bool {
	if s.backend == nil {
		return false
	}
	return s.backend.IsGamepadConnected(player)
}

// SetMouseGrab enables or disables mouse grabbing (relative mode). When
// grabbed the cursor is hidden and locked to the window; mouse motion
// events report relative deltas instead of absolute positions. This is
// essential for first-person look control during gameplay.
func (s *System) SetMouseGrab(grabbed bool) {
	if s.backend != nil {
		s.backend.SetMouseGrab(grabbed)
	}
}

// KeyToString converts an engine key code to its human-readable name as used
// in Quake config files (e.g. 32 → "SPACE", KMouse1 → "MOUSE1"). These
// names are the same ones accepted by the "bind" console command. Returns ""
// for unknown or unmapped key codes.
func KeyToString(key int) string {
	if name, ok := keyToName[key]; ok {
		return name
	}

	// Function keys
	if key >= KF1 && key <= KF12 {
		switch key {
		case KF10:
			return "F10"
		case KF11:
			return "F11"
		case KF12:
			return "F12"
		default:
			return string([]byte{'F', byte('1' + key - KF1)})
		}
	}

	// ASCII printable
	if key >= 32 && key < 127 {
		return string([]byte{byte(key)})
	}

	return ""
}

// StringToKey converts a human-readable key name (as used in config files and
// the "bind" console command) back to an engine key code. The lookup is
// case-insensitive for named keys ("SPACE", "space", and "Space" all work).
// Single-character strings return the ASCII code directly (upper-case letters
// are folded to lower-case). Returns 0 for unrecognised names.
func StringToKey(name string) int {
	if len(name) == 0 {
		return 0
	}

	// Single character
	if len(name) == 1 {
		c := name[0]
		if c >= 'a' && c <= 'z' {
			return int(c)
		}
		if c >= 'A' && c <= 'Z' {
			return int(c - 'A' + 'a')
		}
		return int(c)
	}

	upper := strings.ToUpper(name)
	if key, ok := nameToKey[upper]; ok {
		return key
	}

	// Function keys F1-F12
	if len(upper) >= 2 && upper[0] == 'F' {
		number := 0
		for i := 1; i < len(upper); i++ {
			digit := upper[i]
			if digit < '0' || digit > '9' {
				number = 0
				break
			}
			number = number*10 + int(digit-'0')
		}
		if number >= 1 && number <= 12 {
			return KF1 + number - 1
		}
	}

	if numeric, err := strconv.Atoi(name); err == nil {
		return numeric
	}

	return 0
}
