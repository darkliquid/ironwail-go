package menu

// Multiplayer, host/join game, setup, and load-save menu tests split from manager_test.go.

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/input"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func TestMultiPlayerNavigation(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	mgr.ShowMenu()
	mgr.state = MenuMultiPlayer

	mgr.multiPlayerCursor = 0
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuJoinGame {
		t.Fatalf("join selection should enter join menu, got %v", got)
	}
	mgr.M_Key(input.KEscape)

	mgr.multiPlayerCursor = 1
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuHostGame {
		t.Fatalf("host selection should enter host menu, got %v", got)
	}
	mgr.M_Key(input.KEscape)

	mgr.multiPlayerCursor = 2
	mgr.M_Key(input.KEnter)

	if got := mgr.GetState(); got != MenuSetup {
		t.Fatalf("setup selection should enter setup menu, got %v", got)
	}
}

func TestJoinGameMenuEditingAndConnectCommand(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.ShowMenu()
	mgr.state = MenuMultiPlayer
	mgr.multiPlayerCursor = 0
	mgr.M_Key(input.KEnter)

	if got := mgr.GetState(); got != MenuJoinGame {
		t.Fatalf("expected join game menu, got %v", got)
	}

	mgr.M_Key(input.KBackspace)
	if got := mgr.joinAddress; got != "loca" {
		t.Fatalf("join address after backspace = %q, want %q", got, "loca")
	}
	mgr.M_Char('l')
	mgr.M_Char(':')
	mgr.M_Char('2')
	mgr.M_Char('6')
	mgr.M_Char('0')
	mgr.M_Char('0')

	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KEnter)

	if mgr.IsActive() {
		t.Fatal("connect should hide menu")
	}
	if len(commands) == 0 {
		t.Fatal("expected connect command to be queued")
	}
	if got := commands[len(commands)-1]; got != "connect \"local:2600\"\n" {
		t.Fatalf("unexpected connect command: %q", got)
	}
}

func TestHostGameMenuEditingAndCommands(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	setHostGameTestCVars(t, mgr, 4, 1, 0, 0, 1)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.ShowMenu()
	mgr.state = MenuMultiPlayer
	mgr.multiPlayerCursor = 1
	mgr.M_Key(input.KEnter)

	if got := mgr.GetState(); got != MenuHostGame {
		t.Fatalf("expected host game menu, got %v", got)
	}

	mgr.M_Key(input.KLeftArrow) // max players: 4 -> 3
	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KRightArrow) // mode: coop -> deathmatch
	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KRightArrow) // teamplay: 0 -> 1
	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KRightArrow) // skill: 1 -> 2
	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KRightArrow) // fraglimit: 0 -> 10
	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KRightArrow) // timelimit: 0 -> 5
	mgr.M_Key(input.KDownArrow)
	for i := 0; i < 5; i++ {
		mgr.M_Key(input.KBackspace) // map: start ->
	}
	mgr.M_Char('d')
	mgr.M_Char('m')
	mgr.M_Char('2')
	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KEnter)

	if mgr.IsActive() {
		t.Fatal("host start should hide menu")
	}

	want := []string{
		"disconnect\n",
		"listen 1\n",
		"maxplayers 3\n",
		"deathmatch 1\n",
		"coop 0\n",
		"teamplay 1\n",
		"fraglimit 10\n",
		"timelimit 5\n",
		"skill 2\n",
		"map \"dm2\"\n",
	}
	if len(commands) < len(want) {
		t.Fatalf("expected at least %d commands, got %d (%v)", len(want), len(commands), commands)
	}
	for i, expected := range want {
		if got := commands[i]; got != expected {
			t.Fatalf("command %d = %q, want %q", i, got, expected)
		}
	}
}

func TestHostGameStartQueuesListenZeroForSinglePlayer(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	setHostGameTestCVars(t, mgr, 1, 0, 0, 0, 1)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.ShowMenu()
	mgr.state = MenuMultiPlayer
	mgr.multiPlayerCursor = 1
	mgr.M_Key(input.KEnter)
	if got := mgr.GetState(); got != MenuHostGame {
		t.Fatalf("expected host game menu, got %v", got)
	}
	mgr.hostMaxPlayers = 1

	mgr.hostGameCursor = hostGameItemStart
	mgr.M_Key(input.KEnter)

	want := []string{
		"disconnect\n",
		"listen 0\n",
		"maxplayers 1\n",
		"deathmatch 1\n",
		"coop 0\n",
		"teamplay 0\n",
		"fraglimit 0\n",
		"timelimit 0\n",
		"skill 1\n",
		"map \"start\"\n",
	}
	if len(commands) < len(want) {
		t.Fatalf("expected at least %d commands, got %d (%v)", len(want), len(commands), commands)
	}
	for i, expected := range want {
		if got := commands[i]; got != expected {
			t.Fatalf("command %d = %q, want %q", i, got, expected)
		}
	}
}

