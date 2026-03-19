package menu

import (
	"fmt"
	"strings"
	"time"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/input"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

const joinGameVisibleResults = 6

// loadKey routes keyboard input on the Load Game menu. Selecting a slot issues
// the "load sN" console command where N is the slot index (0–11).
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

// saveKey routes keyboard input on the Save Game menu. Selecting a slot issues
// the "save sN" console command where N is the slot index (0–11).
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

// joinGameKey routes keyboard input on the Join Game menu. The address field
// (item 0) accepts typed characters via joinGameChar; Search LAN (item 1)
// starts a broadcast scan; Connect (item 2) issues "connect <address>".
func (m *Manager) joinGameKey(key int) {
	switch key {
	case input.KUpArrow, input.KMWheelUp:
		m.joinGameCursor--
		if m.joinGameCursor < 0 {
			m.joinGameCursor = m.joinGameItemCount() - 1
		}
		m.playMenuSound(menuSoundNavigate)
	case input.KDownArrow, input.KMWheelDown:
		m.joinGameCursor++
		if m.joinGameCursor >= m.joinGameItemCount() {
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
		default:
			if entry, ok := m.selectedServerResult(); ok {
				m.joinAddress = entry.Address
				m.applyJoinGame()
			}
		}
	case input.KEscape, input.KMouse2:
		m.playMenuSound(menuSoundCancel)
		m.state = MenuMultiPlayer
	}
}

// hostGameKey routes keyboard input on the Host Game menu. Left/Right adjust
// numeric settings (max players, frag limit, etc.); Enter on Start Game
// issues the full sequence of console commands to launch a listen server.
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

// syncHostGameValues reads the current maxplayers, skill, coop/deathmatch,
// teamplay, fraglimit, and timelimit cvars into the Manager's host-game
// editing fields
// so the Host Game menu reflects the engine's active settings.
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
	if cv := cvar.Get("teamplay"); cv != nil {
		m.hostTeamplay = cv.Int
	}
	if m.hostTeamplay < 0 || m.hostTeamplay > 2 {
		m.hostTeamplay = 0
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

// joinGameChar handles typed character input for the Join Game address field.
// Only printable ASCII is accepted, up to joinAddressMax characters.
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

// hostGameChar handles typed character input for the Host Game map name field.
// Only printable ASCII is accepted, up to hostMapMaxLen characters.
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

// deleteJoinAddressRune removes the last rune from the join-game address buffer.
func (m *Manager) deleteJoinAddressRune() {
	m.joinAddress = deleteLastRune(m.joinAddress)
}

// deleteHostMapRune removes the last rune from the host-game map name buffer.
func (m *Manager) deleteHostMapRune() {
	m.hostMapName = deleteLastRune(m.hostMapName)
}

// adjustHostGameSetting modifies the Host Game setting at the current cursor
// by the given delta. Settings include max players, game mode (coop/DM),
// teamplay, skill, frag limit, time limit, and the map name.
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
	case hostGameItemTeamplay:
		m.hostTeamplay = wrapIndex(m.hostTeamplay+delta, 3)
		cvar.SetInt("teamplay", m.hostTeamplay)
	case hostGameItemSkill:
		m.hostSkill += delta
		if m.hostSkill < 0 {
			m.hostSkill = 3
		}
		if m.hostSkill > 3 {
			m.hostSkill = 0
		}
		cvar.SetInt("skill", m.hostSkill)
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
	}
}

// applyJoinGame issues a "connect" console command with the entered address
// and closes the menu. Defaults to "local" if the address field is empty.
func (m *Manager) applyJoinGame() {
	address := strings.TrimSpace(m.joinAddress)
	if address == "" {
		address = "local"
	}
	m.HideMenu()
	m.queueCommand(fmt.Sprintf("connect %q\n", address))
}

func (m *Manager) joinGameItemCount() int {
	return joinGameBaseItems + min(len(m.serverResults), joinGameVisibleResults)
}

func (m *Manager) selectedServerResult() (inet.HostCacheEntry, bool) {
	index := m.joinGameCursor - joinGameBaseItems
	if index < 0 || index >= len(m.serverResults) || index >= joinGameVisibleResults {
		return inet.HostCacheEntry{}, false
	}
	return m.serverResults[index], true
}

// applyHostGame issues the full sequence of console commands to start a listen
// server: disconnect, set maxplayers/deathmatch/coop/teamplay/fraglimit/timelimit/skill,
// then load the selected map.
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
	m.queueCommand(fmt.Sprintf("teamplay %d\n", m.hostTeamplay))
	m.queueCommand(fmt.Sprintf("fraglimit %d\n", m.hostFragLimit))
	m.queueCommand(fmt.Sprintf("timelimit %d\n", m.hostTimeLimit))
	m.queueCommand(fmt.Sprintf("skill %d\n", m.hostSkill))
	m.queueCommand(fmt.Sprintf("map %q\n", mapName))
}

// drawLoad renders the Load Game menu showing all maxSaveGames save slots
// with their display labels and a text-mode arrow cursor.
func (m *Manager) drawLoad(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_load.lmp")

	for i := 0; i < maxSaveGames; i++ {
		m.drawText(dc, 24, 32+i*8, m.loadSlotLabels[i], true)
	}
	m.drawArrowCursor(dc, 8, 32+m.loadCursor*8)
}

