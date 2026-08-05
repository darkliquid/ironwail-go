// savegame.go handles serializing and deserializing the complete server state
// for save/load game functionality.
package savegame

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/qc"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// Server defines the contract required by savegame capture and restore operations.
type Server interface {
	IsActive() bool
	GetName() string
	GetTime() float32
	SetTime(t float32)
	IsPaused() bool
	SetPaused(p bool)
	SetLoadGame(b bool)
	GetModelPrecache() []string
	SetModelPrecache(models []string)
	GetSoundPrecache() []string
	SetSoundPrecache(sounds []string)
	GetStaticEntities() []srvtypes.EntityState
	SetStaticEntities(ents []srvtypes.EntityState)
	GetStaticSounds() []srvtypes.StaticSound
	SetStaticSounds(sounds []srvtypes.StaticSound)
	GetLightStyles() *[256]string
	GetServerFlags() int
	SetServerFlags(flags int)
	GetClients() []SaveClientState
	RestoreClientState(i int, state SaveClientState)
	GetNumEdicts() int
	GetMaxEdicts() int
	MaxClients() int
	GetEdicts() []*srvtypes.Edict
	SetEdicts(edicts []*srvtypes.Edict)
	SetEdict(n int, ent *srvtypes.Edict)
	RebindClientEdicts()
	EdictNum(n int) *srvtypes.Edict
	EnsureTextSaveEdictCapacity(n int) error
	SetNumEdicts(n int)
	GetVM() *qc.VM
	ClearWorld()
	LinkEdict(ent *srvtypes.Edict, touchTriggers bool)
	SyncQCVMState()
	SetQCTimeGlobal(t float32)
	ValidateTextSaveGameDir(gameDir string) error
	ClearQCVMEdictData(entnum int)
	EdictClearQCVMFunc(vm *qc.VM, entnum int)
	EdictDefaultOffsets() map[string]int
}

// stringEntFieldNames lists entity fields whose int32 values are indices
// into the QuakeC VM string table rather than raw numeric values.
var stringEntFieldNames = map[string]struct{}{
	"classname":   {},
	"map":         {},
	"message":     {},
	"model":       {},
	"netname":     {},
	"noise":       {},
	"noise1":      {},
	"noise2":      {},
	"noise3":      {},
	"target":      {},
	"targetname":  {},
	"weaponmodel": {},
}

func normalizeFieldName(s string) string {
	s = stringsToLower(s)
	res := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '_' {
			res = append(res, s[i])
		}
	}
	return string(res)
}

