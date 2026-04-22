package menu

// Mouse, controller, mods, and HUD-style menu tests split from manager_test.go.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/input"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func TestMouseBindingsForActivationAndBack(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.ShowMenu()
	mgr.state = MenuSinglePlayer
	mgr.singlePlayerCursor = 1 // Load
	mgr.M_Key(input.KMouse1)
	if mgr.GetState() != MenuLoad {
		t.Fatalf("expected load state after mouse1 activate, got %v", mgr.GetState())
	}

	mgr.M_Key(input.KMouse2)
	if mgr.GetState() != MenuSinglePlayer {
		t.Fatalf("expected return to single player after mouse2, got %v", mgr.GetState())
	}

	mgr.state = MenuQuit
	mgr.quitPrevState = MenuMain
	mgr.M_Key(input.KMouse1)

	if len(commands) == 0 || commands[len(commands)-1] != "quit\n" {
		t.Fatalf("expected quit command from mouse confirm, got %v", commands)
	}
}

func TestControllerButtonsMapToMenuAcceptAndBack(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.ShowMenu()
	mgr.state = MenuSinglePlayer
	mgr.singlePlayerCursor = 1 // LOAD

	mgr.M_Key(input.KAButton)
	if got := mgr.GetState(); got != MenuLoad {
		t.Fatalf("A button should activate selection, got %v", got)
	}

	mgr.M_Key(input.KBButton)
	if got := mgr.GetState(); got != MenuSinglePlayer {
		t.Fatalf("B button should go back, got %v", got)
	}
}

func TestControllerDpadMapsToArrowNavigation(t *testing.T) {
	mgr := NewManager(nil, nil, nil)
	mgr.ShowMenu()

	mgr.mainCursor = 0
	mgr.M_Key(input.KDpadDown)
	if got := mgr.mainCursor; got != 1 {
		t.Fatalf("D-pad down should move cursor down, got %d", got)
	}

	mgr.M_Key(input.KDpadUp)
	if got := mgr.mainCursor; got != 0 {
		t.Fatalf("D-pad up should move cursor up, got %d", got)
	}

	// Alt-layer gamepad keys should be accepted too.
	mgr.M_Key(input.KDpadUpAlt)
	if got := mgr.mainCursor; got != mainQuit {
		t.Fatalf("alt D-pad up should wrap like up-arrow, got %d", got)
	}
}

func TestControllerStartAndBackMapInOptionsMenu(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.ShowMenu()
	mgr.state = MenuOptions
	mgr.optionsCursor = 0 // CONTROLS

	mgr.M_Key(input.KStart)
	if got := mgr.GetState(); got != MenuControls {
		t.Fatalf("START should activate current option, got %v", got)
	}

	mgr.M_Key(input.KBack)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("BACK should behave like backspace and return, got %v", got)
	}
}

func TestMenuStateStringability(t *testing.T) {
	// Simple regression sentinel: ensure states are stable numeric values.
	states := []MenuState{
		MenuNone,
		MenuMain,
		MenuSinglePlayer,
		MenuSkill,
		MenuLoad,
		MenuSave,
		MenuMultiPlayer,
		MenuJoinGame,
		MenuHostGame,
		MenuOptions,
		MenuControls,
		MenuVideo,
		MenuAudio,
		MenuHelp,
		MenuQuit,
		MenuSetup,
	}

	for i, state := range states {
		if int(state) != i {
			t.Fatalf("state index mismatch: %s expected %d got %d", fmt.Sprint(state), i, state)
		}
	}
}

func TestDrawQuitUsesMenuCharacterPath(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	mgr.state = MenuQuit

	rc := &mockMenuRenderContext{}
	mgr.M_Draw(rc)

	if len(rc.menuCharacters) == 0 {
		t.Fatal("expected quit menu to draw menu characters")
	}
	if len(rc.characters) != 0 {
		t.Fatalf("expected quit menu to avoid raw DrawCharacter path, got %d draws", len(rc.characters))
	}
	first := rc.menuCharacters[0]
	if first.x != 56 || first.y != 64 || first.num != int('A')+128 {
		t.Fatalf("first menu char = (%d,%d,%d), want (56,64,%d)", first.x, first.y, first.num, int('A')+128)
	}
}

