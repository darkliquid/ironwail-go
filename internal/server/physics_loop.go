package server

import (
	"fmt"
	"math"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func (s *Server) Physics() {
	telemetryActive := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
	if telemetryActive {
		s.DebugTelemetry.BeginFrame(s.Time, s.FrameTime)
		s.DebugTelemetry.LogEventf(DebugEventFrame, s.QCVM, 0, s.EdictNum(0),
			"physics begin edicts=%d", s.NumEdicts)
		defer func() {
			s.DebugTelemetry.LogEventf(DebugEventFrame, s.QCVM, 0, s.EdictNum(0),
				"physics end edicts=%d", s.NumEdicts)
			s.DebugTelemetry.EndFrame()
		}()
	}

	if s.QCVM != nil {
		// In C, StartFrame runs against the authoritative edict state that was just
		// updated by SV_RunClients. Keep the QC VM snapshot in sync here so
		// intermission/QC frame logic can observe fresh button presses immediately.
		s.syncQCVMState()
		if startFrame := s.QCVM.FindFunction("StartFrame"); startFrame >= 0 {
			if telemetryActive {
				s.DebugTelemetry.LogEventf(DebugEventFrame, s.QCVM, 0, s.EdictNum(0),
					"startframe begin function=%d", startFrame)
			}
			s.QCVM.SetGlobal("self", 0)
			s.QCVM.SetGlobal("other", 0)
			s.setQCTimeGlobal(s.Time)
			_ = s.executeQCFunction(startFrame)
			if telemetryActive {
				s.DebugTelemetry.LogEventf(DebugEventFrame, s.QCVM, 0, s.EdictNum(0),
					"startframe end function=%d", startFrame)
			}
		}
	}

	freezeNonClients := false
	if cv := cvar.Get("sv_freezenonclients"); cv != nil && cv.Bool() {
		freezeNonClients = true
	}

	entityCap := s.NumEdicts
	if freezeNonClients && s.Static != nil {
		entityCap = s.Static.MaxClients + 1
		if entityCap > s.NumEdicts {
			entityCap = s.NumEdicts
		}
	}

	for i := 0; i < entityCap; i++ {
		ent := s.Edicts[i]
		if ent == nil || ent.Free {
			continue
		}

		if s.QCVM != nil {
			if forceRetouch := s.QCVM.GetGlobalFloat("force_retouch"); forceRetouch != 0 {
				s.LinkEdict(ent, true)
			}
		}

		// Mirror C SV_Physics: client-slot entities (i=1..maxclients) go through
		// SV_Physics_Client which calls PlayerPreThink and PlayerPostThink regardless
		// of movetype. PhysicsWalk already handles MoveTypeWalk; for all other
		// movetypes (especially MoveTypeNone during intermission) we must wrap here.
		// Without this, IntermissionThink in QC never fires during intermission
		// (player movetype = MOVETYPE_NONE) and attack/enter cannot progress the level.
		var clientForPostThink *Client
		if MoveType(ent.Vars.MoveType) != MoveTypeWalk {
			if pc := s.playerClient(ent); pc != nil {
				s.runClientQCThink(pc, "PlayerPreThink")
				if ent.Free {
					// Entity freed by PreThink (e.g. ClientDisconnect during think).
					// Skip movetype dispatch and SendInterval bookkeeping.
					continue
				}
				clientForPostThink = pc
			}
		}

		switch MoveType(ent.Vars.MoveType) {
		case MoveTypePush:
			s.PhysicsPusher(ent)
		case MoveTypeNone:
			s.PhysicsNone(ent)
		case MoveTypeNoClip:
			s.PhysicsNoClip(ent)
		case MoveTypeStep:
			s.PhysicsStep(ent)
		case MoveTypeToss, MoveTypeGib, MoveTypeBounce, MoveTypeFly, MoveTypeFlyMissile:
			s.PhysicsToss(ent)
		case MoveTypeWalk:
			s.PhysicsWalk(ent)
		default:
			panic(fmt.Sprintf("SV_Physics: bad movetype %d", int(ent.Vars.MoveType)))
		}

		if clientForPostThink != nil && !ent.Free {
			s.runClientQCThink(clientForPostThink, "PlayerPostThink")
		}

		ent.SendInterval = false
		if !ent.Free && ent.Vars.NextThink > s.Time &&
			(MoveType(ent.Vars.MoveType) == MoveTypeStep || MoveType(ent.Vars.MoveType) == MoveTypeWalk || ent.Vars.Frame != ent.OldFrame) {
			// Encode the interval to next think as a byte (0-255).
			// Values 25 and 26 are close enough to 0.1 (the client default)
			// that sending them would be redundant.
			j := int(math.Round(float64((ent.Vars.NextThink - ent.OldThinkTime) * 255)))
			if j >= 0 && j < 256 && j != 25 && j != 26 {
				ent.SendInterval = true
			}
		}
	}

	if s.QCVM != nil {
		if forceRetouch := s.QCVM.GetGlobalFloat("force_retouch"); forceRetouch > 0 {
			next := forceRetouch - 1
			if next < 0 {
				next = 0
			}
			s.QCVM.SetGlobal("force_retouch", next)
		}
	}

	if !freezeNonClients {
		s.Time += s.FrameTime
	}

	// Track active edict count and warn if exceeding standard limit of 600.
	// Matches C host.c dev_stats/dev_peakstats tracking.
	active := 0
	for i := 0; i < s.NumEdicts; i++ {
		if s.Edicts[i] != nil && !s.Edicts[i].Free {
			active++
		}
	}
	s.recordDevStatsEdicts(active)
}

// PeakEdicts returns the highest active edict count seen by Physics.
func (s *Server) PeakEdicts() int {
	if s == nil {
		return 0
	}
	return s.peakEdicts
}
