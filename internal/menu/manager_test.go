package menu

import (
	"fmt"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/input"
)

// mockDrawManager is a mock implementation of DrawManager for testing.
type mockDrawManager struct{}

func (m *mockDrawManager) GetPic(name string) *image.QPic {
	return nil
}

type mockMenuRenderContext struct {
	characters     []struct{ x, y, num int }
	menuCharacters []struct{ x, y, num int }
}

func (m *mockMenuRenderContext) Clear(r, g, b, a float32)          {}
func (m *mockMenuRenderContext) DrawTriangle(r, g, b, a float32)   {}
func (m *mockMenuRenderContext) SurfaceView() interface{}          { return nil }
func (m *mockMenuRenderContext) Gamma() float32                    { return 1.0 }
func (m *mockMenuRenderContext) DrawPic(x, y int, pic *image.QPic) {}
func (m *mockMenuRenderContext) DrawMenuPic(x, y int, pic *image.QPic) {
}
func (m *mockMenuRenderContext) DrawFill(x, y, w, h int, color byte) {
}
func (m *mockMenuRenderContext) DrawCharacter(x, y int, num int) {
	m.characters = append(m.characters, struct{ x, y, num int }{x, y, num})
}
func (m *mockMenuRenderContext) DrawMenuCharacter(x, y int, num int) {
	m.menuCharacters = append(m.menuCharacters, struct{ x, y, num int }{x, y, num})
}

func TestNewManager(t *testing.T) {
	drawMgr := &mockDrawManager{}
	inputSys := input.NewSystem(nil)
	mgr := NewManager(drawMgr, inputSys)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	if mgr.IsActive() {
		t.Error("Menu should not be active initially")
	}

	if mgr.GetState() != MenuNone {
		t.Error("Initial state should be MenuNone")
	}
}

func TestToggleMenu(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	// Toggle menu on
	mgr.ToggleMenu()
	if !mgr.IsActive() {
		t.Error("Menu should be active after toggle")
	}
	if mgr.GetState() != MenuMain {
		t.Error("State should be MenuMain after toggle")
	}

	// Toggle menu off
	mgr.ToggleMenu()
	if mgr.IsActive() {
		t.Error("Menu should not be active after second toggle")
	}
	if mgr.GetState() != MenuNone {
		t.Error("State should be MenuNone after second toggle")
	}
}

func TestShowHideMenu(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)
	// Show menu
	mgr.ShowMenu()
	if !mgr.IsActive() {
		t.Error("Menu should be active after ShowMenu")
	}

	// Hide menu
	mgr.HideMenu()
	if mgr.IsActive() {
		t.Error("Menu should not be active after HideMenu")
	}
}

func TestMainMenuKey(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)
	mgr.ShowMenu()

	// Test up arrow wraps to last item.
	mgr.M_Key(input.KUpArrow)
	if mgr.mainCursor != 4 {
		t.Error("Up arrow should wrap cursor to end")
	}

	// Test down arrow wraps back to start.
	mgr.M_Key(input.KDownArrow)
	if mgr.mainCursor != 0 {
		t.Error("Down arrow should wrap cursor to start")
	}

	// Test escape closes menu.
	mgr.M_Key(input.KEscape)
	if mgr.IsActive() {
		t.Error("Escape should hide menu")
	}
}

func TestQuitMenu(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.ShowMenu()

	// Navigate to quit (item 4).
	mgr.M_Key(input.KDownArrow) // Item 1
	mgr.M_Key(input.KDownArrow) // Item 2
	mgr.M_Key(input.KDownArrow) // Item 3
	mgr.M_Key(input.KDownArrow) // Item 4 (Quit)
	mgr.M_Key(input.KEnter)     // Enter to select quit

	if mgr.GetState() != MenuQuit {
		t.Error("State should be MenuQuit after selecting quit")
	}

	// Backspace should cancel quit and return to previous state.
	mgr.M_Key(input.KBackspace)
	if mgr.GetState() != MenuMain {
		t.Error("Backspace should return to main menu")
	}

	// Confirm quit with Y.
	mgr.mainCursor = 4
	mgr.M_Key(input.KEnter)
	mgr.M_Key('y')

	if mgr.IsActive() {
		t.Fatal("Menu should hide after quit confirmation")
	}

	if len(commands) == 0 || commands[len(commands)-1] != "quit\n" {
		t.Fatalf("expected quit command, got %v", commands)
	}
}

