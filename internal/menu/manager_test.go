package menu

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// mockDrawManager is a mock implementation of DrawManager for testing.
type mockDrawManager struct{}

func (m *mockDrawManager) Pic(name string) *image.QPic {
	return nil
}

type mockMenuRenderContext struct {
	characters     []struct{ x, y, num int }
	menuCharacters []struct{ x, y, num int }
	menuPics       []struct {
		x, y int
		pic  *image.QPic
	}
	fills []struct {
		x, y, w, h int
		color      byte
	}
	canvas renderer.CanvasState
}

func (m *mockMenuRenderContext) Clear(r, g, b, a float32)          {}
func (m *mockMenuRenderContext) DrawTriangle(r, g, b, a float32)   {}
func (m *mockMenuRenderContext) SurfaceView() any                  { return nil }
func (m *mockMenuRenderContext) Gamma() float32                    { return 1.0 }
func (m *mockMenuRenderContext) DrawPic(x, y int, pic *image.QPic) {}
func (m *mockMenuRenderContext) DrawMenuPic(x, y int, pic *image.QPic) {
	m.menuPics = append(m.menuPics, struct {
		x, y int
		pic  *image.QPic
	}{x: x, y: y, pic: pic})
}
func (m *mockMenuRenderContext) DrawFill(x, y, w, h int, color byte) {
	m.fills = append(m.fills, struct {
		x, y, w, h int
		color      byte
	}{x: x, y: y, w: w, h: h, color: color})
}
func (m *mockMenuRenderContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
	m.DrawFill(x, y, w, h, color)
}
func (m *mockMenuRenderContext) DrawCharacter(x, y int, num int) {
	m.characters = append(m.characters, struct{ x, y, num int }{x, y, num})
}
func (m *mockMenuRenderContext) DrawMenuCharacter(x, y int, num int) {
	m.menuCharacters = append(m.menuCharacters, struct{ x, y, num int }{x, y, num})
}
func (m *mockMenuRenderContext) SetCanvas(ct renderer.CanvasType) { m.canvas.Type = ct }
func (m *mockMenuRenderContext) Canvas() renderer.CanvasState     { return m.canvas }

func renderedMenuLine(rc *mockMenuRenderContext, y int) string {
	lineChars := make([]struct{ x, num int }, 0)
	for _, ch := range rc.menuCharacters {
		if ch.y == y && ch.x >= 24 {
			lineChars = append(lineChars, struct{ x, num int }{x: ch.x, num: ch.num})
		}
	}
	if len(lineChars) == 0 {
		return ""
	}
	sort.Slice(lineChars, func(i, j int) bool {
		return lineChars[i].x < lineChars[j].x
	})
	var builder strings.Builder
	for _, ch := range lineChars {
		num := ch.num
		if num >= 128 {
			num -= 128
		}
		if num >= 0 && num < 128 {
			builder.WriteByte(byte(num))
		}
	}
	return builder.String()
}

func setSetupTestCVars(t *testing.T, mgr *Manager, hostname, name string, color int) {
	t.Helper()

	hostnameCV := mgr.cvars.Register(setupHostnameCVar, setupDefaultHostname, cvar.FlagServerInfo, "")
	nameCV := mgr.cvars.Register(setupClientNameCVar, setupDefaultName, cvar.FlagArchive|cvar.FlagUserInfo, "")
	colorCV := mgr.cvars.Register(setupClientColorCVar, "0", cvar.FlagArchive|cvar.FlagUserInfo, "")

	oldHostname := hostnameCV.String
	oldName := nameCV.String
	oldColor := colorCV.String

	mgr.cvars.Set(hostnameCV.Name, hostname)
	mgr.cvars.Set(nameCV.Name, name)
	mgr.cvars.SetInt(colorCV.Name, color)

	t.Cleanup(func() {
		mgr.cvars.Set(hostnameCV.Name, oldHostname)
		mgr.cvars.Set(nameCV.Name, oldName)
		mgr.cvars.Set(colorCV.Name, oldColor)
	})
}

