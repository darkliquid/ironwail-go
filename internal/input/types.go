// Package input provides cross-platform input handling for Quake.
//
// This package abstracts keyboard, mouse, and gamepad input behind a
// platform-independent interface. It uses go-sdl3 as the primary backend
// (pure Go, no CGO required) but is designed to support alternative backends.
//
// Key Features:
//   - Quake-compatible key codes for game logic
//   - Key binding system
//   - Mouse movement accumulation
//   - Gamepad support with dead zones
//   - Text input mode for console/chat
package input

// Key codes compatible with Quake's key.h
// These are the key numbers passed to Key_Event
const (
	// ASCII keys (a-z, 0-9) use their ASCII values directly

	KTab    = 9
	KEnter  = 13
	KEscape = 27
	KSpace  = 32
)

const (
	// Backspace and special keys
	KBackspace = 127 + iota

	// Arrow keys
	KUpArrow
	KDownArrow
	KLeftArrow
	KRightArrow

	// Modifier keys
	KAlt
	KCtrl
	KShift

	// Function keys
	KF1
	KF2
	KF3
	KF4
	KF5
	KF6
	KF7
	KF8
	KF9
	KF10
	KF11
	KF12

	// Navigation keys
	KIns
	KDel
	KPgDn
	KPgUp
	KHome
	KEnd

	// Keypad keys
	KKpNumLock
	KKpSlash
	KKpStar
	KKpMinus
	KKpHome
	KKpUpArrow
	KKpPgUp
	KKpPlus
	KKpLeftArrow
	KKp5
	KKpRightArrow
	KKpEnd
	KKpDownArrow
	KKpPgDn
	KKpEnter
	KKpIns
	KKpDel

	// Platform keys
	KCommand

	// Lock keys
	KCapsLock
	KScrollLock
	KPrintScreen
)

const (
	// Mouse buttons (virtual keys)
	KMouseBegin = 200
)

const (
	KMouse1 = KMouseBegin + iota
	KMouse2
	KMouse3
	KMouse4 // Back button
	KMouse5 // Forward button
	KMWheelUp
	KMWheelDown
	KMouseEnd
)

const (
	// Gamepad keys
	KGamepadBegin = KMouseEnd
)

const (
	// Standard gamepad buttons
	KStart = KGamepadBegin + iota
	KBack
	KLThumb
	KRThumb
	KLShoulder
	KRShoulder
	KDpadUp
	KDpadDown
	KDpadLeft
	KDpadRight
	KAButton
	KBButton
	KXButton
	KYButton
	KLTrigger
	KRTrigger

	// Extended gamepad buttons
	KMisc1 // Mute/Capture button
	KPaddle1
	KPaddle2
	KPaddle3
	KPaddle4
	KTouchpad

	// Gamepad alt modifier buttons
	KLThumbAlt
	KRThumbAlt
	KLShoulderAlt
	KRShoulderAlt
	KDpadUpAlt
	KDpadDownAlt
	KDpadLeftAlt
	KDpadRightAlt
	KAButtonAlt
	KBButtonAlt
	KXButtonAlt
	KYButtonAlt
	KLTriggerAlt
	KRTriggerAlt
	KMisc1Alt
	KPaddle1Alt
	KPaddle2Alt
	KPaddle3Alt
	KPaddle4Alt
	KTouchpadAlt

	KGamepadEnd
)

const (
	// Pause key
	KPause = KGamepadEnd

	// Total number of keys
	NumKeycodes = KPause + 1
	NumKeycode  = NumKeycodes
)

// MaxKeys is the maximum number of key bindings.
const MaxKeys = 256

// KeyDest defines where key events are being sent.
type KeyDest int

const (
	KeyGame    KeyDest = iota // Send to game
	KeyConsole                // Send to console
	KeyMessage                // Send to message buffer (chat)
	KeyMenu                   // Send to menu
)

// TextMode defines text input state.
type TextMode int

const (
	TextModeOff     TextMode = iota // No char events
	TextModeOn                      // Char events, show on-screen keyboard
	TextModeNoPopup                 // Char events, don't show on-screen keyboard
)

// KeyDevice identifies the input device type.
type KeyDevice int

const (
	DeviceNone KeyDevice = iota - 1
	DeviceKeyboard
	DeviceMouse
	DeviceGamepad
)

