package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
)

// newInputTestGame builds a Game wired with a real input.System, host command
// system, client, menu, and the gameplay button commands, mirroring the
// production init path closely enough to exercise the full key-event ->
// +forward/-forward -> KButton lifecycle.
func newInputTestGame(t *testing.T) *Game {
	t.Helper()
	g := New()
	g.Host = host.NewHost()
	g.Client = client.NewClient()
	// Real gameplay has an active client; StateActive prevents the console
	// from being treated as forced-up during mode syncing.
	g.Client.State = client.StateActive
	g.Input = input.NewSystem(nil)
	if err := g.Input.Init(); err != nil {
		t.Fatalf("input init: %v", err)
	}

	// Wire the same input callbacks the real init does.
	g.Input.OnKey = g.handleGameKeyEvent
	g.Input.OnChar = g.handleGameCharEvent

	// Register gameplay button commands (+forward/-forward etc).
	registerGameplayButtonCommands(g)

	// Apply default bindings (w -> +forward, mouse1 -> +attack, esc -> menu...)
	g.applyDefaultGameplayBindings()
	g.ensureGameplayBindings()

	return g
}

// registerGameplayButtonCommands exposes the same registrations as
// registerInputCommands so tests can drive +forward/-forward through the real
// command system without full game init.
func registerGameplayButtonCommands(g *Game) {
	g.registerGameplayButtonCommand("forward", func(c *client.Client) *client.KButton { return &c.InputForward })
	g.registerGameplayButtonCommand("back", func(c *client.Client) *client.KButton { return &c.InputBack })
	g.registerGameplayButtonCommand("moveleft", func(c *client.Client) *client.KButton { return &c.InputMoveLeft })
	g.registerGameplayButtonCommand("moveright", func(c *client.Client) *client.KButton { return &c.InputMoveRight })
	g.registerGameplayButtonCommand("left", func(c *client.Client) *client.KButton { return &c.InputLeft })
	g.registerGameplayButtonCommand("right", func(c *client.Client) *client.KButton { return &c.InputRight })
	g.registerGameplayButtonCommand("speed", func(c *client.Client) *client.KButton { return &c.InputSpeed })
	g.registerGameplayButtonCommand("strafe", func(c *client.Client) *client.KButton { return &c.InputStrafe })
	g.registerGameplayButtonCommand("attack", func(c *client.Client) *client.KButton { return &c.InputAttack })
	g.registerGameplayButtonCommand("jump", func(c *client.Client) *client.KButton { return &c.InputJump })
	g.registerGameplayButtonCommand("use", func(c *client.Client) *client.KButton { return &c.InputUse })
	g.registerGameplayButtonCommand("mlook", func(c *client.Client) *client.KButton { return &c.InputMLook })
	g.registerGameplayButtonCommand("klook", func(c *client.Client) *client.KButton { return &c.InputKLook })
	g.registerGameplayButtonCommand("lookup", func(c *client.Client) *client.KButton { return &c.InputLookUp })
	g.registerGameplayButtonCommand("lookdown", func(c *client.Client) *client.KButton { return &c.InputLookDown })
	g.registerGameplayButtonCommand("up", func(c *client.Client) *client.KButton { return &c.InputUp })
	g.registerGameplayButtonCommand("down", func(c *client.Client) *client.KButton { return &c.InputDown })
}

// pressKey feeds a key-down event through the input system exactly as the
// platform backends do.
func pressKey(t *testing.T, g *Game, key int) {
	t.Helper()
	g.Input.HandleKeyEvent(input.KeyEvent{Key: key, Down: true})
}

// releaseKey feeds a key-up event through the input system.
func releaseKey(t *testing.T, g *Game, key int) {
	t.Helper()
	g.Input.HandleKeyEvent(input.KeyEvent{Key: key, Down: false})
}