func TestJoinGameMenuConnectsSelectedServerResult(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}
	mgr.serverResults = []inet.HostCacheEntry{
		{Name: "Alpha", Map: "start", Players: 1, MaxPlayers: 4, Address: "10.0.0.2:26000"},
		{Name: "Beta", Map: "dm2", Players: 3, MaxPlayers: 8, Address: "10.0.0.3:26000"},
	}

	mgr.ShowMenu()
	mgr.state = MenuJoinGame
	mgr.joinGameCursor = joinGameBaseItems + 1
	mgr.M_Key(input.KEnter)

	if mgr.IsActive() {
		t.Fatal("selecting a discovered server should hide menu")
	}
	if got := commands[len(commands)-1]; got != "connect \"10.0.0.3:26000\"\n" {
		t.Fatalf("unexpected connect command: %q", got)
	}
	if got := mgr.joinAddress; got != "10.0.0.3:26000" {
		t.Fatalf("joinAddress = %q, want selected server address", got)
	}
}

func TestHostGameMenuSyncsFromLiveNetgameCVars(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	setHostGameTestCVars(t, mgr, 1, 0, 1, 2, 3)

	mgr.ShowMenu()
	mgr.state = MenuMultiPlayer
	mgr.multiPlayerCursor = 1
	mgr.M_Key(input.KEnter)

	if got := mgr.GetState(); got != MenuHostGame {
		t.Fatalf("expected host game menu, got %v", got)
	}
	if got := mgr.hostMaxPlayers; got != hostMaxPlayersMax {
		t.Fatalf("host maxplayers = %d, want %d", got, hostMaxPlayersMax)
	}
	if got := mgr.hostGameMode; got != 1 {
		t.Fatalf("host mode = %d, want deathmatch mode (1)", got)
	}
	if got := mgr.hostTeamplay; got != 2 {
		t.Fatalf("host teamplay = %d, want 2", got)
	}
	if got := mgr.hostSkill; got != 3 {
		t.Fatalf("host skill = %d, want 3", got)
	}
}

func TestHostGameMenuMaxPlayersClampsAtBounds(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	setHostGameTestCVars(t, mgr, hostMaxPlayersMin, 0, 1, 0, 1)

	mgr.ShowMenu()
	mgr.state = MenuMultiPlayer
	mgr.multiPlayerCursor = 1
	mgr.M_Key(input.KEnter)

	mgr.hostGameCursor = hostGameItemMaxPlayers
	mgr.M_Key(input.KLeftArrow)
	if got := mgr.hostMaxPlayers; got != hostMaxPlayersMin {
		t.Fatalf("host maxplayers after decrement = %d, want %d", got, hostMaxPlayersMin)
	}

	mgr.hostMaxPlayers = hostMaxPlayersMax
	mgr.M_Key(input.KRightArrow)
	if got := mgr.hostMaxPlayers; got != hostMaxPlayersMax {
		t.Fatalf("host maxplayers after increment = %d, want %d", got, hostMaxPlayersMax)
	}
}

func TestSetupMenuLoadsCurrentHostnameNameAndColor(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	setSetupTestCVars(t, mgr, "LAN Party", "Ranger", 0x12)

	mgr.ShowMenu()
	mgr.state = MenuMultiPlayer
	mgr.multiPlayerCursor = 2
	mgr.M_Key(input.KEnter)

	if got := mgr.GetState(); got != MenuSetup {
		t.Fatalf("expected setup state, got %v", got)
	}
	if got := mgr.setupHostname; got != "LAN Party" {
		t.Fatalf("setup hostname = %q, want %q", got, "LAN Party")
	}
	if got := mgr.setupName; got != "Ranger" {
		t.Fatalf("setup name = %q, want %q", got, "Ranger")
	}
	if got := mgr.setupTopColor; got != 1 {
		t.Fatalf("setup top color = %d, want 1", got)
	}
	if got := mgr.setupBottomColor; got != 2 {
		t.Fatalf("setup bottom color = %d, want 2", got)
	}
}

