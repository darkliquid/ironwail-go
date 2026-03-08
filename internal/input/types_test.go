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
