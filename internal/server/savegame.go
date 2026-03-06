package server

import (
	"fmt"
	"reflect"

	"github.com/ironwail/ironwail-go/internal/qc"
)

const SaveGameVersion = 1

type SaveGameState struct {
	Version        int
	MapName        string
	Time           float32
	Paused         bool
	ServerFlags    int
	ModelPrecache  []string
	SoundPrecache  []string
	StaticEntities []EntityState
	StaticSounds   []StaticSound
	Clients        []SaveClientState
	Edicts         []SaveEdictState
	Globals        []SaveGlobalState
}

type SaveClientState struct {
	Name       string
	Color      int
	SpawnParms [NumSpawnParms]float32
}

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
	Vars           EntVars
	Strings        map[string]string
}

type SaveGlobalState struct {
	Name   string
	Type   uint16
	Float  float32
	Vector [3]float32
	Int    int32
	String string
}

func (s *Server) CaptureSaveGameState() (*SaveGameState, error) {
	if s == nil {
		return nil, fmt.Errorf("server is nil")
	}
	if !s.Active {
		return nil, fmt.Errorf("server is not active")
	}

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
	for i := 0; i < s.NumEdicts; i++ {
		state.Edicts[i] = captureSaveEdictState(s.EdictNum(i), s.QCVM)
	}
	state.Globals = captureSaveGlobals(s.QCVM)

	return state, nil
}

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

	s.Time = state.Time
	s.Paused = state.Paused
	s.ModelPrecache = append([]string(nil), state.ModelPrecache...)
	s.SoundPrecache = append([]string(nil), state.SoundPrecache...)
	s.StaticEntities = append([]EntityState(nil), state.StaticEntities...)
	s.StaticSounds = append([]StaticSound(nil), state.StaticSounds...)
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

	s.Edicts = make([]*Edict, len(state.Edicts))
	for i, saved := range state.Edicts {
		ent := &Edict{
			Free:           saved.Free,
			Alpha:          saved.Alpha,
			Scale:          saved.Scale,
			ForceWater:     saved.ForceWater,
			SendForceWater: saved.SendForceWater,
			SendInterval:   saved.SendInterval,
			OldFrame:       saved.OldFrame,
			OldThinkTime:   saved.OldThinkTime,
			FreeTime:       saved.FreeTime,
			Vars:           &EntVars{},
		}
		if ent.Scale == 0 {
			ent.Scale = 16
		}
		*ent.Vars = saved.Vars
		applySavedEdictStrings(ent.Vars, saved.Strings, s.QCVM)
		s.Edicts[i] = ent
	}
	s.NumEdicts = len(s.Edicts)
	if s.MaxEdicts < s.NumEdicts {
		s.MaxEdicts = s.NumEdicts
	}
	if s.Static != nil {
		for i, client := range s.Static.Clients {
			if client == nil || i+1 >= len(s.Edicts) {
				continue
			}
			client.Edict = s.Edicts[i+1]
		}
	}

	s.ClearWorld()
	for i, ent := range s.Edicts {
		if ent == nil {
			continue
		}
		if i == 0 {
			ent.Free = false
			continue
		}
		if ent.Free || ent.Vars == nil {
			continue
		}
		s.LinkEdict(ent, false)
	}

	s.syncQCVMState()
	applySavedGlobals(s.QCVM, state.Globals)

	return nil
}

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
	if ent.Vars != nil {
		state.Vars = *ent.Vars
		state.Strings = captureSavedEdictStrings(ent.Vars, vm)
	}
	return state
}

func captureSavedEdictStrings(vars *EntVars, vm *qc.VM) map[string]string {
	if vars == nil || vm == nil {
		return nil
	}
	strings := make(map[string]string)
	rv := reflect.ValueOf(vars).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if _, ok := stringEntFieldNames[normalizeFieldName(field.Name)]; !ok {
			continue
		}
		idx := int32(rv.Field(i).Int())
		if idx == 0 {
			continue
		}
		if value := vm.GetString(idx); value != "" {
			strings[field.Name] = value
		}
	}
	if len(strings) == 0 {
		return nil
	}
	return strings
}

func applySavedEdictStrings(vars *EntVars, strings map[string]string, vm *qc.VM) {
	if vars == nil || vm == nil || len(strings) == 0 {
		return
	}
	rv := reflect.ValueOf(vars).Elem()
	for name, value := range strings {
		field := rv.FieldByName(name)
		if !field.IsValid() || !field.CanSet() || field.Kind() != reflect.Int32 {
			continue
		}
		if value == "" {
			field.SetInt(0)
			continue
		}
		field.SetInt(int64(vm.AllocString(value)))
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
		name := vm.GetString(def.Name)
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
