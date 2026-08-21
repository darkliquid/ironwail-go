package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
)

// recordingSink records every event it receives.
type recordingSink struct {
	events []input.KeyEvent
}

func (s *recordingSink) send(ev input.KeyEvent) { s.events = append(s.events, ev) }
func (s *recordingSink) count() int             { return len(s.events) }

func testRouter(uiOn bool, capture map[int]bool) (*InputRouter, *recordingSink, *recordingSink) {
	engine := &recordingSink{}
	ui := &recordingSink{}
	return NewInputRouter(
		engine.send,
		ui.send,
		func() bool { return uiOn },
		func(key int) bool { return capture[key] },
	), engine, ui
}

// TestInputRouterExclusiveKeyRouting pins R1.2: with the gogpu/ui path active,
// each KeyDest routes to EXACTLY one sink — menu/console to the UI only,
// game/message to the engine only.
func TestInputRouterExclusiveKeyRouting(t *testing.T) {
	router, engine, ui := testRouter(true, nil)

	dests := []struct {
		name string
		dest input.KeyDest
		want string // "ui" or "engine"
	}{
		{name: "game", dest: input.KeyGame, want: "engine"},
		{name: "message", dest: input.KeyMessage, want: "engine"},
		{name: "console", dest: input.KeyConsole, want: "ui"},
		{name: "menu", dest: input.KeyMenu, want: "ui"},
	}

	for _, tc := range dests {
		engine.events = nil
		ui.events = nil
		ev := input.KeyEvent{Key: int('w'), Down: true}
		got := router.RouteKeyEvent(ev, tc.dest)
		if got != tc.want {
			t.Fatalf("%s: RouteKeyEvent = %q, want %q", tc.name, got, tc.want)
		}

		// Exclusive: exactly one sink received the event.
		engineCount := engine.count()
		uiCount := ui.count()
		total := engineCount + uiCount
		if total != 1 {
			t.Fatalf("%s: sinks received %d events (engine=%d ui=%d), want exactly 1 (exclusive R1.2)", tc.name, total, engineCount, uiCount)
		}
		switch tc.want {
		case "ui":
			if engineCount != 0 || uiCount != 1 {
				t.Fatalf("%s: engine=%d ui=%d, want engine=0 ui=1", tc.name, engineCount, uiCount)
			}
		case "engine":
			if engineCount != 1 || uiCount != 0 {
				t.Fatalf("%s: engine=%d ui=%d, want engine=1 ui=0", tc.name, engineCount, uiCount)
			}
		}
	}
}

// TestInputRouterNoDoubleDispatch pins the v3 regression: on the gogpu/ui
// path, a surface key (menu/console) must never ALSO reach the engine, and a
// gameplay key must never ALSO reach the UI. This is the exact double-delivery
// class of bugs the v3 input backend rewrite fought.
func TestInputRouterNoDoubleDispatch(t *testing.T) {
	router, engine, ui := testRouter(true, nil)

	// Menu key: reaches the ui sink, never the engine.
	ev := input.KeyEvent{Key: input.KEnter, Down: true}
	if got := router.RouteKeyEvent(ev, input.KeyMenu); got != "ui" {
		t.Fatalf("menu key routed to %q, want ui", got)
	}
	if ui.count() != 1 || engine.count() != 0 {
		t.Fatalf("menu key double-dispatched: ui=%d engine=%d, want ui=1 engine=0", ui.count(), engine.count())
	}

	// Console key (backtick) at console dest: ui only.
	engine.events = nil
	ui.events = nil
	if got := router.RouteKeyEvent(input.KeyEvent{Key: int('`'), Down: true}, input.KeyConsole); got != "ui" {
		t.Fatalf("console key routed to %q, want ui", got)
	}
	if ui.count() != 1 || engine.count() != 0 {
		t.Fatalf("console key double-dispatched: ui=%d engine=%d, want ui=1 engine=0", ui.count(), engine.count())
	}

	// Gameplay key (w) at game dest: engine only.
	engine.events = nil
	ui.events = nil
	if got := router.RouteKeyEvent(input.KeyEvent{Key: int('w'), Down: true}, input.KeyGame); got != "engine" {
		t.Fatalf("gameplay key routed to %q, want engine", got)
	}
	if engine.count() != 1 || ui.count() != 0 {
		t.Fatalf("gameplay key double-dispatched: engine=%d ui=%d, want engine=1 ui=0", engine.count(), ui.count())
	}
}

