package menu

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// testDrawManager is a minimal menu.DrawManager that returns no pics.
type testDrawManager struct{}

func (testDrawManager) Pic(name string) *qimage.QPic { return nil }

// testInputBackend is a minimal input.Backend.
type testInputBackend struct{}

func (testInputBackend) Init() error                                        { return nil }
func (testInputBackend) Shutdown()                                          {}
func (testInputBackend) PollEvents() bool                                   { return true }
func (testInputBackend) MouseDelta() (dx, dy int32)                         { return 0, 0 }
func (testInputBackend) MousePosition() (x, y int32, valid bool)            { return 0, 0, false }
func (testInputBackend) ModifierState() input.ModifierState                 { return input.ModifierState{} }
func (testInputBackend) SetTextMode(mode input.TextMode)                    {}
func (testInputBackend) SetCursorMode(mode input.CursorMode)                {}
func (testInputBackend) ShowKeyboard(show bool)                            {}
func (testInputBackend) GamepadState(player int) input.GamepadState         { return input.GamepadState{} }
func (testInputBackend) IsGamepadConnected(player int) bool                 { return false }
func (testInputBackend) SetMouseGrab(grabbed bool)                         {}
func (testInputBackend) SetWindow(win any)                                 {}

// testConchars returns a minimal 128x128 conchars atlas.
func testConchars() []byte {
	data := make([]byte, 128*128)
	for i := range data {
		data[i] = byte(i / 64)
	}
	return data
}

// testMenuManager builds a menu.Manager with local test fixtures.
func testMenuManager(t *testing.T) *menu.Manager {
	t.Helper()
	inputSys := input.NewSystem(testInputBackend{})
	return menu.NewManager(testDrawManager{}, inputSys, nil)
}

// TestMenuRootMainPage asserts the main menu page exposes the expected labels
// and cursor position from a canned Manager state (AC1: same menu structure).
func TestMenuRootMainPage(t *testing.T) {
	mgr := testMenuManager(t)
	mgr.ShowMenu() // state = MenuMain, cursor = 0

	wt := widgets.NewQuakeText(testConchars(), nil)
	root := NewMenuRoot(mgr, wt)

	rows := root.Rows()
	if len(rows) < 5 {
		t.Fatalf("main menu rows = %d, want >= 5", len(rows))
	}
	want := []string{"SINGLE PLAYER", "MULTIPLAYER", "OPTIONS", "HELP", "QUIT"}
	for i, label := range want {
		if rows[i].Label != label {
			t.Fatalf("row %d label = %q, want %q", i, rows[i].Label, label)
		}
	}
	if root.Cursor() != 0 {
		t.Fatalf("Cursor() = %d, want 0", root.Cursor())
	}

	// Move cursor down via the manager action path.
	mgr.M_Key(input.KDownArrow)
	if root.Cursor() != 1 {
		t.Fatalf("Cursor() after down = %d, want 1", root.Cursor())
	}
}

// TestMenuRootSinglePlayerPage asserts the Single Player page rows.
func TestMenuRootSinglePlayerPage(t *testing.T) {
	mgr := testMenuManager(t)
	mgr.ShowState(menu.MenuSinglePlayer)

	wt := widgets.NewQuakeText(testConchars(), nil)
	root := NewMenuRoot(mgr, wt)

	rows := root.Rows()
	want := []string{"NEW GAME", "LOAD", "SAVE"}
	for i, label := range want {
		if rows[i].Label != label {
			t.Fatalf("row %d label = %q, want %q", i, rows[i].Label, label)
		}
	}
}

// TestMenuRootOptionsPage asserts the Options page rows.
func TestMenuRootOptionsPage(t *testing.T) {
	mgr := testMenuManager(t)
	mgr.ShowState(menu.MenuOptions)

	wt := widgets.NewQuakeText(testConchars(), nil)
	root := NewMenuRoot(mgr, wt)

	rows := root.Rows()
	want := []string{"CONTROLS", "VIDEO", "AUDIO", "VSYNC", "BACK"}
	for i, label := range want {
		if rows[i].Label != label {
			t.Fatalf("row %d label = %q, want %q", i, rows[i].Label, label)
		}
	}
}

// TestMenuRootSetupPage asserts the Setup page exposes the text buffers and
// color values.
func TestMenuRootSetupPage(t *testing.T) {
	mgr := testMenuManager(t)
	mgr.ShowState(menu.MenuSetup)

	wt := widgets.NewQuakeText(testConchars(), nil)
	root := NewMenuRoot(mgr, wt)

	rows := root.Rows()
	if len(rows) < 5 {
		t.Fatalf("setup rows = %d, want >= 5", len(rows))
	}
	// Hostname row shows the buffer value.
	foundHostname := false
	for _, r := range rows {
		if r.Label == "HOSTNAME" && r.Value == "UNNAMED" {
			foundHostname = true
		}
	}
	if !foundHostname {
		t.Fatal("setup hostname row missing or wrong value")
	}
}

// TestMenuRootHostPage asserts the Host Game page exposes the settings values.
func TestMenuRootHostPage(t *testing.T) {
	mgr := testMenuManager(t)
	mgr.ShowState(menu.MenuHostGame)

	wt := widgets.NewQuakeText(testConchars(), nil)
	root := NewMenuRoot(mgr, wt)

	rows := root.Rows()
	found := map[string]bool{}
	for _, r := range rows {
		found[r.Label] = true
	}
	for _, label := range []string{"MAX PLAYERS", "MODE", "TEAMPLAY", "SKILL", "FRAG LIMIT", "TIME LIMIT", "MAP", "START GAME", "BACK"} {
		if !found[label] {
			t.Fatalf("host page missing row %q", label)
		}
	}
}

// TestMenuRootLayout asserts the root lays out to the 320x200 menu viewport.
func TestMenuRootLayout(t *testing.T) {
	mgr := testMenuManager(t)
	mgr.ShowMenu()

	wt := widgets.NewQuakeText(testConchars(), nil)
	root := NewMenuRoot(mgr, wt)

	ctx := widget.NewContext()
	size := root.Layout(ctx, geometry.Tight(geometry.Sz(320, 200)))
	if size.Width != 320 || size.Height != 200 {
		t.Fatalf("Layout size = %v, want 320x200", size)
	}
}

// TestMenuRootEventToAction asserts widget key events map to the manager's
// M_Key action path (navigation preserved).
func TestMenuRootEventToAction(t *testing.T) {
	mgr := testMenuManager(t)
	mgr.ShowMenu()

	wt := widgets.NewQuakeText(testConchars(), nil)
	root := NewMenuRoot(mgr, wt)

	// Simulate a down-arrow key event routed to the manager.
	root.handleKey(input.KDownArrow)
	if mgr.CursorFor(menu.MenuMain) != 1 {
		t.Fatalf("CursorFor(MenuMain) after down = %d, want 1", mgr.CursorFor(menu.MenuMain))
	}
}