func TestMainMenuSelections(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)
	mgr.ShowMenu()

	selections := []struct {
		cursor int
		want   MenuState
	}{
		{0, MenuSinglePlayer},
		{1, MenuMultiPlayer},
		{2, MenuOptions},
		{3, MenuHelp},
		{4, MenuQuit},
	}

	for _, tc := range selections {
		mgr.state = MenuMain
		mgr.mainCursor = tc.cursor
		mgr.M_Key(input.KEnter)
		if got := mgr.GetState(); got != tc.want {
			t.Fatalf("cursor %d: expected state %v, got %v", tc.cursor, tc.want, got)
		}
	}
}

func TestSinglePlayerActions(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.ShowMenu()
	mgr.M_Key(input.KEnter) // Main -> Single Player

	if mgr.GetState() != MenuSinglePlayer {
		t.Fatalf("expected single player state, got %v", mgr.GetState())
	}

	// New game selection queues core startup commands and exits menu.
	mgr.M_Key(input.KEnter)
	if mgr.IsActive() {
		t.Fatal("menu should hide when starting new game")
	}

	want := []string{"disconnect\n", "maxplayers 1\n", "deathmatch 0\n", "coop 0\n", "map start\n"}
	if len(commands) < len(want) {
		t.Fatalf("expected at least %d commands, got %d", len(want), len(commands))
	}
	for i, expected := range want {
		if commands[i] != expected {
			t.Fatalf("command %d: expected %q, got %q", i, expected, commands[i])
		}
	}
}

func TestLoadSaveCommands(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	// Load command.
	mgr.ShowMenu()
	mgr.state = MenuSinglePlayer
	mgr.singlePlayerCursor = 1
	mgr.M_Key(input.KEnter)
	if mgr.GetState() != MenuLoad {
		t.Fatalf("expected load state, got %v", mgr.GetState())
	}
	mgr.loadCursor = 3
	mgr.M_Key(input.KEnter)
	if got := commands[len(commands)-1]; got != "load s3\n" {
		t.Fatalf("expected load command for slot 3, got %q", got)
	}

	// Save command.
	mgr.ShowMenu()
	mgr.state = MenuSinglePlayer
	mgr.singlePlayerCursor = 2
	mgr.M_Key(input.KEnter)
	if mgr.GetState() != MenuSave {
		t.Fatalf("expected save state, got %v", mgr.GetState())
	}
	mgr.saveCursor = 5
	mgr.M_Key(input.KEnter)
	if got := commands[len(commands)-1]; got != "save s5\n" {
		t.Fatalf("expected save command for slot 5, got %q", got)
	}
}

func TestHelpNavigation(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	mgr.ShowMenu()
	mgr.state = MenuHelp
	mgr.helpPage = 0

	mgr.M_Key(input.KRightArrow)
	if mgr.helpPage != 1 {
		t.Fatalf("expected help page 1, got %d", mgr.helpPage)
	}

	mgr.helpPage = helpPages - 1
	mgr.M_Key(input.KRightArrow)
	if mgr.helpPage != 0 {
		t.Fatalf("expected help page wrap to 0, got %d", mgr.helpPage)
	}

	mgr.M_Key(input.KEscape)
	if mgr.GetState() != MenuMain {
		t.Fatalf("expected return to main menu, got %v", mgr.GetState())
	}
}

func TestOptionsNavigationAndAction(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	cvar.Register("vid_vsync", "1", cvar.FlagArchive, "Vertical sync")
	cvar.Set("vid_vsync", "1")

	mgr.ShowMenu()
	mgr.state = MenuOptions
	mgr.optionsCursor = 0 // CONTROLS
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuControls {
		t.Fatalf("expected controls menu, got %v", got)
	}

	mgr.controlsCursor = controlItemBack
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("expected return to options from controls, got %v", got)
	}

	mgr.optionsCursor = 1 // VIDEO
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuVideo {
		t.Fatalf("expected video menu, got %v", got)
	}

	mgr.M_Key(input.KEscape)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("expected return to options from video, got %v", got)
	}

	mgr.optionsCursor = 2 // AUDIO
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuAudio {
		t.Fatalf("expected audio menu, got %v", got)
	}

	mgr.M_Key(input.KBackspace)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("expected return to options from audio, got %v", got)
	}

	mgr.optionsCursor = 3 // VSYNC
	mgr.M_Key(input.KEnter)
	if cvar.BoolValue("vid_vsync") {
		t.Fatal("expected options vsync toggle to set cvar off")
	}

	mgr.optionsCursor = 4 // Back
	mgr.M_Key(input.KEnter)
	if mgr.GetState() != MenuMain {
		t.Fatalf("expected back to main menu, got %v", mgr.GetState())
	}
}

