// This file belongs to the Savegame subsystem: save/load game serialization and text format helpers.
//
// TextSaveGameState and ParseTextSaveGame have been moved to
// internal/server/savegame. The RestoreTextSaveGameState method remains
// here because it requires deep access to Server internals.
package server

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/common"
)

// maxInt returns the larger of a and b.
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RestoreTextSaveGameState applies a parsed text save to a live spawned server.
// The caller is expected to spawn the target map first, then invoke this before
// the local signon handshake continues.
func (s *Server) RestoreTextSaveGameState(state *TextSaveGameState) error {
	if s == nil {
		return fmt.Errorf("server is nil")
	}
	if state == nil {
		return fmt.Errorf("text savegame state is nil")
	}
	if state.MapName == "" {
		return fmt.Errorf("savegame map is empty")
	}
	if s.Name != "" && state.MapName != s.Name {
		return fmt.Errorf("savegame map %q does not match active map %q", state.MapName, s.Name)
	}
	if s.QCVM == nil {
		return fmt.Errorf("qcvm is not initialized")
	}
	if err := s.validateTextSaveGameDir(state.GameDir); err != nil {
		return err
	}

	s.Time = state.Time
	s.Paused = true
	s.LoadGame = true
	copy(s.LightStyles[:], state.LightStyles[:])
	for i := range s.LightStyles {
		if s.LightStyles[i] == "" {
			s.LightStyles[i] = "m"
		}
	}
	if s.Static != nil && len(s.Static.Clients) > 0 && s.Static.Clients[0] != nil {
		s.Static.Clients[0].SpawnParms = state.SpawnParms
	}

	em := &EntityManager{
		edicts:     s.Edicts,
		vm:         s.QCVM,
		maxEdicts:  s.MaxEdicts,
		numEdicts:  s.NumEdicts,
		freeTime:   make([]float32, maxInt(s.MaxEdicts, len(s.Edicts))),
		maxClients: s.MaxClients(),
	}
	em.SetCurrentTime(s.Time)

	s.ClearWorld()

	data := state.EntityText
	entnum := -1
	for {
		data = common.COM_Parse(data)
		if common.ComToken == "" {
			break
		}
		if common.ComToken != "{" {
			return fmt.Errorf("first token isn't a brace")
		}
		blockData := "{" + data

		var err error
		if entnum == -1 {
			data, err = em.ED_ParseGlobals(blockData, s.QCVM)
			if err != nil {
				return err
			}
		} else {
			if err := s.ensureTextSaveEdictCapacity(entnum + 1); err != nil {
				return err
			}
			em.edicts = s.Edicts
			if entnum < em.numEdicts {
				em.ED_ClearEdict(entnum)
			} else {
				s.Edicts[entnum] = &Edict{Scale: 16}
				clearQCVMEdictData(s.QCVM, entnum)
			}
			data, err = em.ED_ParseEdict(blockData, entnum)
			if err != nil {
				return err
			}
			if ent := s.Edicts[entnum]; ent != nil && !ent.Free && entnum != 0 {
				if ent.Scale == 0 {
					ent.Scale = 16
				}
				s.LinkEdict(ent, false)
			}
		}

		entnum++
	}

	if entnum <= 0 {
		return fmt.Errorf("savegame contains no edicts")
	}

	for i := entnum; i < s.NumEdicts; i++ {
		em.ED_ClearEdict(i)
	}
	s.NumEdicts = entnum
	s.QCVM.NumEdicts = entnum
	s.setQCTimeGlobal(state.Time)

	if s.Static != nil {
		if serverFlags := s.QCVM.GlobalInt("serverflags"); serverFlags != 0 || s.Static.ServerFlags != 0 {
			s.Static.ServerFlags = serverFlags
		}
		for i, client := range s.Static.Clients {
			if client == nil || i+1 >= len(s.Edicts) {
				continue
			}
			client.Edict = s.Edicts[i+1]
		}
	}

	return nil
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
