package server

import (
	"log/slog"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

// deathmatchRespawnDelay defines how long dead players wait before automatic
// respawn in deathmatch. The delay mirrors classic Quake pacing: the player
// sees death feedback briefly before re-entering the simulation.
const deathmatchRespawnDelay = 2.0

// syncGameModeFromCVars snapshots coop/deathmatch cvars into server booleans.
// This keeps per-frame rule evaluation coupled to console-configured game mode
// without requiring every caller to query cvars directly.
func (s *Server) syncGameModeFromCVars() {
	s.Coop = cvar.BoolValue("coop")
	s.Deathmatch = cvar.BoolValue("deathmatch")
}

// CheckRules enforces match-end conditions after each simulation frame.
// It bridges cvar policy (fraglimit/timelimit) to authoritative server state:
// once a win condition is met, it triggers QuakeC-level transition logic.
func (s *Server) CheckRules() {
	if s == nil || s.Static == nil {
		return
	}
	s.syncGameModeFromCVars()
	if !s.Deathmatch || s.Coop || s.Static.ChangeLevelIssued {
		return
	}

	fragLimit := cvar.FloatValue("fraglimit")
	if fragLimit > 0 {
		for _, client := range s.Static.Clients {
			if client == nil || !client.Active || client.Edict == nil || client.Edict.Free {
				continue
			}
			if float64(client.Edict.Vars.Frags) >= fragLimit {
				s.issueMatchEnd("fraglimit")
				return
			}
		}
	}

	timeLimit := cvar.FloatValue("timelimit")
	if timeLimit > 0 && float64(s.Time) >= timeLimit*60 {
		s.issueMatchEnd("timelimit")
	}
}

// issueMatchEnd triggers the QuakeC NextLevel path exactly once per match end.
// The ChangeLevelIssued guard prevents duplicate transitions when multiple
// clients satisfy end conditions in the same frame.
func (s *Server) issueMatchEnd(reason string) {
	if s == nil || s.Static == nil || s.Static.ChangeLevelIssued {
		return
	}
	s.Static.ChangeLevelIssued = true
	if s.QCVM == nil {
		return
	}

	nextLevel := s.QCVM.FindFunction("NextLevel")
	if nextLevel < 0 {
		return
	}
	s.syncQCVMState()
	s.QCVM.Time = float64(s.Time)
	s.QCVM.SetGlobal("time", s.Time)
	s.QCVM.SetGlobal("frametime", s.FrameTime)
	s.QCVM.SetGlobal("self", 0)
	s.QCVM.SetGlobal("other", 0)
	if err := s.QCVM.ExecuteFunction(nextLevel); err != nil {
		slog.Warn("failed to execute NextLevel", "reason", reason, "error", err)
	}
}

// handleDeathmatchRespawn applies deathmatch-specific delayed auto-respawn.
// It gates dead players for a short timeout, then invokes QC spawn routines so
// spawnpoint selection and inventory reset remain gamecode-driven.
func (s *Server) handleDeathmatchRespawn(client *Client) bool {
	if s == nil || client == nil || client.Edict == nil || client.Edict.Free {
		return false
	}

	s.syncGameModeFromCVars()
	if !s.Deathmatch || s.Coop {
		client.RespawnTime = 0
		return false
	}

	ent := client.Edict
	dead := ent.Vars.Health <= 0 || DeadFlag(ent.Vars.DeadFlag) >= DeadDead
	if !dead {
		client.RespawnTime = 0
		return false
	}

	if client.RespawnTime <= 0 {
		client.RespawnTime = s.Time + deathmatchRespawnDelay
	}
	if s.Time < client.RespawnTime {
		return true
	}

	ent.Vars.DeadFlag = float32(DeadRespawnable)
	if err := s.runClientPutInServerQC(client); err != nil {
		return true
	}
	if ent.Vars.Health <= 0 {
		if err := s.runClientSpawnQC(client); err != nil {
			return true
		}
	}
	ent.Vars.DeadFlag = float32(DeadNo)
	client.RespawnTime = 0
	return false
}
