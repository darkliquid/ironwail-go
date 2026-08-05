// This file belongs to the Savegame subsystem: save/load game serialization and text format helpers.
package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qc"
	sgsavegame "github.com/darkliquid/ironwail-go/internal/server/savegame"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// Type aliases for savegame types moved to the savegame sub-package.
type (
	SaveGameState     = sgsavegame.SaveGameState
	SaveClientState   = sgsavegame.SaveClientState
	SaveEdictState    = sgsavegame.SaveEdictState
	SaveGlobalState   = sgsavegame.SaveGlobalState
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
func (s *Server) CaptureSaveGameState() (*SaveGameState, error) {
	return sgsavegame.CaptureSaveGameState(s)
}

// RestoreSaveGameState rehydrates server, edicts, and QC globals from a captured save snapshot.
func (s *Server) RestoreSaveGameState(state *SaveGameState) error {
	return sgsavegame.RestoreSaveGameState(s, state)
}

// RestoreTextSaveGameState applies a parsed text save to a live spawned server.
func (s *Server) RestoreTextSaveGameState(state *TextSaveGameState) error {
	return sgsavegame.RestoreTextSaveGameState(s, state)
}

// Implementations of savegame.Server interface on *Server

func (s *Server) GetName() string                  { return s.Name }
func (s *Server) SetName(name string)             { s.Name = name }
func (s *Server) SetTime(t float32)               { s.Time = t }
func (s *Server) SetPaused(p bool)                { s.Paused = p }
func (s *Server) GetModelPrecache() []string      { return s.ModelPrecache }
func (s *Server) SetModelPrecache(models []string) { s.ModelPrecache = models }
func (s *Server) GetSoundPrecache() []string      { return s.SoundPrecache }
func (s *Server) SetSoundPrecache(sounds []string) { s.SoundPrecache = sounds }
func (s *Server) GetStaticEntities() []srvtypes.EntityState {
	return s.StaticEntities
}
func (s *Server) SetStaticEntities(ents []srvtypes.EntityState) {
	s.StaticEntities = ents
}
func (s *Server) GetStaticSounds() []srvtypes.StaticSound {
	return s.StaticSounds
}
func (s *Server) SetStaticSounds(sounds []srvtypes.StaticSound) {
	s.StaticSounds = sounds
}
func (s *Server) GetLightStyles() *[256]string {
	return &s.LightStyles
}

func (s *Server) GetServerFlags() int {
	if s.Static != nil {
		return s.Static.ServerFlags
	}
	return 0
}

func (s *Server) SetServerFlags(flags int) {
	if s.Static != nil {
		s.Static.ServerFlags = flags
	}
}

func (s *Server) GetClients() []SaveClientState {
	if s.Static == nil {
		return nil
	}
	clients := make([]SaveClientState, len(s.Static.Clients))
	for i, client := range s.Static.Clients {
		if client == nil {
			continue
		}
		clients[i] = SaveClientState{
			Name:       client.Name,
			Color:      client.Color,
			SpawnParms: client.SpawnParms,
		}
	}
	return clients
}

func (s *Server) RestoreClientState(i int, state SaveClientState) {
	if s.Static == nil {
		return
	}
	if i >= len(s.Static.Clients) {
		return
	}
	client := s.Static.Clients[i]
	if client == nil {
		client = &Client{}
		s.Static.Clients[i] = client
	}
	client.Name = state.Name
	client.Color = state.Color
	client.SpawnParms = state.SpawnParms
	if client.Message == nil {
		client.Message = NewMessageBuffer(MaxDatagram)
	}
	if client.EntityStates == nil {
		client.EntityStates = make(map[int]EntityState)
	} else {
		clear(client.EntityStates)
	}
	if i+1 < len(s.Edicts) {
		client.Edict = s.Edicts[i+1]
	}
}

func (s *Server) GetNumEdicts() int { return s.NumEdicts }
func (s *Server) GetMaxEdicts() int { return s.MaxEdicts }
func (s *Server) SetNumEdicts(n int) {
	s.NumEdicts = n
}
func (s *Server) GetEdicts() []*srvtypes.Edict { return s.Edicts }
func (s *Server) SetEdicts(edicts []*srvtypes.Edict) {
	s.Edicts = edicts
	s.NumEdicts = len(edicts)
	if s.MaxEdicts < s.NumEdicts {
		s.MaxEdicts = s.NumEdicts
	}
	if s.EdictManager != nil {
		s.EdictManager.SetEdicts(edicts)
	}
}
func (s *Server) SetEdict(i int, ent *srvtypes.Edict) {
	if i < len(s.Edicts) {
		s.Edicts[i] = ent
	}
}

func (s *Server) RebindClientEdicts() {
	if s.Static != nil {
		for i, client := range s.Static.Clients {
			if client == nil || i+1 >= len(s.Edicts) {
				continue
			}
			client.Edict = s.Edicts[i+1]
		}
	}
}

func (s *Server) ClearQCVMEdictData(entnum int) {
	clearQCVMEdictData(s.QCVM, entnum)
}

func (s *Server) EdictClearQCVMFunc(vm *qc.VM, entnum int) {
	clearQCVMEdictData(vm, entnum)
}

func (s *Server) EdictDefaultOffsets() map[string]int {
	return EdictDefaultOffsets()
}

func (s *Server) SyncQCVMState() {
	s.syncQCVMState()
}

func (s *Server) SetQCTimeGlobal(t float32) {
	s.setQCTimeGlobal(t)
}

func (s *Server) ValidateTextSaveGameDir(gameDir string) error {
	return s.validateTextSaveGameDir(gameDir)
}

func (s *Server) EnsureTextSaveEdictCapacity(required int) error {
	return s.ensureTextSaveEdictCapacity(required)
}

func (s *Server) validateTextSaveGameDir(gameDir string) error {
	if strings.TrimSpace(gameDir) == "" || s == nil || s.FileSystem == nil {
		return nil
	}

	fsInfo, ok := s.FileSystem.(interface {
		GameDir() string
		BaseDir() string
	})
	if !ok {
		return nil
	}

	current := strings.TrimSpace(fsInfo.GameDir())
	if current == "" {
		current = "id1"
	}
	if strings.EqualFold(current, gameDir) {
		return nil
	}

	baseDir := strings.TrimSpace(fsInfo.BaseDir())
	if baseDir == "" {
		return fmt.Errorf("savegame gamedir %q does not match active gamedir %q", gameDir, current)
	}

	dirInfo, err := os.Stat(filepath.Join(baseDir, gameDir))
	if err != nil || !dirInfo.IsDir() {
		return fmt.Errorf("savegame gamedir %q is unavailable under %q", gameDir, baseDir)
	}

	return fmt.Errorf("savegame gamedir %q does not match active gamedir %q", gameDir, current)
}

func (s *Server) ensureTextSaveEdictCapacity(required int) error {
	if required <= len(s.Edicts) {
		if s.NumEdicts < required {
			s.NumEdicts = required
		}
		s.ensureQCVMEdictStorage()
		return nil
	}
	if required > s.MaxEdicts {
		return fmt.Errorf("savegame entity count %d exceeds max edicts %d", required, s.MaxEdicts)
	}

	extra := required - len(s.Edicts)
	s.Edicts = append(s.Edicts, make([]*Edict, extra)...)
	if s.NumEdicts < required {
		s.NumEdicts = required
	}
	s.ensureQCVMEdictStorage()
	return nil
}
