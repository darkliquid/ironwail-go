// Package input provides cross-platform input handling for Quake.
//
// # Architecture: The 3-Layer Input Pipeline
//
// Input flows through three distinct layers:
//
//  1. Platform Layer (Backend interface implementations):
//     Polls the OS/library for raw hardware events — key-down, mouse motion,
//     gamepad axis changes, gyroscope sensor updates. This layer is
//     platform-specific and lives behind the Backend interface.
//
//  2. Translation Layer (Backend → engine key codes):
//     Converts platform-specific identifiers (backend scancodes, gamepad button
//     enums) into Quake-compatible engine key codes (the K_* constants
//     defined in this file). The mapping preserves Quake's original numbering
//     scheme so that configs, bindings, and demo playback remain compatible.
//
//  3. Routing Layer (System.HandleKeyEvent / HandleCharEvent):
//     Routes translated events to the correct consumer based on the current
//     KeyDest — game logic, the console, the chat prompt, or the menu system.
//     This layer also tracks per-key down/up state, accumulates text input
//     (runes), and maintains modifier flags (Shift, Ctrl, Alt).
//
// # Key Numbering Scheme
//
// The engine key codes follow Quake's original key.h layout:
//
//   - 0–31:   ASCII control codes. Only TAB (9), ENTER (13), and ESCAPE (27)
//     are used; the rest are reserved.
//   - 32–126: Printable ASCII. The key code for 'a' is simply 97. Upper-case
//     letters are folded to lower-case so that 'A' and 'a' share code 97.
//   - 127:    Traditionally DEL in ASCII, but Quake maps it to BACKSPACE.
//   - 128+:   Special keys — arrows, function keys, modifiers, keypad, etc.
//     These use an iota sequence starting at KBackspace (127).
//   - 200+:   Mouse buttons and scroll wheel (KMouse1 … KMWheelDown).
//   - KMouseEnd+: Gamepad buttons, including an "alt-modifier" duplicate set
//     for Ironwail's two-layer gamepad binding system.
//   - KPause:  The very last key code; NumKeycodes = KPause + 1.
//
// This package abstracts keyboard, mouse, and gamepad input behind a
// platform-independent interface. It defines the canonical engine-side routing
// and state model while leaving concrete event backends to other packages.
//
// Key Features:
//   - Quake-compatible key codes for game logic
//   - Key binding system
//   - Mouse movement accumulation
//   - Gamepad support with dead zones
//   - Text input mode for console/chat
package input

// ASCII-range key codes.
//
// Printable ASCII characters (a–z, 0–9, punctuation) use their literal ASCII
// value as the engine key code. Only a handful of non-printable ASCII codes
// are given named constants here because they have special meaning in Quake's
// UI:
//
//   - KTab (9):    Cycles through auto-complete options in the console.
//   - KEnter (13): Submits console commands, confirms menu selections.
//   - KEscape (27): Opens/closes the console or menu (the universal "back").
//   - KSpace (32): The first printable ASCII character; used for jumping in
//     default binds.
//
// Key codes compatible with Quake's key.h
// These are the key numbers passed to Key_Event
const (
	// ASCII keys (a-z, 0-9) use their ASCII values directly

	KTab    = 9
	KEnter  = 13
	KEscape = 27
	KSpace  = 32
)

// Special key codes (128+).
//
// These start at 127 (one past the printable ASCII range) and use Go's iota
// to assign sequential values. The exact numbering must remain stable because
// key codes are serialised into config files ("bind 130 +forward") and
// exchanged in network demos.
//
// The grouping mirrors the original key.h:
//   - Navigation: arrows, Home/End, PgUp/PgDn, Ins/Del
//   - Modifiers: Shift, Ctrl, Alt, Command (macOS)
//   - Function keys: F1–F12
//   - Keypad: numpad variants of navigation + arithmetic keys
//   - Lock keys: CapsLock, ScrollLock, PrintScreen
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

// Mouse key codes.
//
// Mouse buttons occupy the range starting at 200, safely above all keyboard
// codes. KMouseBegin acts as a sentinel for range checks. Scroll wheel
// events are mapped to instantaneous press+release pairs (KMWheelUp/Down)
// so that they can be bound just like buttons ("bind MWHEELUP +jump").
// KMouseEnd marks one-past-the-last mouse code and doubles as the start
// of the gamepad range.
const (
	// Mouse buttons (virtual keys)
	KMouseBegin = 200
)