// TestInputRouterBacktickCapturePreRoutesEngine pins the backtick pre-route:
// regardless of KeyDest, a capture key (binding capture / console toggle)
// reaches the engine first — the widget tree never steals it.
func TestInputRouterBacktickCapturePreRoutesEngine(t *testing.T) {
	router, engine, ui := testRouter(true, map[int]bool{int('`'): true})

	ev := input.KeyEvent{Key: int('`'), Down: true}
	if got := router.RouteKeyEvent(ev, input.KeyMenu); got != "engine-capture" {
		t.Fatalf("capture key routed to %q, want engine-capture", got)
	}
	if engine.count() != 1 || ui.count() != 0 {
		t.Fatalf("capture key double-dispatched: engine=%d ui=%d, want engine=1 ui=0", engine.count(), ui.count())
	}
}

// TestInputRouterLegacyPathRoutesEverythingToEngine pins the fail-open: when
// the gogpu/ui path is NOT active, every KeyDest routes to the engine exactly
// as the pre-rewrite input system did (engine owns menu+console too).
func TestInputRouterLegacyPathRoutesEverythingToEngine(t *testing.T) {
	router, engine, ui := testRouter(false, nil)

	for _, dest := range []input.KeyDest{input.KeyGame, input.KeyMessage, input.KeyConsole, input.KeyMenu} {
		engine.events = nil
		ui.events = nil
		if got := router.RouteKeyEvent(input.KeyEvent{Key: int('w'), Down: true}, dest); got != "engine" {
			t.Fatalf("legacy dest %v routed to %q, want engine", dest, got)
		}
		if engine.count() != 1 || ui.count() != 0 {
			t.Fatalf("legacy dest %v: engine=%d ui=%d, want engine=1 ui=0", dest, engine.count(), ui.count())
		}
	}
}

// recordingForwarder records key forwards for the ui sink (quakeui.Forwarder
// shape: ForwardKey/ForwardChar/ForwardText/HandleEvent).
type recordingForwarder struct {
	keys []input.KeyEvent
}

func (f *recordingForwarder) ForwardKey(ev input.KeyEvent, _ input.ModifierState) {
	f.keys = append(f.keys, ev)
}
func (f *recordingForwarder) ForwardChar(_ rune, _ input.ModifierState) {}
func (f *recordingForwarder) ForwardText(_ string, _ input.ModifierState) {}

// TestInputRouterWiredIntoInputSystemNoDoubleDispatch is the wiring-level RED:
// with the router installed on the gogpu/ui path, a menu-destination key
// routed through the real input System reaches exactly one sink (the ui), and
// a game-destination key reaches the engine exactly once — no double-dispatch
// even when both sinks are live.
func TestInputRouterWiredIntoInputSystemNoDoubleDispatch(t *testing.T) {
	g := New()
	g.Host = host.NewHost()
	g.Client = client.NewClient()
	g.Client.State = client.StateActive
	g.Input = input.NewSystem(nil)
	_ = g.Input.Init()

	// Engine side keeps its real handlers so button latching is observable.
	g.Input.OnKey = g.handleGameKeyEvent
	g.Input.OnMenuKey = g.handleMenuKeyEvent
	registerGameplayButtonCommands(g)
	g.applyDefaultGameplayBindings()
	g.ensureGameplayBindings()

	// ui sink: recording forwarder.
	uiFwd := &recordingForwarder{}
	g.uiInput = uiFwd

	// Install router on the gogpu/ui path (same wiring as installInputRouter:
	// OnMenuKey routes menu→ui; OnKey skips menu-mode re-delivery).
	g.inputRouter = NewInputRouter(
		g.handleGameKeyEvent,
		g.forwardUIKey,
		func() bool { return true }, // forced active path
		func(key int) bool { return key == int('`') },
	)
	uiSink := g.inputRouter.RouteKeyEvent
	g.Input.OnMenuKey = func(ev input.KeyEvent) { uiSink(ev, input.KeyMenu) }
	g.Input.OnKey = func(ev input.KeyEvent) {
		if g.Input.KeyDest() == input.KeyMenu {
			return
		}
		uiSink(ev, g.Input.KeyDest())
	}

	// Menu destination: key reaches the ui sink exactly once, engine button
	// untouched.
	g.Input.SetKeyDest(input.KeyMenu)
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KEnter, Down: true})
	if len(uiFwd.keys) != 1 {
		t.Fatalf("menu key: ui sink received %d, want 1", len(uiFwd.keys))
	}
	if g.Client.InputForward.State&1 != 0 {
		t.Fatal("menu key latched the engine forward button — double-dispatch")
	}
	uiFwd.keys = nil

	// Game destination: key reaches the engine (forward latch), ui untouched.
	g.Input.SetKeyDest(input.KeyGame)
	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	if g.Client.InputForward.State&1 == 0 {
		t.Fatal("gameplay key did not reach the engine button — routing broken")
	}
	if len(uiFwd.keys) != 0 {
		t.Fatalf("gameplay key reached the ui sink (%d), want 0 (exclusive)", len(uiFwd.keys))
	}
}

