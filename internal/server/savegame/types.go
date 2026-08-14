// This file contains portable savegame state types used by the server's
// save/load system.
package savegame

import (
	"github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// NumSpawnParms is the number of per-player spawn parameters carried
// across level transitions. Matches the original Quake constant.
const NumSpawnParms = 16

// Save game format version stamps embedded in save files.
//
// SaveGameVersion identifies Ironwail-Go's current JSON/native format.
// SaveGameVersionKEX matches the Quake rerelease text save header version used
// by canonical Ironwail when loading explicit `load <savename> kex` saves.
const (
	SaveGameVersion    = 1
	SaveGameVersionKEX = 6
)

// SaveGameState captures everything needed to fully restore a game session.
// It is a self-contained, portable snapshot of the entire server at one
// instant in time. Fields include:
//   - MapName / Time / Paused — basic session metadata.
//   - ModelPrecache / SoundPrecache — ordered lists of asset paths the engine
//     loaded; indices into these lists are used by the network protocol, so
//     the order must be preserved exactly.
//   - Edicts — the full entity table (monsters, items, triggers, doors, ...).
//   - Globals — QC global variables flagged for save/restore.
//   - Clients — per-player spawn parameters carried across level transitions.
//   - StaticEntities / StaticSounds — decorative world objects (torches, etc.).
//   - LightStyles — 64 animation strings controlling dynamic lights.
type SaveGameState struct {
	Version        int
	MapName        string
	Time           float32
	Paused         bool
	ServerFlags    int
	ModelPrecache  []string
	SoundPrecache  []string
	StaticEntities []types.EntityState
	StaticSounds   []types.StaticSound
	LightStyles    [256]string
	Clients        []SaveClientState
	Edicts         []SaveEdictState
	Globals        []SaveGlobalState
}

// SaveClientState preserves per-player state that must survive level
// transitions and save/load cycles. The most important piece is SpawnParms,
// an array of 16 floats that QuakeC uses to carry player inventory, health,
// and progression state across changelevel boundaries. For example, parm1
// holds items, parm2 holds health, parm3-7 hold ammo counts, etc. The
// server calls SetChangeParms / SetNewParms in QuakeC to populate these
// before a level change, and they are restored on the new level.
type SaveClientState struct {
	Name       string
	Color      int
	SpawnParms [NumSpawnParms]float32
}

// SaveEdictState captures the full state of a single edict (entity).
//
// Most fields are direct copies of the Edict struct — the interesting one is
// Strings. Because QC string indices are VM-instance-specific, we cannot
// simply persist the int32 index values in EntVars. Instead, the Strings map
// stores field-name → resolved-text pairs for every QC string field. On
// load, applySavedEdictStrings re-allocates fresh indices in the new VM.
// This map is the mechanism that makes save files portable across VM
// instances (see captureSavedEdictStrings / applySavedEdictStrings).
type SaveEdictState struct {
	Free           bool
	Alpha          uint8
	Scale          uint8
	ForceWater     bool
	SendForceWater bool
	SendInterval   bool
	OldFrame       float32
	OldThinkTime   float32
	FreeTime       float32
	RawQCVMData    []byte
	Strings        map[string]string
}

// SaveGlobalState captures a single QC global variable that was marked with
// the DefSaveGlobal flag in the progs.dat definition table. Values are stored
// in a type-discriminated union (Float / Vector / Int / String) keyed by the
// variable's QC type tag. The Name field is the human-readable global name
// (e.g. "killed_monsters"), which is used for name-based lookup on restore —
// this provides forward compatibility if the global table order changes
// between progs.dat versions.
type SaveGlobalState struct {
	Name   string
	Type   uint16
	Float  float32
	Vector qtypes.Vec3
	Int    int32
	String string
}

// TextSaveGameState captures the line-oriented KEX/native text save format
// after header parsing. The entity/global body stays in raw text form so a
// live server can reuse the existing ED_ParseGlobals / ED_ParseEdict paths.
type TextSaveGameState struct {
	Version     int
	GameDir     string
	Title       string
	Skill       int
	MapName     string
	Time        float32
	SpawnParms  [NumSpawnParms]float32
	LightStyles [64]string
	EntityText  string
}