func TestSetupMenuHostnameNameColorAndAccept(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	setSetupTestCVars(t, mgr, "UNNAMED", "player", 0)

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

	for range len("UNNAMED") {
		mgr.M_Key(input.KBackspace)
	}
	mgr.M_Char('H')
	mgr.M_Char('Q')

	mgr.M_Key(input.KDownArrow)
	for range len("player") {
		mgr.M_Key(input.KBackspace)
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
	if commands[0] != "name \"Ranger\"\n" {
		t.Fatalf("unexpected name command: %q", commands[0])
	}
	if commands[1] != "color 1 1\n" {
		t.Fatalf("unexpected color command: %q", commands[1])
	}
	if got := mgr.cvars.StringValue(setupHostnameCVar); got != "HQ" {
		t.Fatalf("hostname cvar = %q, want %q", got, "HQ")
	}
}

func TestSetupMenuBackspaceOnColorRowDoesNotExit(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	setSetupTestCVars(t, mgr, "UNNAMED", "player", 0)

	mgr.enterSetupMenu()
	mgr.setupCursor = setupItemTopColor

	mgr.M_Key(input.KBackspace)

	if got := mgr.GetState(); got != MenuSetup {
		t.Fatalf("backspace on color row should stay in setup, got %v", got)
	}
}

func TestSetupMenuEscapesBackslashesAndQuotesInName(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	setSetupTestCVars(t, mgr, "UNNAMED", "player", 0)

	var commands []string
	mgr.commandText = func(text string) {
		commands = append(commands, text)
	}

	mgr.state = MenuSetup
	mgr.setupHostname = mgr.currentSetupHostname()
	mgr.setupName = `player\t"name"`
	mgr.applySetupChanges()

	if len(commands) != 1 {
		t.Fatalf("expected name command only, got %v", commands)
	}
	if commands[0] != "name \"player\\\\t\\\"name\\\"\"\n" {
		t.Fatalf("unexpected escaped name command: %q", commands[0])
	}
}

type mapDrawManager struct {
	pics map[string]*image.QPic
}

func (m *mapDrawManager) GetPic(name string) *image.QPic {
	if m.pics == nil {
		return nil
	}
	return m.pics[name]
}

func TestDrawSetupUsesTextBoxesAndTranslatedPlayerArt(t *testing.T) {
	box := &image.QPic{Width: 8, Height: 8, Pixels: []byte{1}}
	menuplyr := &image.QPic{
		Width:  2,
		Height: 2,
		Pixels: []byte{16, 31, 96, 111},
	}
	drawMgr := &mapDrawManager{
		pics: map[string]*image.QPic{
			"gfx/bigbox.lmp":   box,
			"gfx/menuplyr.lmp": menuplyr,
			"gfx/box_tl.lmp":   box,
			"gfx/box_ml.lmp":   box,
			"gfx/box_bl.lmp":   box,
			"gfx/box_tm.lmp":   box,
			"gfx/box_mm.lmp":   box,
			"gfx/box_mm2.lmp":  box,
			"gfx/box_bm.lmp":   box,
			"gfx/box_tr.lmp":   box,
			"gfx/box_mr.lmp":   box,
			"gfx/box_br.lmp":   box,
		},
	}
	mgr := NewManager(drawMgr, input.NewSystem(nil), nil)
	mgr.setupTopColor = 2
	mgr.setupBottomColor = 9

	rc := &mockMenuRenderContext{}
	mgr.drawSetup(rc)

	if len(rc.fills) != 0 {
		t.Fatalf("drawSetup should not use color swatch DrawFill, got %d calls", len(rc.fills))
	}

	var foundBigBox bool
	var translated *image.QPic
	for _, call := range rc.menuPics {
		if call.x == 160 && call.y == 64 && call.pic == box {
			foundBigBox = true
		}
		if call.x == 172 && call.y == 72 {
			translated = call.pic
		}
	}
	if !foundBigBox {
		t.Fatalf("expected bigbox preview draw call at (160,64), calls=%v", rc.menuPics)
	}
	if translated == nil {
		t.Fatalf("expected translated player preview draw call at (172,72), calls=%v", rc.menuPics)
	}
	if translated == menuplyr {
		t.Fatal("expected translated player preview pic copy, got original pic pointer")
	}
	if got, want := translated.Pixels, []byte{32, 47, 159, 144}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
		t.Fatalf("translated preview pixels = %v, want %v", got, want)
	}
}

func TestTranslateSetupPlayerPicMapsTopAndBottomRanges(t *testing.T) {
	pic := &image.QPic{
		Width:  5,
		Height: 1,
		Pixels: []byte{15, 16, 31, 96, 111},
	}

	got := translateSetupPlayerPic(pic, 2, 9)
	want := []byte{15, 32, 47, 159, 144}
	if len(got.Pixels) != len(want) {
		t.Fatalf("translated length = %d, want %d", len(got.Pixels), len(want))
	}
	for i := range want {
		if got.Pixels[i] != want[i] {
			t.Fatalf("translated[%d] = %d, want %d (all=%v)", i, got.Pixels[i], want[i], got.Pixels)
		}
	}

	if pic.Pixels[1] != 16 || pic.Pixels[2] != 31 || pic.Pixels[3] != 96 || pic.Pixels[4] != 111 {
		t.Fatalf("source pic mutated: %v", pic.Pixels)
	}
}

func TestLoadAndSaveCursorWrap(t *testing.T) {
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)

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
	mgr := NewManager(drawMgr, inputSys, nil)

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
