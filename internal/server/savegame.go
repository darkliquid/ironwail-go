// savegame.go handles serializing and deserializing the complete server state
// for save/load game functionality.
//
// The key challenge is QC (QuakeC) string index portability. The QuakeC VM
// stores strings in an internal table and references them by integer index.
// These indices are only valid within a specific VM instance — they cannot be
// carried across save/load boundaries because a freshly loaded VM will have a
// different string table layout. To solve this, strings must be resolved to
// their actual text content on save (index → text) and re-allocated as fresh
// indices on load (text → new index via vm.AllocString).
//
// The overall flow is:
//
//	Save:  Server state → CaptureSaveGameState() → SaveGameState (portable snapshot)
//	Load:  SaveGameState → RestoreSaveGameState() → rebuilt Server state

// This file belongs to the Savegame subsystem: save/load game serialization and text format helpers.
//
// Portable types (SaveGameState, SaveClientState, SaveEdictState, SaveGlobalState)
// and version constants have been moved to internal/server/savegame. Aliases
// below preserve backward compatibility. The serialization methods remain here
// because they require deep access to Server internals.
package server

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/qc"
	sgsavegame "github.com/darkliquid/ironwail-go/internal/server/savegame"
)

// Type aliases for savegame types moved to the savegame sub-package.
type (
	SaveGameState    = sgsavegame.SaveGameState
	SaveClientState  = sgsavegame.SaveClientState
	SaveEdictState   = sgsavegame.SaveEdictState
	SaveGlobalState  = sgsavegame.SaveGlobalState
	TextSaveGameState = sgsavegame.TextSaveGameState
)

// Save game version constants re-exported from the savegame sub-package.
const (
	SaveGameVersion    = sgsavegame.SaveGameVersion
	SaveGameVersionKEX = sgsavegame.SaveGameVersionKEX
)

// ParseTextSaveGame is re-exported from the savegame sub-package.
var ParseTextSaveGame = sgsavegame.ParseTextSaveGame

// CaptureSaveGameState snapshots the full authoritative server world for savegame serialization.
// CaptureSaveGameState takes a snapshot of the entire server state, producing
// a portable SaveGameState that can be serialized to disk.
//
// Deep copies are made of all slice fields using the Go idiom:
//
//	append([]T(nil), slice...)
//
// This creates a new backing array so the snapshot is decoupled from the live
// server — mutations to the server after this call won't affect the snapshot,
// and vice versa. A nil-typed slice is used as the first argument because
// append on a nil slice allocates exactly len(slice) capacity, which is both
// concise and allocation-efficient.
func (s *Server) CaptureSaveGameState() (*SaveGameState, error) {
	if s == nil {
		return nil, fmt.Errorf("server is nil")
	}
	if !s.Active {
		return nil, fmt.Errorf("server is not active")
	}

	// Build the snapshot. Scalar fields are value-copied; slices get deep
	// copies via the append(nil, ...) idiom described above.
	state := &SaveGameState{
		Version:        SaveGameVersion,
		MapName:        s.Name,
		Time:           s.Time,
		Paused:         s.Paused,
		ModelPrecache:  append([]string(nil), s.ModelPrecache...),
		SoundPrecache:  append([]string(nil), s.SoundPrecache...),
		StaticEntities: append([]EntityState(nil), s.StaticEntities...),
		StaticSounds:   append([]StaticSound(nil), s.StaticSounds...),
		Edicts:         make([]SaveEdictState, s.NumEdicts),
	}
	// LightStyles is a fixed-size array, so copy() is used instead of append.
	copy(state.LightStyles[:], s.LightStyles[:])

	// Capture persistent server state (ServerFlags) and per-client spawn
	// parameters. s.Static survives level changes; it may be nil during
	// unit tests or headless operation.
	if s.Static != nil {
		state.ServerFlags = s.Static.ServerFlags
		state.Clients = make([]SaveClientState, len(s.Static.Clients))
		for i, client := range s.Static.Clients {
			if client == nil {
				continue
			}
			state.Clients[i] = SaveClientState{
				Name:       client.Name,
				Color:      client.Color,
				SpawnParms: client.SpawnParms,
			}
		}
	}
	// Capture every edict, converting QC string indices to portable text.
	for i := 0; i < s.NumEdicts; i++ {
		state.Edicts[i] = captureSaveEdictState(s.EdictNum(i), s.QCVM)
	}

	// Capture QC globals marked with DefSaveGlobal.
	state.Globals = captureSaveGlobals(s.QCVM)

	return state, nil
}

