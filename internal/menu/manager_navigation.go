// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package menu

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
	if m.state == MenuControls && m.controlsRebinding {
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
