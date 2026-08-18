package menu

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/menu"
)

// rowsForState builds the row model for the given menu state, mirroring the
// legacy M_Draw layout constants (research 0001 §3). Each row's Label is the
// left-aligned text and Value the right-aligned setting, matching the legacy
// draw positions.
func rowsForState(mgr *menu.Manager, cvs *cvar.CVarSystem, state menu.MenuState) []MenuRow {
	switch state {
	case menu.MenuMain:
		return mainRows(mgr)
	case menu.MenuSinglePlayer:
		return []MenuRow{{Label: "NEW GAME"}, {Label: "LOAD"}, {Label: "SAVE"}}
	case menu.MenuLoad, menu.MenuSave:
		return saveRows(mgr)
	case menu.MenuMultiPlayer:
		return []MenuRow{{Label: "JOIN GAME"}, {Label: "HOST GAME"}, {Label: "SETUP"}}
	case menu.MenuJoinGame:
		return joinRows(mgr)
	case menu.MenuHostGame:
		return hostRows(mgr)
	case menu.MenuOptions:
		return []MenuRow{{Label: "CONTROLS"}, {Label: "VIDEO"}, {Label: "AUDIO"}, {Label: "VSYNC"}, {Label: "BACK"}}
	case menu.MenuControls:
		return controlsRows(mgr, cvs)
	case menu.MenuVideo:
		return videoRows(mgr, cvs)
	case menu.MenuAudio:
		return audioRows(cvs)
	case menu.MenuHelp:
		return helpRows(mgr)
	case menu.MenuQuit:
		return quitRows(mgr)
	case menu.MenuSetup:
		return setupRows(mgr)
	case menu.MenuMods:
		return modsRows(mgr)
	default:
		return nil
	}
}

// mainRows builds the main menu rows. When mods are available, a MODS row is
// inserted between OPTIONS and HELP (legacy split behavior).
func mainRows(mgr *menu.Manager) []MenuRow {
	rows := []MenuRow{
		{Label: "SINGLE PLAYER"},
		{Label: "MULTIPLAYER"},
		{Label: "OPTIONS"},
	}
	if len(mgr.Mods()) > 0 {
		rows = append(rows, MenuRow{Label: "MODS"})
	}
	rows = append(rows,
		MenuRow{Label: "HELP"},
		MenuRow{Label: "QUIT"},
	)
	return rows
}

// saveRows builds the Load/Save slot rows from the manager's SaveSlots.
func saveRows(mgr *menu.Manager) []MenuRow {
	slots := mgr.SaveSlots()
	rows := make([]MenuRow, 0, len(slots))
	for _, label := range slots {
		rows = append(rows, MenuRow{Label: label})
	}
	return rows
}

// joinRows builds the Join Game rows (address, search, connect, back).
func joinRows(mgr *menu.Manager) []MenuRow {
	return []MenuRow{
		{Label: "ADDRESS", Value: mgr.TextBuffer("address")},
		{Label: "SEARCH LAN"},
		{Label: "CONNECT"},
		{Label: "BACK"},
	}
}

// hostRows builds the Host Game rows from HostSettings.
func hostRows(mgr *menu.Manager) []MenuRow {
	hs := mgr.HostSettings()
	mode := "COOP"
	if hs.GameMode == 1 {
		mode = "DEATHMATCH"
	}
	return []MenuRow{
		{Label: "MAX PLAYERS", Value: fmt.Sprintf("%d", hs.MaxPlayers)},
		{Label: "MODE", Value: mode},
		{Label: "TEAMPLAY", Value: teamplayLabel(hs.Teamplay)},
		{Label: "SKILL", Value: fmt.Sprintf("%d", hs.Skill)},
		{Label: "FRAG LIMIT", Value: fmt.Sprintf("%d", hs.FragLimit)},
		{Label: "TIME LIMIT", Value: fmt.Sprintf("%d", hs.TimeLimit)},
		{Label: "MAP", Value: hs.MapName},
		{Label: "START GAME"},
		{Label: "BACK"},
	}
}

// teamplayLabel mirrors the legacy hostTeamplayLabel.
func teamplayLabel(v int) string {
	switch v {
	case 1:
		return "NO TEAMPLAY"
	case 2:
		return "TEAMPLAY"
	default:
		return "OFF"
	}
}

// controlsRows builds the Controls rows (sliders/toggles + key bindings).
func controlsRows(mgr *menu.Manager, cvs *cvar.CVarSystem) []MenuRow {
	rows := []MenuRow{
		{Label: "MOUSE SPEED", Value: fmt.Sprintf("%.1f", floatVal(cvs, "sensitivity"))},
		{Label: "INVERT MOUSE", Value: boolLabel(floatVal(cvs, "m_pitch") < 0)},
		{Label: "ALWAYS RUN", Value: boolLabel(boolVal(cvs, "cl_alwaysrun"))},
		{Label: "MOUSE LOOK", Value: boolLabel(boolVal(cvs, "freelook"))},
	}
	rows = append(rows, MenuRow{Label: "BACK"})
	return rows
}

