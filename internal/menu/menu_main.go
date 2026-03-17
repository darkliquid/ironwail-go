package menu

import (
	"fmt"
	"strings"
	"time"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// mainKey routes keyboard input while the Main menu page is active.
// Up/Down (and mouse wheel) move the cursor; Enter/Space/Mouse1 selects;
// Escape/Mouse2 closes the menu entirely.
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

// singlePlayerKey routes keyboard input on the Single Player menu page.
// Items: 0 = New Game (issues "map start"), 1 = Load, 2 = Save.
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

// multiPlayerKey routes keyboard input on the Multiplayer menu page.
// Items: 0 = Join Game, 1 = Host Game, 2 = Player Setup.
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

// enterSetupMenu synchronises the setup fields with current cvar values and
// transitions to the Player Setup menu page.
func (m *Manager) enterSetupMenu() {
	m.syncSetupValues()
	m.state = MenuSetup
	m.setupCursor = setupItemHostname
}

// syncSetupValues reads the current hostname, player name, and shirt/pants
// color cvars into the Manager's editing buffers so the Setup menu shows
// up-to-date values when opened.
func (m *Manager) syncSetupValues() {
	m.setupHostname = currentSetupHostname()
	m.setupName = currentSetupName()
	m.setupTopColor, m.setupBottomColor = splitSetupColors(currentSetupColor())
}

// currentSetupHostname returns the current hostname cvar value, falling back
// to the default "UNNAMED" if the cvar is missing or empty.
func currentSetupHostname() string {
	if cv := cvar.Get(setupHostnameCVar); cv != nil && cv.String != "" {
		return cv.String
	}
	return setupDefaultHostname
}

// currentSetupName returns the current player name cvar value, falling back
// to "player" if the cvar is missing.
func currentSetupName() string {
	if cv := cvar.Get(setupClientNameCVar); cv != nil {
		return cv.String
	}
	return setupDefaultName
}

// currentSetupColor returns the packed color byte from the _cl_color cvar.
// The upper nibble is shirt color and the lower nibble is pants color.
func currentSetupColor() int {
	if cv := cvar.Get(setupClientColorCVar); cv != nil {
		return cv.Int
	}
	return 0
}

// splitSetupColors unpacks a combined color byte into separate top (shirt) and
// bottom (pants) color indices, each in the range [0, setupColorMax].
func splitSetupColors(color int) (top, bottom int) {
	return wrapSetupColor((color >> 4) & 0x0f), wrapSetupColor(color & 0x0f)
}

// helpKey routes keyboard input on the Help screens. Left/Right (and scroll)
// page through the help images; Escape returns to the Main menu.
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

// setupKey routes keyboard input on the Player Setup menu page. Text fields
// (hostname, name) accept typed characters via setupChar; color selectors
// respond to left/right arrows; Accept applies changes via console commands.
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

// setupChar handles typed character input for the Player Setup menu's text
// fields (hostname and player name). Only printable ASCII is accepted.
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

// deleteSetupNameRune removes the last rune from the player name editing buffer.
func (m *Manager) deleteSetupNameRune() {
	m.setupName = deleteLastRune(m.setupName)
}

// deleteSetupHostnameRune removes the last rune from the hostname editing buffer.
func (m *Manager) deleteSetupHostnameRune() {
	m.setupHostname = deleteLastRune(m.setupHostname)
}

// deleteLastRune returns text with the final rune removed, handling multi-byte
// UTF-8 correctly by converting to a rune slice first.
func deleteLastRune(text string) string {
	if len(text) == 0 {
		return text
	}
	runes := []rune(text)
	return string(runes[:len(runes)-1])
}

// adjustSetupColor changes the shirt or pants color by delta, wrapping around
// the [0, setupColorMax] range.
func (m *Manager) adjustSetupColor(delta int) {
	switch m.setupCursor {
	case setupItemTopColor:
		m.setupTopColor = wrapSetupColor(m.setupTopColor + delta)
	case setupItemBottomColor:
		m.setupBottomColor = wrapSetupColor(m.setupBottomColor + delta)
	}
}

// wrapSetupColor clamps a player color index to the valid range [0, setupColorMax],
// wrapping around when the value exceeds either bound.
func wrapSetupColor(value int) int {
	if value > setupColorMax {
		return 0
	}
	if value < 0 {
		return setupColorMax
	}
	return value
}

// applySetupChanges commits the edited hostname, player name, and colors to
// the engine by issuing "name" and "color" console commands and setting the
// hostname cvar directly.
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

// drawSinglePlayer renders the Single Player sub-menu with its three items
// (New Game, Load, Save) using either a graphic sprite or text fallback.
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

// drawMultiPlayer renders the Multiplayer sub-menu with its three items
// (Join Game, Host Game, Setup) using either a graphic sprite or text fallback.
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

// drawHelp renders one of the six help screens. If the corresponding graphic
// (gfx/helpN.lmp) is available it fills the screen; otherwise a text fallback
// with page number and navigation instructions is shown.
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

// drawSetup renders the Player Setup menu with editable hostname and name
// text fields, shirt/pants color selectors with a color-translated player
// preview sprite, and an Accept button.
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