// TestMenuNavigationKeysReachUIOnly is the M3.1 GAME-level RED (AC9): with a
// real quakeui overlay + decoupled router wired on the gogpu/ui path, a menu
// navigation key at KeyMenu reaches the UI widget tree (advancing the menu
// cursor via M_Key) and never also latches an engine gameplay button.
func TestMenuNavigationKeysReachUIOnly(t *testing.T) {
	g := New()
	g.Host = host.NewHost()
	g.Client = client.NewClient()
	g.Client.State = client.StateActive
	g.Input = input.NewSystem(nil)
	_ = g.Input.Init()

	// Real menu manager (legacy state machine driving the quakeui MenuRoot).
	g.Menu = menu.NewManager(nil, g.Input, g.Host.CVar)
	g.Input.OnMenuKey = g.handleMenuKeyEvent
	g.Menu.SetCommandText(g.Host.Cmd.AddText)
	registerGameplayButtonCommands(g)
	g.applyDefaultGameplayBindings()
	g.ensureGameplayBindings()

	// Wire the quakeui overlay + forwarder (ensureQuakeUIOverlay builds the
	// real MenuRoot into the stack and installs uiInput).
	overlay := g.ensureQuakeUIOverlay()
	if overlay == nil || overlay.MenuRoot() == nil {
		t.Fatal("quakeui overlay/MenuRoot not built")
	}

	// Install the router exactly as installInputRouter does.
	g.inputRouter = NewInputRouter(
		g.handleGameKeyEvent,
		g.forwardUIKey,
		func() bool { return true },
		func(key int) bool { return key == int('`') },
	)
	uiSink := g.inputRouter.RouteKeyEvent
	g.Input.OnMenuKey = func(ev input.KeyEvent) { uiSink(ev, input.KeyMenu) }
	g.Input.OnKey = func(ev input.KeyEvent) {
		if g.Input.KeyDest() == input.KeyMenu {
			return
		}
		uiSink(ev, g.Input.KeyDest())
	}

	// Open the menu, capture the cursor position.
	g.Menu.ToggleMenu()
	g.Menu.ShowState(menu.MenuMain)
	if g.Input.KeyDest() != input.KeyMenu {
		t.Fatalf("KeyDest = %v, want KeyMenu", g.Input.KeyDest())
	}
	cursorBefore := g.Menu.MainCursor()

	// Down arrow at KeyMenu.
	g.Input.HandleKeyEvent(input.KeyEvent{Key: input.KDownArrow, Down: true})

	cursorAfter := g.Menu.MainCursor()
	if cursorAfter == cursorBefore {
		t.Fatal("menu navigation key did not advance the menu cursor — the UI did not receive it")
	}
	// No engine side effect: forward button must stay up.
	if g.Client.InputForward.State&1 != 0 {
		t.Fatal("menu navigation key latched an engine gameplay button (double-dispatch)")
	}
}