func stringsToLower(s string) string {
	b := []byte(s)
	for i := 0; i < len(b); i++ {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

// CaptureSaveGameState snapshots the full authoritative server world for savegame serialization.
func CaptureSaveGameState(s Server) (*SaveGameState, error) {
	if s == nil {
		return nil, fmt.Errorf("server is nil")
	}
	if !s.IsActive() {
		return nil, fmt.Errorf("server is not active")
	}

	lightStyles := s.GetLightStyles()
	state := &SaveGameState{
		Version:        SaveGameVersion,
		MapName:        s.GetName(),
		Time:           s.GetTime(),
		Paused:         s.IsPaused(),
		ModelPrecache:  append([]string(nil), s.GetModelPrecache()...),
		SoundPrecache:  append([]string(nil), s.GetSoundPrecache()...),
		StaticEntities: append([]srvtypes.EntityState(nil), s.GetStaticEntities()...),
		StaticSounds:   append([]srvtypes.StaticSound(nil), s.GetStaticSounds()...),
		Edicts:         make([]SaveEdictState, s.GetNumEdicts()),
	}
	if lightStyles != nil {
		copy(state.LightStyles[:], lightStyles[:])
	}

	state.ServerFlags = s.GetServerFlags()
	state.Clients = s.GetClients()

	vm := s.GetVM()
	numEdicts := s.GetNumEdicts()
	for i := 0; i < numEdicts; i++ {
		state.Edicts[i] = captureSaveEdictState(s.EdictNum(i), vm, s.EdictDefaultOffsets())
	}

	state.Globals = captureSaveGlobals(vm)

	return state, nil
}

// RestoreSaveGameState rehydrates server, edicts, and QC globals from a captured save snapshot.
func RestoreSaveGameState(s Server, state *SaveGameState) error {
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
	if s.GetName() != "" && state.MapName != s.GetName() {
		return fmt.Errorf("savegame map %q does not match active map %q", state.MapName, s.GetName())
	}
	if len(state.Edicts) == 0 {
		return fmt.Errorf("savegame contains no edicts")
	}

	s.SetTime(state.Time)
	s.SetPaused(state.Paused)
	s.SetModelPrecache(append([]string(nil), state.ModelPrecache...))
	s.SetSoundPrecache(append([]string(nil), state.SoundPrecache...))
	s.SetStaticEntities(append([]srvtypes.EntityState(nil), state.StaticEntities...))
	s.SetStaticSounds(append([]srvtypes.StaticSound(nil), state.StaticSounds...))

	lightStyles := s.GetLightStyles()
	if lightStyles != nil {
		copy(lightStyles[:], state.LightStyles[:])
		for i := range lightStyles {
			if lightStyles[i] == "" {
				lightStyles[i] = "m"
			}
		}
	}

	s.SetServerFlags(state.ServerFlags)
	for i, clientState := range state.Clients {
		s.RestoreClientState(i, clientState)
	}

	vm := s.GetVM()
	edicts := make([]*srvtypes.Edict, len(state.Edicts))
	defaultOffsets := s.EdictDefaultOffsets()
	for i, saved := range state.Edicts {
		ent := &srvtypes.Edict{
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
		if ent.Scale == 0 {
			ent.Scale = 16
		}
		if vm != nil && len(saved.RawQCVMData) > 0 {
			data := vm.EdictData(i)
			if data != nil && len(data) >= len(saved.RawQCVMData) {
				copy(data, saved.RawQCVMData)
			}
			applySavedEdictStringsQCVM(i, saved.Strings, vm, defaultOffsets)
		}
		edicts[i] = ent
	}

	s.SetEdicts(edicts)
	s.RebindClientEdicts()

	s.ClearWorld()
	for i, ent := range edicts {
		if ent == nil {
			continue
		}
		if i == 0 {
			ent.Free = false
			continue
		}
		if ent.Free {
			continue
		}
		s.LinkEdict(ent, false)
	}

	s.SyncQCVMState()
	applySavedGlobals(vm, state.Globals)

	return nil
}

func captureSaveEdictState(ent *srvtypes.Edict, vm *qc.VM, defaultOffsets map[string]int) SaveEdictState {
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
			state.Strings = captureSavedEdictStringsQCVM(ent.Num, vm, defaultOffsets)
		}
	}
	return state
}

func captureSavedEdictStringsQCVM(entNum int, vm *qc.VM, defaultOffsets map[string]int) map[string]string {
	if vm == nil {
		return nil
	}
	res := make(map[string]string)
	for fieldName := range stringEntFieldNames {
		normalized := normalizeFieldName(fieldName)
		ofs, ok := defaultOffsets[normalized]
		if !ok {
			continue
		}
		idx := vm.EInt(entNum, ofs)
		if idx == 0 {
			continue
		}
		if value := vm.String(idx); value != "" {
			res[normalized] = value
		}
	}
	if len(res) == 0 {
		return nil
	}
	return res
}

func applySavedEdictStringsQCVM(entNum int, strings map[string]string, vm *qc.VM, defaultOffsets map[string]int) {
	if vm == nil || len(strings) == 0 {
		return
	}
	for fieldName, value := range strings {
		ofs, ok := defaultOffsets[fieldName]
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