// RestoreSaveGameState rehydrates server, edicts, and QC globals from a captured save snapshot.
// RestoreSaveGameState rebuilds the server from a previously captured
// snapshot. This is the inverse of CaptureSaveGameState.
//
// The restore process has several subtle requirements:
//  1. Precache lists and light styles are restored first because edict
//     linking may reference model indices.
//  2. Each edict's EntVars are bulk-copied, then applySavedEdictStrings
//     re-allocates fresh QC string indices in the current VM instance.
//  3. After all edicts are reconstructed, ClearWorld resets the spatial
//     data structure (area nodes / BSP link lists), and LinkEdict re-inserts
//     each entity into the world BSP so collision detection works.
//  4. Finally, QC globals are restored via applySavedGlobals.
//
// Light style note: empty light style strings are defaulted to "m".
// In Quake's light-style encoding, each character maps to a brightness
// level: 'a' = total darkness, 'm' = normal (default) brightness, 'z' =
// maximum brightness. The string is cycled as an animation pattern — a
// single "m" means constant normal light.
func (s *Server) RestoreSaveGameState(state *SaveGameState) error {
	if s == nil {
		return fmt.Errorf("server is nil")
	}
	if state == nil {
		return fmt.Errorf("savegame state is nil")
	}
	if state.Version != SaveGameVersion {
		return fmt.Errorf("unsupported savegame version %d", state.Version)
	}
	if state.MapName == "" {
		return fmt.Errorf("savegame map is empty")
	}
	if s.Name != "" && state.MapName != s.Name {
		return fmt.Errorf("savegame map %q does not match active map %q", state.MapName, s.Name)
	}
	if len(state.Edicts) == 0 {
		return fmt.Errorf("savegame contains no edicts")
	}

	// Restore basic server metadata and deep-copy all precache/asset lists.
	s.Time = state.Time
	s.Paused = state.Paused
	s.ModelPrecache = append([]string(nil), state.ModelPrecache...)
	s.SoundPrecache = append([]string(nil), state.SoundPrecache...)
	s.StaticEntities = append([]EntityState(nil), state.StaticEntities...)
	s.StaticSounds = append([]StaticSound(nil), state.StaticSounds...)
	copy(s.LightStyles[:], state.LightStyles[:])
	// Default empty light styles to "m" (normal brightness). See the
	// function doc comment for the Quake light-style alphabet.
	for i := range s.LightStyles {
		if s.LightStyles[i] == "" {
			s.LightStyles[i] = "m"
		}
	}
	if s.Static != nil {
		s.Static.ServerFlags = state.ServerFlags
		for i := range s.Static.Clients {
			if i >= len(state.Clients) {
				continue
			}
			client := s.Static.Clients[i]
			if client == nil {
				client = &Client{}
				s.Static.Clients[i] = client
			}
			client.Name = state.Clients[i].Name
			client.Color = state.Clients[i].Color
			client.SpawnParms = state.Clients[i].SpawnParms
			if client.Message == nil {
				client.Message = NewMessageBuffer(MaxDatagram)
			}
			if client.EntityStates == nil {
				client.EntityStates = make(map[int]EntityState)
			} else {
				clear(client.EntityStates)
			}
		}
	}

	// Rebuild the edict table from the snapshot. Each SaveEdictState is
	// converted back into a live Edict with freshly allocated QC strings.
	s.Edicts = make([]*Edict, len(state.Edicts))
	for i, saved := range state.Edicts {
		ent := &Edict{
			Num:            i,
			Free:           saved.Free,
			Alpha:          saved.Alpha,
			Scale:          saved.Scale,
			ForceWater:     saved.ForceWater,
			SendForceWater: saved.SendForceWater,
			SendInterval:   saved.SendInterval,
			OldFrame:       saved.OldFrame,
			OldThinkTime:   saved.OldThinkTime,
			FreeTime:       saved.FreeTime,
		}
		// Scale 0 is invalid; 16 means 1.0× in the Quake byte encoding
		// (the network protocol transmits scale as a byte, where 16 = 100%).
		if ent.Scale == 0 {
			ent.Scale = 16
		}
		// Restore raw QCVM edict data, then re-allocate string indices.
		if s.QCVM != nil && len(saved.RawQCVMData) > 0 {
			data := s.QCVM.EdictData(i)
			if data != nil && len(data) >= len(saved.RawQCVMData) {
				copy(data, saved.RawQCVMData)
			}
			applySavedEdictStringsQCVM(i, saved.Strings, s.QCVM)
		}
		s.Edicts[i] = ent
	}
	s.NumEdicts = len(s.Edicts)
	if s.MaxEdicts < s.NumEdicts {
		s.MaxEdicts = s.NumEdicts
	}
	// Re-associate each client with its player edict. Player edicts start
	// at index 1 (edict 0 is always the world entity), so client[i]
	// corresponds to edict[i+1].
	if s.Static != nil {
		for i, client := range s.Static.Clients {
			if client == nil || i+1 >= len(s.Edicts) {
				continue
			}
			client.Edict = s.Edicts[i+1]
		}
	}

	// Reset the spatial partitioning structure (area nodes / BSP link
	// lists) and re-insert every active entity so that collision
	// detection and trigger-touch logic work correctly.
	s.ClearWorld()
	for i, ent := range s.Edicts {
		if ent == nil {
			continue
		}
		// Edict 0 is the world entity — it must never be marked free.
		if i == 0 {
			ent.Free = false
			continue
		}
		if ent.Free {
			continue
		}
		s.LinkEdict(ent, false)
	}

	// Synchronize engine-side state into the QC VM (edict pointers, etc.),
	// then overwrite QC globals from the snapshot.
	s.syncQCVMState()
	applySavedGlobals(s.QCVM, state.Globals)

	return nil
}

