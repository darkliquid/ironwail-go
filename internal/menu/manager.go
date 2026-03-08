package menu

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ironwail/ironwail-go/internal/cmdsys"
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
	MenuOptions                       // Options submenu
	MenuHelp                          // Help screens
	MenuQuit                          // Quit confirmation screen
	MenuSetup                         // Player setup screen
)

const (
	mainItems = 5

	singlePlayerItems = 3
	multiPlayerItems  = 3
	optionsItems      = 5

	maxSaveGames = 12
	helpPages    = 6

	setupItems      = 4
	setupNameMaxLen = 15
	setupColorMax   = 13
)

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
	helpPage           int
	setupCursor        int
	setupName          string
	setupTopColor      int
	setupBottomColor   int

	quitPrevState MenuState

	// drawManager provides access to graphics assets.
	drawManager DrawManager

	// inputSystem provides access to input state.
	inputSystem *input.System

	// active indicates whether the menu is currently displayed.
	active bool

	// commandText queues engine commands (map/load/save/quit).
	commandText func(text string)
}

// DrawManager defines the interface for loading menu graphics.
type DrawManager interface {
	GetPic(name string) *image.QPic
}

// NewManager creates a new menu manager.
func NewManager(drawMgr DrawManager, inputSys *input.System) *Manager {
	return &Manager{
		state:              MenuNone,
		mainCursor:         0,
		singlePlayerCursor: 0,
		loadCursor:         0,
		saveCursor:         0,
		multiPlayerCursor:  0,
		optionsCursor:      0,
		helpPage:           0,
		setupCursor:        0,
		setupName:          "player",
		setupTopColor:      0,
		setupBottomColor:   0,
		drawManager:        drawMgr,
		inputSystem:        inputSys,
		active:             false,
		commandText:        cmdsys.AddText,
	}
}

// ToggleMenu toggles the menu on or off.
// If the menu is off, it shows the main menu.
// If the menu is on, it closes and returns to the game.
func (m *Manager) ToggleMenu() {
	if m.active {
		// Close the menu
		m.active = false
		m.state = MenuNone
		// Restore input destination to game
		if m.inputSystem != nil {
			m.inputSystem.SetKeyDest(input.KeyGame)
		}
	} else {
		// Open the menu
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
	case MenuOptions:
		m.optionsKey(key)
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
	if m.state != MenuSetup {
		return
	}
	m.setupChar(char)
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
	case MenuOptions:
		m.drawOptions(dc)
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
	case input.KDownArrow, input.KMWheelDown:
		m.mainCursor++
		if m.mainCursor >= mainItems {
			m.mainCursor = 0
		}
	case input.KEnter, input.KSpace, input.KMouse1:
		m.mainSelect()
	case input.KEscape, input.KMouse2:
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
	case input.KDownArrow, input.KMWheelDown:
		m.singlePlayerCursor++
		if m.singlePlayerCursor >= singlePlayerItems {
			m.singlePlayerCursor = 0
		}
	case input.KEnter, input.KSpace, input.KMouse1:
		switch m.singlePlayerCursor {
		case 0:
			m.HideMenu()
			m.queueCommand("disconnect\n")
			m.queueCommand("maxplayers 1\n")
			m.queueCommand("deathmatch 0\n")
			m.queueCommand("coop 0\n")
			m.queueCommand("map start\n")
		case 1:
			m.state = MenuLoad
		case 2:
			m.state = MenuSave
		}
	case input.KEscape, input.KBackspace, input.KMouse2:
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
	case input.KDownArrow, input.KRightArrow, input.KMWheelDown:
		m.loadCursor++
		if m.loadCursor >= maxSaveGames {
			m.loadCursor = 0
		}
	case input.KEnter, input.KSpace, input.KMouse1:
		m.HideMenu()
		m.queueCommand(fmt.Sprintf("load s%d\n", m.loadCursor))
	case input.KEscape, input.KBackspace, input.KMouse2:
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
	case input.KDownArrow, input.KRightArrow, input.KMWheelDown:
		m.saveCursor++
		if m.saveCursor >= maxSaveGames {
			m.saveCursor = 0
		}
	case input.KEnter, input.KSpace, input.KMouse1:
		m.HideMenu()
		m.queueCommand(fmt.Sprintf("save s%d\n", m.saveCursor))
	case input.KEscape, input.KBackspace, input.KMouse2:
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
	case input.KDownArrow, input.KMWheelDown:
		m.multiPlayerCursor++
		if m.multiPlayerCursor >= multiPlayerItems {
			m.multiPlayerCursor = 0
		}
	case input.KEnter, input.KSpace, input.KMouse1:
		switch m.multiPlayerCursor {
		case 0:
			m.queueCommand("echo Join game menu is TODO\n")
		case 1:
			m.queueCommand("echo Host game menu is TODO\n")
		case 2:
			m.enterSetupMenu()
		}
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.state = MenuMain
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
	case input.KDownArrow, input.KMWheelDown:
		m.optionsCursor++
		if m.optionsCursor >= optionsItems {
			m.optionsCursor = 0
		}
	case input.KEnter, input.KSpace, input.KMouse1:
		switch m.optionsCursor {
		case 0:
			m.queueCommand("echo Controls menu is TODO\n")
		case 1:
			m.queueCommand("echo Video menu is TODO\n")
		case 2:
			m.queueCommand("echo Audio menu is TODO\n")
		case 3:
			m.queueCommand("toggle vid_vsync\n")
		case 4:
			m.state = MenuMain
		}
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.state = MenuMain
	}
}