// Individual mouse button and wheel key codes. KMouse1 is the left button,
// KMouse2 is right, KMouse3 is middle. KMouse4/5 are the back/forward side
// buttons. KMWheelUp/Down represent a single scroll notch; the backend emits
// a simultaneous press+release for each notch.
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

// Gamepad key code range sentinel. Gamepad buttons start immediately after
// the last mouse code so the entire key-code space is a single contiguous
// numbering sequence: ASCII → special → mouse → gamepad → pause.
const (
	// Gamepad keys
	KGamepadBegin = KMouseEnd
)

// Standard and extended gamepad button key codes.
//
// The first set (KStart … KRTrigger) covers the standard Xbox-style layout.
// The extended set (KMisc1 … KTouchpad) covers additional buttons found on
// modern controllers (DualSense touchpad, paddles, etc.).
//
// Ironwail adds a second "alt-modifier" layer (KLThumbAlt … KTouchpadAlt)
// that allows each physical gamepad button to have two different bindings.
// The alternate codes are offset from the primary codes by a fixed amount
// (altGamepadOffset = KLThumbAlt - KLThumb). When the "+altmodifier" console
// command is active, the backend shifts gamepad key codes into the alt range
// before dispatching them to the input system. This effectively doubles the
// number of available gamepad bindings without requiring a separate binding
// UI — users simply hold a modifier button and press any other button to
// activate its alternate function.
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

// KPause is the very last key code. NumKeycodes (= KPause + 1) defines the
// total size of key-state arrays throughout the engine. NumKeycode is an alias
// kept for source compatibility with Quake code that used the singular form.
const (
	// Pause key
	KPause = KGamepadEnd

	// Total number of keys
	NumKeycodes = KPause + 1
	NumKeycode  = NumKeycodes
)

// MaxKeys is the maximum number of key bindings. This is separate from
// NumKeycodes because Quake's original config system only saved the first 256
// bindings to config.cfg. Bindings above this index (gamepad buttons) may be
// handled differently by the serialisation layer.
const MaxKeys = 256

// KeyDest defines where key events are being routed.
//
// This is the "Routing" part of the 3-layer input architecture. At any given
// moment the engine is in exactly one of these modes:
//
//   - KeyGame: Keys are interpreted as gameplay input — bound commands like
//     +forward, +attack, etc. Repeat key-down events are suppressed.
//   - KeyConsole: Keys go to the interactive console for command entry.
//   - KeyMessage: Keys go to the chat message buffer (team/all chat).
//   - KeyMenu: Keys navigate the in-game menu system.
//
// The destination is changed by the engine when the console is toggled,
// a menu is opened, or chat is initiated. The input system adjusts text-input
// mode (SDL_StartTextInput) and cursor visibility to match.
type KeyDest int

const (
	KeyGame    KeyDest = iota // Send to game
	KeyConsole                // Send to console
	KeyMessage                // Send to message buffer (chat)
	KeyMenu                   // Send to menu
)

// TextMode defines the text input state used to control whether the platform
// backend generates character events (runes) in addition to key events. When
// TextModeOn is active the backend calls SDL_StartTextInput (or equivalent),
// which enables IME composition and on-screen keyboards on mobile platforms.
// TextModeNoPopup requests character events without showing an on-screen
// keyboard — useful for desktop overlay UIs.
type TextMode int

const (
	TextModeOff     TextMode = iota // No char events
	TextModeOn                      // Char events, show on-screen keyboard
	TextModeNoPopup                 // Char events, don't show on-screen keyboard
)

// KeyDevice identifies which physical input device generated an event.
// The engine uses this to decide whether to show keyboard-style or
// gamepad-style UI prompts and to separate mouse sensitivity from gamepad
// stick sensitivity in the view-angle calculation.
type KeyDevice int

const (
	DeviceNone KeyDevice = iota - 1
	DeviceKeyboard
	DeviceMouse
	DeviceGamepad
)

// KeyEvent represents a single key press or release.
//
// Key is an engine key code (one of the K_* constants). Down is true for a
// press and false for a release. Device indicates the source hardware so the
// engine can distinguish a keyboard ENTER from a gamepad A-button. Character
// is non-zero only for text-input events where a Unicode codepoint is
// available (e.g. from SDL_EVENT_TEXT_INPUT); for most key events it is zero.
type KeyEvent struct {
	Key       int       // Key code (K_* constant)
	Down      bool      // True if pressed, false if released
	Device    KeyDevice // Which device generated this event
	Character rune      // Unicode character if this is a text event
}