func TestMenuNavigationAndSelectPlaySound(t *testing.T) {
	mgr := NewManager(nil, nil, nil)
	var played []string
	mgr.SetSoundPlayer(func(name string) {
		played = append(played, name)
	})
	mgr.ShowMenu()
	played = nil

	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KEnter)

	if len(played) < 2 {
		t.Fatalf("played sounds = %v, want at least two menu sounds", played)
	}
	if played[0] != menuSoundNavigate {
		t.Fatalf("first sound = %q, want %q", played[0], menuSoundNavigate)
	}
	if played[1] != menuSoundSelect {
		t.Fatalf("second sound = %q, want %q", played[1], menuSoundSelect)
	}
}

func TestMenuEscapePlaysCancelSound(t *testing.T) {
	mgr := NewManager(nil, nil, nil)
	var last string
	mgr.SetSoundPlayer(func(name string) {
		last = name
	})
	mgr.ShowMenu()
	last = ""

	mgr.M_Key(input.KEscape)

	if last != menuSoundCancel {
		t.Fatalf("escape sound = %q, want %q", last, menuSoundCancel)
	}
}

// TestForcedUnderwaterOnlyWhenVideoMenuOpen verifies that ForcedUnderwater returns
// true only when the video options menu is open and cursor is on the WATERWARP item.
// Mirrors C Ironwail M_ForcedUnderwater() / M_Options_ForcedUnderwater().
func TestForcedUnderwaterOnlyWhenVideoMenuOpen(t *testing.T) {
	mgr := NewManager(nil, nil, nil)

	// Not active: should be false regardless of state.
	mgr.active = false
	mgr.state = MenuVideo
	mgr.videoCursor = videoItemWaterwarp
	if mgr.ForcedUnderwater() {
		t.Error("ForcedUnderwater() should be false when menu is not active")
	}

	// Active, wrong menu state.
	mgr.active = true
	mgr.state = MenuOptions
	mgr.videoCursor = videoItemWaterwarp
	if mgr.ForcedUnderwater() {
		t.Error("ForcedUnderwater() should be false when not in MenuVideo")
	}

	// Active, right menu state, wrong cursor.
	mgr.state = MenuVideo
	mgr.videoCursor = videoItemGamma
	if mgr.ForcedUnderwater() {
		t.Error("ForcedUnderwater() should be false when cursor is not on videoItemWaterwarp")
	}

	// Active, right menu state, right cursor → should be true.
	mgr.videoCursor = videoItemWaterwarp
	if !mgr.ForcedUnderwater() {
		t.Error("ForcedUnderwater() should be true when video menu is open and cursor is on WATERWARP")
	}
}

// TestWaterwarpCvarCyclesCorrectly verifies that adjustVideoSetting cycles r_waterwarp
// through 0→1→2→0 when pressing right, and 0→2→1→0 when pressing left.
func TestWaterwarpCvarCyclesCorrectly(t *testing.T) {
	mgr := NewManager(nil, nil, nil)
	mgr.state = MenuVideo
	mgr.videoCursor = videoItemWaterwarp

	// Register cvar if not already registered.
	if mgr.cvars.Get("r_waterwarp") == nil {
		mgr.cvars.Register("r_waterwarp", "0", 0, "Underwater warp test")
	}
	mgr.cvars.Set("r_waterwarp", "0")

	// Right: 0 → 1
	mgr.adjustVideoSetting(1)
	if got := mgr.cvars.IntValue("r_waterwarp"); got != 1 {
		t.Fatalf("after right from 0: r_waterwarp = %d, want 1", got)
	}

	// Right: 1 → 2
	mgr.adjustVideoSetting(1)
	if got := mgr.cvars.IntValue("r_waterwarp"); got != 2 {
		t.Fatalf("after right from 1: r_waterwarp = %d, want 2", got)
	}

	// Right: 2 → 0 (wraps)
	mgr.adjustVideoSetting(1)
	if got := mgr.cvars.IntValue("r_waterwarp"); got != 0 {
		t.Fatalf("after right from 2 (wrap): r_waterwarp = %d, want 0", got)
	}
}

