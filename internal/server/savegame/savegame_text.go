// This file belongs to the Savegame subsystem: save/load game serialization and text format helpers.
package savegame

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/common"
	"github.com/darkliquid/ironwail-go/internal/server/edict"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// RestoreTextSaveGameState applies a parsed text save to a live spawned server.
func RestoreTextSaveGameState(s Server, state *TextSaveGameState) error {
	if s == nil {
		return fmt.Errorf("server is nil")
	}
	if state == nil {
		return fmt.Errorf("text savegame state is nil")
	}
	if state.MapName == "" {
		return fmt.Errorf("savegame map is empty")
	}
	if s.GetName() != "" && state.MapName != s.GetName() {
		return fmt.Errorf("savegame map %q does not match active map %q", state.MapName, s.GetName())
	}
	qcvm := s.GetVM()
	if qcvm == nil {
		return fmt.Errorf("qcvm is not initialized")
	}
	if err := s.ValidateTextSaveGameDir(state.GameDir); err != nil {
		return err
	}

	s.SetTime(state.Time)
	s.SetPaused(true)
	s.SetLoadGame(true)

	lightStyles := s.GetLightStyles()
	if lightStyles != nil {
		copy(lightStyles[:], state.LightStyles[:])
		for i := range lightStyles {
			if lightStyles[i] == "" {
				lightStyles[i] = "m"
			}
		}
	}

	if len(state.SpawnParms) > 0 {
		clients := s.GetClients()
		if len(clients) > 0 {
			client := clients[0]
			client.SpawnParms = state.SpawnParms
			s.RestoreClientState(0, client)
		}
	}

	edicts := s.GetEdicts()
	em := edict.NewManager(edicts, qcvm, s.GetMaxEdicts(), s.GetNumEdicts(), s.MaxClients(), make([]float32, maxInt(s.GetMaxEdicts(), len(edicts))), s.EdictClearQCVMFunc, s.EdictDefaultOffsets())
	em.SetCurrentTime(s.GetTime())

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
			data, err = em.ED_ParseGlobals(blockData, qcvm)
			if err != nil {
				return err
			}
		} else {
			if err := s.EnsureTextSaveEdictCapacity(entnum + 1); err != nil {
				return err
			}
			edicts = s.GetEdicts()
			em.SetEdicts(edicts)
			if entnum < em.NumEdicts() {
				em.ED_ClearEdict(entnum)
			} else {
				s.SetEdict(entnum, &srvtypes.Edict{Scale: 16})
				s.ClearQCVMEdictData(entnum)
			}
			data, err = em.ED_ParseEdict(blockData, entnum)
			if err != nil {
				return err
			}
			edicts = s.GetEdicts()
			if entnum < len(edicts) {
				if ent := edicts[entnum]; ent != nil && !ent.Free && entnum != 0 {
					if ent.Scale == 0 {
						ent.Scale = 16
					}
					s.LinkEdict(ent, false)
				}
			}
		}

		entnum++
	}

	if entnum <= 0 {
		return fmt.Errorf("savegame contains no edicts")
	}

	for i := entnum; i < s.GetNumEdicts(); i++ {
		em.ED_ClearEdict(i)
	}
	s.SetNumEdicts(entnum)
	qcvm.NumEdicts = entnum
	s.SetQCTimeGlobal(state.Time)

	serverFlags := qcvm.GlobalInt("serverflags")
	if serverFlags != 0 || s.GetServerFlags() != 0 {
		s.SetServerFlags(serverFlags)
	}

	s.RebindClientEdicts()
	return nil
}