func TestControlsMenuRebindingAndClearing(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	inputSys.SetBinding(int('w'), "+forward")
	inputSys.SetBinding(input.KUpArrow, "+forward")

	mgr.state = MenuControls
	mgr.controlsCursor = controlItemForward
	mgr.M_Key(input.KEnter)
	if !mgr.controlsRebinding {
		t.Fatal("expected controls menu to enter rebinding mode")
	}
	mgr.M_Key(int('i'))
	if mgr.controlsRebinding {
		t.Fatal("expected controls menu to exit rebinding mode after key selection")
	}
	if got := inputSys.GetBinding(int('i')); got != "+forward" {
		t.Fatalf("binding for i = %q, want +forward", got)
	}
	if got := inputSys.GetBinding(int('w')); got != "" {
		t.Fatalf("binding for w should be cleared by menu rebind, got %q", got)
	}
	if got := inputSys.GetBinding(input.KUpArrow); got != "" {
		t.Fatalf("binding for UPARROW should be cleared by menu rebind, got %q", got)
	}

	mgr.M_Key(input.KLeftArrow)
	if got := inputSys.GetBinding(int('i')); got != "" {
		t.Fatalf("binding for i should be cleared by menu clear action, got %q", got)
	}
}

func TestControlsMenuCancelRebinding(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	inputSys.SetBinding(input.KMouse1, "+attack")

	mgr.state = MenuControls
	mgr.controlsCursor = controlItemAttack
	mgr.M_Key(input.KEnter)
	mgr.M_Key(input.KEscape)
	if mgr.controlsRebinding {
		t.Fatal("expected rebinding mode to cancel on escape")
	}
	if got := inputSys.GetBinding(input.KMouse1); got != "+attack" {
		t.Fatalf("attack binding should be unchanged after cancel, got %q", got)
	}
}

func TestVideoMenuAdjustmentsWriteCvars(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	cvar.Register("vid_width", "1280", cvar.FlagArchive, "Video width")
	cvar.Register("vid_height", "720", cvar.FlagArchive, "Video height")
	cvar.Register("vid_fullscreen", "0", cvar.FlagArchive, "Fullscreen mode")
	cvar.Register("host_maxfps", "250", cvar.FlagArchive, "Maximum frames per second")
	cvar.Register("r_gamma", "1.0", cvar.FlagArchive, "Gamma correction")
	cvar.Register("r_drawviewmodel", "1", cvar.FlagArchive, "Draw first-person viewmodel")
	cvar.Set("vid_width", "1280")
	cvar.Set("vid_height", "720")
	cvar.Set("vid_fullscreen", "0")
	cvar.Set("host_maxfps", "250")
	cvar.Set("r_gamma", "1.0")
	cvar.Set("r_drawviewmodel", "1")

	mgr.state = MenuVideo
	mgr.videoCursor = videoItemResolution
	mgr.M_Key(input.KRightArrow)
	if gotW, gotH := cvar.IntValue("vid_width"), cvar.IntValue("vid_height"); gotW != 1366 || gotH != 768 {
		t.Fatalf("resolution cvars = %dx%d, want 1366x768", gotW, gotH)
	}

	mgr.videoCursor = videoItemFullscreen
	mgr.M_Key(input.KEnter)
	if !cvar.BoolValue("vid_fullscreen") {
		t.Fatal("fullscreen toggle did not update cvar")
	}

	mgr.videoCursor = videoItemMaxFPS
	mgr.M_Key(input.KLeftArrow)
	if got := cvar.IntValue("host_maxfps"); got != 240 {
		t.Fatalf("host_maxfps = %d, want 240", got)
	}

	mgr.videoCursor = videoItemGamma
	mgr.M_Key(input.KRightArrow)
	if got := cvar.FloatValue("r_gamma"); got != 1.1 {
		t.Fatalf("r_gamma = %.1f, want 1.1", got)
	}

	mgr.videoCursor = videoItemViewModel
	mgr.M_Key(input.KEnter)
	if cvar.BoolValue("r_drawviewmodel") {
		t.Fatal("viewmodel toggle did not update cvar")
	}

	mgr.videoCursor = videoItemBack
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("video back should return to options, got %v", got)
	}
}