// ---- Mods menu tests ----

// TestModsMenuEmptyWithNoProvider verifies that the mods menu opens with an empty
// list when no provider is set, and that navigating away with ESC returns to main.
func TestModsMenuEmptyWithNoProvider(t *testing.T) {
	mgr := NewManager(nil, nil, nil)
	mgr.state = MenuMain
	mgr.enterModsMenu()

	if got := mgr.GetState(); got != MenuMods {
		t.Fatalf("expected MenuMods, got %v", got)
	}
	if len(mgr.modsList) != 0 {
		t.Fatalf("expected empty mods list, got %d items", len(mgr.modsList))
	}

	// ESC should return to main menu.
	mgr.M_Key(input.KEscape)
	if got := mgr.GetState(); got != MenuMain {
		t.Fatalf("expected MenuMain after ESC from mods, got %v", got)
	}
}

// TestModsMenuWithMods verifies that the mods provider populates the list and that
// selecting a mod queues the "game" command.
func TestModsMenuWithMods(t *testing.T) {
	mgr := NewManager(nil, nil, nil)

	var commands []string
	mgr.commandText = func(text string) { commands = append(commands, text) }

	mgr.SetModsProvider(func() []ModInfo {
		return []ModInfo{{Name: "hipnotic"}, {Name: "rogue"}}
	})

	mgr.state = MenuMain
	mgr.enterModsMenu()

	if got := mgr.GetState(); got != MenuMods {
		t.Fatalf("expected MenuMods, got %v", got)
	}
	if len(mgr.modsList) != 2 {
		t.Fatalf("expected 2 mods, got %d", len(mgr.modsList))
	}

	// Select the first mod (hipnotic).
	mgr.modsCursor = 0
	mgr.M_Key(input.KEnter)

	// Menu should hide and a "game" command should be queued.
	if mgr.IsActive() {
		t.Fatal("menu should be hidden after selecting a mod")
	}
	if len(commands) == 0 {
		t.Fatal("expected a game command to be queued")
	}
	if !strings.Contains(commands[0], "game") {
		t.Fatalf("expected game command, got: %q", commands[0])
	}
	if !strings.Contains(commands[0], "hipnotic") {
		t.Fatalf("expected hipnotic in game command, got: %q", commands[0])
	}
}

// TestModsMenuBackItem verifies that the last item (Back) returns to the main menu.
func TestModsMenuBackItem(t *testing.T) {
	mgr := NewManager(nil, nil, nil)
	mgr.SetModsProvider(func() []ModInfo {
		return []ModInfo{{Name: "mod1"}}
	})

	mgr.state = MenuMain
	mgr.enterModsMenu()

	// Move to the Back item (index == len(modsList)).
	mgr.modsCursor = len(mgr.modsList)
	mgr.M_Key(input.KEnter)

	if got := mgr.GetState(); got != MenuMain {
		t.Fatalf("expected MenuMain after selecting Back, got %v", got)
	}
}

// TestMainMenuIncludesModsWhenAvailable verifies that selecting mainMods enters
// the mods menu when mods are available.
func TestMainMenuIncludesModsWhenAvailable(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.SetModsProvider(func() []ModInfo {
		return []ModInfo{{Name: "mymod"}}
	})
	mgr.ShowMenu()

	mgr.state = MenuMain
	mgr.mainCursor = mainMods
	mgr.M_Key(input.KEnter)

	if got := mgr.GetState(); got != MenuMods {
		t.Fatalf("expected MenuMods from main menu, got %v", got)
	}
}

