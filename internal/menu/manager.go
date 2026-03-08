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
)

const (
	mainItems = 5

	singlePlayerItems = 3
	multiPlayerItems  = 3
	joinGameItems     = 3
	hostGameItems     = 6
	optionsItems      = 5
	controlsItems     = 13
	videoItems        = 7
	audioItems        = 2

	maxSaveGames = 12
	helpPages    = 6

	setupItems      = 4
	setupNameMaxLen = 15
	setupColorMax   = 13
	joinAddressMax  = 63
	hostMapMaxLen   = 32

	menuSoundNavigate = "misc/menu1.wav"
	menuSoundSelect   = "misc/menu2.wav"
	menuSoundCancel   = "misc/menu3.wav"
)

const (
	hostGameItemMaxPlayers = iota
	hostGameItemMode
	hostGameItemSkill
	hostGameItemMap
	hostGameItemStart
	hostGameItemBack
)

const (
	controlItemForward = iota
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

const (
	videoItemResolution = iota
	videoItemFullscreen
	videoItemVSync
	videoItemMaxFPS
	videoItemGamma
	videoItemViewModel
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
	setupName          string
	setupTopColor      int
	setupBottomColor   int
	joinGameCursor     int
	joinAddress        string
	hostGameCursor     int
	hostMaxPlayers     int
	hostGameMode       int
	hostSkill          int
	hostMapName        string

	quitPrevState MenuState

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
		setupName:          "player",
		setupTopColor:      0,
		setupBottomColor:   0,
		joinGameCursor:     0,
		joinAddress:        "local",
		hostGameCursor:     0,
		hostMaxPlayers:     4,
		hostGameMode:       0,
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

// GetState returns the current menu state.
func (m *Manager) GetState() MenuState {
	return m.state
}

// M_Key handles keyboard input for the menu.
func (m *Manager) M_Key(key int) {
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
	}
}

// mainKey handles input when in the main menu.
func (m *Manager) mainKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.mainCursor--
		if m.mainCursor < 0 {
			m.mainCursor = mainItems - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.mainCursor++
		if m.mainCursor >= mainItems {
			m.mainCursor = 0
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
	case 0: // Single Player
		m.state = MenuSinglePlayer
	case 1: // Multi Player
		m.state = MenuMultiPlayer
	case 2: // Options
		m.state = MenuOptions
	case 3: // Help
		m.state = MenuHelp
		m.helpPage = 0
	case 4: // Quit
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
			m.applyJoinGame()
		case 2:
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
	m.state = MenuSetup
	m.setupCursor = 0
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
	case input.KLeftArrow, input.KBackspace:
		if m.controlsCursor == controlItemBack {
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
		m.controlsRebinding = true
		m.playMenuSound(menuSoundSelect)
	case input.KEnter, input.KSpace, input.KMouse1:
		if m.controlsCursor == controlItemBack {
			m.playMenuSound(menuSoundSelect)
			m.state = MenuOptions
			return
		}
		m.controlsRebinding = true
		m.playMenuSound(menuSoundSelect)
	case input.KEscape, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuOptions
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
	if index < 0 || index >= len(controlBindings) {
		return "", false
	}
	return controlBindings[index].command, true
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
		m.adjustSetupColor(-1)
		m.playMenuSound(menuSoundNavigate)
	case input.KRightArrow:
		m.adjustSetupColor(1)
		m.playMenuSound(menuSoundNavigate)
	case input.KBackspace:
		if m.setupCursor == 0 {
			m.deleteSetupNameRune()
			m.playMenuSound(menuSoundCancel)
			return
		}
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMultiPlayer
	case input.KEnter, input.KSpace, input.KMouse1:
		m.playMenuSound(menuSoundSelect)
		switch m.setupCursor {
		case 1, 2:
			m.adjustSetupColor(1)
		case 3:
			m.applySetupChanges()
			m.state = MenuMultiPlayer
		}
	}
}

func (m *Manager) setupChar(char rune) {
	if m.setupCursor != 0 {
		return
	}
	if char < 32 || char > 126 {
		return
	}
	if len(m.setupName) >= setupNameMaxLen {
		return
	}
	m.setupName += string(char)
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
	if len(m.setupName) == 0 {
		return
	}
	m.setupName = m.setupName[:len(m.setupName)-1]
}

func (m *Manager) deleteJoinAddressRune() {
	if len(m.joinAddress) == 0 {
		return
	}
	runes := []rune(m.joinAddress)
	m.joinAddress = string(runes[:len(runes)-1])
}

func (m *Manager) deleteHostMapRune() {
	if len(m.hostMapName) == 0 {
		return
	}
	runes := []rune(m.hostMapName)
	m.hostMapName = string(runes[:len(runes)-1])
}

func (m *Manager) adjustSetupColor(delta int) {
	switch m.setupCursor {
	case 1:
		m.setupTopColor = wrapSetupColor(m.setupTopColor + delta)
	case 2:
		m.setupBottomColor = wrapSetupColor(m.setupBottomColor + delta)
	}
}

func (m *Manager) adjustHostGameSetting(delta int) {
	switch m.hostGameCursor {
	case hostGameItemMaxPlayers:
		m.hostMaxPlayers += delta
		if m.hostMaxPlayers < 2 {
			m.hostMaxPlayers = 16
		}
		if m.hostMaxPlayers > 16 {
			m.hostMaxPlayers = 2
		}
	case hostGameItemMode:
		m.hostGameMode = wrapIndex(m.hostGameMode+delta, 2)
	case hostGameItemSkill:
		m.hostSkill += delta
		if m.hostSkill < 0 {
			m.hostSkill = 3
		}
		if m.hostSkill > 3 {
			m.hostSkill = 0
		}
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
	if name != "" {
		escaped := strings.ReplaceAll(name, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
		m.queueCommand(fmt.Sprintf("name \"%s\"\n", escaped))
	}
	m.queueCommand(fmt.Sprintf("color %d %d\n", m.setupTopColor, m.setupBottomColor))
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
	m.queueCommand(fmt.Sprintf("maxplayers %d\n", m.hostMaxPlayers))
	m.queueCommand(fmt.Sprintf("deathmatch %d\n", deathmatch))
	m.queueCommand(fmt.Sprintf("coop %d\n", coop))
	m.queueCommand(fmt.Sprintf("skill %d\n", m.hostSkill))
	m.queueCommand(fmt.Sprintf("map %q\n", mapName))
}

// drawMain renders the main menu.
func (m *Manager) drawMain(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/ttl_main.lmp")

	if pic := m.getPic("gfx/mainmenu.lmp"); pic != nil {
		dc.DrawMenuPic(72, 32, pic)
	} else {
		m.drawText(dc, 84, 32, "SINGLE PLAYER", true)
		m.drawText(dc, 84, 52, "MULTIPLAYER", true)
		m.drawText(dc, 84, 72, "OPTIONS", true)
		m.drawText(dc, 84, 92, "HELP", true)
		m.drawText(dc, 84, 112, "QUIT", true)
	}

	m.drawCursor(dc, 54, 32+m.mainCursor*20)
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
	m.drawText(dc, 56, 72, "CONNECT", true)
	m.drawText(dc, 56, 96, "BACK", true)
	m.drawText(dc, 160, 48, m.joinAddress, true)
	m.drawText(dc, 40, 136, "REMOTE CONNECT IS STILL PENDING", true)
	m.drawText(dc, 40, 152, "USE LOCAL FOR LOOPBACK JOIN", true)

	m.drawArrowCursor(dc, 40, 48+m.joinGameCursor*24)
	if m.joinGameCursor == 0 {
		cursorX := 160 + len(m.joinAddress)*8
		cursorChar := 10 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
		dc.DrawMenuCharacter(cursorX, 48, cursorChar)
	}
}

func (m *Manager) drawHostGame(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_multi.lmp")

	m.drawText(dc, 56, 32, "MAX PLAYERS", true)
	m.drawText(dc, 56, 48, "MODE", true)
	m.drawText(dc, 56, 64, "SKILL", true)
	m.drawText(dc, 56, 80, "MAP", true)
	m.drawText(dc, 56, 112, "START GAME", true)
	m.drawText(dc, 56, 136, "BACK", true)

	m.drawText(dc, 192, 32, fmt.Sprintf("%d", m.hostMaxPlayers), true)
	modeLabel := "COOP"
	if m.hostGameMode == 1 {
		modeLabel = "DEATHMATCH"
	}
	m.drawText(dc, 192, 48, modeLabel, true)
	m.drawText(dc, 192, 64, fmt.Sprintf("%d", m.hostSkill), true)
	m.drawText(dc, 192, 80, m.hostMapName, true)
	m.drawText(dc, 40, 168, "HOSTING USES EXISTING LOCAL LOOPBACK", true)

	cursorRows := []int{32, 48, 64, 80, 112, 136}
	m.drawArrowCursor(dc, 40, cursorRows[m.hostGameCursor])
	if m.hostGameCursor == hostGameItemMap {
		cursorX := 192 + len(m.hostMapName)*8
		cursorChar := 10 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
		dc.DrawMenuCharacter(cursorX, 80, cursorChar)
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
	m.drawText(dc, 56, 136, "BACK", true)

	m.drawArrowCursor(dc, 40, 32+m.videoCursor*16)
	m.drawText(dc, 40, 168, "VIDEO CHANGES ARE SAVED TO CONFIG", true)
	m.drawText(dc, 40, 184, "SOME SETTINGS MAY NEED RESTART", true)
}

func (m *Manager) drawControls(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_option.lmp")

	for i, binding := range controlBindings {
		y := 32 + i*12
		m.drawText(dc, 40, y, binding.label, true)
		m.drawText(dc, 200, y, m.controlBindingLabel(i), true)
	}
	m.drawText(dc, 40, 32+controlItemBack*12, "BACK", true)

	m.drawArrowCursor(dc, 24, 32+m.controlsCursor*12)
	if m.controlsRebinding {
		m.drawText(dc, 24, 188, "PRESS A KEY OR ESC TO CANCEL", true)
		return
	}
	m.drawText(dc, 24, 188, "ENTER/RIGHT BIND LEFT/BKSP CLEAR", true)
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

func (m *Manager) drawSetup(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_multi.lmp")

	m.drawText(dc, 64, 48, "YOUR NAME", true)
	m.drawText(dc, 64, 72, "SHIRT COLOR", true)
	m.drawText(dc, 64, 96, "PANTS COLOR", true)
	m.drawText(dc, 72, 128, "ACCEPT CHANGES", true)

	m.drawText(dc, 176, 48, m.setupName, true)
	m.drawText(dc, 176, 72, fmt.Sprintf("%d", m.setupTopColor), true)
	m.drawText(dc, 176, 96, fmt.Sprintf("%d", m.setupBottomColor), true)

	swatchX := 224
	swatchY := 68
	swatchW := 32
	swatchHalfH := 16
	dc.DrawFill(swatchX, swatchY, swatchW, swatchHalfH, byte(m.setupTopColor*16))
	dc.DrawFill(swatchX, swatchY+swatchHalfH, swatchW, swatchHalfH, byte(m.setupBottomColor*16))

	setupCursorTable := []int{48, 72, 96, 128}
	m.drawArrowCursor(dc, 56, setupCursorTable[m.setupCursor])

	if m.setupCursor == 0 {
		cursorX := 176 + len(m.setupName)*8
		cursorChar := 10 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
		dc.DrawMenuCharacter(cursorX, 48, cursorChar)
	}
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