// captureSaveEdictState serializes one edict's transient + QC-visible fields into save representation.
// captureSaveEdictState extracts the saveable state from a single edict.
// If the edict pointer is nil (e.g. a gap in the edict table), a minimal
// state with Free=true and the default Scale of 16 (1.0×) is returned so
// the edict slot is preserved in the save file. For live edicts, all server-
// side fields are copied and QC string fields are resolved to text via
// captureSavedEdictStrings.
func captureSaveEdictState(ent *Edict, vm *qc.VM) SaveEdictState {
	state := SaveEdictState{Scale: 16}
	if ent == nil {
		state.Free = true
		return state
	}
	state.Free = ent.Free
	state.Alpha = ent.Alpha
	state.Scale = ent.Scale
	state.ForceWater = ent.ForceWater
	state.SendForceWater = ent.SendForceWater
	state.SendInterval = ent.SendInterval
	state.OldFrame = ent.OldFrame
	state.OldThinkTime = ent.OldThinkTime
	state.FreeTime = ent.FreeTime
	if vm != nil && vm.EdictSize > 0 {
		data := vm.EdictData(ent.Num)
		if data != nil {
			state.RawQCVMData = append([]byte(nil), data...)
			state.Strings = captureSavedEdictStringsQCVM(ent.Num, vm)
		}
	}
	return state
}