// KeyEvent represents a key press or release event.
type KeyEvent struct {
	Key       int       // Key code (K_* constant)
	Down      bool      // True if pressed, false if released
	Device    KeyDevice // Which device generated this event
	Character rune      // Unicode character if this is a text event
}

// MouseEvent represents mouse movement or button event.
type MouseEvent struct {
	X, Y    int32  // Relative movement
	Wheel   int32  // Scroll wheel delta
	Buttons uint32 // Current button state bitmask
}

// GamepadState represents the current state of a gamepad.
type GamepadState struct {
	// Analog axes (-1.0 to 1.0)
	LeftX, LeftY   float32
	RightX, RightY float32

	// Triggers (0.0 to 1.0)
	LeftTrigger, RightTrigger float32

	// Button state bitmask
	Buttons uint32
}

// InputState contains the accumulated input state for a frame.
type InputState struct {
	// Mouse movement accumulated since last frame
	MouseDX, MouseDY int32

	// Key states (true = down)
	Keys [NumKeycodes]bool

	// Text input
	Chars []rune

	// Gamepad states (index by player)
	Gamepads [4]GamepadState
}

// CursorMode defines how the cursor behaves.
type CursorMode int

const (
	CursorModeNormal  CursorMode = iota // Visible, free movement
	CursorModeHidden                    // Hidden, free movement
	CursorModeGrabbed                   // Hidden, grabbed to center
)

// ModifierState tracks keyboard modifier state.
type ModifierState struct {
	Shift bool
	Ctrl  bool
	Alt   bool
}

// Backend defines the interface for input backends.
// Different platforms can implement this to provide input.
type Backend interface {
	// Initialize the input system
	Init() error

	// Shutdown the input system
	Shutdown()

	// Poll for events, returns false when quit requested
	PollEvents() bool

	// Get accumulated mouse movement and reset counters
	GetMouseDelta() (dx, dy int32)

	// Get current modifier key state
	GetModifierState() ModifierState

	// Set text input mode (enables character events)
	SetTextMode(mode TextMode)

	// Set cursor mode
	SetCursorMode(mode CursorMode)

	// Show or hide the on-screen keyboard (mobile)
	ShowKeyboard(show bool)

	// Get gamepad state for player index
	GetGamepadState(player int) GamepadState

	// Check if a gamepad is connected
	IsGamepadConnected(player int) bool

	// Set mouse grab mode
	SetMouseGrab(grabbed bool)

	// Attach a platform window to the backend (best-effort). The parameter is
	// intentionally typed as `interface{}` to avoid importing platform-specific
	// window types in this package; backends should type-assert to the concrete
	// window type they expect (for example, `*sdl.Window`).
	SetWindow(win interface{})
}

// KeyEventCallback is called when a key event occurs.
type KeyEventCallback func(event KeyEvent)

// MouseEventCallback is called when a mouse event occurs.
type MouseEventCallback func(event MouseEvent)

// CharEventCallback is called when a character is typed.
type CharEventCallback func(char rune)

// System provides the main input system.
type System struct {
	backend Backend

	// Event callbacks
	OnKey  KeyEventCallback
	OnChar CharEventCallback

	// Menu-specific callbacks (called when keyDest == KeyMenu)
	OnMenuKey  KeyEventCallback
	OnMenuChar CharEventCallback

	// Current state
	state     InputState
	keyDest   KeyDest
	textMode  TextMode
	modifiers ModifierState

	// Key bindings (key -> command string)
	bindings [NumKeycode]string

	// Console-only keys (can't be rebound in console)
	consoleKeys [NumKeycode]bool

	// Menu-only keys (can't be rebound in menu)
	menuBound [NumKeycode]bool
}

// NewSystem creates a new input system with the given backend.
func NewSystem(backend Backend) *System {
	return &System{
		backend: backend,
	}
}

// Init initializes the input system.
func (s *System) Init() error {
	if s.backend == nil {
		// No backend - input will be handled by renderer
		return nil
	}
	return s.backend.Init()
}

// Shutdown cleans up the input system.
func (s *System) Shutdown() {
	if s.backend != nil {
		s.backend.Shutdown()
	}
}

// PollEvents polls for and processes input events.
// Returns false if the application should quit.
func (s *System) PollEvents() bool {
	if s.backend == nil {
		// No backend: nothing to poll, continue running
		return true
	}
	return s.backend.PollEvents()
}

