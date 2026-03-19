// Package input provides cross-platform input handling for Quake.
//
// This file implements key binding and key-state management methods.
package input

import "strings"

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
	switch key {
	case KTab:
		return "TAB"
	case KEnter:
		return "ENTER"
	case KEscape:
		return "ESCAPE"
	case KSpace:
		return "SPACE"
	case KBackspace:
		return "BACKSPACE"
	case KUpArrow:
		return "UPARROW"
	case KDownArrow:
		return "DOWNARROW"
	case KLeftArrow:
		return "LEFTARROW"
	case KRightArrow:
		return "RIGHTARROW"
	case KAlt:
		return "ALT"
	case KCtrl:
		return "CTRL"
	case KShift:
		return "SHIFT"
	case KMouse1:
		return "MOUSE1"
	case KMouse2:
		return "MOUSE2"
	case KMouse3:
		return "MOUSE3"
	case KMouse4:
		return "MOUSE4"
	case KMouse5:
		return "MOUSE5"
	case KMWheelUp:
		return "MWHEELUP"
	case KMWheelDown:
		return "MWHEELDOWN"
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

	// Named keys (case insensitive)
	name = strings.ToUpper(name)
	switch name {
	case "TAB", "Tab":
		return KTab
	case "ENTER", "Enter":
		return KEnter
	case "ESCAPE", "Escape":
		return KEscape
	case "SPACE", "Space":
		return KSpace
	case "BACKSPACE", "Backspace":
		return KBackspace
	case "UPARROW", "UpArrow":
		return KUpArrow
	case "DOWNARROW", "DownArrow":
		return KDownArrow
	case "LEFTARROW", "LeftArrow":
		return KLeftArrow
	case "RIGHTARROW", "RightArrow":
		return KRightArrow
	case "ALT", "Alt":
		return KAlt
	case "CTRL", "Ctrl":
		return KCtrl
	case "SHIFT", "Shift":
		return KShift
	case "MOUSE1", "Mouse1":
		return KMouse1
	case "MOUSE2", "Mouse2":
		return KMouse2
	case "MOUSE3", "Mouse3":
		return KMouse3
	case "MOUSE4", "Mouse4":
		return KMouse4
	case "MOUSE5", "Mouse5":
		return KMouse5
	case "MWHEELUP", "MWheelUp":
		return KMWheelUp
	case "MWHEELDOWN", "MWheelDown":
		return KMWheelDown
	}

	// Function keys F1-F12
	if len(name) >= 2 && (name[0] == 'F' || name[0] == 'f') {
		number := 0
		for i := 1; i < len(name); i++ {
			digit := name[i]
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

	return 0
}