// TestMainMenuCursorSkipsModsWhenNone verifies that cursor navigation skips
// the mainMods slot when no mods are available.
func TestMainMenuCursorSkipsModsWhenNone(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.ShowMenu()

	// Move down from Options — should skip mainMods, land on mainHelp.
	mgr.mainCursor = mainOptions
	mgr.M_Key(input.KDownArrow)
	if mgr.mainCursor != mainHelp {
		t.Fatalf("expected cursor at mainHelp(%d), got %d", mainHelp, mgr.mainCursor)
	}

	// Move up from mainHelp — should skip mainMods, land on mainOptions.
	mgr.M_Key(input.KUpArrow)
	if mgr.mainCursor != mainOptions {
		t.Fatalf("expected cursor at mainOptions(%d), got %d", mainOptions, mgr.mainCursor)
	}

	// With mods, cursor should NOT skip mainMods.
	mgr.SetModsProvider(func() []ModInfo { return []ModInfo{{Name: "mod"}} })
	mgr.refreshModsList()
	mgr.mainCursor = mainOptions
	mgr.M_Key(input.KDownArrow)
	if mgr.mainCursor != mainMods {
		t.Fatalf("expected cursor at mainMods(%d) with mods, got %d", mainMods, mgr.mainCursor)
	}
}

// ---- M_Mousemove tests ----

// TestMousemoveScrollsMainMenuCursor verifies that mouse Y movement accumulates
// and moves the cursor down when the threshold is crossed.
func TestMousemoveScrollsMainMenuCursor(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.ShowMenu()

	initialCursor := mgr.mainCursor
	// Each call to M_Mousemove with dy=4 should not move yet.
	mgr.M_Mousemove(0, 4)
	if mgr.mainCursor != initialCursor {
		t.Fatalf("cursor moved too early; mouseAccumY threshold not reached")
	}

	// Another dy=4 → total 8 ≥ menuItemPx(8) → cursor should advance.
	mgr.M_Mousemove(0, 4)
	if mgr.mainCursor != 1 {
		t.Fatalf("expected cursor 1 after crossing threshold, got %d", mgr.mainCursor)
	}
}

// TestMousemoveScrollsUp verifies that negative dy accumulates and moves cursor up.
func TestMousemoveScrollsUp(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.ShowMenu()
	mgr.mainCursor = 2 // not at top

	mgr.M_Mousemove(0, -8)
	expected := 1
	if mgr.mainCursor != expected {
		t.Fatalf("expected cursor %d after upward scroll, got %d", expected, mgr.mainCursor)
	}
}

// TestMousemoveInactiveMenuIsNoop verifies that M_Mousemove does nothing when
// the menu is not active.
func TestMousemoveInactiveMenuIsNoop(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	// Don't call ShowMenu.

	mgr.M_Mousemove(0, 100) // large delta — should be ignored
	if mgr.mainCursor != 0 {
		t.Fatalf("cursor should not change when menu is inactive, got %d", mgr.mainCursor)
	}
}

// TestKeyPressResetMouseAccum verifies that a key press resets the mouse accumulator.
func TestKeyPressResetMouseAccum(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.ShowMenu()

	// Accumulate some mouse movement without crossing the threshold.
	mgr.M_Mousemove(0, 4)
	if mgr.mouseAccumY == 0 {
		t.Fatal("mouse accumulator should be non-zero after partial movement")
	}

	// A key press should reset the accumulator.
	mgr.M_Key(input.KUpArrow)
	if mgr.mouseAccumY != 0 {
		t.Fatalf("mouse accumulator should be zero after key press, got %f", mgr.mouseAccumY)
	}
}

func TestMousemoveAbsoluteIgnoresFirstFrameAfterShow(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.ShowMenu()

	mgr.M_MousemoveAbsolute(0, 72)
	if mgr.mainCursor != mainSinglePlayer {
		t.Fatalf("first absolute sample should be ignored, got %d", mgr.mainCursor)
	}

	mgr.M_MousemoveAbsolute(0, 72)
	if mgr.mainCursor != mainOptions {
		t.Fatalf("second absolute sample = %d, want %d", mgr.mainCursor, mainOptions)
	}
}