// TestMovementCommandsReleaseOnKeyUp validates the core regression: a
// movement button pressed and released through the full input->command->button
// path must not remain latched. Covers forward, strafe, attack, and mouse1.
func TestMovementCommandsReleaseOnKeyUp(t *testing.T) {
	g := newInputTestGame(t)
	if g.Input.KeyDest() != input.KeyGame {
		g.Input.SetKeyDest(input.KeyGame)
	}

	cases := []struct {
		name   string
		key    int
		button func(*client.Client) *client.KButton
	}{
		{name: "forward", key: int('w'), button: func(c *client.Client) *client.KButton { return &c.InputForward }},
		{name: "back", key: int('s'), button: func(c *client.Client) *client.KButton { return &c.InputBack }},
		{name: "moveleft", key: int('a'), button: func(c *client.Client) *client.KButton { return &c.InputMoveLeft }},
		{name: "moveright", key: int('d'), button: func(c *client.Client) *client.KButton { return &c.InputMoveRight }},
		{name: "strafe", key: input.KAlt, button: func(c *client.Client) *client.KButton { return &c.InputStrafe }},
		{name: "attack", key: input.KCtrl, button: func(c *client.Client) *client.KButton { return &c.InputAttack }},
		{name: "attack-mouse1", key: input.KMouse1, button: func(c *client.Client) *client.KButton { return &c.InputAttack }},
		{name: "jump", key: input.KSpace, button: func(c *client.Client) *client.KButton { return &c.InputJump }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			btn := tc.button(g.Client)

			pressKey(t, g, tc.key)
			if btn.State&1 == 0 {
				t.Fatalf("after press: button state = %d, want bit0 (down) set", btn.State)
			}

			releaseKey(t, g, tc.key)
			if btn.State&1 != 0 {
				t.Fatalf("after release: button state = %d, want bit0 (down) cleared — button stuck", btn.State)
			}
			if btn.Down[0] != 0 || btn.Down[1] != 0 {
				t.Fatalf("after release: Down slots = %v, want [0 0] — key still tracked", btn.Down)
			}
		})
	}
}

// TestRepeatedPressReleaseCycle validates that a button can be pressed and
// released multiple times without latching (guards the "only works once"
// regression, notably after the menu grab/release path runs).
func TestRepeatedPressReleaseCycle(t *testing.T) {
	g := newInputTestGame(t)
	g.Input.SetKeyDest(input.KeyGame)

	key := int('w')
	btn := &g.Client.InputForward

	for i := 0; i < 5; i++ {
		pressKey(t, g, key)
		if btn.State&1 == 0 {
			t.Fatalf("cycle %d press: state = %d, want down", i, btn.State)
		}
		releaseKey(t, g, key)
		if btn.State&1 != 0 {
			t.Fatalf("cycle %d release: state = %d, want up", i, btn.State)
		}
	}
}

// TestEscapeOpensMenuAndReleasesButtons validates that pressing escape in
// gameplay opens the menu (KeyDest becomes KeyMenu) and releases all held
// gameplay buttons + mouse grab, and that returning to the game with escape
// re-grabs and clears stale key state so movement works again.
func TestEscapeOpensMenuAndReleasesButtons(t *testing.T) {
	g := newInputTestGame(t)
	g.Input.SetKeyDest(input.KeyGame)

	// Give the game a menu manager with the input system wired.
	g.Menu = menu.NewManager(nil, g.Input, g.Host.CVar)
	g.Input.OnMenuKey = g.handleMenuKeyEvent
	g.Menu.SetCommandText(g.Host.Cmd.AddText)

	// Hold forward and attack, then press escape.
	pressKey(t, g, int('w'))
	pressKey(t, g, input.KMouse1)
	if g.Client.InputForward.State&1 == 0 || g.Client.InputAttack.State&1 == 0 {
		t.Fatalf("pre-escape: forward=%d attack=%d, want both down", g.Client.InputForward.State, g.Client.InputAttack.State)
	}

	// Escape keydown: should toggle the menu on.
	pressKey(t, g, input.KEscape)

	if g.Input.KeyDest() != input.KeyMenu {
		t.Fatalf("after escape: keyDest = %v, want menu (menu never opened)", g.Input.KeyDest())
	}
	if g.Client.InputForward.State&1 != 0 || g.Client.InputAttack.State&1 != 0 {
		t.Fatalf("after escape: forward=%d attack=%d, want both released by menu grab-release", g.Client.InputForward.State, g.Client.InputAttack.State)
	}

	// Release the physical keys that are still held (menu is up; key-ups are
	// swallowed but must not corrupt state).
	releaseKey(t, g, int('w'))
	releaseKey(t, g, input.KMouse1)

	// Escape again: should close the menu and return to gameplay.
	pressKey(t, g, input.KEscape)
	if g.Input.KeyDest() != input.KeyGame {
		t.Fatalf("second escape: keyDest = %v, want game", g.Input.KeyDest())
	}

	// Normal movement must work again.
	pressKey(t, g, int('w'))
	if g.Client.InputForward.State&1 == 0 {
		t.Fatalf("post-menu forward press: state = %d, want down", g.Client.InputForward.State)
	}
	releaseKey(t, g, int('w'))
	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("post-menu forward release: state = %d, want up", g.Client.InputForward.State)
	}
}