func setHostGameTestCVars(t *testing.T, mgr *Manager, maxPlayers, coop, deathmatch, teamplay, skill int) {
	t.Helper()

	maxPlayersCV := mgr.cvars.Register("maxplayers", "16", cvar.FlagServerInfo, "")
	coopCV := mgr.cvars.Register("coop", "0", cvar.FlagServerInfo, "")
	deathmatchCV := mgr.cvars.Register("deathmatch", "0", cvar.FlagServerInfo, "")
	teamplayCV := mgr.cvars.Register("teamplay", "0", cvar.FlagServerInfo, "")
	skillCV := mgr.cvars.Register("skill", "1", cvar.FlagArchive, "")

	oldMaxPlayers := maxPlayersCV.String
	oldCoop := coopCV.String
	oldDeathmatch := deathmatchCV.String
	oldTeamplay := teamplayCV.String
	oldSkill := skillCV.String

	mgr.cvars.SetInt(maxPlayersCV.Name, maxPlayers)
	mgr.cvars.SetInt(coopCV.Name, coop)
	mgr.cvars.SetInt(deathmatchCV.Name, deathmatch)
	mgr.cvars.SetInt(teamplayCV.Name, teamplay)
	mgr.cvars.SetInt(skillCV.Name, skill)

	t.Cleanup(func() {
		mgr.cvars.Set(maxPlayersCV.Name, oldMaxPlayers)
		mgr.cvars.Set(coopCV.Name, oldCoop)
		mgr.cvars.Set(deathmatchCV.Name, oldDeathmatch)
		mgr.cvars.Set(teamplayCV.Name, oldTeamplay)
		mgr.cvars.Set(skillCV.Name, oldSkill)
	})
}

func TestNewManager(t *testing.T) {
	drawMgr := &mockDrawManager{}
	inputSys := input.NewSystem(nil)
	mgr := NewManager(drawMgr, inputSys, nil)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	if mgr.IsActive() {
		t.Error("Menu should not be active initially")
	}

	if mgr.State() != MenuNone {
		t.Error("Initial state should be MenuNone")
	}
}

func TestToggleMenu(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	// Toggle menu on
	mgr.ToggleMenu()
	if !mgr.IsActive() {
		t.Error("Menu should be active after toggle")
	}
	if mgr.State() != MenuMain {
		t.Error("State should be MenuMain after toggle")
	}

	// Toggle menu off
	mgr.ToggleMenu()
	if mgr.IsActive() {
		t.Error("Menu should not be active after second toggle")
	}
	if mgr.State() != MenuNone {
		t.Error("State should be MenuNone after second toggle")
	}
}