// captureSavedEdictStringsQCVM resolves QC string field indices back to text
// by reading the edict's QCVM data directly. It uses defaultEntFieldOffsets
// and stringEntFieldNames to identify which fields are string-typed, reads
// their int32 values from the VM, and resolves them to text via vm.String.
func captureSavedEdictStringsQCVM(entNum int, vm *qc.VM) map[string]string {
	if vm == nil {
		return nil
	}
	strings := make(map[string]string)
	for fieldName := range stringEntFieldNames {
		normalized := normalizeFieldName(fieldName)
		ofs, ok := defaultEntFieldOffsets[normalized]
		if !ok {
			continue
		}
		idx := vm.EInt(entNum, ofs)
		if idx == 0 {
			continue
		}
		if value := vm.String(idx); value != "" {
			strings[normalized] = value
		}
	}
	if len(strings) == 0 {
		return nil
	}
	return strings
}

// applySavedEdictStringsQCVM re-allocates saved text into the VM string table
// and writes the new indices back into the edict's QCVM data.
func applySavedEdictStringsQCVM(entNum int, strings map[string]string, vm *qc.VM) {
	if vm == nil || len(strings) == 0 {
		return
	}
	for fieldName, value := range strings {
		ofs, ok := defaultEntFieldOffsets[fieldName]
		if !ok {
			continue
		}
		var idx int32
		if value != "" {
			idx = vm.AllocString(value)
		}
		vm.SetEInt(entNum, ofs, idx)
	}
}

// captureSaveGlobals serializes QC globals marked DefSaveGlobal, preserving game-script state.
// captureSaveGlobals serializes all QC global variables that have the
// DefSaveGlobal flag set in their definition. The progs.dat compiler marks
// globals that should be persisted (e.g. killed_monsters, found_secrets)
// with this flag; engine-internal or transient globals are left unmarked.
//
// Each global's value is read from the VM's global memory area using type-
// specific accessors:
//   - EvString  → vm.GString (resolves index to text for portability)
//   - EvVector  → vm.GVector (3-component float vector)
//   - EvFloat   → vm.GFloat
//   - default   → vm.GInt (entity references, function indices, etc.)
//
// The DefSaveGlobal flag is masked off to recover the base type before the
// type switch.
func captureSaveGlobals(vm *qc.VM) []SaveGlobalState {
	if vm == nil {
		return nil
	}
	globals := make([]SaveGlobalState, 0)
	for _, def := range vm.GlobalDefs {
		if def.Type&qc.DefSaveGlobal == 0 {
			continue
		}
		name := vm.String(def.Name)
		if name == "" {
			continue
		}
		baseType := def.Type &^ qc.DefSaveGlobal
		entry := SaveGlobalState{Name: name, Type: baseType}
		ofs := int(def.Ofs)
		switch qc.EType(baseType) {
		case qc.EvString:
			entry.String = vm.GString(ofs)
		case qc.EvVector:
			entry.Vector = vm.GVector(ofs)
		case qc.EvFloat:
			entry.Float = vm.GFloat(ofs)
		default:
			entry.Int = vm.GInt(ofs)
		}
		globals = append(globals, entry)
	}
	return globals
}

// applySavedGlobals restores persisted QC globals after map/world entities are rebuilt.
// applySavedGlobals is the inverse of captureSaveGlobals. It restores QC
// global variables from the save file into the current VM.
//
// Globals are looked up by name (vm.FindGlobal) rather than by offset,
// providing forward compatibility — if a new progs.dat reorders its global
// table, name-based lookup still finds the right slot. If a global no
// longer exists (FindGlobal returns < 0), it is silently skipped.
//
// String globals go through vm.SetGString which internally calls
// vm.AllocString, so QC string portability is handled transparently.
func applySavedGlobals(vm *qc.VM, globals []SaveGlobalState) {
	if vm == nil {
		return
	}
	for _, global := range globals {
		ofs := vm.FindGlobal(global.Name)
		if ofs < 0 {
			continue
		}
		switch qc.EType(global.Type) {
		case qc.EvString:
			if global.String == "" {
				vm.SetGInt(ofs, 0)
			} else {
				vm.SetGString(ofs, global.String)
			}
		case qc.EvVector:
			vm.SetGVector(ofs, global.Vector)
		case qc.EvFloat:
			vm.SetGFloat(ofs, global.Float)
		default:
			vm.SetGInt(ofs, global.Int)
		}
	}
}
