package menu

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