func TestShowHideMenu(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
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
	mgr := NewManager(drawMgr, inputSys, nil)
	mgr.ShowMenu()

	// Test up arrow wraps to last item (mainQuit=5).
	mgr.M_Key(input.KUpArrow)
	if mgr.mainCursor != mainQuit {
		t.Errorf("Up arrow should wrap cursor to mainQuit(%d), got %d", mainQuit, mgr.mainCursor)
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
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.ShowMenu()

	// Navigate to quit — 4 down presses (skips mainMods when no mods).
	mgr.M_Key(input.KDownArrow) // 0→1
	mgr.M_Key(input.KDownArrow) // 1→2
	mgr.M_Key(input.KDownArrow) // 2→4 (skip 3)
	mgr.M_Key(input.KDownArrow) // 4→5 (Quit)
	mgr.M_Key(input.KEnter)     // Enter to select quit

	if mgr.State() != MenuQuit {
		t.Error("State should be MenuQuit after selecting quit")
	}

	// Backspace should cancel quit and return to previous state.
	mgr.M_Key(input.KBackspace)
	if mgr.State() != MenuMain {
		t.Error("Backspace should return to main menu")
	}

	// Confirm quit with Y.
	mgr.mainCursor = mainQuit
	mgr.M_Key(input.KEnter)
	mgr.M_Key('y')

	if mgr.IsActive() {
		t.Fatal("Menu should hide after quit confirmation")
	}

	if len(commands) == 0 || commands[len(commands)-1] != "quit\n" {
		t.Fatalf("expected quit command, got %v", commands)
	}
}

func TestShowConfirmationPromptCancelHidesMenuAndRunsCallback(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	confirmed := false
	cancelled := false
	mgr.ShowConfirmationPrompt([]string{
		"LOAD LAST SAVE? (Y/N)",
		"PRESS Y OR ENTER TO LOAD",
		"PRESS N OR ESC TO CONTINUE",
	}, func() {
		confirmed = true
	}, func() {
		cancelled = true
	}, MenuNone)

	mgr.M_Key('n')

	if confirmed {
		t.Fatal("confirm callback ran on cancel")
	}
	if !cancelled {
		t.Fatal("cancel callback did not run")
	}
	if mgr.IsActive() {
		t.Fatal("menu should hide after cancel when returning to game")
	}
	if got := mgr.State(); got != MenuNone {
		t.Fatalf("state = %v, want %v", got, MenuNone)
	}
}

func TestMainMenuSelections(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	mgr.ShowMenu()

	selections := []struct {
		cursor int
		want   MenuState
	}{
		{mainSinglePlayer, MenuSinglePlayer},
		{mainMultiPlayer, MenuMultiPlayer},
		{mainOptions, MenuOptions},
		{mainHelp, MenuHelp},
		{mainQuit, MenuQuit},
	}

	for _, tc := range selections {
		mgr.state = MenuMain
		mgr.mainCursor = tc.cursor
		mgr.M_Key(input.KEnter)
		if got := mgr.State(); got != tc.want {
			t.Fatalf("cursor %d: expected state %v, got %v", tc.cursor, tc.want, got)
		}
	}
}

func TestSinglePlayerActions(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	skillCV := mgr.cvars.Register("skill", "1", cvar.FlagArchive, "")
	oldSkill := skillCV.String
	defer func() {
		mgr.cvars.Set(skillCV.Name, oldSkill)
	}()
	mgr.cvars.SetInt(skillCV.Name, 2)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.ShowMenu()
	mgr.M_Key(input.KEnter) // Main -> Single Player

	if mgr.State() != MenuSinglePlayer {
		t.Fatalf("expected single player state, got %v", mgr.State())
	}

	// New Game enters the skill menu first.
	mgr.M_Key(input.KEnter)
	if !mgr.IsActive() {
		t.Fatal("menu should remain active in skill menu before selection")
	}
	if got := mgr.State(); got != MenuSkill {
		t.Fatalf("state = %v, want %v", got, MenuSkill)
	}

	// Accept default (current cvar skill) and start game.
	mgr.M_Key(input.KEnter)
	if mgr.IsActive() {
		t.Fatal("menu should hide when starting new game")
	}

	want := []string{"disconnect\n", "skill 2\n", "maxplayers 1\n", "deathmatch 0\n", "coop 0\n", "map start\n"}
	if len(commands) < len(want) {
		t.Fatalf("expected at least %d commands, got %d", len(want), len(commands))
	}
	for i, expected := range want {
		if commands[i] != expected {
			t.Fatalf("command %d: expected %q, got %q", i, expected, commands[i])
		}
	}
}

func TestSinglePlayerNewGamePromptsWhenProviderRequiresConfirmation(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}
	mgr.SetNewGameConfirmationProvider(func() bool { return true })

	mgr.ShowMenu()
	mgr.M_Key(input.KEnter) // Main -> Single Player
	mgr.M_Key(input.KEnter) // New Game -> prompt

	if !mgr.IsActive() {
		t.Fatal("menu should stay active for new game confirmation")
	}
	if got := mgr.State(); got != MenuQuit {
		t.Fatalf("state = %v, want %v", got, MenuQuit)
	}
	if len(commands) != 0 {
		t.Fatalf("commands should not be queued before confirming, got %v", commands)
	}
}

func TestSinglePlayerNewGamePromptConfirmStartsGame(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}
	mgr.SetNewGameConfirmationProvider(func() bool { return true })

	mgr.ShowMenu()
	mgr.M_Key(input.KEnter) // Main -> Single Player
	mgr.M_Key(input.KEnter) // New Game -> prompt
	mgr.M_Key('y')          // Confirm

	if !mgr.IsActive() {
		t.Fatal("menu should stay active after confirming prompt to allow skill selection")
	}
	if got := mgr.State(); got != MenuSkill {
		t.Fatalf("state = %v, want %v", got, MenuSkill)
	}
	if len(commands) != 0 {
		t.Fatalf("commands should not be queued until skill is selected, got %v", commands)
	}

	mgr.M_Key(input.KEnter) // Select skill
	if mgr.IsActive() {
		t.Fatal("menu should hide after confirming skill selection")
	}
	want := []string{"disconnect\n", "skill 1\n", "maxplayers 1\n", "deathmatch 0\n", "coop 0\n", "map start\n"}
	if len(commands) < len(want) {
		t.Fatalf("expected at least %d commands, got %d", len(want), len(commands))
	}
	for i, expected := range want {
		if commands[i] != expected {
			t.Fatalf("command %d: expected %q, got %q", i, expected, commands[i])
		}
	}
}