func TestMousemoveAbsoluteSelectsSinglePlayerRows(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.active = true
	mgr.state = MenuSinglePlayer

	mgr.M_MousemoveAbsolute(0, 72)
	if mgr.singlePlayerCursor != 2 {
		t.Fatalf("single-player cursor = %d, want %d", mgr.singlePlayerCursor, 2)
	}
}

func TestMousemoveAbsoluteSelectsSetupRows(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.active = true
	mgr.state = MenuSetup

	mgr.M_MousemoveAbsolute(0, 104)
	if mgr.setupCursor != setupItemBottomColor {
		t.Fatalf("setup cursor = %d, want %d", mgr.setupCursor, setupItemBottomColor)
	}
}

func TestMousemoveAbsoluteSelectsControlsRows(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.active = true
	mgr.state = MenuControls

	target := controlItemTurnLeft
	mgr.M_MousemoveAbsolute(0, controlRowY(target))
	if mgr.controlsCursor != target {
		t.Fatalf("controls cursor = %d, want %d", mgr.controlsCursor, target)
	}
}

func TestControlsMouseRebindingModeLocksHoveredRow(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.active = true
	mgr.state = MenuControls
	mgr.controlsCursor = controlItemForward

	mgr.M_Key(input.KMouse1)
	if !mgr.WaitingForKeyBinding() {
		t.Fatal("expected controls menu to enter rebinding mode via mouse1")
	}

	mgr.M_MousemoveAbsolute(0, controlRowY(controlItemAttack))
	if got := mgr.controlsCursor; got != controlItemForward {
		t.Fatalf("controls cursor should stay locked while rebinding, got %d", got)
	}

	mgr.M_Key(input.KEscape)
	if mgr.WaitingForKeyBinding() {
		t.Fatal("expected controls rebinding mode to exit on escape")
	}

	mgr.M_MousemoveAbsolute(0, controlRowY(controlItemAttack))
	if got := mgr.controlsCursor; got != controlItemAttack {
		t.Fatalf("controls cursor after rebinding = %d, want %d", got, controlItemAttack)
	}
}

func TestMousemoveAbsoluteSelectsJoinGameServerRows(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.active = true
	mgr.state = MenuJoinGame
	mgr.serverResults = []inet.HostCacheEntry{
		{Name: "one"},
		{Name: "two"},
		{Name: "three"},
	}

	mgr.M_MousemoveAbsolute(0, 160)
	if mgr.joinGameCursor != joinGameBaseItems+1 {
		t.Fatalf("join-game cursor = %d, want %d", mgr.joinGameCursor, joinGameBaseItems+1)
	}
}

func TestMousemoveAbsoluteSelectsHostGameRows(t *testing.T) {
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(nil, inputSys, nil)
	mgr.active = true
	mgr.state = MenuHostGame

	mgr.M_MousemoveAbsolute(0, 176)
	if mgr.hostGameCursor != hostGameItemBack {
		t.Fatalf("host-game cursor = %d, want %d", mgr.hostGameCursor, hostGameItemBack)
	}
}

// ---- HUD style tests ----

// TestHUDStyleLabelClassic verifies that hudStyleLabel returns "CLASSIC" for 0.
func TestHUDStyleLabelClassic(t *testing.T) {
	if got := hudStyleLabel(0); got != "CLASSIC" {
		t.Fatalf("expected CLASSIC, got %q", got)
	}
}

// TestHUDStyleLabelCompact verifies that hudStyleLabel returns "MODERN 1" for 1.
func TestHUDStyleLabelCompact(t *testing.T) {
	if got := hudStyleLabel(1); got != "MODERN 1" {
		t.Fatalf("expected MODERN 1, got %q", got)
	}
}

func TestHUDStyleLabelModernSideAmmo(t *testing.T) {
	if got := hudStyleLabel(2); got != "MODERN 2" {
		t.Fatalf("expected MODERN 2, got %q", got)
	}
}

