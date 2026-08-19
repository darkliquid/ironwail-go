package menu

import (
	"fmt"

	inet "github.com/darkliquid/ironwail-go/internal/net"
)

// Exported read accessors for the menu state machine (R1.2, gap G.13).
//
// The widget pages in internal/quakeui/menu consume these instead of the
// unexported Manager fields, keeping the legacy state machine the source of
// truth while the presentation moves to gogpu/ui. The action side (M_Key,
// M_Char, ShowState, ToggleMenu, ...) is already public and reused verbatim.

// HostSettings is a snapshot of the Host Game menu configuration values.
type HostSettings struct {
	MaxPlayers int
	GameMode   int
	Teamplay   int
	Skill      int
	FragLimit  int
	TimeLimit  int
	MapName    string
}

// CursorFor returns the current selection cursor for the given menu state.
// States without a cursor (MenuNone, MenuHelp, MenuQuit) return 0.
func (m *Manager) CursorFor(state MenuState) int {
	if m == nil {
		return 0
	}
	switch state {
	case MenuMain:
		return m.mainCursor
	case MenuSinglePlayer:
		return m.singlePlayerCursor
	case MenuLoad:
		return m.loadCursor
	case MenuSave:
		return m.saveCursor
	case MenuMultiPlayer:
		return m.multiPlayerCursor
	case MenuJoinGame:
		return m.joinGameCursor
	case MenuHostGame:
		return m.hostGameCursor
	case MenuOptions:
		return m.optionsCursor
	case MenuControls:
		return m.controlsCursor
	case MenuVideo:
		return m.videoCursor
	case MenuAudio:
		return m.audioCursor
	case MenuSetup:
		return m.setupCursor
	case MenuMods:
		return m.modsCursor
	default:
		return 0
	}
}

// TextBuffer returns the editable text field value for the named field.
// Supported names: "hostname" and "name" (Player Setup), "address" (Join
// Game), and "map" (Host Game). Unknown names return "".
func (m *Manager) TextBuffer(name string) string {
	if m == nil {
		return ""
	}
	switch name {
	case "hostname":
		return m.setupHostname
	case "name":
		return m.setupName
	case "address":
		return m.joinAddress
	case "map":
		return m.hostMapName
	default:
		return ""
	}
}

// HostSettings returns a snapshot of the Host Game menu configuration.
func (m *Manager) HostSettings() HostSettings {
	if m == nil {
		return HostSettings{}
	}
	return HostSettings{
		MaxPlayers: m.hostMaxPlayers,
		GameMode:   m.hostGameMode,
		Teamplay:   m.hostTeamplay,
		Skill:      m.hostSkill,
		FragLimit:  m.hostFragLimit,
		TimeLimit:  m.hostTimeLimit,
		MapName:    m.hostMapName,
	}
}

// Mods returns the cached list of available mod directories.
func (m *Manager) Mods() []ModInfo {
	if m == nil {
		return nil
	}
	return m.modsList
}

// CurrentMod returns the currently active mod directory name ("" or "id1"
// for vanilla).
func (m *Manager) CurrentMod() string {
	if m == nil {
		return ""
	}
	return m.currentMod
}

// SaveSlots returns the current load/save slot display labels (indexed by
// slot number). The labels reflect the save files on disk after the Load or
// Save menu is entered.
func (m *Manager) SaveSlots() []string {
	if m == nil {
		return nil
	}
	labels := make([]string, maxSaveGames)
	copy(labels, m.loadSlotLabels[:])
	return labels
}

// SetupTopColor returns the Player Setup shirt color index.
func (m *Manager) SetupTopColor() int {
	if m == nil {
		return 0
	}
	return m.setupTopColor
}

// SetupBottomColor returns the Player Setup pants color index.
func (m *Manager) SetupBottomColor() int {
	if m == nil {
		return 0
	}
	return m.setupBottomColor
}

// HelpPage returns the currently active help screen page index (0 to helpPages-1).
func (m *Manager) HelpPage() int {
	if m == nil {
		return 0
	}
	return m.helpPage
}

// ConfirmLines returns the active 3-line prompt for the quit/confirm screen.
func (m *Manager) ConfirmLines() [3]string {
	if m == nil {
		return [3]string{}
	}
	return m.confirmLines
}

