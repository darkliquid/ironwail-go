package input

import "testing"

func TestFunctionKeyStringRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		key  int
		name string
	}{
		{key: KF1, name: "F1"},
		{key: KF9, name: "F9"},
		{key: KF10, name: "F10"},
		{key: KF11, name: "F11"},
		{key: KF12, name: "F12"},
	} {
		if got := KeyToString(tc.key); got != tc.name {
			t.Fatalf("KeyToString(%d) = %q, want %q", tc.key, got, tc.name)
		}
		if got := StringToKey(tc.name); got != tc.key {
			t.Fatalf("StringToKey(%q) = %d, want %d", tc.name, got, tc.key)
		}
	}
}

type textModeBackend struct {
	lastMode TextMode
}

func (b *textModeBackend) Init() error                             { return nil }
func (b *textModeBackend) Shutdown()                               {}
func (b *textModeBackend) PollEvents() bool                        { return true }
func (b *textModeBackend) GetMouseDelta() (dx, dy int32)           { return 0, 0 }
func (b *textModeBackend) GetModifierState() ModifierState         { return ModifierState{} }
func (b *textModeBackend) SetTextMode(mode TextMode)               { b.lastMode = mode }
func (b *textModeBackend) SetCursorMode(mode CursorMode)           {}
func (b *textModeBackend) ShowKeyboard(show bool)                  {}
func (b *textModeBackend) GetGamepadState(player int) GamepadState { return GamepadState{} }
func (b *textModeBackend) IsGamepadConnected(player int) bool      { return false }
func (b *textModeBackend) SetMouseGrab(grabbed bool)               {}
func (b *textModeBackend) SetWindow(win interface{})               {}

func TestHandleCharEventRoutesToMenuCharCallback(t *testing.T) {
	sys := NewSystem(nil)
	sys.SetKeyDest(KeyMenu)

	var menuChars []rune
	var allChars []rune
	sys.OnMenuChar = func(char rune) { menuChars = append(menuChars, char) }
	sys.OnChar = func(char rune) { allChars = append(allChars, char) }

	sys.HandleCharEvent('a')

	if len(menuChars) != 1 || menuChars[0] != 'a' {
		t.Fatalf("menu chars = %q, want [a]", string(menuChars))
	}
	if len(allChars) != 1 || allChars[0] != 'a' {
		t.Fatalf("all chars = %q, want [a]", string(allChars))
	}
}

func TestUpdateTextModeEnablesTextForMenu(t *testing.T) {
	backend := &textModeBackend{}
	sys := NewSystem(backend)
	sys.SetKeyDest(KeyMenu)

	if backend.lastMode != TextModeOn {
		t.Fatalf("text mode = %v, want %v", backend.lastMode, TextModeOn)
	}

	sys.SetKeyDest(KeyGame)
	if backend.lastMode != TextModeOff {
		t.Fatalf("text mode after returning to game = %v, want %v", backend.lastMode, TextModeOff)
	}
}

func TestHandleKeyEventFiltersAutorepeatOnlyInGame(t *testing.T) {
	sys := NewSystem(nil)

	var gameEvents []KeyEvent
	sys.OnKey = func(event KeyEvent) { gameEvents = append(gameEvents, event) }

	sys.SetKeyDest(KeyGame)
	sys.HandleKeyEvent(KeyEvent{Key: int('x'), Down: true})
	sys.HandleKeyEvent(KeyEvent{Key: int('x'), Down: true})

	if len(gameEvents) != 1 {
		t.Fatalf("game key callback count = %d, want 1", len(gameEvents))
	}

	sys.HandleKeyEvent(KeyEvent{Key: int('x'), Down: false})

	var menuEvents []KeyEvent
	var menuOnlyEvents []KeyEvent
	sys.OnKey = func(event KeyEvent) { menuEvents = append(menuEvents, event) }
	sys.OnMenuKey = func(event KeyEvent) { menuOnlyEvents = append(menuOnlyEvents, event) }
	sys.SetKeyDest(KeyMenu)
	sys.HandleKeyEvent(KeyEvent{Key: int('x'), Down: true})
	sys.HandleKeyEvent(KeyEvent{Key: int('x'), Down: true})

	if len(menuEvents) != 2 {
		t.Fatalf("menu OnKey callback count = %d, want 2", len(menuEvents))
	}
	if len(menuOnlyEvents) != 2 {
		t.Fatalf("menu OnMenuKey callback count = %d, want 2", len(menuOnlyEvents))
	}
}

func TestHandleKeyEventStopsGeneralDispatchWhenMenuChangesDest(t *testing.T) {
	sys := NewSystem(nil)
	sys.SetKeyDest(KeyMenu)

	var menuEvents []KeyEvent
	var gameEvents []KeyEvent
	sys.OnMenuKey = func(event KeyEvent) {
		menuEvents = append(menuEvents, event)
		sys.SetKeyDest(KeyGame)
	}
	sys.OnKey = func(event KeyEvent) {
		gameEvents = append(gameEvents, event)
	}

	sys.HandleKeyEvent(KeyEvent{Key: KEscape, Down: true})

	if len(menuEvents) != 1 {
		t.Fatalf("menu OnMenuKey callback count = %d, want 1", len(menuEvents))
	}
	if len(gameEvents) != 0 {
		t.Fatalf("general OnKey callback count = %d, want 0", len(gameEvents))
	}
	if got := sys.GetKeyDest(); got != KeyGame {
		t.Fatalf("key destination after menu handler = %v, want %v", got, KeyGame)
	}
}

func TestHandleKeyEventIgnoresStrayKeyUp(t *testing.T) {
	sys := NewSystem(nil)
	sys.SetKeyDest(KeyGame)

	var events []KeyEvent
	sys.OnKey = func(event KeyEvent) { events = append(events, event) }

	sys.HandleKeyEvent(KeyEvent{Key: int('z'), Down: false})
	if len(events) != 0 {
		t.Fatalf("stray key up should not dispatch callbacks, got %d", len(events))
	}
	if sys.IsKeyDown(int('z')) {
		t.Fatalf("stray key up should not mark key down")
	}

	sys.HandleKeyEvent(KeyEvent{Key: int('z'), Down: true})
	sys.HandleKeyEvent(KeyEvent{Key: int('z'), Down: false})

	if len(events) != 2 {
		t.Fatalf("expected down/up callbacks after valid press, got %d", len(events))
	}
}