// TestHeldKeyThroughMenuToggle guards the exact user-visible regression:
// a key held while the menu opens (its key-up is swallowed by menu mode)
// must not stick, and re-pressing the key after closing the menu must work.
func TestHeldKeyThroughMenuToggle(t *testing.T) {
	g := newInputTestGame(t)
	g.Input.SetKeyDest(input.KeyGame)
	g.Menu = menu.NewManager(nil, g.Input, g.Host.CVar)
	g.Input.OnMenuKey = g.handleMenuKeyEvent
	g.Menu.SetCommandText(g.Host.Cmd.AddText)

	// Hold forward and strafe, then open the menu with escape while they are
	// still physically held.
	pressKey(t, g, int('w'))
	pressKey(t, g, input.KAlt)
	pressKey(t, g, input.KEscape)

	// Opening the menu must release both buttons immediately.
	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("menu open: forward state = %d, want released", g.Client.InputForward.State)
	}
	if g.Client.InputStrafe.State&1 != 0 {
		t.Fatalf("menu open: strafe state = %d, want released", g.Client.InputStrafe.State)
	}

	// Physically release the keys while still in menu mode (their key-up is
	// routed to the menu handler and consumed there).
	releaseKey(t, g, int('w'))
	releaseKey(t, g, input.KAlt)

	// Close the menu.
	pressKey(t, g, input.KEscape)
	if g.Input.KeyDest() != input.KeyGame {
		t.Fatalf("menu close: keyDest = %v, want game", g.Input.KeyDest())
	}

	// Both keys must work again (guards "strafe only works once").
	for _, tc := range []struct {
		name   string
		key    int
		button *client.KButton
	}{
		{name: "forward", key: int('w'), button: &g.Client.InputForward},
		{name: "strafe", key: input.KAlt, button: &g.Client.InputStrafe},
	} {
		pressKey(t, g, tc.key)
		if tc.button.State&1 == 0 {
			t.Fatalf("%s re-press: state = %d, want down", tc.name, tc.button.State)
		}
		releaseKey(t, g, tc.key)
		if tc.button.State&1 != 0 {
			t.Fatalf("%s re-release: state = %d, want up", tc.name, tc.button.State)
		}
	}
}