func TestSinglePlayerNewGamePromptCancelReturnsToSinglePlayer(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}
	mgr.SetNewGameConfirmationProvider(func() bool { return true })

	mgr.ShowMenu()
	mgr.M_Key(input.KEnter) // Main -> Single Player
	mgr.M_Key(input.KEnter) // New Game -> prompt
	mgr.M_Key('n')          // Cancel

	if !mgr.IsActive() {
		t.Fatal("menu should remain active after declining new game prompt")
	}
	if got := mgr.State(); got != MenuSinglePlayer {
		t.Fatalf("state = %v, want %v", got, MenuSinglePlayer)
	}
	if len(commands) != 0 {
		t.Fatalf("commands should not be queued after declining prompt, got %v", commands)
	}
}

func TestSinglePlayerSkillMenuShowsResumeWhenAutosaveAvailable(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}
	mgr.SetResumeGameAvailableProvider(func() bool { return true })

	mgr.ShowMenu()
	mgr.M_Key(input.KEnter) // Main -> Single Player
	mgr.M_Key(input.KEnter) // New Game -> skill menu

	if !mgr.IsActive() {
		t.Fatal("menu should stay active for skill menu")
	}
	if got := mgr.State(); got != MenuSkill {
		t.Fatalf("state = %v, want %v", got, MenuSkill)
	}
	if got := mgr.skillCursor; got != 4 {
		t.Fatalf("skill cursor = %d, want resume row 4", got)
	}
	if len(commands) != 0 {
		t.Fatalf("commands should not be queued before selecting skill/resume, got %v", commands)
	}
}

func TestSinglePlayerSkillMenuResumeLoadsAutosave(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}
	mgr.SetResumeGameAvailableProvider(func() bool { return true })

	mgr.ShowMenu()
	mgr.M_Key(input.KEnter) // Main -> Single Player
	mgr.M_Key(input.KEnter) // New Game -> skill menu
	mgr.M_Key(input.KEnter) // Resume

	if mgr.IsActive() {
		t.Fatal("menu should hide after selecting resume")
	}
	if len(commands) != 1 {
		t.Fatalf("commands = %v, want single autosave load", commands)
	}
	if got := commands[0]; got != "load \"autosave/start\"\n" {
		t.Fatalf("command = %q, want %q", got, "load \"autosave/start\"\\n")
	}
}

func TestSinglePlayerSkillMenuCanChooseFreshGameWhenResumeAvailable(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}
	mgr.SetResumeGameAvailableProvider(func() bool { return true })

	mgr.ShowMenu()
	mgr.M_Key(input.KEnter) // Main -> Single Player
	mgr.M_Key(input.KEnter) // New Game -> skill menu
	mgr.M_Key(input.KUpArrow)
	mgr.M_Key(input.KEnter) // Fresh start with NIGHTMARE from resume row wrap

	if mgr.IsActive() {
		t.Fatal("menu should hide after selecting skill")
	}
	want := []string{"disconnect\n", "skill 3\n", "maxplayers 1\n", "deathmatch 0\n", "coop 0\n", "map start\n"}
	if len(commands) < len(want) {
		t.Fatalf("expected at least %d commands, got %d", len(want), len(commands))
	}
	for i, expected := range want {
		if commands[i] != expected {
			t.Fatalf("command %d: expected %q, got %q", i, expected, commands[i])
		}
	}
}

func TestSinglePlayerNewGameConfirmationTakesPrecedenceOverResumePrompt(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}
	mgr.SetNewGameConfirmationProvider(func() bool { return true })
	mgr.SetResumeGameAvailableProvider(func() bool { return true })

	mgr.ShowMenu()
	mgr.M_Key(input.KEnter) // Main -> Single Player
	mgr.M_Key(input.KEnter) // New Game -> active-session prompt
	mgr.M_Key('n')          // Decline

	if !mgr.IsActive() {
		t.Fatal("menu should remain active after declining active-session prompt")
	}
	if got := mgr.State(); got != MenuSinglePlayer {
		t.Fatalf("state = %v, want %v", got, MenuSinglePlayer)
	}
	if len(commands) != 0 {
		t.Fatalf("commands should not be queued when active-session prompt is declined, got %v", commands)
	}
}

