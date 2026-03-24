// Package menu implements the Quake in-game menu system as a finite state
// machine. Each MenuState value represents a distinct menu page (Main,
// Single Player, Options, Video, etc.). The Manager struct owns the complete
// menu state — cursors, text-entry buffers, cached graphics — and routes
// keyboard/mouse input to the active page via M_Key / M_Char / M_Mousemove.
//
// Rendering uses the Quake 320×200 virtual coordinate system: every draw
// call positions characters and pictures on a fixed grid that the renderer
// scales to the real window size. This mirrors the C Ironwail approach where
// menu drawing is a 2D overlay pass that happens after 3D world rendering.
//
// Navigation is hierarchical: the main menu leads to sub-menus (Options →
// Video, Multiplayer → Host Game, etc.) and Escape always returns one level
// up. Sound effects punctuate navigation (menu1.wav), selection (menu2.wav),
// and cancellation (menu3.wav).
package menu

import (
	"log/slog"
	"math"

	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/input"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// MenuState represents the current active menu.
type MenuState int

const (
	MenuNone         MenuState = iota // No menu active
	MenuMain                          // Main menu
	MenuSinglePlayer                  // Single player submenu
	MenuSkill                         // Skill / resume submenu for New Game
	MenuLoad                          // Load game submenu
	MenuSave                          // Save game submenu
	MenuMultiPlayer                   // Multiplayer submenu
	MenuJoinGame                      // Join game submenu
	MenuHostGame                      // Host game submenu
	MenuOptions                       // Options submenu
	MenuControls                      // Controls options submenu
	MenuVideo                         // Video options submenu
	MenuAudio                         // Audio options submenu
	MenuHelp                          // Help screens
	MenuQuit                          // Quit confirmation screen
	MenuSetup                         // Player setup screen
	MenuMods                          // Mods browser
)

// Main-menu item indices and per-page item counts.
//
// Each menu page has a fixed number of selectable items. The cursor for that
// page wraps modulo that count. These constants mirror the hard-coded slot
// layout in C Ironwail's M_Main_Key(), M_Options_Key(), etc.
const (
	// Main menu item indices — fixed enum matching C Ironwail.
	// When no mods are available, mainMods is skipped during navigation.
	mainSinglePlayer = 0
	mainMultiPlayer  = 1
	mainOptions      = 2
	mainMods         = 3
	mainHelp         = 4
	mainQuit         = 5
	mainItems        = 6 // total slots (Mods may be skipped)

	singlePlayerItems = 3
	skillBaseItems    = 4
	multiPlayerItems  = 3
	joinGameBaseItems = 4
	hostGameItems     = 9
	optionsItems      = 5
	controlsItems     = 26
	videoItems        = 12 // added HUD-style plus telemetry toggles
	audioItems        = 2

	maxSaveGames = 12
	helpPages    = 6

	setupNameMaxLen     = 15
	setupHostnameMaxLen = 15
	setupColorMax       = 13
	joinAddressMax      = 63
	hostMapMaxLen       = 32
	hostMaxPlayersMin   = 2
	hostMaxPlayersMax   = 16

	menuSoundNavigate = "misc/menu1.wav"
	menuSoundSelect   = "misc/menu2.wav"
	menuSoundCancel   = "misc/menu3.wav"
)

// CVar names and defaults used by the Player Setup menu.
// These correspond to the engine console variables that persist across sessions.
const (
	setupClientNameCVar  = "_cl_name"
	setupClientColorCVar = "_cl_color"
	setupHostnameCVar    = "hostname"

	setupDefaultName     = "player"
	setupDefaultHostname = "UNNAMED"
)

// Host Game menu item indices — ordered top-to-bottom as they appear on screen.
// Each index maps to a row the cursor can land on, and the key handler uses
// these values to decide which setting to adjust or which action to perform.
const (
	hostGameItemMaxPlayers = iota
	hostGameItemMode
	hostGameItemTeamplay
	hostGameItemSkill
	hostGameItemFragLimit
	hostGameItemTimeLimit
	hostGameItemMap
	hostGameItemStart
	hostGameItemBack
)

// Player Setup menu item indices. The setup screen lets the player edit their
// name, hostname, and shirt/pants colors before joining a multiplayer game.
const (
	setupItemHostname = iota
	setupItemName
	setupItemTopColor
	setupItemBottomColor
	setupItemAccept
	setupItems
)

// Controls menu item indices. Items before controlsBindingStart are simple
// toggle/slider settings (mouse speed, invert, always-run, freelook). Items
// from controlsBindingStart onward are key-binding rows where the player
// presses Enter to rebind or Backspace/Left to clear.
const (
	controlItemMouseSpeed = iota
	controlItemInvertMouse
	controlItemAlwaysRun
	controlItemFreeLook
	controlItemForward
	controlItemBackward
	controlItemTurnLeft
	controlItemTurnRight
	controlItemStrafeLeft
	controlItemStrafeRight
	controlItemJump
	controlItemAttack
	controlItemUse
	controlItemRun
	controlItemStrafe
	controlItemLookUp
	controlItemLookDown
	controlItemCenterView
	controlItemMouseLook
	controlItemKeyboardLook
	controlItemMoveUp
	controlItemMoveDown
	controlItemNextWeapon
	controlItemPrevWeapon
	controlItemToggleConsole
	controlItemBack
)

// controlsBindingStart is the first Controls-menu index that corresponds to a
// key-binding row (as opposed to a slider/toggle). Items at or above this
// index enter "rebinding mode" when activated.
const controlsBindingStart = controlItemForward

// Video menu item indices. Each row maps to a cvar that the left/right keys
// cycle through. videoItemBack returns to the Options parent menu.
const (
	videoItemResolution = iota
	videoItemFullscreen
	videoItemVSync
	videoItemMaxFPS
	videoItemGamma
	videoItemViewModel
	videoItemWaterwarp // r_waterwarp: mirrors C Ironwail options-menu OPT_WATERWARP preview
	videoItemHUDStyle  // hud_style: selects classic, compact, or QuakeWorld HUD
	videoItemShowFPS   // scr_showfps: toggles the runtime FPS counter
	videoItemShowSpeed // scr_showspeed: toggles the runtime speed overlay
	videoItemShowTime  // scr_clock: toggles the runtime level clock
	videoItemBack
)

// Audio menu item indices.
const (
	audioItemVolume = iota
	audioItemBack
)

// videoResolution pairs a display width and height. The Video menu cycles
// through a predefined list of these resolutions, matching the choices
// available in C Ironwail's video options.
type videoResolution struct {
	width  int
	height int
}

// SaveSlotInfo describes a single save-game slot as returned by the host's
// save-slot provider. Name is the internal slot identifier (e.g. "s0") and
// DisplayName is the human-readable label shown in the Load/Save menu.
type SaveSlotInfo struct {
	Name        string
	DisplayName string
}

// ModInfo describes a mod directory available for selection.
type ModInfo struct {
	// Name is the directory name (e.g. "hipnotic", "rogue", "mymod").
	Name string
}

// videoResolutions is the ordered list of display modes the Video menu cycles
// through. They range from 640×480 (classic 4:3) up to 1920×1080 (Full HD).
var videoResolutions = []videoResolution{
	{width: 640, height: 480},
	{width: 800, height: 600},
	{width: 1024, height: 768},
	{width: 1280, height: 720},
	{width: 1366, height: 768},
	{width: 1600, height: 900},
	{width: 1920, height: 1080},
}

// maxFPSValues is the ordered list of frame-rate caps the Video menu offers.
// 72 is the original Quake physics rate; higher values are common for modern
// monitors (144 Hz, 165 Hz, etc.).
var maxFPSValues = []int{60, 72, 120, 144, 165, 240, 250, 300}

// controlBinding pairs a display label (shown in the Controls menu) with the
// console command string that the key will be bound to (e.g. "+forward",
// "+attack", "impulse 10"). This mirrors the bind_name/bind_command arrays
// in C Ironwail's M_Controls_Key().
type controlBinding struct {
	label   string
	command string
}

// controlBindings is the full list of rebindable actions shown in the Controls
// menu, in display order. Each entry maps a human-readable label to the engine
// command that will be bound when the player presses a key.
var controlBindings = []controlBinding{
	{label: "FORWARD", command: "+forward"},
	{label: "BACKWARD", command: "+back"},
	{label: "TURN LEFT", command: "+left"},
	{label: "TURN RIGHT", command: "+right"},
	{label: "STRAFE LEFT", command: "+moveleft"},
	{label: "STRAFE RIGHT", command: "+moveright"},
	{label: "JUMP", command: "+jump"},
	{label: "ATTACK", command: "+attack"},
	{label: "USE", command: "+use"},
	{label: "RUN", command: "+speed"},
	{label: "STRAFE", command: "+strafe"},
	{label: "LOOK UP", command: "+lookup"},
	{label: "LOOK DOWN", command: "+lookdown"},
	{label: "CENTER VIEW", command: "centerview"},
	{label: "MOUSE LOOK", command: "+mlook"},
	{label: "KEYBOARD LOOK", command: "+klook"},
	{label: "MOVE UP", command: "+moveup"},
	{label: "MOVE DOWN", command: "+movedown"},
	{label: "NEXT WEAPON", command: "impulse 10"},
	{label: "PREV WEAPON", command: "impulse 12"},
	{label: "TOGGLE CONSOLE", command: "toggleconsole"},
}

// Manager handles the Quake menu system including navigation and rendering.
type Manager struct {
	// state is the current active menu state.
	state MenuState

	// mainCursor is the current selection in the main menu.
	mainCursor int

	singlePlayerCursor int
	skillCursor        int
	skillCanResume     bool
	loadCursor         int
	saveCursor         int
	multiPlayerCursor  int
	optionsCursor      int
	controlsCursor     int
	controlsRebinding  bool
	videoCursor        int
	audioCursor        int
	helpPage           int
	setupCursor        int
	setupHostname      string
	setupName          string
	setupTopColor      int
	setupBottomColor   int
	joinGameCursor     int
	joinAddress        string
	serverBrowser      *inet.ServerBrowser
	serverResults      []inet.HostCacheEntry
	hostGameCursor     int
	hostMaxPlayers     int
	hostGameMode       int
	hostTeamplay       int
	hostFragLimit      int
	hostTimeLimit      int
	hostSkill          int
	hostMapName        string

	quitPrevState MenuState
	confirmLines  [3]string
	onConfirm     func()
	onCancel      func()

	// modsCursor is the current selection in the mods menu.
	modsCursor int
	// modsList holds the mods available for selection, populated on menu entry.
	modsList []ModInfo
	// modsProvider is called to enumerate available mods.
	modsProvider func() []ModInfo
	// currentMod is the currently active mod directory name ("" or "id1" = vanilla).
	currentMod string

	// mouseAccumY accumulates mouse Y movement (in menu-space pixels) to drive
	// cursor selection. Mirrors C Ironwail M_Mousemove() scroll semantics.
	mouseAccumY float32
	// ignoreMouseFrame suppresses one absolute-position update after the menu is
	// shown so the first ungrabbed mouse sample does not immediately jump the
	// hovered selection.
	ignoreMouseFrame bool

	// Cached main menu sub-pics (computed once, reused).
	mainMenuTop    *image.QPic // Top portion of mainmenu.lmp (rows 0-59)
	mainMenuBottom *image.QPic // Bottom portion of mainmenu.lmp (rows 60+)
	mainMenuSplit  bool        // true if sub-pics have been computed

	// drawManager provides access to graphics assets.
	drawManager DrawManager

	// inputSystem provides access to input state.
	inputSystem *input.System

	// active indicates whether the menu is currently displayed.
	active bool

	// commandText queues engine commands (map/load/save/quit).
	commandText func(text string)

	playSound func(name string)

	saveSlotProvider     func(slotCount int) []SaveSlotInfo
	loadSlotLabels       [maxSaveGames]string
	saveSlotLabels       [maxSaveGames]string
	shouldConfirmNewGame func() bool
	resumeGameAvailable  func() bool
	saveEntryAllowed     func() bool
}

// DrawManager defines the interface for loading menu graphics.
type DrawManager interface {
	GetPic(name string) *image.QPic
}

// SetSoundPlayer registers the callback used to play menu sound effects.
// The three standard Quake menu sounds are navigate (menu1.wav), select
// (menu2.wav), and cancel (menu3.wav).
func (m *Manager) SetSoundPlayer(play func(name string)) {
	m.playSound = play
}

// SetSaveSlotProvider registers a callback that returns display labels for
// save-game slots. The provider is called each time the Load or Save menu is
// opened, allowing the labels to reflect the current save files on disk.
func (m *Manager) SetSaveSlotProvider(provider func(slotCount int) []SaveSlotInfo) {
	m.saveSlotProvider = provider
}

// SetModsProvider sets the callback used to enumerate available mod directories.
// The callback is invoked each time the Mods menu is opened to refresh the list.
func (m *Manager) SetModsProvider(provider func() []ModInfo) {
	m.modsProvider = provider
}

// SetCurrentMod records the currently active mod directory name so the mods
// menu can highlight it.  Pass "" or "id1" for vanilla.
func (m *Manager) SetCurrentMod(name string) {
	m.currentMod = name
}

// SetNewGameConfirmationProvider sets a callback that decides whether selecting
// Single Player -> New Game should show a confirmation prompt before starting.
func (m *Manager) SetNewGameConfirmationProvider(provider func() bool) {
	m.shouldConfirmNewGame = provider
}

// SetResumeGameAvailableProvider sets a callback that decides whether selecting
// Single Player -> New Game should offer resuming the canonical autosave first.
func (m *Manager) SetResumeGameAvailableProvider(provider func() bool) {
	m.resumeGameAvailable = provider
}

// SetSaveEntryAllowedProvider sets a callback that decides whether selecting
// Single Player -> Save is currently allowed.
func (m *Manager) SetSaveEntryAllowedProvider(provider func() bool) {
	m.saveEntryAllowed = provider
}

// NewManager creates a new menu manager.
func NewManager(drawMgr DrawManager, inputSys *input.System) *Manager {
	mgr := &Manager{
		state:              MenuNone,
		mainCursor:         0,
		singlePlayerCursor: 0,
		skillCursor:        0,
		skillCanResume:     false,
		loadCursor:         0,
		saveCursor:         0,
		multiPlayerCursor:  0,
		optionsCursor:      0,
		controlsCursor:     0,
		controlsRebinding:  false,
		videoCursor:        0,
		audioCursor:        0,
		helpPage:           0,
		setupCursor:        0,
		setupHostname:      setupDefaultHostname,
		setupName:          setupDefaultName,
		setupTopColor:      0,
		setupBottomColor:   0,
		joinGameCursor:     0,
		joinAddress:        "local",
		serverBrowser:      inet.NewServerBrowser(),
		hostGameCursor:     0,
		hostMaxPlayers:     hostMaxPlayersMax,
		hostGameMode:       1,
		hostTeamplay:       0,
		hostFragLimit:      0,
		hostTimeLimit:      0,
		hostSkill:          1,
		hostMapName:        "start",
		drawManager:        drawMgr,
		inputSystem:        inputSys,
		active:             false,
		commandText:        cmdsys.AddText,
	}
	mgr.resetSaveSlotLabels()
	return mgr
}

// ToggleMenu toggles the menu on or off.
// If the menu is off, it shows the main menu.
// If the menu is on, it closes and returns to the game.
func (m *Manager) ToggleMenu() {
	if m.active {
		// Close the menu
		m.playMenuSound(menuSoundCancel)
		m.active = false
		m.state = MenuNone
		// Restore input destination to game
		if m.inputSystem != nil {
			m.inputSystem.SetKeyDest(input.KeyGame)
		}
	} else {
		// Open the menu
		m.playMenuSound(menuSoundSelect)
		m.active = true
		m.ignoreMouseFrame = true
		m.state = MenuMain
		m.mainCursor = 0
		m.refreshModsList()
		// Redirect input to menu
		if m.inputSystem != nil {
			m.inputSystem.SetKeyDest(input.KeyMenu)
		}
	}
}

// ShowMenu shows the menu, defaulting to main menu.
func (m *Manager) ShowMenu() {
	m.active = true
	m.ignoreMouseFrame = true
	if m.state == MenuNone {
		m.state = MenuMain
		m.mainCursor = 0
		m.refreshModsList()
	}
	if m.inputSystem != nil {
		m.inputSystem.SetKeyDest(input.KeyMenu)
	}
}

// ShowConfirmationPrompt displays a yes/no confirmation screen.
// Confirm hides the menu and runs onConfirm. Cancel either returns to cancelState
// or hides the menu when cancelState is MenuNone, then runs onCancel.
func (m *Manager) ShowConfirmationPrompt(lines []string, onConfirm, onCancel func(), cancelState MenuState) {
	m.active = true
	m.ignoreMouseFrame = true
	m.state = MenuQuit
	m.quitPrevState = cancelState
	m.onConfirm = onConfirm
	m.onCancel = onCancel
	m.confirmLines = [3]string{}
	for i := 0; i < len(lines) && i < len(m.confirmLines); i++ {
		m.confirmLines[i] = lines[i]
	}
	if m.inputSystem != nil {
		m.inputSystem.SetKeyDest(input.KeyMenu)
	}
}

// ShowQuitPrompt displays the standard quit confirmation prompt.
func (m *Manager) ShowQuitPrompt() {
	m.ShowConfirmationPrompt([]string{
		"ARE YOU SURE YOU WANT TO QUIT?",
		"PRESS Y OR ENTER TO QUIT",
		"PRESS N OR ESC TO CANCEL",
	}, func() {
		m.queueCommand("quit\n")
	}, nil, MenuMain)
}

// HideMenu hides the menu and returns to the game.
func (m *Manager) HideMenu() {
	m.active = false
	m.ignoreMouseFrame = false
	m.state = MenuNone
	m.clearConfirmationPrompt()
	if m.inputSystem != nil {
		m.inputSystem.SetKeyDest(input.KeyGame)
	}
}

func (m *Manager) clearConfirmationPrompt() {
	m.confirmLines = [3]string{}
	m.onConfirm = nil
	m.onCancel = nil
}

// RequestQuit requests the host to quit.
// This is called from the menu when the user confirms quit.
func (m *Manager) RequestQuit() {
	// The host will need to check for quit requests
	// For now, we just hide the menu
	m.HideMenu()
}

// IsActive returns true if the menu is currently displayed.
func (m *Manager) IsActive() bool {
	return m.active
}

// ForcedUnderwater returns true when the Video options menu is open and the cursor
// is positioned on the WATERWARP option. This causes the renderer to preview the
// underwater warp effect even if the camera is not in a liquid leaf.
//
// Mirrors C Ironwail M_ForcedUnderwater() / M_Options_ForcedUnderwater():
//
//	qboolean M_ForcedUnderwater(void) {
//	    return key_dest == key_menu && M_GetBaseState(m_state) == m_options
//	        && M_Options_ForcedUnderwater();
//	}
//	static qboolean M_Options_ForcedUnderwater(void) {
//	    return optionsmenu.preview.id == OPT_WATERWARP;
//	}
func (m *Manager) ForcedUnderwater() bool {
	return m.active && m.state == MenuVideo && m.videoCursor == videoItemWaterwarp
}

// GetState returns the current menu state.
func (m *Manager) GetState() MenuState {
	return m.state
}

func (m *Manager) WaitingForKeyBinding() bool {
	return m != nil && m.active && m.state == MenuControls && m.controlsRebinding
}

func (m *Manager) MainCursor() int {
	if m == nil {
		return 0
	}
	return m.mainCursor
}

// M_Key handles keyboard input for the menu.
func (m *Manager) M_Key(key int) {
	// Any key resets the mouse accumulator so keyboard nav takes precedence.
	m.mouseAccumY = 0
	key = normalizeMenuKey(key)
	switch m.state {
	case MenuMain:
		m.mainKey(key)
	case MenuSinglePlayer:
		m.singlePlayerKey(key)
	case MenuSkill:
		m.skillKey(key)
	case MenuLoad:
		m.loadKey(key)
	case MenuSave:
		m.saveKey(key)
	case MenuMultiPlayer:
		m.multiPlayerKey(key)
	case MenuJoinGame:
		m.joinGameKey(key)
	case MenuHostGame:
		m.hostGameKey(key)
	case MenuOptions:
		m.optionsKey(key)
	case MenuControls:
		m.controlsKey(key)
	case MenuVideo:
		m.videoKey(key)
	case MenuAudio:
		m.audioKey(key)
	case MenuHelp:
		m.helpKey(key)
	case MenuQuit:
		m.quitKey(key)
	case MenuSetup:
		m.setupKey(key)
	case MenuMods:
		m.modsKey(key)
	}
}

// normalizeMenuKey maps gamepad buttons to equivalent menu/navigation keys so
// menu pages can be fully operated from a controller.
func normalizeMenuKey(key int) int {
	switch key {
	case input.KDpadUp, input.KDpadUpAlt:
		return input.KUpArrow
	case input.KDpadDown, input.KDpadDownAlt:
		return input.KDownArrow
	case input.KDpadLeft, input.KDpadLeftAlt:
		return input.KLeftArrow
	case input.KDpadRight, input.KDpadRightAlt:
		return input.KRightArrow
	case input.KAButton, input.KAButtonAlt, input.KStart:
		return input.KEnter
	case input.KBButton, input.KBButtonAlt:
		return input.KEscape
	case input.KBack:
		return input.KBackspace
	default:
		return key
	}
}

// M_Char handles typed characters for menu text-entry fields.
func (m *Manager) M_Char(char rune) {
	switch m.state {
	case MenuSetup:
		m.setupChar(char)
	case MenuJoinGame:
		m.joinGameChar(char)
	case MenuHostGame:
		m.hostGameChar(char)
	}
}

// M_Draw renders the current menu state.
func (m *Manager) M_Draw(dc renderer.RenderContext) {
	switch m.state {
	case MenuMain:
		m.drawMain(dc)
	case MenuSinglePlayer:
		m.drawSinglePlayer(dc)
	case MenuSkill:
		m.drawSkill(dc)
	case MenuLoad:
		m.drawLoad(dc)
	case MenuSave:
		m.drawSave(dc)
	case MenuMultiPlayer:
		m.drawMultiPlayer(dc)
	case MenuJoinGame:
		m.drawJoinGame(dc)
	case MenuHostGame:
		m.drawHostGame(dc)
	case MenuOptions:
		m.drawOptions(dc)
	case MenuControls:
		m.drawControls(dc)
	case MenuVideo:
		m.drawVideo(dc)
	case MenuAudio:
		m.drawAudio(dc)
	case MenuHelp:
		m.drawHelp(dc)
	case MenuQuit:
		m.drawQuit(dc)
	case MenuSetup:
		m.drawSetup(dc)
	case MenuMods:
		m.drawMods(dc)
	}
}

// M_Mousemove drives menu cursor selection from accumulated mouse movement.
// It mirrors C Ironwail's M_Mousemove() semantics: mouse Y movement is
// accumulated in menu-space units and translated to cursor steps.
// dx is currently unused but accepted for future left/right slider support.
//
// Callers should pass the raw mouse delta (in screen pixels) each frame when
// the menu is active. The method translates these to menu-space units using
// the 320×200 reference grid, then moves the cursor when the accumulator
// crosses a full item height (8 px in menu space).
func (m *Manager) M_Mousemove(dx, dy int) {
	if !m.active || dy == 0 {
		return
	}
	// Scale dy to menu-space coordinates (320×200 reference).
	// Each menu item is approximately 8–20 px tall in menu space;
	// use 8 px as the scroll threshold (one text line).
	const menuItemPx = 8
	m.mouseAccumY += float32(dy)
	for m.mouseAccumY >= menuItemPx {
		m.mouseAccumY -= menuItemPx
		m.moveCursorDown()
	}
	for m.mouseAccumY <= -menuItemPx {
		m.mouseAccumY += menuItemPx
		m.moveCursorUp()
	}
}

// M_MousemoveAbsolute updates the active menu cursor from an absolute mouse
// position already converted into 320x200 menu-space coordinates.
func (m *Manager) M_MousemoveAbsolute(x, y int) {
	if !m.active {
		return
	}
	if m.ignoreMouseFrame {
		m.ignoreMouseFrame = false
		return
	}
	cursor, ok := m.menuCursorForPoint(x, y)
	if !ok {
		return
	}
	m.setMenuCursor(cursor)
}

func (m *Manager) setMenuCursor(cursor int) {
	changed := false
	switch m.state {
	case MenuMain:
		changed = m.mainCursor != cursor
		m.mainCursor = cursor
	case MenuSinglePlayer:
		changed = m.singlePlayerCursor != cursor
		m.singlePlayerCursor = cursor
	case MenuSkill:
		changed = m.skillCursor != cursor
		m.skillCursor = cursor
	case MenuLoad:
		changed = m.loadCursor != cursor
		m.loadCursor = cursor
	case MenuSave:
		changed = m.saveCursor != cursor
		m.saveCursor = cursor
	case MenuMultiPlayer:
		changed = m.multiPlayerCursor != cursor
		m.multiPlayerCursor = cursor
	case MenuOptions:
		changed = m.optionsCursor != cursor
		m.optionsCursor = cursor
	case MenuControls:
		changed = m.controlsCursor != cursor
		m.controlsCursor = cursor
	case MenuVideo:
		changed = m.videoCursor != cursor
		m.videoCursor = cursor
	case MenuAudio:
		changed = m.audioCursor != cursor
		m.audioCursor = cursor
	case MenuSetup:
		changed = m.setupCursor != cursor
		m.setupCursor = cursor
	case MenuJoinGame:
		changed = m.joinGameCursor != cursor
		m.joinGameCursor = cursor
	case MenuHostGame:
		changed = m.hostGameCursor != cursor
		m.hostGameCursor = cursor
	case MenuMods:
		changed = m.modsCursor != cursor
		m.modsCursor = cursor
	default:
		return
	}
	if changed {
		m.playMenuSound(menuSoundNavigate)
	}
}

func (m *Manager) menuCursorForPoint(x, y int) (int, bool) {
	_ = x
	switch m.state {
	case MenuMain:
		visible := []int{mainSinglePlayer, mainMultiPlayer, mainOptions, mainHelp, mainQuit}
		if len(m.modsList) > 0 {
			visible = []int{mainSinglePlayer, mainMultiPlayer, mainOptions, mainMods, mainHelp, mainQuit}
		}
		slot, ok := hitTestStride(y, 32, 20, len(visible))
		if !ok {
			return 0, false
		}
		return visible[slot], true
	case MenuSinglePlayer:
		return hitTestStride(y, 32, 20, singlePlayerItems)
	case MenuSkill:
		return hitTestTable(y, m.skillRowPositions(), 8)
	case MenuLoad:
		return hitTestStride(y, 32, 8, maxSaveGames)
	case MenuSave:
		return hitTestStride(y, 32, 8, maxSaveGames)
	case MenuMultiPlayer:
		return hitTestStride(y, 32, 20, multiPlayerItems)
	case MenuOptions:
		return hitTestTable(y, []int{32, 52, 72, 92, 112}, 8)
	case MenuControls:
		rows := make([]int, controlsItems)
		for i := 0; i < controlsItems; i++ {
			rows[i] = controlRowY(i)
		}
		return hitTestTable(y, rows, 8)
	case MenuVideo:
		return hitTestTable(y, []int{32, 48, 64, 80, 96, 112, 128, 144, 168}, 8)
	case MenuAudio:
		return hitTestTable(y, []int{56, 88}, 8)
	case MenuSetup:
		return hitTestTable(y, []int{40, 56, 80, 104, 140}, 8)
	case MenuJoinGame:
		if cursor, ok := hitTestTable(y, []int{48, 72, 96, 120}, 8); ok {
			return cursor, true
		}
		serverRows := min(len(m.serverResults), joinGameVisibleResults)
		if slot, ok := hitTestStride(y, 152, 8, serverRows); ok {
			return joinGameBaseItems + slot, true
		}
		return 0, false
	case MenuHostGame:
		return hitTestTable(y, []int{32, 48, 64, 80, 96, 112, 128, 152, 176}, 8)
	case MenuMods:
		if len(m.modsList) == 0 {
			if y >= 136 && y < 144 {
				return 0, true
			}
			return 0, false
		}
		rows := make([]int, 0, len(m.modsList)+1)
		for i := range m.modsList {
			rows = append(rows, 32+i*8)
		}
		rows = append(rows, 32+len(m.modsList)*8+8)
		return hitTestTable(y, rows, 8)
	default:
		return 0, false
	}
}

func hitTestStride(y, startY, itemHeight, numItems int) (int, bool) {
	if numItems <= 0 || y < startY {
		return 0, false
	}
	slot := (y - startY) / itemHeight
	if slot < 0 || slot >= numItems {
		return 0, false
	}
	return slot, true
}

func hitTestTable(y int, rows []int, height int) (int, bool) {
	for i, rowY := range rows {
		if y >= rowY && y < rowY+height {
			return i, true
		}
	}
	return 0, false
}

// moveCursorDown moves the active menu cursor down one step.
func (m *Manager) moveCursorDown() {
	switch m.state {
	case MenuMain:
		m.mainCursor++
		if m.mainCursor >= mainItems {
			m.mainCursor = 0
		}
		if len(m.modsList) == 0 && m.mainCursor == mainMods {
			m.mainCursor++
		}
	case MenuSinglePlayer:
		m.singlePlayerCursor = (m.singlePlayerCursor + 1) % singlePlayerItems
	case MenuSkill:
		m.skillCursor = (m.skillCursor + 1) % m.skillItemCount()
	case MenuLoad:
		m.loadCursor = (m.loadCursor + 1) % maxSaveGames
	case MenuSave:
		m.saveCursor = (m.saveCursor + 1) % maxSaveGames
	case MenuMultiPlayer:
		m.multiPlayerCursor = (m.multiPlayerCursor + 1) % multiPlayerItems
	case MenuOptions:
		m.optionsCursor = (m.optionsCursor + 1) % optionsItems
	case MenuControls:
		m.controlsCursor = (m.controlsCursor + 1) % controlsItems
	case MenuVideo:
		m.videoCursor = (m.videoCursor + 1) % videoItems
	case MenuAudio:
		m.audioCursor = (m.audioCursor + 1) % audioItems
	case MenuMods:
		if len(m.modsList) > 0 {
			m.modsCursor = (m.modsCursor + 1) % (len(m.modsList) + 1) // +1 for Back
		}
	case MenuSetup:
		m.setupCursor = (m.setupCursor + 1) % setupItems
	case MenuJoinGame:
		m.joinGameCursor = (m.joinGameCursor + 1) % m.joinGameItemCount()
	case MenuHostGame:
		m.hostGameCursor = (m.hostGameCursor + 1) % hostGameItems
	}
	m.playMenuSound(menuSoundNavigate)
}

// moveCursorUp moves the active menu cursor up one step.
func (m *Manager) moveCursorUp() {
	switch m.state {
	case MenuMain:
		m.mainCursor--
		if m.mainCursor < 0 {
			m.mainCursor = mainItems - 1
		}
		if len(m.modsList) == 0 && m.mainCursor == mainMods {
			m.mainCursor--
		}
	case MenuSinglePlayer:
		m.singlePlayerCursor--
		if m.singlePlayerCursor < 0 {
			m.singlePlayerCursor = singlePlayerItems - 1
		}
	case MenuSkill:
		m.skillCursor--
		if m.skillCursor < 0 {
			m.skillCursor = m.skillItemCount() - 1
		}
	case MenuLoad:
		m.loadCursor--
		if m.loadCursor < 0 {
			m.loadCursor = maxSaveGames - 1
		}
	case MenuSave:
		m.saveCursor--
		if m.saveCursor < 0 {
			m.saveCursor = maxSaveGames - 1
		}
	case MenuMultiPlayer:
		m.multiPlayerCursor--
		if m.multiPlayerCursor < 0 {
			m.multiPlayerCursor = multiPlayerItems - 1
		}
	case MenuOptions:
		m.optionsCursor--
		if m.optionsCursor < 0 {
			m.optionsCursor = optionsItems - 1
		}
	case MenuControls:
		m.controlsCursor--
		if m.controlsCursor < 0 {
			m.controlsCursor = controlsItems - 1
		}
	case MenuVideo:
		m.videoCursor--
		if m.videoCursor < 0 {
			m.videoCursor = videoItems - 1
		}
	case MenuAudio:
		m.audioCursor--
		if m.audioCursor < 0 {
			m.audioCursor = audioItems - 1
		}
	case MenuMods:
		total := len(m.modsList) + 1 // +1 for Back
		if total > 0 {
			m.modsCursor--
			if m.modsCursor < 0 {
				m.modsCursor = total - 1
			}
		}
	case MenuSetup:
		m.setupCursor--
		if m.setupCursor < 0 {
			m.setupCursor = setupItems - 1
		}
	case MenuJoinGame:
		m.joinGameCursor--
		if m.joinGameCursor < 0 {
			m.joinGameCursor = m.joinGameItemCount() - 1
		}
	case MenuHostGame:
		m.hostGameCursor--
		if m.hostGameCursor < 0 {
			m.hostGameCursor = hostGameItems - 1
		}
	}
	m.playMenuSound(menuSoundNavigate)
}

// wrapIndex wraps value into the range [0, count). If value underflows it
// wraps to count-1; if it overflows it wraps to 0. Used for circular menu
// cursor navigation.
func wrapIndex(value, count int) int {
	if count <= 0 {
		return 0
	}
	if value < 0 {
		return count - 1
	}
	if value >= count {
		return 0
	}
	return value
}

// controlRowY converts a Controls-menu item index to a Y pixel coordinate.
// Items are spaced 8 pixels apart starting at Y=24.
func controlRowY(index int) int {
	return 24 + index*8
}

// videoRowY converts a Video-menu item index to a compact Y coordinate.
// The tighter 14px spacing leaves room for a few more bounded option slices
// without changing menu behavior beyond layout.
func videoRowY(index int) int {
	return 28 + index*14
}

// clampFloat restricts value to the closed interval [min, max].
func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// roundToTenth rounds a float64 to one decimal place (e.g. 0.72 → 0.7).
// Used for display-friendly cvar values like sensitivity and gamma.
func roundToTenth(value float64) float64 {
	return math.Round(value*10) / 10
}

// boolLabel returns "ON" or "OFF" for display in menu toggle items.
func boolLabel(value bool) string {
	if value {
		return "ON"
	}
	return "OFF"
}

// getPic is a nil-safe wrapper around drawManager.GetPic. Returns nil if the
// draw manager is not set (headless / test mode).
func (m *Manager) getPic(name string) *image.QPic {
	if m.drawManager == nil {
		return nil
	}
	return m.drawManager.GetPic(name)
}

// queueCommand sends a console command string to the engine's command buffer.
// Falls back to a debug log if no command callback has been registered.
func (m *Manager) queueCommand(text string) {
	if m.commandText != nil {
		m.commandText(text)
		return
	}

	slog.Debug("menu command dropped", "command", text)
}

// playMenuSound plays a menu sound effect if a sound player callback has been
// registered. Silently does nothing otherwise (e.g. in tests).
func (m *Manager) playMenuSound(name string) {
	if m.playSound == nil {
		return
	}
	m.playSound(name)
}