// TestFireHeldThroughMenuToggle validates the attack-latch regression: holding
// mouse1 (attack) while opening the menu must not keep the player firing, and
// attack must work again after the menu closes.
func TestFireHeldThroughMenuToggle(t *testing.T) {
	g := newInputTestGame(t)
	g.Input.SetKeyDest(input.KeyGame)
	g.Menu = menu.NewManager(nil, g.Input, g.Host.CVar)
	g.Input.OnMenuKey = g.handleMenuKeyEvent
	g.Menu.SetCommandText(g.Host.Cmd.AddText)

	pressKey(t, g, input.KMouse1)
	pressKey(t, g, input.KEscape)

	if g.Client.InputAttack.State&1 != 0 {
		t.Fatalf("menu open: attack state = %d, want released", g.Client.InputAttack.State)
	}
	releaseKey(t, g, input.KMouse1)

	pressKey(t, g, input.KEscape)
	if g.Input.KeyDest() != input.KeyGame {
		t.Fatalf("menu close: keyDest = %v, want game", g.Input.KeyDest())
	}

	pressKey(t, g, input.KMouse1)
	if g.Client.InputAttack.State&1 == 0 {
		t.Fatalf("attack re-press: state = %d, want down", g.Client.InputAttack.State)
	}
	releaseKey(t, g, input.KMouse1)
	if g.Client.InputAttack.State&1 != 0 {
		t.Fatalf("attack re-release: state = %d, want up", g.Client.InputAttack.State)
	}
}

// TestStaleHeldKeysClearedOnMenuOpen guards the in-game regression where keys
// held while the menu opens remain latched (their physical key-ups are routed
// to the menu handler and swallowed). The latched button then keeps the player
// moving/firing, and the stale physical key state filters the next press as an
// OS key-repeat until escape "resets" things. Opening the menu must clear both
// the button latch and the physical key state immediately.
func TestStaleHeldKeysClearedOnMenuOpen(t *testing.T) {
	g := newInputTestGame(t)
	g.Input.SetKeyDest(input.KeyGame)
	g.Menu = menu.NewManager(nil, g.Input, g.Host.CVar)
	g.Input.OnMenuKey = g.handleMenuKeyEvent
	g.Menu.SetCommandText(g.Host.Cmd.AddText)

	// Simulate a stale physical state: W was pressed, but its key-up never
	// arrived (e.g. it was swallowed by an earlier menu focus change).
	g.Input.HandleKeyEvent(input.KeyEvent{Key: int('w'), Down: true})
	pressKey(t, g, input.KEscape) // open menu -> must release + clear

	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("after menu open: forward state = %d, want released", g.Client.InputForward.State)
	}
	if g.Input.IsKeyDown(int('w')) {
		t.Fatal("after menu open: physical W state still set, want cleared")
	}
	if g.Input.KeyDest() != input.KeyMenu {
		t.Fatalf("after escape: keyDest = %v, want menu", g.Input.KeyDest())
	}

	// Close the menu and verify W works again (not filtered as a repeat).
	pressKey(t, g, input.KEscape)
	if g.Input.KeyDest() != input.KeyGame {
		t.Fatalf("menu close: keyDest = %v, want game", g.Input.KeyDest())
	}
	pressKey(t, g, int('w'))
	if g.Client.InputForward.State&1 == 0 {
		t.Fatalf("post-reset forward: state = %d, want down (stale state filtered re-press)", g.Client.InputForward.State)
	}
	releaseKey(t, g, int('w'))
	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("post-reset forward release: state = %d, want up", g.Client.InputForward.State)
	}
}

// TestMenuMouseClickAdvancesExactlyOnePage guards the double-dispatch
// regression: a single physical mouse click must advance the menu exactly one
// page. When the gogpu input backend double-delivered a click (once via the
// EventSource callback path and once via raw polling), clicking "Single
// Player" instantly started a new game, skipping the New Game/Load Game page.
func TestMenuMouseClickAdvancesExactlyOnePage(t *testing.T) {
	g := newInputTestGame(t)
	g.Menu = menu.NewManager(nil, g.Input, g.Host.CVar)
	g.Input.OnMenuKey = g.handleMenuKeyEvent
	g.Menu.SetCommandText(g.Host.Cmd.AddText)

	g.Menu.ToggleMenu()
	if g.Input.KeyDest() != input.KeyMenu {
		t.Fatalf("menu toggle: keyDest = %v, want menu", g.Input.KeyDest())
	}
	g.Menu.ShowState(menu.MenuMain)

	// One physical click = one down + one up, delivered exactly once by the
	// (fixed) backend. Main -> SinglePlayer must happen exactly once.
	pressKey(t, g, input.KMouse1)
	releaseKey(t, g, input.KMouse1)

	if got := g.Menu.State(); got != menu.MenuSinglePlayer {
		t.Fatalf("after one click: menu state = %v, want singleplayer — click double-advanced the menu", got)
	}
}