// videoRows builds the Video rows from cvars.
func videoRows(mgr *menu.Manager, cvs *cvar.CVarSystem) []MenuRow {
	return []MenuRow{
		{Label: "RESOLUTION", Value: resolutionLabel(cvs)},
		{Label: "FULLSCREEN", Value: boolLabel(boolVal(cvs, "vid_fullscreen"))},
		{Label: "VSYNC", Value: boolLabel(boolVal(cvs, "vid_vsync"))},
		{Label: "MAX FPS", Value: fmt.Sprintf("%d", intVal(cvs, "host_maxfps"))},
		{Label: "GAMMA", Value: fmt.Sprintf("%.1f", floatVal(cvs, "r_gamma"))},
		{Label: "VIEWMODEL", Value: boolLabel(boolVal(cvs, "r_drawviewmodel"))},
		{Label: "WATERWARP", Value: waterwarpLabel(intVal(cvs, "r_waterwarp"))},
		{Label: "HUD STYLE", Value: hudStyleLabel(intVal(cvs, "hud_style"))},
		{Label: "SHOW FPS", Value: boolLabel(floatVal(cvs, "scr_showfps") != 0)},
		{Label: "SHOW SPEED", Value: boolLabel(boolVal(cvs, "scr_showspeed"))},
		{Label: "SHOW TIME", Value: boolLabel(boolVal(cvs, "scr_clock"))},
		{Label: "BACK"},
	}
}

// audioRows builds the Audio rows.
func audioRows(cvs *cvar.CVarSystem) []MenuRow {
	volume := int(clampFloat(floatVal(cvs, "s_volume"), 0, 1)*100 + 0.5)
	return []MenuRow{
		{Label: "SOUND VOLUME", Value: fmt.Sprintf("%d%%", volume)},
		{Label: "BACK"},
	}
}

// helpRows builds the Help page rows.
func helpRows(mgr *menu.Manager) []MenuRow {
	return []MenuRow{
		{Label: "HELP PAGE"},
		{Label: "LEFT/RIGHT OR MOUSE1 TO CHANGE"},
		{Label: "ESC TO RETURN"},
	}
}

// quitRows builds the Quit confirmation rows.
func quitRows(mgr *menu.Manager) []MenuRow {
	return []MenuRow{
		{Label: "ARE YOU SURE YOU WANT TO QUIT?"},
		{Label: "PRESS Y OR ENTER TO QUIT"},
		{Label: "PRESS N OR ESC TO CANCEL"},
	}
}

// setupRows builds the Player Setup rows (hostname, name, colors).
func setupRows(mgr *menu.Manager) []MenuRow {
	return []MenuRow{
		{Label: "HOSTNAME", Value: mgr.TextBuffer("hostname")},
		{Label: "YOUR NAME", Value: mgr.TextBuffer("name")},
		{Label: "SHIRT COLOR", Value: fmt.Sprintf("%d", mgr.SetupTopColor())},
		{Label: "PANTS COLOR", Value: fmt.Sprintf("%d", mgr.SetupBottomColor())},
		{Label: "ACCEPT CHANGES"},
	}
}

// modsRows builds the Mods browser rows from the manager's Mods list.
func modsRows(mgr *menu.Manager) []MenuRow {
	rows := make([]MenuRow, 0, len(mgr.Mods())+1)
	current := mgr.CurrentMod()
	for _, mod := range mgr.Mods() {
		label := mod.Name
		if equalFold(mod.Name, current) {
			label += " *"
		}
		rows = append(rows, MenuRow{Label: label})
	}
	rows = append(rows, MenuRow{Label: "BACK"})
	return rows
}

// --- small helpers (nil-safe cvar reads and labels) ---

func boolVal(cvs *cvar.CVarSystem, name string) bool {
	if cvs == nil {
		return false
	}
	return cvs.BoolValue(name)
}

func intVal(cvs *cvar.CVarSystem, name string) int {
	if cvs == nil {
		return 0
	}
	return cvs.IntValue(name)
}

func floatVal(cvs *cvar.CVarSystem, name string) float64 {
	if cvs == nil {
		return 0
	}
	return cvs.FloatValue(name)
}

func boolLabel(v bool) string {
	if v {
		return "ON"
	}
	return "OFF"
}

func clampFloat(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
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
	switch v {
	case 1:
		return "MODERN 1"
	case 2:
		return "MODERN 2"
	case 3:
		return "QUAKEWORLD"
	default:
		return "CLASSIC"
	}
}

func resolutionLabel(cvs *cvar.CVarSystem) string {
	w := intVal(cvs, "vid_width")
	h := intVal(cvs, "vid_height")
	if w <= 0 || h <= 0 {
		return "640x480"
	}
	return fmt.Sprintf("%dx%d", w, h)
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 'a' - 'A'
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 'a' - 'A'
		}
		if ca != cb {
			return false
		}
	}
	return true
}