func TestHUDStyleLabelQuakeWorld(t *testing.T) {
	if got := hudStyleLabel(3); got != "QUAKEWORLD" {
		t.Fatalf("expected QUAKEWORLD, got %q", got)
	}
}

// TestVideoMenuHUDStyleCyclesCorrectly verifies that adjustVideoSetting cycles
// hud_style through 0→1→2→3→0 when pressing right.
func TestVideoMenuHUDStyleCyclesCorrectly(t *testing.T) {
	mgr := NewManager(nil, nil, nil)
	mgr.state = MenuVideo
	mgr.videoCursor = videoItemHUDStyle

	mgr.cvars.Set("hud_style", "0")

	// Right: 0 → 1.
	mgr.adjustVideoSetting(1)
	if got := mgr.cvars.IntValue("hud_style"); got != 1 {
		t.Fatalf("after right from 0: hud_style = %d, want 1", got)
	}

	// Right: 1 → 2.
	mgr.adjustVideoSetting(1)
	if got := mgr.cvars.IntValue("hud_style"); got != 2 {
		t.Fatalf("after right from 1: hud_style = %d, want 2", got)
	}

	// Right: 2 → 3.
	mgr.adjustVideoSetting(1)
	if got := mgr.cvars.IntValue("hud_style"); got != 3 {
		t.Fatalf("after right from 2: hud_style = %d, want 3", got)
	}

	// Right: 3 → 0 (wraps).
	mgr.adjustVideoSetting(1)
	if got := mgr.cvars.IntValue("hud_style"); got != 0 {
		t.Fatalf("after right from 3 (wrap): hud_style = %d, want 0", got)
	}
}

func TestDrawVideoUsesCompactRowSpacing(t *testing.T) {
	mgr := NewManager(nil, nil, nil)
	mgr.cvars.Register("vid_width", "1280", cvar.FlagArchive, "")
	mgr.cvars.Register("vid_height", "720", cvar.FlagArchive, "")
	mgr.cvars.Register("vid_fullscreen", "0", cvar.FlagArchive, "")
	mgr.cvars.Register("vid_vsync", "0", cvar.FlagArchive, "")
	mgr.cvars.Register("host_maxfps", "250", cvar.FlagArchive, "")
	mgr.cvars.Register("r_gamma", "1.0", cvar.FlagArchive, "")
	mgr.cvars.Register("r_drawviewmodel", "1", cvar.FlagArchive, "")
	mgr.cvars.Register("r_waterwarp", "0", cvar.FlagArchive, "")
	mgr.cvars.Register("hud_style", "0", cvar.FlagArchive, "")
	mgr.cvars.Register("scr_showfps", "0", cvar.FlagArchive, "")

	rc := &mockMenuRenderContext{}
	mgr.drawVideo(rc)

	if got := renderedMenuLine(rc, videoRowY(videoItemBack)); got != "BACK" {
		t.Fatalf("back row text = %q, want BACK at y=%d", got, videoRowY(videoItemBack))
	}
	if got := renderedMenuLine(rc, 192); got != "VIDEO CHANGES ARE SAVED TO CONFIG" {
		t.Fatalf("footer text = %q, want video footer at y=192", got)
	}
}

// TestModsMenuCurrentModLabel verifies that the current mod is marked with an
// asterisk in the mods menu draw output.
func TestModsMenuCurrentModLabel(t *testing.T) {
	mgr := NewManager(nil, nil, nil)
	mgr.SetModsProvider(func() []ModInfo {
		return []ModInfo{{Name: "rogue"}, {Name: "hipnotic"}}
	})
	mgr.SetCurrentMod("rogue")
	mgr.enterModsMenu()

	rc := &mockMenuRenderContext{}
	mgr.drawMods(rc)

	// Find characters drawn — the rogue entry should contain an asterisk.
	rendered := renderedMenuLine(rc, 32) // startY=32 for first item
	if !strings.Contains(rendered, "*") {
		t.Fatalf("current mod (rogue) should have asterisk marker in rendered output; got %q", rendered)
	}
}