// TestSinglePlayerMenuNewGameRequiresSecondClick guards that reaching the
// New Game page requires an explicit second click/Enter: after one click on
// "Single Player", the menu must sit on the New Game/Load Game page and NOT
// auto-start a new game.
func TestSinglePlayerMenuNewGameRequiresSecondClick(t *testing.T) {
	g := newInputTestGame(t)
	g.Menu = menu.NewManager(nil, g.Input, g.Host.CVar)
	g.Input.OnMenuKey = g.handleMenuKeyEvent
	var cmds []string
	g.Menu.SetCommandText(func(text string) { cmds = append(cmds, text) })

	g.Menu.ToggleMenu()
	g.Menu.ShowState(menu.MenuMain)

	// First action: select Single Player from the Main menu.
	pressKey(t, g, input.KEnter)
	releaseKey(t, g, input.KEnter)
	if g.Menu.State() != menu.MenuSinglePlayer {
		t.Fatalf("after first Enter: menu state = %v, want singleplayer", g.Menu.State())
	}
	if len(cmds) != 0 {
		t.Fatalf("Enter on Main menu queued commands = %v, want none until New Game selected", cmds)
	}

	// Second action on the SinglePlayer page: New Game (cursor 0) starts the
	// map exactly once and hides the menu.
	pressKey(t, g, input.KEnter)
	if g.Menu.IsActive() {
		t.Fatal("menu should hide after selecting New Game")
	}
	mapStarts := 0
	for _, c := range cmds {
		if c == "map start\n" {
			mapStarts++
		}
	}
	if mapStarts != 1 {
		t.Fatalf("map start commands = %d, want exactly 1", mapStarts)
	}
}

// TestInGameMovementRepeatedPressRelease exercises the FULL in-game chain:
// key event -> input System -> runtime frame -> client AccumulateCmd/BaseMove
// -> PendingCmd.Forward. It guards the regression where in-game movement only
// worked once until escape "reset" the input state: repeated press/release
// cycles must each produce forward movement only while held.
func TestInGameMovementRepeatedPressRelease(t *testing.T) {
	g := newInputTestGame(t)
	g.Input.SetKeyDest(input.KeyGame)
	g.Client.State = client.StateActive

	// A fully-signed-on client emits movement in AccumulateCmd (the loopback
	// client's per-frame Frame() calls this).
	g.Client.Signon = client.Signons

	// Use the mute backend so pollRuntimeInputEvents NoOps while we drive the
	// exact command-accumulation path the loopback client uses each frame.
	if err := g.Input.SetBackend(&noopInputBackend{}); err != nil {
		t.Fatalf("set backend: %v", err)
	}
	runFrame := func() {
		g.Client.AccumulateCmd(0.016)
	}

	// Warm one frame.
	runFrame()

	for i := 0; i < 5; i++ {
		pressKey(t, g, int('w'))
		runFrame()
		if g.Client.PendingCmd.Forward <= 0 {
			t.Fatalf("cycle %d press: PendingCmd.Forward = %v, want > 0 (forward not registering)", i, g.Client.PendingCmd.Forward)
		}

		releaseKey(t, g, int('w'))
		runFrame()
		if g.Client.PendingCmd.Forward != 0 {
			t.Fatalf("cycle %d release: PendingCmd.Forward = %v, want 0 (forward stuck)", i, g.Client.PendingCmd.Forward)
		}
	}
}

// noopInputBackend is a minimal input.Backend that never produces events,
// letting the runtime frame path run without a real window.
type noopInputBackend struct{}

