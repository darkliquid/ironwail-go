package menu

import (
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/cvar"
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
	multiPlayerItems  = 3
	joinGameItems     = 4
	hostGameItems     = 8
	optionsItems      = 5
	controlsItems     = 17
	videoItems        = 9 // added videoItemHUDStyle
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

const (
	setupClientNameCVar  = "_cl_name"
	setupClientColorCVar = "_cl_color"
	setupHostnameCVar    = "hostname"

	setupDefaultName     = "player"
	setupDefaultHostname = "UNNAMED"
)

const (
	hostGameItemMaxPlayers = iota
	hostGameItemMode
	hostGameItemFragLimit
	hostGameItemTimeLimit
	hostGameItemSkill
	hostGameItemMap
	hostGameItemStart
	hostGameItemBack
)

const (
	setupItemHostname = iota
	setupItemName
	setupItemTopColor
	setupItemBottomColor
	setupItemAccept
	setupItems
)

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
	controlItemNextWeapon
	controlItemPrevWeapon
	controlItemToggleConsole
	controlItemBack
)

const controlsBindingStart = controlItemForward

const (
	videoItemResolution = iota
	videoItemFullscreen
	videoItemVSync
	videoItemMaxFPS
	videoItemGamma
	videoItemViewModel
	videoItemWaterwarp // r_waterwarp: mirrors C Ironwail options-menu OPT_WATERWARP preview
	videoItemHUDStyle  // hud_style: selects classic vs compact HUD
	videoItemBack
)

const (
	audioItemVolume = iota
	audioItemBack
)

type videoResolution struct {
	width  int
	height int
}

type SaveSlotInfo struct {
	Name        string
	DisplayName string
}

// ModInfo describes a mod directory available for selection.
type ModInfo struct {
	// Name is the directory name (e.g. "hipnotic", "rogue", "mymod").
	Name string
}

var videoResolutions = []videoResolution{
	{width: 640, height: 480},
	{width: 800, height: 600},
	{width: 1024, height: 768},
	{width: 1280, height: 720},
	{width: 1366, height: 768},
	{width: 1600, height: 900},
	{width: 1920, height: 1080},
}

var maxFPSValues = []int{60, 72, 120, 144, 165, 240, 250, 300}

type controlBinding struct {
	label   string
	command string
}

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
	hostFragLimit      int
	hostTimeLimit      int
	hostSkill          int
	hostMapName        string

	quitPrevState MenuState

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

	saveSlotProvider func(slotCount int) []SaveSlotInfo
	loadSlotLabels   [maxSaveGames]string
	saveSlotLabels   [maxSaveGames]string
}

// DrawManager defines the interface for loading menu graphics.
type DrawManager interface {
	GetPic(name string) *image.QPic
}

func (m *Manager) SetSoundPlayer(play func(name string)) {
	m.playSound = play
}

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