// MouseEvent represents accumulated mouse state for a frame.
//
// X and Y hold relative movement in pixels since the last poll; these are
// accumulated across multiple OS motion events within a single engine frame
// to ensure no movement is lost even at high polling rates. Wheel holds
// scroll-wheel delta (positive = up). Buttons is a bitmask of currently
// held buttons (bit 0 = left, bit 1 = right, etc.).
type MouseEvent struct {
	X, Y    int32  // Relative movement
	Wheel   int32  // Scroll wheel delta
	Buttons uint32 // Current button state bitmask
}

// GamepadState represents the current polled state of a gamepad for one frame.
//
// Analog sticks are normalised to [-1.0, +1.0] after deadzone and response-
// curve processing (see applyDeadzoneAndCurve). Triggers are normalised to
// [0.0, 1.0]. Buttons is a bitmask of pressed face buttons. GyroYawDelta and
// GyroPitchDelta accumulate gyroscope-derived rotation (in degrees) since the
// last frame — these feed directly into the view-angle update so that
// gyro aiming feels integrated with stick aiming.
type GamepadState struct {
	// Analog axes (-1.0 to 1.0)
	LeftX, LeftY   float32
	RightX, RightY float32

	// Triggers (0.0 to 1.0)
	LeftTrigger, RightTrigger float32

	// Button state bitmask
	Buttons uint32

	// Gyro deltas accumulated from the last frame (degrees).
	// These are backend-provided and currently only populated by SDL3.
	GyroYawDelta, GyroPitchDelta float32
}

// InputState contains all accumulated input for a single engine frame.
//
// The System fills this struct during PollEvents and the game reads it once
// per frame via GetState. MouseDX/DY are reset after reading; Chars is
// cleared by ClearState at the end of the frame. Keys is a snapshot of which
// engine key codes are currently held down — indexed directly by key code
// (e.g. Keys[KSpace] == true means space is pressed). Gamepads holds up to
// 4 player slots of gamepad state.
type InputState struct {
	// Mouse movement accumulated since last frame
	MouseDX, MouseDY int32
	MouseX, MouseY   int32
	MouseValid       bool

	// Key states (true = down)
	Keys [NumKeycodes]bool

	// Text input
	Chars []rune

	// Gamepad states (index by player)
	Gamepads [4]GamepadState
}

// CursorMode defines OS cursor visibility and confinement.
//
// During gameplay the cursor is grabbed (hidden and locked to the window
// centre) so that mouse movement translates to view rotation. In menus and
// the console the cursor is visible and free-moving.
type CursorMode int

const (
	CursorModeNormal  CursorMode = iota // Visible, free movement
	CursorModeHidden                    // Hidden, free movement
	CursorModeGrabbed                   // Hidden, grabbed to center
)

// ModifierState tracks the current state of the three standard keyboard
// modifier keys. This is updated both by direct key events (KShift/KCtrl/KAlt
// press/release) and by querying the platform's modifier bitmask
// (SDL_GetModState) to stay in sync even if the window loses and regains
// focus while a modifier is held.
type ModifierState struct {
	Shift bool
	Ctrl  bool
	Alt   bool
}