// IsSearchingServers reports whether a LAN server search is currently active.
func (m *Manager) IsSearchingServers() bool {
	return m != nil && m.serverBrowser != nil && m.serverBrowser.IsSearching()
}

// JoinServerResults returns the server browser results list.
func (m *Manager) JoinServerResults() []inet.HostCacheEntry {
	if m == nil {
		return nil
	}
	if m.serverBrowser != nil {
		m.serverResults = m.serverBrowser.Results()
	}
	return m.serverResults
}

// ControlsRebinding reports whether the Controls menu is waiting for a key press.
func (m *Manager) ControlsRebinding() bool {
	return m != nil && m.controlsRebinding
}

// ControlBindingInfo describes a single rebindable action in the Controls menu.
type ControlBindingInfo struct {
	Label   string
	Binding string
}

// ControlBindings returns the full list of actions and their current key bindings.
func (m *Manager) ControlBindings() []ControlBindingInfo {
	if m == nil {
		return nil
	}
	res := make([]ControlBindingInfo, len(controlBindings))
	for i, cb := range controlBindings {
		res[i] = ControlBindingInfo{
			Label:   cb.label,
			Binding: m.controlBindingLabel(controlsBindingStart + i),
		}
	}
	return res
}

// ControlMouseSpeed returns the sensitivity cvar value.
func (m *Manager) ControlMouseSpeed() float32 {
	if m == nil || m.cvars == nil {
		return 0
	}
	return float32(m.cvars.FloatValue("sensitivity"))
}

// ControlInvertMouse returns true if m_pitch is negative.
func (m *Manager) ControlInvertMouse() bool {
	if m == nil || m.cvars == nil {
		return false
	}
	return m.cvars.FloatValue("m_pitch") < 0
}

// ControlAlwaysRun returns true if cl_alwaysrun is enabled.
func (m *Manager) ControlAlwaysRun() bool {
	if m == nil || m.cvars == nil {
		return false
	}
	return m.cvars.BoolValue("cl_alwaysrun")
}

// ControlFreeLook returns true if freelook is enabled.
func (m *Manager) ControlFreeLook() bool {
	if m == nil || m.cvars == nil {
		return false
	}
	return m.cvars.BoolValue("freelook")
}

// VideoRowInfo describes a single row on the Video settings menu.
type VideoRowInfo struct {
	Label string
	Value string
}

// VideoRows returns all configurable rows on the Video settings menu.
func (m *Manager) VideoRows() []VideoRowInfo {
	if m == nil || m.cvars == nil {
		return nil
	}
	mode := videoResolutions[m.currentResolutionIndex()]
	return []VideoRowInfo{
		{Label: "RESOLUTION", Value: fmt.Sprintf("%dx%d", mode.width, mode.height)},
		{Label: "FULLSCREEN", Value: boolLabel(m.cvars.BoolValue("vid_fullscreen"))},
		{Label: "VSYNC", Value: boolLabel(m.cvars.BoolValue("vid_vsync"))},
		{Label: "MAX FPS", Value: fmt.Sprintf("%d", m.cvars.IntValue("host_maxfps"))},
		{Label: "GAMMA", Value: fmt.Sprintf("%.1f", m.cvars.FloatValue("r_gamma"))},
		{Label: "VIEWMODEL", Value: boolLabel(m.cvars.BoolValue("r_drawviewmodel"))},
		{Label: "WATERWARP", Value: waterwarpLabel(m.cvars.IntValue("r_waterwarp"))},
		{Label: "HUD STYLE", Value: hudStyleLabel(m.cvars.IntValue("hud_style"))},
		{Label: "SHOW FPS", Value: boolLabel(m.cvars.FloatValue("scr_showfps") != 0)},
		{Label: "SHOW SPEED", Value: boolLabel(m.cvars.BoolValue("scr_showspeed"))},
		{Label: "SHOW TIME", Value: boolLabel(m.cvars.BoolValue("scr_clock"))},
		{Label: "BACK", Value: ""},
	}
}

// AudioVolume returns the sound volume percentage (0 to 100).
func (m *Manager) AudioVolume() int {
	if m == nil || m.cvars == nil {
		return 0
	}
	return int(clampFloat(m.cvars.FloatValue("s_volume"), 0, 1)*100 + 0.5)
}