func (m *Manager) helpKey(key int) {
	switch key {
	case input.KEscape, input.KBackspace, input.KMouse2:
		m.state = MenuMain
	case input.KUpArrow, input.KRightArrow, input.KMWheelDown, input.KMouse1:
		m.helpPage++
		if m.helpPage >= helpPages {
			m.helpPage = 0
		}
	case input.KDownArrow, input.KLeftArrow, input.KMWheelUp:
		m.helpPage--
		if m.helpPage < 0 {
			m.helpPage = helpPages - 1
		}
	}
}

// quitKey handles input when in the quit confirmation screen.
func (m *Manager) quitKey(key int) {
	switch key {
	case input.KEnter, input.KSpace, input.KMouse1, 'y', 'Y':
		m.queueCommand("quit\n")
		m.HideMenu()
	case input.KEscape, input.KBackspace, input.KMouse2, 'n', 'N':
		// Cancel - return to main menu
		m.state = m.quitPrevState
	}
}

func (m *Manager) setupKey(key int) {
	switch key {
	case input.KEscape, input.KMouse2:
		m.state = MenuMultiPlayer
	case input.KUpArrow, input.KMWheelUp:
		m.setupCursor--
		if m.setupCursor < 0 {
			m.setupCursor = setupItems - 1
		}
	case input.KDownArrow, input.KMWheelDown:
		m.setupCursor++
		if m.setupCursor >= setupItems {
			m.setupCursor = 0
		}
	case input.KLeftArrow:
		m.adjustSetupColor(-1)
	case input.KRightArrow:
		m.adjustSetupColor(1)
	case input.KBackspace:
		if m.setupCursor == 0 {
			m.deleteSetupNameRune()
			return
		}
		m.state = MenuMultiPlayer
	case input.KEnter, input.KSpace, input.KMouse1:
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

func (m *Manager) deleteSetupNameRune() {
	if len(m.setupName) == 0 {
		return
	}
	m.setupName = m.setupName[:len(m.setupName)-1]
}

func (m *Manager) adjustSetupColor(delta int) {
	switch m.setupCursor {
	case 1:
		m.setupTopColor = wrapSetupColor(m.setupTopColor + delta)
	case 2:
		m.setupBottomColor = wrapSetupColor(m.setupBottomColor + delta)
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

// drawMain renders the main menu.
func (m *Manager) drawMain(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/ttl_main.lmp")

	if pic := m.getPic("gfx/mainmenu.lmp"); pic != nil {
		dc.DrawPic(72, 32, pic)
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
		dc.DrawPic(72, 32, pic)
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
		m.drawText(dc, 24, 32+i*8, fmt.Sprintf("s%d", i), true)
	}
	m.drawArrowCursor(dc, 8, 32+m.loadCursor*8)
}

func (m *Manager) drawSave(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_save.lmp")

	for i := 0; i < maxSaveGames; i++ {
		m.drawText(dc, 24, 32+i*8, fmt.Sprintf("s%d", i), true)
	}
	m.drawArrowCursor(dc, 8, 32+m.saveCursor*8)
}

func (m *Manager) drawMultiPlayer(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_multi.lmp")

	if pic := m.getPic("gfx/mp_menu.lmp"); pic != nil {
		dc.DrawPic(72, 32, pic)
	} else {
		m.drawText(dc, 84, 32, "JOIN GAME", true)
		m.drawText(dc, 84, 52, "HOST GAME", true)
		m.drawText(dc, 84, 72, "SETUP", true)
	}

	m.drawCursor(dc, 54, 32+m.multiPlayerCursor*20)
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

func (m *Manager) drawHelp(dc renderer.RenderContext) {
	if pic := m.getPic(fmt.Sprintf("gfx/help%d.lmp", m.helpPage)); pic != nil {
		dc.DrawPic(0, 0, pic)
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
		dc.DrawPic(16, 4, pic)
	}

	if titlePic == "" {
		return
	}

	if pic := m.getPic(titlePic); pic != nil {
		x := (320 - int(pic.Width)) / 2
		dc.DrawPic(x, 4, pic)
	}
}

func (m *Manager) drawCursor(dc renderer.RenderContext, x, y int) {
	frame := (time.Now().UnixNano()/int64(200*time.Millisecond))%6 + 1
	picName := fmt.Sprintf("gfx/menudot%d.lmp", frame)
	if pic := m.getPic(picName); pic != nil {
		dc.DrawPic(x, y, pic)
		return
	}

	if pic := m.getPic("gfx/m_surfs.lmp"); pic != nil {
		dc.DrawPic(x, y, pic)
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