// Backend defines the platform-specific interface for input backends.
//
// This is the "Platform Layer" of the 3-layer architecture. Each backend
// implementation (SDL3, GLFW, etc.) translates raw OS events into the
// engine's key-code domain and provides state-query methods for mouse
// deltas, modifiers, and gamepads. The engine interacts exclusively through
// this interface, making it straightforward to swap backends or add new ones.
//
// The lifecycle is: Init → (PollEvents per frame) → Shutdown.
type Backend interface {
	// Initialize the input system
	Init() error

	// Shutdown the input system
	Shutdown()

	// Poll for events, returns false when quit requested
	PollEvents() bool

	// Get accumulated mouse movement and reset counters
	GetMouseDelta() (dx, dy int32)

	// Get the last known absolute mouse position in window coordinates.
	GetMousePosition() (x, y int32, valid bool)

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

// KeyEventCallback is invoked by the System when a key is pressed or released.
// The routing layer calls this after updating internal state (key-down
// tracking, modifier flags). Subscribers use this to trigger bound commands,
// handle console keystrokes, etc.
type KeyEventCallback func(event KeyEvent)

// MouseEventCallback is invoked when a mouse button or motion event occurs.
type MouseEventCallback func(event MouseEvent)

// CharEventCallback is invoked when a text character is typed. This is
// separate from KeyEventCallback because text input goes through the OS's
// IME / dead-key composition pipeline and may produce characters that don't
// correspond to a single physical key press.
type CharEventCallback func(char rune)

// System is the engine's top-level input manager — the "Routing Layer" of the
// 3-layer architecture. It owns the Backend, accumulates per-frame input
// state, tracks key-down/up status, manages modifier flags, and dispatches
// events to registered callbacks based on the current KeyDest.
//
// The System also holds the key-binding table (bindings) which maps engine
// key codes to console command strings. consoleKeys and menuBound are
// reservation masks that prevent certain keys from being rebound when the
// console or menu is active (e.g. the '`' key always opens the console).
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

// NewSystem creates a new input System wired to the given Backend. The Backend
// is not initialised here — call Init() separately so that initialisation
// errors can be handled.
func NewSystem(backend Backend) *System {
	return &System{
		backend: backend,
	}
}

// Init initialises the input system by delegating to the Backend's Init.
// If no backend is set (nil), Init is a silent no-op — the renderer may
// provide input through a different path.
func (s *System) Init() error {
	if s.backend == nil {
		// No backend - input will be handled by renderer
		return nil
	}
	return s.backend.Init()
}

// Shutdown tears down the input system and releases platform resources. After
// Shutdown the System should not be used.
func (s *System) Shutdown() {
	if s.backend != nil {
		s.backend.Shutdown()
	}
}

// PollEvents drains the platform event queue and processes every pending
// event (key presses, mouse moves, gamepad updates, etc.). Returns false
// when the platform signals that the application should quit (e.g. window
// close or SDL_QUIT). Must be called once per engine frame, before GetState.
func (s *System) PollEvents() bool {
	if s.backend == nil {
		// No backend: nothing to poll, continue running
		return true
	}
	return s.backend.PollEvents()
}

// GetState returns the accumulated input state for this frame. The mouse
// deltas are fetched from the backend and written into the returned struct.
// Call this once per frame after PollEvents — calling it multiple times will
// return zero mouse deltas on the second call because the backend resets its
// accumulators.
func (s *System) GetState() *InputState {
	// Get mouse delta
	if s.backend != nil {
		s.state.MouseDX, s.state.MouseDY = s.backend.GetMouseDelta()
		s.state.MouseX, s.state.MouseY, s.state.MouseValid = s.backend.GetMousePosition()
	} else {
		s.state.MouseDX, s.state.MouseDY = 0, 0
		s.state.MouseX, s.state.MouseY, s.state.MouseValid = 0, 0, false
	}
	return &s.state
}

// ClearState resets the per-frame accumulators (mouse deltas and character
// buffer). Call at the end of frame processing so that the next frame starts
// with a clean slate.
func (s *System) ClearState() {
	s.state.MouseDX = 0
	s.state.MouseDY = 0
	s.state.Chars = s.state.Chars[:0]
}

// SetKeyDest changes the key-event routing destination and adjusts text input
// mode accordingly. Switching to KeyConsole, KeyMessage, or KeyMenu enables
// text input (so the user gets character events for typing); switching to
// KeyGame disables it (so keys only trigger bindings, not character input).
func (s *System) SetKeyDest(dest KeyDest) {
	s.keyDest = dest
	s.UpdateTextMode()
}

// GetKeyDest returns the current key-event routing destination.
func (s *System) GetKeyDest() KeyDest {
	return s.keyDest
}

// UpdateTextMode synchronises the backend's text-input state with the current
// KeyDest. Console, message, and menu modes all require character events;
// game mode does not. This is called automatically by SetKeyDest but can also
// be invoked manually if the backend is replaced at runtime.
func (s *System) UpdateTextMode() {
	if s.backend == nil {
		return
	}
	switch s.keyDest {
	case KeyConsole, KeyMessage, KeyMenu:
		s.backend.SetTextMode(TextModeOn)
	default:
		s.backend.SetTextMode(TextModeOff)
	}
}

// SetBackend hot-swaps the input backend. The new backend is immediately
// initialised. Pass nil to detach the backend entirely (useful during
// renderer transitions).
func (s *System) SetBackend(b Backend) error {
	s.backend = b
	if s.backend == nil {
		return nil
	}
	return s.backend.Init()
}

// Backend returns the currently attached Backend, which may be nil if no
// backend has been set or if it was explicitly cleared.
func (s *System) Backend() Backend { return s.backend }
