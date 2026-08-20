package menu

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/gogpu/ui/widget"
)

// drawMultiPlayer renders the Multiplayer sub-menu with its three items
// (Join Game, Host Game, Setup) using either a graphic sprite or text fallback.
func (r *MenuRoot) drawMultiPlayer(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/p_multi.lmp")

	if pic := r.pic("gfx/mp_menu.lmp"); pic != nil {
		r.drawPic(canvas, 72, 32, pic)
	} else {
		r.drawText(canvas, 84, 32, "JOIN GAME", true)
		r.drawText(canvas, 84, 52, "HOST GAME", true)
		r.drawText(canvas, 84, 72, "SETUP", true)
	}

	r.drawCursor(canvas, 54, 32+r.mgr.CursorFor(menu.MenuMultiPlayer)*20)
}

// drawJoinGame renders the Join Game menu with address input, Search LAN,
// Connect, Back items, and server browser results list.
func (r *MenuRoot) drawJoinGame(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/p_multi.lmp")

	r.drawText(canvas, 56, 48, "ADDRESS", true)
	r.drawText(canvas, 56, 72, "SEARCH LAN", true)
	r.drawText(canvas, 56, 96, "CONNECT", true)
	r.drawText(canvas, 56, 120, "BACK", true)
	addr := r.mgr.TextBuffer("address")
	r.drawText(canvas, 160, 48, addr, true)

	cursor := r.mgr.CursorFor(menu.MenuJoinGame)
	cursorY := 48 + cursor*24
	if cursor >= 4 {
		cursorY = 152 + (cursor-4)*8
	}
	r.drawArrowCursor(canvas, 40, cursorY)
	if cursor == 0 {
		cursorX := 160 + len(addr)*8
		r.drawCharacter(canvas, cursorX, 48, blinkingCursorChar())
	}

	// Server list display
	y := 152
	if r.mgr.IsSearchingServers() {
		r.drawText(canvas, 40, y, "SEARCHING...", true)
	} else {
		results := r.mgr.JoinServerResults()
		if len(results) == 0 {
			r.drawText(canvas, 40, y, "NO SERVERS FOUND", true)
		} else {
			for i, entry := range results {
				if i >= 5 || y+i*8 > 192 {
					break
				}
				line := fmt.Sprintf("%-15s %-8s %d/%d", entry.Name, entry.Map, entry.Players, entry.MaxPlayers)
				r.drawText(canvas, 40, y+i*8, line, true)
			}
		}
	}
}

// drawHostGame renders the Host Game menu with configurable settings.
func (r *MenuRoot) drawHostGame(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/p_multi.lmp")

	hs := r.mgr.HostSettings()
	r.drawText(canvas, 56, 32, "MAX PLAYERS", true)
	r.drawText(canvas, 56, 48, "MODE", true)
	r.drawText(canvas, 56, 64, "TEAMPLAY", true)
	r.drawText(canvas, 56, 80, "SKILL", true)
	r.drawText(canvas, 56, 96, "FRAG LIMIT", true)
	r.drawText(canvas, 56, 112, "TIME LIMIT", true)
	r.drawText(canvas, 56, 128, "MAP", true)
	r.drawText(canvas, 56, 152, "START GAME", true)
	r.drawText(canvas, 56, 176, "BACK", true)

	r.drawText(canvas, 192, 32, fmt.Sprintf("%d", hs.MaxPlayers), true)
	modeLabel := "COOP"
	if hs.GameMode == 1 {
		modeLabel = "DEATHMATCH"
	}
	r.drawText(canvas, 192, 48, modeLabel, true)
	r.drawText(canvas, 192, 64, hostTeamplayLabel(hs.Teamplay), true)
	r.drawText(canvas, 192, 80, fmt.Sprintf("%d", hs.Skill), true)
	fragLabel := "NONE"
	if hs.FragLimit > 0 {
		fragLabel = fmt.Sprintf("%d FRAGS", hs.FragLimit)
	}
	r.drawText(canvas, 192, 96, fragLabel, true)
	timeLabel := "NONE"
	if hs.TimeLimit > 0 {
		timeLabel = fmt.Sprintf("%d MINUTES", hs.TimeLimit)
	}
	r.drawText(canvas, 192, 112, timeLabel, true)
	r.drawText(canvas, 192, 128, hs.MapName, true)
	r.drawText(canvas, 32, 200, "LOOPBACK ALWAYS ACTIVE; LISTEN ALLOWS JOINS", true)

	cursorRows := []int{32, 48, 64, 80, 96, 112, 128, 152, 176}
	cursor := r.mgr.CursorFor(menu.MenuHostGame)
	if cursor >= 0 && cursor < len(cursorRows) {
		r.drawArrowCursor(canvas, 40, cursorRows[cursor])
	}
	if cursor == 6 {
		cursorX := 192 + len(hs.MapName)*8
		r.drawCharacter(canvas, cursorX, 128, blinkingCursorChar())
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

// drawSetup renders the Player Setup menu with name/hostname boxes, color selectors,
// player preview sprite, and Accept button.
func (r *MenuRoot) drawSetup(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/p_multi.lmp")

	r.drawText(canvas, 64, 40, "HOSTNAME", true)
	r.drawText(canvas, 64, 56, "YOUR NAME", true)
	r.drawText(canvas, 64, 80, "SHIRT COLOR", true)
	r.drawText(canvas, 64, 104, "PANTS COLOR", true)
	r.drawText(canvas, 72, 140, "ACCEPT CHANGES", true)

	r.drawMenuTextBox(canvas, 160, 32, 16, 1)
	r.drawMenuTextBox(canvas, 160, 48, 16, 1)
	r.drawMenuTextBox(canvas, 64, 132, 14, 1)

	hostname := r.mgr.TextBuffer("hostname")
	name := r.mgr.TextBuffer("name")
	r.drawText(canvas, 176, 40, hostname, true)
	r.drawText(canvas, 176, 56, name, true)
	r.drawText(canvas, 176, 80, fmt.Sprintf("%d", r.mgr.SetupTopColor()), true)
	r.drawText(canvas, 176, 104, fmt.Sprintf("%d", r.mgr.SetupBottomColor()), true)

	if bigBox := r.pic("gfx/bigbox.lmp"); bigBox != nil {
		r.drawPic(canvas, 160, 64, bigBox)
	}
	if playerImg := r.setupPlayerPic(); playerImg != nil {
		r.drawPic(canvas, 172, 72, playerImg)
	}

	setupCursorTable := []int{40, 56, 80, 104, 140}
	cursor := r.mgr.CursorFor(menu.MenuSetup)
	if cursor >= 0 && cursor < len(setupCursorTable) {
		r.drawArrowCursor(canvas, 56, setupCursorTable[cursor])
	}

	if cursor == 0 {
		cursorX := 176 + len(hostname)*8
		r.drawCharacter(canvas, cursorX, 40, blinkingCursorChar())
	}
	if cursor == 1 {
		cursorX := 176 + len(name)*8
		r.drawCharacter(canvas, cursorX, 56, blinkingCursorChar())
	}
}
