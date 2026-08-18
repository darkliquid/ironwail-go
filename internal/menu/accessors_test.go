package menu

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/input"
)

// newAccessorTestManager builds a Manager with the standard test fixtures.
func newAccessorTestManager(t *testing.T) *Manager {
	t.Helper()
	drawMgr := &mockDrawManager{}
	backend := &mockInputBackend{}
	inputSys := input.NewSystem(backend)
	mgr := NewManager(drawMgr, inputSys, nil)
	return mgr
}

// TestAccessorStateAndActive asserts State() and IsActive() reflect the menu
// state machine (R1.2: read accessors over unexported fields).
func TestAccessorStateAndActive(t *testing.T) {
	mgr := newAccessorTestManager(t)

	if mgr.State() != MenuNone {
		t.Fatalf("State() = %v, want MenuNone", mgr.State())
	}
	if mgr.IsActive() {
		t.Fatal("IsActive() = true before ShowMenu")
	}

	mgr.ShowMenu()
	if !mgr.IsActive() {
		t.Fatal("IsActive() = false after ShowMenu")
	}
	if mgr.State() != MenuMain {
		t.Fatalf("State() = %v, want MenuMain", mgr.State())
	}
}

// TestAccessorCursorFor asserts CursorFor returns the per-state cursor.
func TestAccessorCursorFor(t *testing.T) {
	mgr := newAccessorTestManager(t)

	mgr.ShowMenu()
	mgr.M_Key(input.KDownArrow)
	if got := mgr.CursorFor(MenuMain); got != 1 {
		t.Fatalf("CursorFor(MenuMain) = %d, want 1", got)
	}

	mgr.ShowState(MenuOptions)
	mgr.M_Key(input.KDownArrow)
	mgr.M_Key(input.KDownArrow)
	if got := mgr.CursorFor(MenuOptions); got != 2 {
		t.Fatalf("CursorFor(MenuOptions) = %d, want 2", got)
	}
}

// TestAccessorTextBuffer asserts TextBuffer returns the per-page text buffers
// (setup hostname/name, join address, host map name).
func TestAccessorTextBuffer(t *testing.T) {
	mgr := newAccessorTestManager(t)

	mgr.ShowState(MenuSetup)
	if got := mgr.TextBuffer("hostname"); got != setupDefaultHostname {
		t.Fatalf("TextBuffer(hostname) = %q, want %q", got, setupDefaultHostname)
	}
	if got := mgr.TextBuffer("name"); got != setupDefaultName {
		t.Fatalf("TextBuffer(name) = %q, want %q", got, setupDefaultName)
	}

	mgr.ShowState(MenuJoinGame)
	if got := mgr.TextBuffer("address"); got != "local" {
		t.Fatalf("TextBuffer(address) = %q, want local", got)
	}

	mgr.ShowState(MenuHostGame)
	if got := mgr.TextBuffer("map"); got != "start" {
		t.Fatalf("TextBuffer(map) = %q, want start", got)
	}
}

// TestAccessorHostSettings asserts HostSettings exposes the Host Game menu
// configuration values.
func TestAccessorHostSettings(t *testing.T) {
	mgr := newAccessorTestManager(t)

	mgr.ShowState(MenuHostGame)
	hs := mgr.HostSettings()
	if hs.MaxPlayers != hostMaxPlayersMax {
		t.Fatalf("HostSettings.MaxPlayers = %d, want %d", hs.MaxPlayers, hostMaxPlayersMax)
	}
	if hs.GameMode != 1 {
		t.Fatalf("HostSettings.GameMode = %d, want 1", hs.GameMode)
	}
	if hs.Skill != 1 {
		t.Fatalf("HostSettings.Skill = %d, want 1", hs.Skill)
	}
}

// TestAccessorMods asserts Mods exposes the mod list and current mod.
func TestAccessorMods(t *testing.T) {
	mgr := newAccessorTestManager(t)

	mgr.SetModsProvider(func() []ModInfo {
		return []ModInfo{{Name: "hipnotic"}, {Name: "rogue"}}
	})
	mgr.ShowState(MenuMods)

	mods := mgr.Mods()
	if len(mods) != 2 {
		t.Fatalf("Mods() length = %d, want 2", len(mods))
	}
	if mods[0].Name != "hipnotic" || mods[1].Name != "rogue" {
		t.Fatalf("Mods() = %+v, want [hipnotic rogue]", mods)
	}
	if got := mgr.CurrentMod(); got != "" {
		t.Fatalf("CurrentMod() = %q, want empty", got)
	}

	mgr.SetCurrentMod("rogue")
	if got := mgr.CurrentMod(); got != "rogue" {
		t.Fatalf("CurrentMod() = %q, want rogue", got)
	}
}

// TestAccessorSaveSlots asserts SaveSlots exposes the load/save slot labels.
func TestAccessorSaveSlots(t *testing.T) {
	mgr := newAccessorTestManager(t)

	mgr.SetSaveSlotProvider(func(slotCount int) []SaveSlotInfo {
		infos := make([]SaveSlotInfo, slotCount)
		for i := range infos {
			infos[i] = SaveSlotInfo{Name: "s" + string(rune('0'+i)), DisplayName: "slot" + string(rune('0'+i))}
		}
		return infos
	})
	mgr.ShowState(MenuLoad)

	slots := mgr.SaveSlots()
	if len(slots) != maxSaveGames {
		t.Fatalf("SaveSlots() length = %d, want %d", len(slots), maxSaveGames)
	}
	if slots[0] != "slot0" {
		t.Fatalf("SaveSlots()[0] = %q, want slot0", slots[0])
	}
}

// TestAccessorSetupColors asserts the setup color accessors.
func TestAccessorSetupColors(t *testing.T) {
	mgr := newAccessorTestManager(t)

	mgr.ShowState(MenuSetup)
	if mgr.SetupTopColor() != 0 || mgr.SetupBottomColor() != 0 {
		t.Fatalf("Setup colors = (%d,%d), want (0,0)", mgr.SetupTopColor(), mgr.SetupBottomColor())
	}
}