func TestAudioMenuVolumeAdjustment(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	cvar.Register("s_volume", "0.7", cvar.FlagArchive, "Sound volume")
	cvar.Set("s_volume", "0.7")

	mgr.state = MenuAudio
	mgr.audioCursor = audioItemVolume
	mgr.M_Key(input.KRightArrow)
	if got := cvar.FloatValue("s_volume"); got != 0.8 {
		t.Fatalf("s_volume after right = %.1f, want 0.8", got)
	}

	mgr.M_Key(input.KLeftArrow)
	if got := cvar.FloatValue("s_volume"); got != 0.7 {
		t.Fatalf("s_volume after left = %.1f, want 0.7", got)
	}

	mgr.audioCursor = audioItemBack
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuOptions {
		t.Fatalf("audio back should return to options, got %v", got)
	}
}

func TestMultiPlayerNavigation(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.ShowMenu()
	mgr.state = MenuMultiPlayer

	mgr.multiPlayerCursor = 0
	mgr.M_Key(input.KEnter)
	mgr.multiPlayerCursor = 1
	mgr.M_Key(input.KEnter)
	mgr.multiPlayerCursor = 2
	mgr.M_Key(input.KEnter)

	if len(commands) != 2 {
		t.Fatalf("expected 2 multiplayer commands, got %d", len(commands))
	}
	if got := mgr.GetState(); got != MenuSetup {
		t.Fatalf("setup selection should enter setup menu, got %v", got)
	}
}

func TestSetupMenuNameColorAndAccept(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.ShowMenu()
	mgr.state = MenuMultiPlayer
	mgr.multiPlayerCursor = 2
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuSetup {
		t.Fatalf("expected setup state, got %v", got)
	}

	mgr.M_Char('R')
	mgr.M_Char('a')
	mgr.M_Char('n')
	mgr.M_Char('g')
	mgr.M_Char('e')
	mgr.M_Char('r')

	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KRightArrow)
	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KRightArrow)
	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KEnter)

	if got := mgr.GetState(); got != MenuMultiPlayer {
		t.Fatalf("accept should return to multiplayer menu, got %v", got)
	}
	if len(commands) != 2 {
		t.Fatalf("expected name and color commands, got %v", commands)
	}
	if commands[0] != "name \"playerRanger\"\n" {
		t.Fatalf("unexpected name command: %q", commands[0])
	}
	if commands[1] != "color 1 1\n" {
		t.Fatalf("unexpected color command: %q", commands[1])
	}
}

func TestSetupMenuEscapesBackslashesAndQuotesInName(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.state = MenuSetup
	mgr.setupName = `player\t"name"`
	mgr.applySetupChanges()

	if len(commands) != 2 {
		t.Fatalf("expected name and color commands, got %v", commands)
	}
	if commands[0] != "name \"player\\\\t\\\"name\\\"\"\n" {
		t.Fatalf("unexpected escaped name command: %q", commands[0])
	}
}

func TestLoadAndSaveCursorWrap(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	mgr.state = MenuLoad
	mgr.loadCursor = 0
	mgr.M_Key(input.KUpArrow)
	if mgr.loadCursor != maxSaveGames-1 {
		t.Fatalf("load cursor should wrap to end, got %d", mgr.loadCursor)
	}

	mgr.state = MenuSave
	mgr.saveCursor = maxSaveGames - 1
	mgr.M_Key(input.KDownArrow)
	if mgr.saveCursor != 0 {
		t.Fatalf("save cursor should wrap to start, got %d", mgr.saveCursor)
	}
}

func TestMultiPlayerAndOptionsEscBack(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

	mgr.ShowMenu()

	mgr.state = MenuMultiPlayer
	mgr.M_Key(input.KEscape)
	if mgr.GetState() != MenuMain {
		t.Fatalf("expected main from multiplayer esc, got %v", mgr.GetState())
	}

	mgr.state = MenuOptions
	mgr.M_Key(input.KBackspace)
	if mgr.GetState() != MenuMain {
		t.Fatalf("expected main from options backspace, got %v", mgr.GetState())
	}
}

func TestMouseBindingsForActivationAndBack(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys)

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

func TestMenuStateStringability(t *testing.T) {
	// Simple regression sentinel: ensure states are stable numeric values.
	states := []MenuState{
		MenuNone,
		MenuMain,
		MenuSinglePlayer,
		MenuLoad,
		MenuSave,
		MenuMultiPlayer,
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
	mgr := NewManager(drawMgr, inputSys)
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
	mgr := NewManager(nil, nil)
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
	mgr := NewManager(nil, nil)
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
