package menu

import (
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
	MenuMultiPlayer                   // Multiplayer submenu
	MenuOptions                       // Options submenu
	MenuVideo                         // Video options submenu
	MenuQuit                          // Quit confirmation screen
)

// Manager handles the Quake menu system including navigation and rendering.
type Manager struct {
	// state is the current active menu state.
	state MenuState

	// mainCursor is the current selection in the main menu.
	mainCursor int

	// drawManager provides access to graphics assets.
	drawManager DrawManager

	// inputSystem provides access to input state.
	inputSystem *input.System

	// active indicates whether the menu is currently displayed.
	active bool
}

// DrawManager defines the interface for loading menu graphics.
type DrawManager interface {
	GetPic(name string) *image.QPic
}

// NewManager creates a new menu manager.
func NewManager(drawMgr DrawManager, inputSys *input.System) *Manager {
	return &Manager{
		state:       MenuNone,
		mainCursor:  0,
		drawManager: drawMgr,
		inputSystem: inputSys,
		active:      false,
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
	case MenuQuit:
		m.quitKey(key)
	}
}

// M_Draw renders the current menu state.
func (m *Manager) M_Draw(dc renderer.RenderContext) {
	switch m.state {
	case MenuMain:
		m.drawMain(dc)
	case MenuQuit:
		m.drawQuit(dc)
	}
}

// mainKey handles input when in the main menu.
func (m *Manager) mainKey(key int) {
	numItems := 6 // SinglePlayer, Multiplayer, Options, Quit, etc.

	switch key {
	case input.KUpArrow:
		m.mainCursor--
		if m.mainCursor < 0 {
			m.mainCursor = numItems - 1
		}
	case input.KDownArrow:
		m.mainCursor++
		if m.mainCursor >= numItems {
			m.mainCursor = 0
		}
	case input.KEnter:
		m.mainSelect()
	case input.KEscape:
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
	case 3: // Video
		m.state = MenuVideo
	case 5: // Quit
		m.state = MenuQuit
	}
}

// quitKey handles input when in the quit confirmation screen.
func (m *Manager) quitKey(key int) {
	switch key {
	case input.KEnter:
		// Confirm quit - hide menu and let host handle the quit
		m.HideMenu()
	case input.KEscape, input.KBackspace:
		// Cancel - return to main menu
		m.state = MenuMain
	}
}

// drawMain renders the main menu.
func (m *Manager) drawMain(dc renderer.RenderContext) {
	// Clear to black
	dc.Clear(0, 0, 0, 1)

	// Draw title plaque
	if pic := m.drawManager.GetPic("gfx/qplaque.lmp"); pic != nil {
		dc.DrawPic(0, 0, pic)
	}

	// Draw main menu graphic
	if pic := m.drawManager.GetPic("gfx/mainmenu.lmp"); pic != nil {
		dc.DrawPic(0, 40, pic)
	}

	// Draw cursor (selection bar)
	if cursorPic := m.drawManager.GetPic("gfx/m_surfs.lmp"); cursorPic != nil {
		// Position cursor based on selection
		yPos := 100 + m.mainCursor*20
		dc.DrawPic(40, yPos, cursorPic)
	}
}

// drawQuit renders the quit confirmation screen.
func (m *Manager) drawQuit(dc renderer.RenderContext) {
	// Clear to black
	dc.Clear(0, 0, 0, 1)

	// Draw quit message (simplified for now)
	// In a full implementation, this would use proper graphics
	dc.DrawFill(0, 0, 320, 20, 15) // Gray background for title

	// Draw "Quit Game?" text
	// This is simplified - in the full implementation we'd use proper font rendering
}