// NewManager creates a new menu manager.
func NewManager(drawMgr DrawManager, inputSys *input.System) *Manager {
	mgr := &Manager{
		state:              MenuNone,
		mainCursor:         0,
		singlePlayerCursor: 0,
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
	if m.state == MenuNone {
		m.state = MenuMain
		m.mainCursor = 0
		m.refreshModsList()
	}
	if m.inputSystem != nil {
		m.inputSystem.SetKeyDest(input.KeyMenu)
	}
}

// HideMenu hides the menu and returns to the game.
func (m *Manager) HideMenu() {
	m.active = false
	m.state = MenuNone
	if m.inputSystem != nil {
		m.inputSystem.SetKeyDest(input.KeyGame)
	}
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

// M_Key handles keyboard input for the menu.
func (m *Manager) M_Key(key int) {
	// Any key resets the mouse accumulator so keyboard nav takes precedence.
	m.mouseAccumY = 0
	switch m.state {
	case MenuMain:
		m.mainKey(key)
	case MenuSinglePlayer:
		m.singlePlayerKey(key)
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
		m.joinGameCursor = (m.joinGameCursor + 1) % joinGameItems
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
			m.joinGameCursor = joinGameItems - 1
		}
	case MenuHostGame:
		m.hostGameCursor--
		if m.hostGameCursor < 0 {
			m.hostGameCursor = hostGameItems - 1
		}
	}
	m.playMenuSound(menuSoundNavigate)
}

func (m *Manager) mainKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.mainCursor--
		if m.mainCursor < 0 {
			m.mainCursor = mainItems - 1
		}
		if len(m.modsList) == 0 && m.mainCursor == mainMods {
			m.mainCursor--
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.mainCursor++
		if m.mainCursor >= mainItems {
			m.mainCursor = 0
		}
		if len(m.modsList) == 0 && m.mainCursor == mainMods {
			m.mainCursor++
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		m.mainSelect()
	case input.KEscape, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.HideMenu()
	}
}

// mainSelect handles selecting an item from the main menu.
func (m *Manager) mainSelect() {
	switch m.mainCursor {
	case mainSinglePlayer:
		m.state = MenuSinglePlayer
	case mainMultiPlayer:
		m.state = MenuMultiPlayer
	case mainOptions:
		m.state = MenuOptions
	case mainMods:
		m.enterModsMenu()
	case mainHelp:
		m.state = MenuHelp
		m.helpPage = 0
	case mainQuit:
		m.quitPrevState = MenuMain
		m.state = MenuQuit
	}
}

func (m *Manager) singlePlayerKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.singlePlayerCursor--
		if m.singlePlayerCursor < 0 {
			m.singlePlayerCursor = singlePlayerItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.singlePlayerCursor++
		if m.singlePlayerCursor >= singlePlayerItems {
			m.singlePlayerCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		switch m.singlePlayerCursor {
		case 0:
			m.HideMenu()
			m.queueCommand("disconnect\n")
			m.queueCommand("maxplayers 1\n")
			m.queueCommand("deathmatch 0\n")
			m.queueCommand("coop 0\n")
			m.queueCommand("map start\n")
		case 1:
			m.enterLoadMenu()
		case 2:
			m.enterSaveMenu()
		}
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMain
	}
}

func (m *Manager) loadKey(key int) {
	switch key {
	case input.KUpArrow, input.KLeftArrow, input.KMWheelUp:
		m.loadCursor--
		if m.loadCursor < 0 {
			m.loadCursor = maxSaveGames - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KRightArrow, input.KMWheelDown:
		m.loadCursor++
		if m.loadCursor >= maxSaveGames {
			m.loadCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		m.HideMenu()
		m.queueCommand(fmt.Sprintf("load s%d\n", m.loadCursor))
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuSinglePlayer
	}
}

func (m *Manager) saveKey(key int) {
	switch key {
	case input.KUpArrow, input.KLeftArrow, input.KMWheelUp:
		m.saveCursor--
		if m.saveCursor < 0 {
			m.saveCursor = maxSaveGames - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KRightArrow, input.KMWheelDown:
		m.saveCursor++
		if m.saveCursor >= maxSaveGames {
			m.saveCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		m.HideMenu()
		m.queueCommand(fmt.Sprintf("save s%d\n", m.saveCursor))
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuSinglePlayer
	}
}

func (m *Manager) multiPlayerKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.multiPlayerCursor--
		if m.multiPlayerCursor < 0 {
			m.multiPlayerCursor = multiPlayerItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.multiPlayerCursor++
		if m.multiPlayerCursor >= multiPlayerItems {
			m.multiPlayerCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		switch m.multiPlayerCursor {
		case 0:
			m.state = MenuJoinGame
			m.joinGameCursor = 0
		case 1:
			m.syncHostGameValues()
			m.state = MenuHostGame
			m.hostGameCursor = 0
		case 2:
			m.enterSetupMenu()
		}
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMain
	}
}

func (m *Manager) joinGameKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.joinGameCursor--
		if m.joinGameCursor < 0 {
			m.joinGameCursor = joinGameItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.joinGameCursor++
		if m.joinGameCursor >= joinGameItems {
			m.joinGameCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KBackspace:
		if m.joinGameCursor == 0 {
			m.deleteJoinAddressRune()
			m.playMenuSound(menuSoundCancel)
			return
		}
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMultiPlayer
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		switch m.joinGameCursor {
		case 1:
			m.startServerSearch()
		case 2:
			m.applyJoinGame()
		case 3:
			m.state = MenuMultiPlayer
		}
	case input.KEscape, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMultiPlayer
	}
}

func (m *Manager) hostGameKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.hostGameCursor--
		if m.hostGameCursor < 0 {
			m.hostGameCursor = hostGameItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.hostGameCursor++
		if m.hostGameCursor >= hostGameItems {
			m.hostGameCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KLeftArrow:
		m.adjustHostGameSetting(-1)
		m.playMenuSound(menuSoundNavigate)
	case input.KRightArrow:
		m.adjustHostGameSetting(1)
		m.playMenuSound(menuSoundNavigate)
	case input.KBackspace:
		if m.hostGameCursor == hostGameItemMap {
			m.deleteHostMapRune()
			m.playMenuSound(menuSoundCancel)
			return
		}
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMultiPlayer
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		switch m.hostGameCursor {
		case hostGameItemStart:
			m.applyHostGame()
		case hostGameItemBack:
			m.state = MenuMultiPlayer
		default:
			m.adjustHostGameSetting(1)
		}
	case input.KEscape, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMultiPlayer
	}
}

func (m *Manager) enterSetupMenu() {
	m.syncSetupValues()
	m.state = MenuSetup
	m.setupCursor = setupItemHostname
}

func (m *Manager) syncSetupValues() {
	m.setupHostname = currentSetupHostname()
	m.setupName = currentSetupName()
	m.setupTopColor, m.setupBottomColor = splitSetupColors(currentSetupColor())
}

func (m *Manager) syncHostGameValues() {
	maxPlayers := m.hostMaxPlayers
	if cv := cvar.Get("maxplayers"); cv != nil {
		maxPlayers = cv.Int
	}
	if maxPlayers < hostMaxPlayersMin {
		maxPlayers = hostMaxPlayersMax
	}
	if maxPlayers > hostMaxPlayersMax {
		maxPlayers = hostMaxPlayersMax
	}
	m.hostMaxPlayers = maxPlayers

	if cv := cvar.Get("skill"); cv != nil {
		m.hostSkill = cv.Int
	}
	if m.hostSkill < 0 {
		m.hostSkill = 3
	}
	if m.hostSkill > 3 {
		m.hostSkill = 0
	}

	m.hostGameMode = 1
	if cv := cvar.Get("coop"); cv != nil && cv.Int != 0 {
		m.hostGameMode = 0
	}

	if cv := cvar.Get("fraglimit"); cv != nil {
		m.hostFragLimit = cv.Int
	}
	if m.hostFragLimit < 0 {
		m.hostFragLimit = 0
	}
	if m.hostFragLimit > 100 {
		m.hostFragLimit = 100
	}

	if cv := cvar.Get("timelimit"); cv != nil {
		m.hostTimeLimit = cv.Int
	}
	if m.hostTimeLimit < 0 {
		m.hostTimeLimit = 0
	}
	if m.hostTimeLimit > 60 {
		m.hostTimeLimit = 60
	}
}

func currentSetupHostname() string {
	if cv := cvar.Get(setupHostnameCVar); cv != nil && cv.String != "" {
		return cv.String
	}
	return setupDefaultHostname
}

func currentSetupName() string {
	if cv := cvar.Get(setupClientNameCVar); cv != nil {
		return cv.String
	}
	return setupDefaultName
}

func currentSetupColor() int {
	if cv := cvar.Get(setupClientColorCVar); cv != nil {
		return cv.Int
	}
	return 0
}

func splitSetupColors(color int) (top, bottom int) {
	return wrapSetupColor((color >> 4) & 0x0f), wrapSetupColor(color & 0x0f)
}

func (m *Manager) optionsKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.optionsCursor--
		if m.optionsCursor < 0 {
			m.optionsCursor = optionsItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.optionsCursor++
		if m.optionsCursor >= optionsItems {
			m.optionsCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		switch m.optionsCursor {
		case 0:
			m.controlsCursor = 0
			m.controlsRebinding = false
			m.state = MenuControls
		case 1:
			m.videoCursor = 0
			m.state = MenuVideo
		case 2:
			m.audioCursor = 0
			m.state = MenuAudio
		case 3:
			cvar.SetBool("vid_vsync", !cvar.BoolValue("vid_vsync"))
		case 4:
			m.state = MenuMain
		}
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMain
	}
}

func (m *Manager) controlsKey(key int) {
	if m.controlsRebinding {
		switch key {
		case input.KEscape, input.KMouse2:
			m.controlsRebinding = false
			m.playMenuSound(menuSoundCancel)
		default:
			m.setControlBinding(m.controlsCursor, key)
			m.controlsRebinding = false
			m.playMenuSound(menuSoundSelect)
		}
		return
	}

	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.controlsCursor--
		if m.controlsCursor < 0 {
			m.controlsCursor = controlsItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.controlsCursor++
		if m.controlsCursor >= controlsItems {
			m.controlsCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KLeftArrow:
		if m.controlsCursor == controlItemBack {
			m.playMenuSound(menuSoundCancel)
			m.state = MenuOptions
			return
		}
		if m.controlsCursor < controlsBindingStart {
			m.adjustControlSetting(-1)
			m.playMenuSound(menuSoundNavigate)
			return
		}
		m.clearControlBinding(m.controlsCursor)
		m.playMenuSound(menuSoundCancel)
	case input.KBackspace:
		if m.controlsCursor < controlsBindingStart || m.controlsCursor == controlItemBack {
			m.playMenuSound(menuSoundCancel)
			m.state = MenuOptions
			return
		}
		m.clearControlBinding(m.controlsCursor)
		m.playMenuSound(menuSoundCancel)
	case input.KRightArrow:
		if m.controlsCursor == controlItemBack {
			m.playMenuSound(menuSoundCancel)
			m.state = MenuOptions
			return
		}
		if m.controlsCursor < controlsBindingStart {
			m.adjustControlSetting(1)
			m.playMenuSound(menuSoundNavigate)
			return
		}
		m.controlsRebinding = true
		m.playMenuSound(menuSoundSelect)
	case input.KEnter, input.KSpace, input.KMouse1:
		if m.controlsCursor == controlItemBack {
			m.playMenuSound(menuSoundSelect)
			m.state = MenuOptions
			return
		}
		if m.controlsCursor < controlsBindingStart {
			m.adjustControlSetting(1)
			m.playMenuSound(menuSoundSelect)
			return
		}
		m.controlsRebinding = true
		m.playMenuSound(menuSoundSelect)
	case input.KEscape, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuOptions
	}
}

func (m *Manager) adjustControlSetting(delta int) {
	switch m.controlsCursor {
	case controlItemMouseSpeed:
		speed := cvar.FloatValue("sensitivity") + 0.5*float64(delta)
		speed = clampFloat(speed, 1, 11)
		cvar.SetFloat("sensitivity", roundToTenth(speed))
	case controlItemInvertMouse:
		pitch := cvar.FloatValue("m_pitch")
		if pitch == 0 {
			pitch = 0.0176
		}
		cvar.SetFloat("m_pitch", -pitch)
	case controlItemAlwaysRun:
		cvar.SetBool("cl_alwaysrun", !cvar.BoolValue("cl_alwaysrun"))
	case controlItemFreeLook:
		cvar.SetBool("freelook", !cvar.BoolValue("freelook"))
	}
}

func (m *Manager) videoKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.videoCursor--
		if m.videoCursor < 0 {
			m.videoCursor = videoItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.videoCursor++
		if m.videoCursor >= videoItems {
			m.videoCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KLeftArrow:
		m.adjustVideoSetting(-1)
		m.playMenuSound(menuSoundNavigate)
	case input.KRightArrow:
		m.adjustVideoSetting(1)
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		if m.videoCursor == videoItemBack {
			m.state = MenuOptions
			return
		}
		m.adjustVideoSetting(1)
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuOptions
	}
}

func (m *Manager) audioKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.audioCursor--
		if m.audioCursor < 0 {
			m.audioCursor = audioItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.audioCursor++
		if m.audioCursor >= audioItems {
			m.audioCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KLeftArrow:
		m.adjustAudioSetting(-1)
		m.playMenuSound(menuSoundNavigate)
	case input.KRightArrow:
		m.adjustAudioSetting(1)
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		if m.audioCursor == audioItemBack {
			m.state = MenuOptions
			return
		}
		m.adjustAudioSetting(1)
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuOptions
	}
}

func (m *Manager) adjustVideoSetting(delta int) {
	switch m.videoCursor {
	case videoItemResolution:
		index := m.currentResolutionIndex()
		index = wrapIndex(index+delta, len(videoResolutions))
		selected := videoResolutions[index]
		cvar.SetInt("vid_width", selected.width)
		cvar.SetInt("vid_height", selected.height)
	case videoItemFullscreen:
		cvar.SetBool("vid_fullscreen", !cvar.BoolValue("vid_fullscreen"))
	case videoItemVSync:
		cvar.SetBool("vid_vsync", !cvar.BoolValue("vid_vsync"))
	case videoItemMaxFPS:
		index := nearestMaxFPSIndex(cvar.IntValue("host_maxfps"))
		index = wrapIndex(index+delta, len(maxFPSValues))
		cvar.SetInt("host_maxfps", maxFPSValues[index])
	case videoItemGamma:
		gamma := cvar.FloatValue("r_gamma") + 0.1*float64(delta)
		gamma = clampFloat(gamma, 0.5, 1.5)
		cvar.SetFloat("r_gamma", roundToTenth(gamma))
	case videoItemViewModel:
		cvar.SetBool("r_drawviewmodel", !cvar.BoolValue("r_drawviewmodel"))
	case videoItemWaterwarp:
		// Cycle through 0=off, 1=screen warp, 2=FOV warp.
		next := (cvar.IntValue("r_waterwarp") + delta + 3) % 3
		cvar.SetInt("r_waterwarp", next)
	case videoItemHUDStyle:
		// Cycle through 0=classic, 1=compact.
		next := (cvar.IntValue("hud_style") + delta + 2) % 2
		cvar.SetInt("hud_style", next)
	}
}

func (m *Manager) adjustAudioSetting(delta int) {
	if m.audioCursor != audioItemVolume {
		return
	}

	volume := cvar.FloatValue("s_volume") + 0.1*float64(delta)
	volume = clampFloat(volume, 0, 1)
	cvar.SetFloat("s_volume", roundToTenth(volume))
}

func (m *Manager) controlBindingLabel(index int) string {
	command, ok := m.controlCommand(index)
	if !ok {
		return ""
	}
	keys := m.keysForBinding(command)
	if len(keys) == 0 {
		return "UNBOUND"
	}
	if len(keys) == 1 {
		return keys[0]
	}
	return fmt.Sprintf("%s +%d", keys[0], len(keys)-1)
}

func (m *Manager) controlCommand(index int) (string, bool) {
	bindingIndex := index - controlsBindingStart
	if bindingIndex < 0 || bindingIndex >= len(controlBindings) {
		return "", false
	}
	return controlBindings[bindingIndex].command, true
}

func (m *Manager) setControlBinding(index, key int) {
	command, ok := m.controlCommand(index)
	if !ok || m.inputSystem == nil || key < 0 || key >= input.NumKeycode {
		return
	}
	m.clearControlBinding(index)
	m.inputSystem.SetBinding(key, command)
}

func (m *Manager) clearControlBinding(index int) {
	command, ok := m.controlCommand(index)
	if !ok || m.inputSystem == nil {
		return
	}
	for key := 0; key < input.NumKeycode; key++ {
		if strings.TrimSpace(m.inputSystem.GetBinding(key)) == command {
			m.inputSystem.SetBinding(key, "")
		}
	}
}

func (m *Manager) keysForBinding(command string) []string {
	if m.inputSystem == nil {
		return nil
	}
	keys := make([]string, 0, 2)
	for key := 0; key < input.NumKeycode; key++ {
		if strings.TrimSpace(m.inputSystem.GetBinding(key)) != command {
			continue
		}
		name := input.KeyToString(key)
		if name == "" {
			name = strconv.Itoa(key)
		}
		keys = append(keys, name)
	}
	return keys
}

func (m *Manager) currentResolutionIndex() int {
	width := cvar.IntValue("vid_width")
	height := cvar.IntValue("vid_height")
	for i, mode := range videoResolutions {
		if mode.width == width && mode.height == height {
			return i
		}
	}
	return nearestResolutionIndex(width, height)
}

func nearestResolutionIndex(width, height int) int {
	for i, mode := range videoResolutions {
		if mode.width >= width && mode.height >= height {
			return i
		}
	}
	return len(videoResolutions) - 1
}

func nearestMaxFPSIndex(value int) int {
	for i, maxFPS := range maxFPSValues {
		if maxFPS >= value {
			return i
		}
	}
	return len(maxFPSValues) - 1
}

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

func controlRowY(index int) int {
	return 24 + index*8
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func roundToTenth(value float64) float64 {
	return math.Round(value*10) / 10
}

func boolLabel(value bool) string {
	if value {
		return "ON"
	}
	return "OFF"
}

func (m *Manager) helpKey(key int) {
	switch key {
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMain
	case input.KUpArrow, input.KRightArrow, input.KMWheelDown, input.KMouse1:
		m.helpPage++
		if m.helpPage >= helpPages {
			m.helpPage = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KLeftArrow, input.KMWheelUp:
		m.helpPage--
		if m.helpPage < 0 {
			m.helpPage = helpPages - 1
		}
		m.playMenuSound(menuSoundNavigate)
	}
}

// quitKey handles input when in the quit confirmation screen.
func (m *Manager) quitKey(key int) {
	switch key {
	case input.KEnter, input.KSpace, input.KMouse1, 'y', 'Y':
		m.playMenuSound(menuSoundSelect)
		m.queueCommand("quit\n")
		m.HideMenu()
	case input.KEscape, input.KBackspace, input.KMouse2, 'n', 'N':
		m.playMenuSound(menuSoundCancel)
		// Cancel - return to main menu
		m.state = m.quitPrevState
	}
}

// enterModsMenu refreshes the mod list and navigates to the Mods menu.
func (m *Manager) enterModsMenu() {
	m.refreshModsList()
	m.modsCursor = 0
	m.state = MenuMods
}

// refreshModsList calls the provider (if set) to update the cached mod list.
func (m *Manager) refreshModsList() {
	if m.modsProvider != nil {
		m.modsList = m.modsProvider()
	}
}

// modsKey handles input when in the Mods browser menu.
func (m *Manager) modsKey(key int) {
	total := len(m.modsList) + 1 // items + "Back"
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.modsCursor--
		if m.modsCursor < 0 {
			m.modsCursor = total - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.modsCursor++
		if m.modsCursor >= total {
			m.modsCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		if m.modsCursor == len(m.modsList) {
			// "Back" item
			m.state = MenuMain
			return
		}
		if m.modsCursor >= 0 && m.modsCursor < len(m.modsList) {
			mod := m.modsList[m.modsCursor]
			m.HideMenu()
			// Relaunch with the selected game directory via the host "game" command.
			m.queueCommand(fmt.Sprintf("game %q\n", mod.Name))
		}
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMain
	}
}

func (m *Manager) setupKey(key int) {
	switch key {
	case input.KEscape, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMultiPlayer
	case input.KUpArrow, input.KMWheelUp:
		m.setupCursor--
		if m.setupCursor < 0 {
			m.setupCursor = setupItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.setupCursor++
		if m.setupCursor >= setupItems {
			m.setupCursor = 0
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KLeftArrow:
		if m.setupCursor < setupItemTopColor || m.setupCursor > setupItemBottomColor {
			return
		}
		m.adjustSetupColor(-1)
		m.playMenuSound(menuSoundNavigate)
	case input.KRightArrow:
		if m.setupCursor < setupItemTopColor || m.setupCursor > setupItemBottomColor {
			return
		}
		m.adjustSetupColor(1)
		m.playMenuSound(menuSoundNavigate)
	case input.KBackspace:
		switch m.setupCursor {
		case setupItemHostname:
			m.deleteSetupHostnameRune()
			m.playMenuSound(menuSoundCancel)
		case setupItemName:
			m.deleteSetupNameRune()
			m.playMenuSound(menuSoundCancel)
		}
	case input.KEnter, input.KSpace, input.KMouse1:
		switch m.setupCursor {
		case setupItemTopColor, setupItemBottomColor:
			m.adjustSetupColor(1)
			m.playMenuSound(menuSoundSelect)
		case setupItemAccept:
			m.applySetupChanges()
			m.playMenuSound(menuSoundSelect)
			m.state = MenuMultiPlayer
		}
	}
}

func (m *Manager) setupChar(char rune) {
	if char < 32 || char > 126 {
		return
	}
	switch m.setupCursor {
	case setupItemHostname:
		if len(m.setupHostname) >= setupHostnameMaxLen {
			return
		}
		m.setupHostname += string(char)
	case setupItemName:
		if len(m.setupName) >= setupNameMaxLen {
			return
		}
		m.setupName += string(char)
	}
}

func (m *Manager) joinGameChar(char rune) {
	if m.joinGameCursor != 0 {
		return
	}
	if char < 32 || char > 126 {
		return
	}
	if len(m.joinAddress) >= joinAddressMax {
		return
	}
	m.joinAddress += string(char)
}

func (m *Manager) hostGameChar(char rune) {
	if m.hostGameCursor != hostGameItemMap {
		return
	}
	if char < 32 || char > 126 {
		return
	}
	if len(m.hostMapName) >= hostMapMaxLen {
		return
	}
	m.hostMapName += string(char)
}

func (m *Manager) deleteSetupNameRune() {
	m.setupName = deleteLastRune(m.setupName)
}

func (m *Manager) deleteSetupHostnameRune() {
	m.setupHostname = deleteLastRune(m.setupHostname)
}

func deleteLastRune(text string) string {
	if len(text) == 0 {
		return text
	}
	runes := []rune(text)
	return string(runes[:len(runes)-1])
}

func (m *Manager) deleteJoinAddressRune() {
	m.joinAddress = deleteLastRune(m.joinAddress)
}

func (m *Manager) deleteHostMapRune() {
	m.hostMapName = deleteLastRune(m.hostMapName)
}

func (m *Manager) adjustSetupColor(delta int) {
	switch m.setupCursor {
	case setupItemTopColor:
		m.setupTopColor = wrapSetupColor(m.setupTopColor + delta)
	case setupItemBottomColor:
		m.setupBottomColor = wrapSetupColor(m.setupBottomColor + delta)
	}
}

func (m *Manager) adjustHostGameSetting(delta int) {
	switch m.hostGameCursor {
	case hostGameItemMaxPlayers:
		m.hostMaxPlayers += delta
		if m.hostMaxPlayers < hostMaxPlayersMin {
			m.hostMaxPlayers = hostMaxPlayersMin
		}
		if m.hostMaxPlayers > hostMaxPlayersMax {
			m.hostMaxPlayers = hostMaxPlayersMax
		}
	case hostGameItemMode:
		m.hostGameMode = wrapIndex(m.hostGameMode+delta, 2)
		if m.hostGameMode == 0 {
			cvar.SetInt("coop", 1)
			cvar.SetInt("deathmatch", 0)
		} else {
			cvar.SetInt("coop", 0)
			cvar.SetInt("deathmatch", 1)
		}
	case hostGameItemFragLimit:
		m.hostFragLimit += delta * 10
		if m.hostFragLimit < 0 {
			m.hostFragLimit = 100
		}
		if m.hostFragLimit > 100 {
			m.hostFragLimit = 0
		}
	case hostGameItemTimeLimit:
		m.hostTimeLimit += delta * 5
		if m.hostTimeLimit < 0 {
			m.hostTimeLimit = 60
		}
		if m.hostTimeLimit > 60 {
			m.hostTimeLimit = 0
		}
	case hostGameItemSkill:
		m.hostSkill += delta
		if m.hostSkill < 0 {
			m.hostSkill = 3
		}
		if m.hostSkill > 3 {
			m.hostSkill = 0
		}
		cvar.SetInt("skill", m.hostSkill)
	}
}

func wrapSetupColor(value int) int {
	if value > setupColorMax {
		return 0
	}
	if value < 0 {
		return setupColorMax
	}
	return value
}

func (m *Manager) applySetupChanges() {
	name := strings.TrimSpace(m.setupName)
	if name != "" && name != currentSetupName() {
		m.queueCommand(fmt.Sprintf("name %q\n", name))
	}
	if m.setupHostname != currentSetupHostname() {
		cvar.Set(setupHostnameCVar, m.setupHostname)
	}
	top, bottom := splitSetupColors(currentSetupColor())
	if m.setupTopColor != top || m.setupBottomColor != bottom {
		m.queueCommand(fmt.Sprintf("color %d %d\n", m.setupTopColor, m.setupBottomColor))
	}
}

func (m *Manager) applyJoinGame() {
	address := strings.TrimSpace(m.joinAddress)
	if address == "" {
		address = "local"
	}
	m.HideMenu()
	m.queueCommand(fmt.Sprintf("connect %q\n", address))
}

func (m *Manager) applyHostGame() {
	mapName := strings.TrimSpace(m.hostMapName)
	if mapName == "" {
		mapName = "start"
	}
	coop := 0
	deathmatch := 0
	if m.hostGameMode == 0 {
		coop = 1
	} else {
		deathmatch = 1
	}
	m.HideMenu()
	m.queueCommand("disconnect\n")
	m.queueCommand("listen 0\n")
	m.queueCommand(fmt.Sprintf("maxplayers %d\n", m.hostMaxPlayers))
	m.queueCommand(fmt.Sprintf("deathmatch %d\n", deathmatch))
	m.queueCommand(fmt.Sprintf("coop %d\n", coop))
	m.queueCommand(fmt.Sprintf("fraglimit %d\n", m.hostFragLimit))
	m.queueCommand(fmt.Sprintf("timelimit %d\n", m.hostTimeLimit))
	m.queueCommand(fmt.Sprintf("skill %d\n", m.hostSkill))
	m.queueCommand(fmt.Sprintf("map %q\n", mapName))
}

// drawMain renders the main menu.
func (m *Manager) drawMain(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/ttl_main.lmp")

	pic := m.getPic("gfx/mainmenu.lmp")
	if pic == nil {
		// Text-only fallback (no graphics loaded).
		m.drawText(dc, 84, 32, "SINGLE PLAYER", true)
		m.drawText(dc, 84, 52, "MULTIPLAYER", true)
		m.drawText(dc, 84, 72, "OPTIONS", true)
		if len(m.modsList) > 0 {
			m.drawText(dc, 84, 92, "MODS", true)
		}
		m.drawText(dc, 84, 32+mainHelp*20, "HELP", true)
		m.drawText(dc, 84, 32+mainQuit*20, "QUIT", true)
		m.drawMainCursor(dc)
		return
	}

	if len(m.modsList) > 0 {
		// Split the graphic and insert MODS between OPTIONS and HELP.
		m.ensureMainMenuSplit(pic)

		const split = 60 // pixel row to split at (after OPTIONS)

		// Draw top portion (SP, MP, OPTIONS).
		if m.mainMenuTop != nil {
			dc.DrawMenuPic(72, 32, m.mainMenuTop)
		}

		// Draw MODS item (sprite or text fallback).
		if modsPic := m.getPic("gfx/menumods.lmp"); modsPic != nil {
			dc.DrawMenuPic(72, 32+split, modsPic)
		} else {
			m.drawText(dc, 74, 32+split+1, "MODS", true)
		}

		// Draw bottom portion (HELP, QUIT).
		if m.mainMenuBottom != nil {
			dc.DrawMenuPic(72, 32+split+20, m.mainMenuBottom)
		}
	} else {
		// No mods — draw full graphic.
		dc.DrawMenuPic(72, 32, pic)
	}

	m.drawMainCursor(dc)
}

// ensureMainMenuSplit creates cached sub-pics from the full main menu graphic.
func (m *Manager) ensureMainMenuSplit(pic *image.QPic) {
	if m.mainMenuSplit {
		return
	}
	const split = 60
	m.mainMenuTop = pic.SubPic(0, 0, int(pic.Width), split)
	m.mainMenuBottom = pic.SubPic(0, split, int(pic.Width), int(pic.Height)-split)
	m.mainMenuSplit = true
}

// drawMainCursor draws the animated main menu cursor at the correct visual position.
func (m *Manager) drawMainCursor(dc renderer.RenderContext) {
	cursor := m.mainCursor
	// When no mods, items after the mods slot shift up visually.
	if len(m.modsList) == 0 && cursor > mainMods {
		cursor--
	}
	m.drawCursor(dc, 54, 32+cursor*20)
}

func (m *Manager) drawSinglePlayer(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/ttl_sgl.lmp")

	if pic := m.getPic("gfx/sp_menu.lmp"); pic != nil {
		dc.DrawMenuPic(72, 32, pic)
	} else {
		m.drawText(dc, 84, 32, "NEW GAME", true)
		m.drawText(dc, 84, 52, "LOAD", true)
		m.drawText(dc, 84, 72, "SAVE", true)
	}

	m.drawCursor(dc, 54, 32+m.singlePlayerCursor*20)
}

func (m *Manager) drawLoad(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_load.lmp")

	for i := 0; i < maxSaveGames; i++ {
		m.drawText(dc, 24, 32+i*8, m.loadSlotLabels[i], true)
	}
	m.drawArrowCursor(dc, 8, 32+m.loadCursor*8)
}

func (m *Manager) drawSave(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_save.lmp")

	for i := 0; i < maxSaveGames; i++ {
		m.drawText(dc, 24, 32+i*8, m.saveSlotLabels[i], true)
	}
	m.drawArrowCursor(dc, 8, 32+m.saveCursor*8)
}

func (m *Manager) drawMultiPlayer(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_multi.lmp")

	if pic := m.getPic("gfx/mp_menu.lmp"); pic != nil {
		dc.DrawMenuPic(72, 32, pic)
	} else {
		m.drawText(dc, 84, 32, "JOIN GAME", true)
		m.drawText(dc, 84, 52, "HOST GAME", true)
		m.drawText(dc, 84, 72, "SETUP", true)
	}

	m.drawCursor(dc, 54, 32+m.multiPlayerCursor*20)
}

func (m *Manager) drawJoinGame(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_multi.lmp")

	m.drawText(dc, 56, 48, "ADDRESS", true)
	m.drawText(dc, 56, 72, "SEARCH LAN", true)
	m.drawText(dc, 56, 96, "CONNECT", true)
	m.drawText(dc, 56, 120, "BACK", true)
	m.drawText(dc, 160, 48, m.joinAddress, true)

	m.drawArrowCursor(dc, 40, 48+m.joinGameCursor*24)
	if m.joinGameCursor == 0 {
		cursorX := 160 + len(m.joinAddress)*8
		cursorChar := 10 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
		dc.DrawMenuCharacter(cursorX, 48, cursorChar)
	}

	// Server list display
	y := 152
	if m.serverBrowser != nil && m.serverBrowser.IsSearching() {
		m.drawText(dc, 40, y, "SEARCHING...", true)
	} else if m.serverBrowser != nil {
		// Refresh results from browser (safe — called from main thread)
		m.serverResults = m.serverBrowser.Results()
		if len(m.serverResults) == 0 {
			m.drawText(dc, 40, y, "NO SERVERS FOUND", true)
		} else {
			for i, entry := range m.serverResults {
				if y+i*8 > 192 {
					break // Don't draw off screen
				}
				line := fmt.Sprintf("%-15s %-8s %d/%d", entry.Name, entry.Map, entry.Players, entry.MaxPlayers)
				m.drawText(dc, 40, y+i*8, line, true)
			}
		}
	} else {
		m.drawText(dc, 40, y, "PRESS SEARCH LAN TO FIND SERVERS", true)
	}
}

// startServerSearch initiates a LAN server scan and stores results when done.
func (m *Manager) startServerSearch() {
	if m.serverBrowser == nil {
		m.serverBrowser = inet.NewServerBrowser()
	}
	if m.serverBrowser.IsSearching() {
		return
	}
	m.serverResults = nil
	m.serverBrowser.Start()
}

func (m *Manager) drawHostGame(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_multi.lmp")

	m.drawText(dc, 56, 32, "MAX PLAYERS", true)
	m.drawText(dc, 56, 48, "MODE", true)
	m.drawText(dc, 56, 64, "FRAG LIMIT", true)
	m.drawText(dc, 56, 80, "TIME LIMIT", true)
	m.drawText(dc, 56, 96, "SKILL", true)
	m.drawText(dc, 56, 112, "MAP", true)
	m.drawText(dc, 56, 144, "START GAME", true)
	m.drawText(dc, 56, 168, "BACK", true)

	m.drawText(dc, 192, 32, fmt.Sprintf("%d", m.hostMaxPlayers), true)
	modeLabel := "COOP"
	if m.hostGameMode == 1 {
		modeLabel = "DEATHMATCH"
	}
	m.drawText(dc, 192, 48, modeLabel, true)
	fragLabel := "NONE"
	if m.hostFragLimit > 0 {
		fragLabel = fmt.Sprintf("%d FRAGS", m.hostFragLimit)
	}
	m.drawText(dc, 192, 64, fragLabel, true)
	timeLabel := "NONE"
	if m.hostTimeLimit > 0 {
		timeLabel = fmt.Sprintf("%d MINUTES", m.hostTimeLimit)
	}
	m.drawText(dc, 192, 80, timeLabel, true)
	m.drawText(dc, 192, 96, fmt.Sprintf("%d", m.hostSkill), true)
	m.drawText(dc, 192, 112, m.hostMapName, true)
	m.drawText(dc, 40, 200, "HOSTING USES EXISTING LOCAL LOOPBACK", true)

	cursorRows := []int{32, 48, 64, 80, 96, 112, 144, 168}
	m.drawArrowCursor(dc, 40, cursorRows[m.hostGameCursor])
	if m.hostGameCursor == hostGameItemMap {
		cursorX := 192 + len(m.hostMapName)*8
		cursorChar := 10 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
		dc.DrawMenuCharacter(cursorX, 112, cursorChar)
	}
}

func (m *Manager) drawOptions(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_option.lmp")

	m.drawText(dc, 84, 32, "CONTROLS", true)
	m.drawText(dc, 84, 52, "VIDEO", true)
	m.drawText(dc, 84, 72, "AUDIO", true)
	m.drawText(dc, 84, 92, "VSYNC", true)
	m.drawText(dc, 84, 112, "BACK", true)

	m.drawCursor(dc, 54, 32+m.optionsCursor*20)
}

func waterwarpLabel(v int) string {
	switch v {
	case 1:
		return "SCREEN WARP"
	case 2:
		return "FOV WARP"
	default:
		return "OFF"
	}
}

func hudStyleLabel(v int) string {
	if v == 1 {
		return "COMPACT"
	}
	return "CLASSIC"
}

func (m *Manager) drawVideo(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_option.lmp")

	mode := videoResolutions[m.currentResolutionIndex()]
	m.drawText(dc, 56, 32, "RESOLUTION", true)
	m.drawText(dc, 184, 32, fmt.Sprintf("%dx%d", mode.width, mode.height), true)
	m.drawText(dc, 56, 48, "FULLSCREEN", true)
	m.drawText(dc, 184, 48, boolLabel(cvar.BoolValue("vid_fullscreen")), true)
	m.drawText(dc, 56, 64, "VSYNC", true)
	m.drawText(dc, 184, 64, boolLabel(cvar.BoolValue("vid_vsync")), true)
	m.drawText(dc, 56, 80, "MAX FPS", true)
	m.drawText(dc, 184, 80, fmt.Sprintf("%d", cvar.IntValue("host_maxfps")), true)
	m.drawText(dc, 56, 96, "GAMMA", true)
	m.drawText(dc, 184, 96, fmt.Sprintf("%.1f", cvar.FloatValue("r_gamma")), true)
	m.drawText(dc, 56, 112, "VIEWMODEL", true)
	m.drawText(dc, 184, 112, boolLabel(cvar.BoolValue("r_drawviewmodel")), true)
	m.drawText(dc, 56, 128, "WATERWARP", true)
	m.drawText(dc, 184, 128, waterwarpLabel(cvar.IntValue("r_waterwarp")), true)
	m.drawText(dc, 56, 144, "HUD STYLE", true)
	m.drawText(dc, 184, 144, hudStyleLabel(cvar.IntValue("hud_style")), true)
	m.drawText(dc, 56, 168, "BACK", true)

	m.drawArrowCursor(dc, 40, 32+m.videoCursor*16)
	m.drawText(dc, 40, 180, "VIDEO CHANGES ARE SAVED TO CONFIG", true)
}

func (m *Manager) drawControls(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_option.lmp")

	m.drawText(dc, 32, controlRowY(controlItemMouseSpeed), "MOUSE SPEED", true)
	m.drawText(dc, 208, controlRowY(controlItemMouseSpeed), fmt.Sprintf("%.1f", cvar.FloatValue("sensitivity")), true)
	m.drawText(dc, 32, controlRowY(controlItemInvertMouse), "INVERT MOUSE", true)
	m.drawText(dc, 208, controlRowY(controlItemInvertMouse), boolLabel(cvar.FloatValue("m_pitch") < 0), true)
	m.drawText(dc, 32, controlRowY(controlItemAlwaysRun), "ALWAYS RUN", true)
	m.drawText(dc, 208, controlRowY(controlItemAlwaysRun), boolLabel(cvar.BoolValue("cl_alwaysrun")), true)
	m.drawText(dc, 32, controlRowY(controlItemFreeLook), "MOUSE LOOK", true)
	m.drawText(dc, 208, controlRowY(controlItemFreeLook), boolLabel(cvar.BoolValue("freelook")), true)

	for i, binding := range controlBindings {
		y := controlRowY(controlsBindingStart + i)
		m.drawText(dc, 40, y, binding.label, true)
		m.drawText(dc, 200, y, m.controlBindingLabel(controlsBindingStart+i), true)
	}
	m.drawText(dc, 40, controlRowY(controlItemBack), "BACK", true)

	m.drawArrowCursor(dc, 24, controlRowY(m.controlsCursor))
	if m.controlsRebinding {
		m.drawText(dc, 24, 176, "PRESS A KEY OR ESC TO CANCEL", true)
		return
	}
	if m.controlsCursor < controlsBindingStart {
		m.drawText(dc, 24, 176, "LEFT/RIGHT/ENTER CHANGE, ESC BACK", true)
		return
	}
	m.drawText(dc, 24, 176, "ENTER/RIGHT BIND LEFT/BKSP CLEAR", true)
}

func (m *Manager) drawAudio(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_option.lmp")

	volumePercent := int(clampFloat(cvar.FloatValue("s_volume"), 0, 1)*100 + 0.5)
	m.drawText(dc, 72, 56, "SOUND VOLUME", true)
	m.drawText(dc, 200, 56, fmt.Sprintf("%d%%", volumePercent), true)
	m.drawText(dc, 72, 88, "BACK", true)

	m.drawArrowCursor(dc, 56, 56+m.audioCursor*32)
}

func (m *Manager) drawHelp(dc renderer.RenderContext) {
	if pic := m.getPic(fmt.Sprintf("gfx/help%d.lmp", m.helpPage)); pic != nil {
		dc.DrawMenuPic(0, 0, pic)
		return
	}

	m.drawPlaqueAndTitle(dc, "gfx/ttl_main.lmp")
	m.drawText(dc, 48, 64, "HELP PAGE", true)
	m.drawText(dc, 136, 64, fmt.Sprintf("%d/%d", m.helpPage+1, helpPages), true)
	m.drawText(dc, 48, 88, "LEFT/RIGHT OR MOUSE1 TO CHANGE", true)
	m.drawText(dc, 48, 104, "ESC TO RETURN", true)
}

// drawQuit renders the quit confirmation screen.
func (m *Manager) drawQuit(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "")
	m.drawText(dc, 56, 64, "ARE YOU SURE YOU WANT TO QUIT?", true)
	m.drawText(dc, 56, 88, "PRESS Y OR ENTER TO QUIT", true)
	m.drawText(dc, 56, 104, "PRESS N OR ESC TO CANCEL", true)
}

// drawMods renders the Mods browser screen.
// It lists every valid mod directory found in the base directory, highlights
// the currently active mod, and shows a "Back" item at the bottom.
func (m *Manager) drawMods(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/ttl_main.lmp")
	m.drawText(dc, 84, 16, "MODS", true)

	if len(m.modsList) == 0 {
		m.drawText(dc, 48, 56, "NO MODS FOUND", true)
		m.drawText(dc, 48, 80, "PLACE MOD DIRECTORIES", true)
		m.drawText(dc, 48, 96, "NEXT TO ID1 IN YOUR", true)
		m.drawText(dc, 48, 112, "QUAKE DIRECTORY", true)
		m.drawText(dc, 48, 136, "BACK", true)
		m.drawArrowCursor(dc, 32, 136)
		return
	}

	const startY = 32
	const lineH = 8
	for i, mod := range m.modsList {
		y := startY + i*lineH
		label := strings.ToUpper(mod.Name)
		// Mark the currently active mod with a trailing asterisk.
		if strings.EqualFold(mod.Name, m.currentMod) {
			label += " *"
		}
		m.drawText(dc, 48, y, label, true)
	}
	// "Back" item after the list
	backY := startY + len(m.modsList)*lineH + lineH
	m.drawText(dc, 48, backY, "BACK", true)

	// Cursor
	cursorY := startY + m.modsCursor*lineH
	if m.modsCursor == len(m.modsList) {
		cursorY = backY
	}
	m.drawArrowCursor(dc, 32, cursorY)
}

func (m *Manager) drawSetup(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_multi.lmp")

	m.drawText(dc, 64, 40, "HOSTNAME", true)
	m.drawText(dc, 64, 56, "YOUR NAME", true)
	m.drawText(dc, 64, 80, "SHIRT COLOR", true)
	m.drawText(dc, 64, 104, "PANTS COLOR", true)
	m.drawText(dc, 72, 140, "ACCEPT CHANGES", true)

	m.drawMenuTextBox(dc, 160, 32, 16, 1)
	m.drawMenuTextBox(dc, 160, 48, 16, 1)
	m.drawMenuTextBox(dc, 64, 132, 14, 1)

	m.drawText(dc, 176, 40, m.setupHostname, true)
	m.drawText(dc, 176, 56, m.setupName, true)
	m.drawText(dc, 176, 80, fmt.Sprintf("%d", m.setupTopColor), true)
	m.drawText(dc, 176, 104, fmt.Sprintf("%d", m.setupBottomColor), true)

	if bigBox := m.getPic("gfx/bigbox.lmp"); bigBox != nil {
		dc.DrawMenuPic(160, 64, bigBox)
	}
	if player := m.getPic("gfx/menuplyr.lmp"); player != nil {
		dc.DrawMenuPic(172, 72, translateSetupPlayerPic(player, m.setupTopColor, m.setupBottomColor))
	}

	setupCursorTable := []int{40, 56, 80, 104, 140}
	m.drawArrowCursor(dc, 56, setupCursorTable[m.setupCursor])

	if m.setupCursor == setupItemHostname {
		cursorX := 176 + len(m.setupHostname)*8
		cursorChar := 10 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
		dc.DrawMenuCharacter(cursorX, 40, cursorChar)
	}

	if m.setupCursor == setupItemName {
		cursorX := 176 + len(m.setupName)*8
		cursorChar := 10 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
		dc.DrawMenuCharacter(cursorX, 56, cursorChar)
	}
}

func (m *Manager) drawMenuTextBox(dc renderer.RenderContext, x, y, width, lines int) {
	cx := x
	cy := y

	if pic := m.getPic("gfx/box_tl.lmp"); pic != nil {
		dc.DrawMenuPic(cx, cy, pic)
	}
	if pic := m.getPic("gfx/box_ml.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			dc.DrawMenuPic(cx, cy, pic)
		}
	}
	if pic := m.getPic("gfx/box_bl.lmp"); pic != nil {
		dc.DrawMenuPic(cx, cy+8, pic)
	}

	cx += 8
	for remaining := width; remaining > 0; remaining -= 2 {
		cy = y
		if pic := m.getPic("gfx/box_tm.lmp"); pic != nil {
			dc.DrawMenuPic(cx, cy, pic)
		}
		for n := 0; n < lines; n++ {
			cy += 8
			name := "gfx/box_mm.lmp"
			if n == 1 {
				name = "gfx/box_mm2.lmp"
			}
			if pic := m.getPic(name); pic != nil {
				dc.DrawMenuPic(cx, cy, pic)
			}
		}
		if pic := m.getPic("gfx/box_bm.lmp"); pic != nil {
			dc.DrawMenuPic(cx, cy+8, pic)
		}
		cx += 16
	}

	cy = y
	if pic := m.getPic("gfx/box_tr.lmp"); pic != nil {
		dc.DrawMenuPic(cx, cy, pic)
	}
	if pic := m.getPic("gfx/box_mr.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			dc.DrawMenuPic(cx, cy, pic)
		}
	}
	if pic := m.getPic("gfx/box_br.lmp"); pic != nil {
		dc.DrawMenuPic(cx, cy+8, pic)
	}
}

func translateSetupPlayerPic(pic *image.QPic, topColor, bottomColor int) *image.QPic {
	if pic == nil {
		return nil
	}

	translated := make([]byte, len(pic.Pixels))
	copy(translated, pic.Pixels)

	topStart := byte((topColor & 15) << 4)
	bottomStart := byte((bottomColor & 15) << 4)

	for i, pixel := range translated {
		switch {
		case pixel >= 16 && pixel < 32:
			translated[i] = translatedPlayerColor(topStart, pixel-16)
		case pixel >= 96 && pixel < 112:
			translated[i] = translatedPlayerColor(bottomStart, pixel-96)
		}
	}

	return &image.QPic{
		Width:  pic.Width,
		Height: pic.Height,
		Pixels: translated,
	}
}

func translatedPlayerColor(start, offset byte) byte {
	if start < 128 {
		return start + offset
	}
	return start + (15 - offset)
}

func (m *Manager) drawPlaqueAndTitle(dc renderer.RenderContext, titlePic string) {
	if pic := m.getPic("gfx/qplaque.lmp"); pic != nil {
		dc.DrawMenuPic(16, 4, pic)
	}

	if titlePic == "" {
		return
	}

	if pic := m.getPic(titlePic); pic != nil {
		x := (320 - int(pic.Width)) / 2
		dc.DrawMenuPic(x, 4, pic)
	}
}

func (m *Manager) drawCursor(dc renderer.RenderContext, x, y int) {
	frame := (time.Now().UnixNano()/int64(200*time.Millisecond))%6 + 1
	picName := fmt.Sprintf("gfx/menudot%d.lmp", frame)
	if pic := m.getPic(picName); pic != nil {
		dc.DrawMenuPic(x, y, pic)
		return
	}

	if pic := m.getPic("gfx/m_surfs.lmp"); pic != nil {
		dc.DrawMenuPic(x, y, pic)
		return
	}

	dc.DrawMenuCharacter(x, y, 12)
}

func (m *Manager) drawArrowCursor(dc renderer.RenderContext, x, y int) {
	char := 12 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
	dc.DrawMenuCharacter(x, y, char)
}

func (m *Manager) drawText(dc renderer.RenderContext, x, y int, text string, white bool) {
	for i, r := range text {
		ch := int(r)
		if white {
			ch += 128
		}
		dc.DrawMenuCharacter(x+i*8, y, ch)
	}
}

func (m *Manager) getPic(name string) *image.QPic {
	if m.drawManager == nil {
		return nil
	}
	return m.drawManager.GetPic(name)
}

func (m *Manager) queueCommand(text string) {
	if m.commandText != nil {
		m.commandText(text)
		return
	}

	slog.Debug("menu command dropped", "command", text)
}

func (m *Manager) playMenuSound(name string) {
	if m.playSound == nil {
		return
	}
	m.playSound(name)
}

func (m *Manager) enterLoadMenu() {
	m.state = MenuLoad
	m.refreshSaveSlotLabels()
}

func (m *Manager) enterSaveMenu() {
	m.state = MenuSave
	m.refreshSaveSlotLabels()
}

func (m *Manager) resetSaveSlotLabels() {
	for i := 0; i < maxSaveGames; i++ {
		label := fmt.Sprintf("s%d", i)
		m.loadSlotLabels[i] = label
		m.saveSlotLabels[i] = label
	}
}

func (m *Manager) refreshSaveSlotLabels() {
	if m.saveSlotProvider == nil {
		m.resetSaveSlotLabels()
		return
	}

	slotInfos := m.saveSlotProvider(maxSaveGames)
	for i := 0; i < maxSaveGames; i++ {
		label := fmt.Sprintf("s%d", i)
		if i < len(slotInfos) {
			if slotLabel := strings.TrimSpace(slotInfos[i].DisplayName); slotLabel != "" {
				label = slotLabel
			}
		}
		m.loadSlotLabels[i] = label
		m.saveSlotLabels[i] = label
	}
}