// GetState returns the current accumulated input state.
// Call this once per frame after PollEvents.
func (s *System) GetState() *InputState {
	// Get mouse delta
	if s.backend != nil {
		s.state.MouseDX, s.state.MouseDY = s.backend.GetMouseDelta()
	} else {
		s.state.MouseDX, s.state.MouseDY = 0, 0
	}
	return &s.state
}

// ClearState clears the accumulated input state.
// Call at the end of frame processing.
func (s *System) ClearState() {
	s.state.MouseDX = 0
	s.state.MouseDY = 0
	s.state.Chars = s.state.Chars[:0]
}

// SetKeyDest sets where key events are being sent.
func (s *System) SetKeyDest(dest KeyDest) {
	s.keyDest = dest
	s.UpdateTextMode()
}

// GetKeyDest returns the current key destination.
func (s *System) GetKeyDest() KeyDest {
	return s.keyDest
}

// UpdateTextMode updates text input mode based on key destination.
func (s *System) UpdateTextMode() {
	if s.backend == nil {
		return
	}
	switch s.keyDest {
	case KeyConsole, KeyMessage:
		s.backend.SetTextMode(TextModeOn)
	default:
		s.backend.SetTextMode(TextModeOff)
	}
}

// SetBackend replaces the current backend and initializes it.
// Pass nil to clear the backend.
func (s *System) SetBackend(b Backend) error {
	s.backend = b
	if s.backend == nil {
		return nil
	}
	return s.backend.Init()
}

// Backend returns the currently set Backend (may be nil).
func (s *System) Backend() Backend { return s.backend }

// SetBinding sets a key binding.
func (s *System) SetBinding(key int, binding string) {
	if key >= 0 && key < NumKeycode {
		s.bindings[key] = binding
	}
}

// GetBinding returns the command bound to a key.
func (s *System) GetBinding(key int) string {
	if key >= 0 && key < NumKeycode {
		return s.bindings[key]
	}
	return ""
}

// IsKeyDown returns true if a key is currently pressed.
func (s *System) IsKeyDown(key int) bool {
	if key >= 0 && key < NumKeycode {
		return s.state.Keys[key]
	}
	return false
}

// AnyKeyDown returns true if any key is pressed.
func (s *System) AnyKeyDown() bool {
	for _, down := range s.state.Keys {
		if down {
			return true
		}
	}
	return false
}

// ClearKeyStates clears all key down states.
func (s *System) ClearKeyStates() {
	for i := range s.state.Keys {
		s.state.Keys[i] = false
	}
}

// HandleKeyEvent processes a key event from the backend.
func (s *System) HandleKeyEvent(event KeyEvent) {
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
		// Still forward to general callback for game state tracking
		if s.OnKey != nil {
			s.OnKey(event)
		}
	} else {
		// Forward to general callback for game/console
		if s.OnKey != nil {
			s.OnKey(event)
		}
	}

}

// HandleCharEvent processes a character input event.
func (s *System) HandleCharEvent(char rune) {
	s.state.Chars = append(s.state.Chars, char)

	if s.OnChar != nil {
		s.OnChar(char)
	}
}

// GetModifierState returns current modifier key state.
func (s *System) GetModifierState() ModifierState {
	if s.backend == nil {
		return ModifierState{}
	}
	return s.backend.GetModifierState()
}

// SetCursorMode sets the cursor behavior.
func (s *System) SetCursorMode(mode CursorMode) {
	if s.backend != nil {
		s.backend.SetCursorMode(mode)
	}
}

// ShowKeyboard shows or hides the on-screen keyboard.
func (s *System) ShowKeyboard(show bool) {
	if s.backend != nil {
		s.backend.ShowKeyboard(show)
	}
}

// GetGamepadState returns the state of a gamepad.
func (s *System) GetGamepadState(player int) GamepadState {
	if s.backend == nil {
		return GamepadState{}
	}
	return s.backend.GetGamepadState(player)
}

// IsGamepadConnected returns true if a gamepad is connected.
func (s *System) IsGamepadConnected(player int) bool {
	if s.backend == nil {
		return false
	}
	return s.backend.IsGamepadConnected(player)
}

// SetMouseGrab enables or disables mouse grabbing.
func (s *System) SetMouseGrab(grabbed bool) {
	if s.backend != nil {
		s.backend.SetMouseGrab(grabbed)
	}
}

// KeyToString converts a key code to its string name.
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

// StringToKey converts a string name to a key code.
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