func TestLoadSaveCommands(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	// Load command.
	mgr.ShowMenu()
	mgr.state = MenuSinglePlayer
	mgr.singlePlayerCursor = 1
	mgr.M_Key(input.KEnter)
	if mgr.State() != MenuLoad {
		t.Fatalf("expected load state, got %v", mgr.State())
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
	if mgr.State() != MenuSave {
		t.Fatalf("expected save state, got %v", mgr.State())
	}
	mgr.saveCursor = 5
	mgr.M_Key(input.KEnter)
	if got := commands[len(commands)-1]; got != "save s5\n" {
		t.Fatalf("expected save command for slot 5, got %q", got)
	}
}

func TestSinglePlayerSaveEntryAllowedTransitionsToSave(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.SetSaveEntryAllowedProvider(func() bool { return true })
	mgr.ShowMenu()
	mgr.state = MenuSinglePlayer
	mgr.singlePlayerCursor = 2

	mgr.M_Key(input.KEnter)

	if got := mgr.State(); got != MenuSave {
		t.Fatalf("state = %v, want %v", got, MenuSave)
	}
}

func TestSinglePlayerSaveEntryDisallowedStaysOnSinglePlayerAndPlaysCancel(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var played []string
	mgr.SetSoundPlayer(func(name string) {
		played = append(played, name)
	})
	mgr.SetSaveEntryAllowedProvider(func() bool { return false })
	mgr.ShowMenu()
	mgr.state = MenuSinglePlayer
	mgr.singlePlayerCursor = 2
	played = nil

	mgr.M_Key(input.KEnter)

	if got := mgr.State(); got != MenuSinglePlayer {
		t.Fatalf("state = %v, want %v", got, MenuSinglePlayer)
	}
	if len(played) < 2 {
		t.Fatalf("played sounds = %v, want select+cancel feedback", played)
	}
	if played[0] != menuSoundSelect {
		t.Fatalf("first sound = %q, want %q", played[0], menuSoundSelect)
	}
	if played[1] != menuSoundCancel {
		t.Fatalf("second sound = %q, want %q", played[1], menuSoundCancel)
	}
}

func TestLoadSaveMenusRefreshLabelsFromProvider(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	providerCalls := 0
	mgr.SetSaveSlotProvider(func(slotCount int) []SaveSlotInfo {
		providerCalls++
		slots := make([]SaveSlotInfo, 0, slotCount)
		for i := 0; i < slotCount; i++ {
			slots = append(slots, SaveSlotInfo{
				Name:        fmt.Sprintf("s%d", i),
				DisplayName: "--- UNUSED SLOT ---",
			})
		}
		slots[0].DisplayName = "e1m1"
		return slots
	})

	mgr.ShowMenu()
	mgr.state = MenuSinglePlayer
	mgr.singlePlayerCursor = 1
	mgr.M_Key(input.KEnter)
	if got := mgr.State(); got != MenuLoad {
		t.Fatalf("expected load state, got %v", got)
	}
	loadRC := &mockMenuRenderContext{}
	mgr.M_Draw(loadRC)
	if got := renderedMenuLine(loadRC, 32); got != "e1m1" {
		t.Fatalf("load slot 0 label = %q, want %q", got, "e1m1")
	}
	if got := renderedMenuLine(loadRC, 40); got != "--- UNUSED SLOT ---" {
		t.Fatalf("load slot 1 label = %q, want %q", got, "--- UNUSED SLOT ---")
	}

	mgr.ShowMenu()
	mgr.state = MenuSinglePlayer
	mgr.singlePlayerCursor = 2
	mgr.M_Key(input.KEnter)
	if got := mgr.State(); got != MenuSave {
		t.Fatalf("expected save state, got %v", got)
	}
	saveRC := &mockMenuRenderContext{}
	mgr.M_Draw(saveRC)
	if got := renderedMenuLine(saveRC, 32); got != "e1m1" {
		t.Fatalf("save slot 0 label = %q, want %q", got, "e1m1")
	}
	if got := renderedMenuLine(saveRC, 40); got != "--- UNUSED SLOT ---" {
		t.Fatalf("save slot 1 label = %q, want %q", got, "--- UNUSED SLOT ---")
	}

	if providerCalls != 2 {
		t.Fatalf("provider calls = %d, want 2", providerCalls)
	}
}