// drawSave renders the Save Game menu, identical in layout to Load but
// writing to save slots instead of reading from them.
func (m *Manager) drawSave(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_save.lmp")

	for i := 0; i < maxSaveGames; i++ {
		m.drawText(dc, 24, 32+i*8, m.saveSlotLabels[i], true)
	}
	m.drawArrowCursor(dc, 8, 32+m.saveCursor*8)
}

// drawJoinGame renders the Join Game menu with the address text field,
// Search LAN / Connect / Back items, and the server browser results list.
func (m *Manager) drawJoinGame(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_multi.lmp")

	m.drawText(dc, 56, 48, "ADDRESS", true)
	m.drawText(dc, 56, 72, "SEARCH LAN", true)
	m.drawText(dc, 56, 96, "CONNECT", true)
	m.drawText(dc, 56, 120, "BACK", true)
	m.drawText(dc, 160, 48, m.joinAddress, true)

	cursorY := 48 + m.joinGameCursor*24
	if entry, ok := m.selectedServerResult(); ok {
		_ = entry
		cursorY = yForJoinServerResult(m.joinGameCursor - joinGameBaseItems)
	}
	m.drawArrowCursor(dc, 40, cursorY)
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
		if m.joinGameCursor >= m.joinGameItemCount() {
			m.joinGameCursor = m.joinGameItemCount() - 1
		}
		if len(m.serverResults) == 0 {
			m.drawText(dc, 40, y, "NO SERVERS FOUND", true)
		} else {
			for i, entry := range m.serverResults {
				if i >= joinGameVisibleResults || y+i*8 > 192 {
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
	if m.joinGameCursor >= joinGameBaseItems {
		m.joinGameCursor = 1
	}
	m.serverBrowser.Start()
}

func yForJoinServerResult(index int) int {
	return 152 + index*8
}

// drawHostGame renders the Host Game menu with all configurable settings
// (max players, mode, teamplay, skill, frag/time limits, map name) and action buttons.
func (m *Manager) drawHostGame(dc renderer.RenderContext) {
	m.drawPlaqueAndTitle(dc, "gfx/p_multi.lmp")

	m.drawText(dc, 56, 32, "MAX PLAYERS", true)
	m.drawText(dc, 56, 48, "MODE", true)
	m.drawText(dc, 56, 64, "TEAMPLAY", true)
	m.drawText(dc, 56, 80, "SKILL", true)
	m.drawText(dc, 56, 96, "FRAG LIMIT", true)
	m.drawText(dc, 56, 112, "TIME LIMIT", true)
	m.drawText(dc, 56, 128, "MAP", true)
	m.drawText(dc, 56, 152, "START GAME", true)
	m.drawText(dc, 56, 176, "BACK", true)

	m.drawText(dc, 192, 32, fmt.Sprintf("%d", m.hostMaxPlayers), true)
	modeLabel := "COOP"
	if m.hostGameMode == 1 {
		modeLabel = "DEATHMATCH"
	}
	m.drawText(dc, 192, 48, modeLabel, true)
	m.drawText(dc, 192, 64, hostTeamplayLabel(m.hostTeamplay), true)
	m.drawText(dc, 192, 80, fmt.Sprintf("%d", m.hostSkill), true)
	fragLabel := "NONE"
	if m.hostFragLimit > 0 {
		fragLabel = fmt.Sprintf("%d FRAGS", m.hostFragLimit)
	}
	m.drawText(dc, 192, 96, fragLabel, true)
	timeLabel := "NONE"
	if m.hostTimeLimit > 0 {
		timeLabel = fmt.Sprintf("%d MINUTES", m.hostTimeLimit)
	}
	m.drawText(dc, 192, 112, timeLabel, true)
	m.drawText(dc, 192, 128, m.hostMapName, true)
	m.drawText(dc, 40, 200, "HOSTING USES EXISTING LOCAL LOOPBACK", true)

	cursorRows := []int{32, 48, 64, 80, 96, 112, 128, 152, 176}
	m.drawArrowCursor(dc, 40, cursorRows[m.hostGameCursor])
	if m.hostGameCursor == hostGameItemMap {
		cursorX := 192 + len(m.hostMapName)*8
		cursorChar := 10 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
		dc.DrawMenuCharacter(cursorX, 128, cursorChar)
	}
}

func hostTeamplayLabel(value int) string {
	switch value {
	case 1:
		return "NO FRIENDLY FIRE"
	case 2:
		return "FRIENDLY FIRE"
	default:
		return "OFF"
	}
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

// enterLoadMenu transitions to the Load Game menu and refreshes the save-slot
// labels from disk so the display names reflect the current save files.
func (m *Manager) enterLoadMenu() {
	m.state = MenuLoad
	m.refreshSaveSlotLabels()
}

// enterSaveMenu transitions to the Save Game menu and refreshes the save-slot
// labels from disk so the display names reflect the current save files.
func (m *Manager) enterSaveMenu() {
	m.state = MenuSave
	m.refreshSaveSlotLabels()
}

// resetSaveSlotLabels initialises all load/save slot labels to their default
// names ("s0", "s1", …, "s11") when no provider is available.
func (m *Manager) resetSaveSlotLabels() {
	for i := 0; i < maxSaveGames; i++ {
		label := fmt.Sprintf("s%d", i)
		m.loadSlotLabels[i] = label
		m.saveSlotLabels[i] = label
	}
}

// refreshSaveSlotLabels queries the save-slot provider (if set) and updates
// both loadSlotLabels and saveSlotLabels with the display names of existing
// save files.
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