func (b *noopInputBackend) Init() error                         { return nil }
func (b *noopInputBackend) Shutdown()                           {}
func (b *noopInputBackend) PollEvents() bool                    { return true }
func (b *noopInputBackend) MouseDelta() (int32, int32)          { return 0, 0 }
func (b *noopInputBackend) MousePosition() (int32, int32, bool) { return 0, 0, false }
func (b *noopInputBackend) ModifierState() input.ModifierState  { return input.ModifierState{} }
func (b *noopInputBackend) SetTextMode(input.TextMode)          {}
func (b *noopInputBackend) SetCursorMode(input.CursorMode)      {}
func (b *noopInputBackend) ShowKeyboard(bool)                   {}
func (b *noopInputBackend) GamepadState(int) input.GamepadState { return input.GamepadState{} }
func (b *noopInputBackend) IsGamepadConnected(int) bool         { return false }
func (b *noopInputBackend) SetMouseGrab(bool)                   {}
func (b *noopInputBackend) SetWindow(any)                       {}

// TestApplyStartupGameplayInputModeNoopWhenAlreadyActive guards the in-game
// regression where ApplyStartupGameplayInputMode was invoked every frame and
// its unconditional ClearKeyStates() dropped the release of any held key,
// latching the button until escape reset it. Applied only on the transition
// INTO active gameplay (as C does: key states are cleared only on input-grab
// begin/end), an already-active client must leave key state untouched.
func TestApplyStartupGameplayInputModeNoopWhenAlreadyActive(t *testing.T) {
	g := newInputTestGame(t)
	g.Input.SetKeyDest(input.KeyGame)
	g.Client.State = client.StateActive
	g.Client.Signon = client.Signons

	// Press forward.
	pressKey(t, g, int('w'))
	if g.Client.InputForward.State&1 == 0 {
		t.Fatal("after press: forward should be down")
	}

	// Re-applying startup gameplay input mode on an already-active client must
	// not clear the physical key state (that dropped the upcoming release).
	g.ApplyStartupGameplayInputMode()
	if !g.Input.IsKeyDown(int('w')) {
		t.Fatal("ApplyStartupGameplayInputMode on an active client must not clear physical key state")
	}

	// The physical release must still release the button.
	releaseKey(t, g, int('w'))
	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("forward state = %d after release, want up — release dropped, button latched", g.Client.InputForward.State)
	}

	// And a fresh press must work again.
	pressKey(t, g, int('w'))
	if g.Client.InputForward.State&1 == 0 {
		t.Fatalf("re-press: forward state = %d, want down", g.Client.InputForward.State)
	}
	releaseKey(t, g, int('w'))
	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("re-release: forward state = %d, want up", g.Client.InputForward.State)
	}
}

// TestProcessConsoleCommandsDoesNotClearKeysEveryFrame guards the every-frame
// call site that caused the regression: ProcessConsoleCommands must only apply
// the startup gameplay input mode on a client transition, keeping per-frame
// key state intact during active gameplay.
func TestProcessConsoleCommandsDoesNotClearKeysEveryFrame(t *testing.T) {
	g := newInputTestGame(t)
	g.Input.SetKeyDest(input.KeyGame)
	g.Client.State = client.StateActive
	g.Client.Signon = client.Signons

	// Prevent nil-panic in the used callbacks path but keep it minimal.
	g.Host.Cmd.AddText("")

	// First transition applies startup mode (state: disconnected -> active).
	cb := gameCallbacks{g: g}
	cb.ProcessConsoleCommands()

	// Press forward after entering gameplay.
	pressKey(t, g, int('w'))
	wasDown := g.Client.InputForward.State&1 != 0
	if !wasDown {
		t.Fatal("forward should be down after press")
	}

	// A subsequent ProcessConsoleCommands frame must NOT clear the physical
	// key state (client already active: no transition).
	cb.ProcessConsoleCommands()

	if !g.Input.IsKeyDown(int('w')) {
		t.Fatal("ProcessConsoleCommands on an active frame must not clear physical key state")
	}

	// Release must still work.
	releaseKey(t, g, int('w'))
	if g.Client.InputForward.State&1 != 0 {
		t.Fatalf("forward state = %d after release, want up", g.Client.InputForward.State)
	}
}
