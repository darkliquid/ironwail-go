package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
)

// spyForwarder records ForwardKey/ForwardChar calls so the game routing tests
// can assert which events reach the ui widget tree (M1.5 / ADR-0007).
type spyForwarder struct {
	keys  []input.KeyEvent
	mods  []input.ModifierState
	chars []rune
}

func (s *spyForwarder) ForwardKey(ev input.KeyEvent, m input.ModifierState) {
	s.keys = append(s.keys, ev)
	s.mods = append(s.mods, m)
}

func (s *spyForwarder) ForwardChar(r rune, m input.ModifierState) {
	s.chars = append(s.chars, r)
}

func (s *spyForwarder) ForwardText(text string, m input.ModifierState) {}

// newUIRoutingGame builds a Game wired with a real input.System and a
// spyForwarder installed through the production quakui path (AttachKeyForwarder
// sets g.uiInput). The KeyDest router then drives the shim.
func newUIRoutingGame(t *testing.T) (*Game, *spyForwarder) {
	t.Helper()
	g := newInputTestGame(t)
	g.Client.State = client.StateActive
	g.Host = host.NewHost()
	g.Input = input.NewSystem(nil)
	if err := g.Input.Init(); err != nil {
		t.Fatalf("input init: %v", err)
	}
	g.Input.OnKey = g.handleGameKeyEvent
	g.Input.OnChar = g.handleGameCharEvent
	g.Input.OnMenuKey = g.handleMenuKeyEvent
	g.Input.OnMenuChar = g.handleMenuCharEvent
	registerGameplayButtonCommands(g)
	g.applyDefaultGameplayBindings()
	g.ensureGameplayBindings()
	g.Menu = menu.NewManager(nil, g.Input, nil)

	spy := &spyForwarder{}
	host := &quakuiHost{g: g}
	host.AttachKeyForwarder(spy)
	return g, spy
}

// TestUIRoutingMenuKeyForwards asserts that a key-down while KeyDest==KeyMenu
// (menu captures) is forwarded to the ui KeyForwarder. The game also keeps
// processing the menu (M1.x transitional), but the shim is invoked.
func TestUIRoutingMenuKeyForwards(t *testing.T) {
	g, spy := newUIRoutingGame(t)
	g.Input.SetKeyDest(input.KeyMenu)

	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})

	if len(spy.keys) != 1 {
		t.Fatalf("menu key forwarded %d times, want 1 (got %d)", len(spy.keys), len(spy.keys))
	}
	if spy.keys[0].Key != input.KEnter {
		t.Errorf("forwarded key = %d, want KEnter (%d)", spy.keys[0].Key, input.KEnter)
	}
}

// TestUIRoutingGameKeyNotForwarded asserts that gameplay input (KeyDest==KeyGame)
// never reaches the ui forwarder (fallthrough: HUD/game is non-interactive).
func TestUIRoutingGameKeyNotForwarded(t *testing.T) {
	g, spy := newUIRoutingGame(t)
	g.Input.SetKeyDest(input.KeyGame)

	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: false})

	if len(spy.keys) != 0 {
		t.Fatalf("game key forwarded %d times, want 0 (fallthrough)", len(spy.keys))
	}
}

// TestUIRoutingCharForwards asserts character events are forwarded while the
// console (KeyDest==KeyConsole) is active; backtick is never forwarded; and
// gameplay chars are not forwarded (fallthrough).
func TestUIRoutingCharForwards(t *testing.T) {
	g, spy := newUIRoutingGame(t)
	g.Input.SetKeyDest(input.KeyConsole)
	g.Input.HandleCharEvent('h')

	if len(spy.chars) != 1 || spy.chars[0] != 'h' {
		t.Fatalf("console char = %q, want 1 forwarded 'h' (chars=%q)", spy.chars, spy.chars)
	}

	// Backtick toggles console and is not part of the typed input.
	g.Input.HandleCharEvent('`')
	if len(spy.chars) != 1 {
		t.Fatalf("backtick forwarded %d times, want 0", len(spy.chars)-1)
	}

	// Gameplay char must not be forwarded (fallthrough).
	g.Input.SetKeyDest(input.KeyGame)
	g.Input.HandleCharEvent('g')
	if len(spy.chars) != 1 {
		t.Fatalf("game char forwarded into ui (chars=%q)", spy.chars)
	}
}

// TestUIRoutingMenuCharForwards asserts menu character input reaches the ui.
func TestUIRoutingMenuCharForwards(t *testing.T) {
	g, spy := newUIRoutingGame(t)
	g.Input.SetKeyDest(input.KeyMenu)
	g.Input.HandleCharEvent('m')

	if len(spy.chars) != 1 || spy.chars[0] != 'm' {
		t.Fatalf("menu char = %q, want 1 forwarded 'm'", spy.chars)
	}
}

// TestLegacyPathNoForwarder asserts the default path (quakui Host absent) leaves
// g.uiInput nil, so the routing guards are no-ops and gameplay is untouched.
func TestLegacyPathNoForwarder(t *testing.T) {
	g := newInputTestGame(t)
	if g.uiInput != nil {
		t.Fatalf("legacy path unexpectedly has a KeyForwarder")
	}
	// Existing latching path must still work.
	g.Input.SetKeyDest(input.KeyGame)
	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: false})
}

// TestUIRoutingMenuKeyDoesNotDoubleDispatch asserts that when uiInput is present,
// key events are forwarded to the UI tree and not executed twice on the legacy manager.
func TestUIRoutingMenuKeyDoesNotDoubleDispatch(t *testing.T) {
	g, spy := newUIRoutingGame(t)
	g.Input.SetKeyDest(input.KeyMenu)
	g.Menu.ShowState(menu.MenuMain)

	initialCursor := g.Menu.CursorFor(menu.MenuMain)
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})

	// Legacy menu cursor should NOT have moved on its own because uiInput is attached;
	// the UI widget tree is responsible for handling the forwarded event.
	afterCursor := g.Menu.CursorFor(menu.MenuMain)
	if afterCursor != initialCursor {
		t.Fatalf("legacy menu cursor moved directly from %d to %d despite uiInput active", initialCursor, afterCursor)
	}
	if len(spy.keys) != 1 {
		t.Fatalf("expected exactly 1 forwarded key event, got %d", len(spy.keys))
	}
}
