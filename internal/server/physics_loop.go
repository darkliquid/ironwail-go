package server

import (
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func (s *Server) Physics() {
	physicsStart := time.Now()
	hostSpeeds := cvar.BoolValue("host_speeds")
	phaseStart := time.Time{}
	measureEnabled := func() bool {
		return hostSpeeds
	}
	phaseBegin := func() {
		if measureEnabled() {
			phaseStart = time.Now()
		}
	}
	phaseEnd := func(total *float64) {
		if measureEnabled() {
			*total += float64(time.Since(phaseStart)) / float64(time.Millisecond)
		}
	}
	var startFrameMS float64
	var forceRetouchMS float64
	var preThinkMS float64
	var postThinkMS float64
	var bookkeepingMS float64
	var devStatsMS float64
	var physicsPushMS float64
	var physicsNoneMS float64
	var physicsNoClipMS float64
	var physicsStepMS float64
	var physicsTossMS float64
	var physicsWalkMS float64
	var physicsPushCount int
	var physicsNoneCount int
	var physicsNoClipCount int
	var physicsStepCount int
	var physicsTossCount int
	var physicsWalkCount int

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

	phaseBegin()
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
	phaseEnd(&startFrameMS)

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

	forceRetouch := float32(0)
	if s.QCVM != nil {
		forceRetouch = s.QCVM.GetGlobalFloat("force_retouch")
	}

	for i := 0; i < entityCap; i++ {
		ent := s.Edicts[i]
		if ent == nil || ent.Free {
			continue
		}

		if forceRetouch != 0 {
			phaseBegin()
			s.LinkEdict(ent, true)
			phaseEnd(&forceRetouchMS)
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
				phaseBegin()
				s.runClientQCThinkWithMode(pc, "PlayerPreThink", false)
				phaseEnd(&preThinkMS)
				if ent.Free {
					// Entity freed by PreThink (e.g. ClientDisconnect during think).
					// Skip movetype dispatch and SendInterval bookkeeping.
					continue
				}
				clientForPostThink = pc
			}
		}

		moveType := MoveType(ent.Vars.MoveType)
		switch moveType {
		case MoveTypePush:
			physicsPushCount++
			phaseBegin()
			s.PhysicsPusher(ent)
			phaseEnd(&physicsPushMS)
		case MoveTypeNone:
			physicsNoneCount++
			phaseBegin()
			s.PhysicsNone(ent)
			phaseEnd(&physicsNoneMS)
		case MoveTypeNoClip:
			physicsNoClipCount++
			phaseBegin()
			s.PhysicsNoClip(ent)
			phaseEnd(&physicsNoClipMS)
		case MoveTypeStep:
			physicsStepCount++
			phaseBegin()
			s.PhysicsStep(ent)
			phaseEnd(&physicsStepMS)
		case MoveTypeToss, MoveTypeGib, MoveTypeBounce, MoveTypeFly, MoveTypeFlyMissile:
			physicsTossCount++
			phaseBegin()
			s.PhysicsToss(ent)
			phaseEnd(&physicsTossMS)
		case MoveTypeWalk:
			physicsWalkCount++
			phaseBegin()
			s.PhysicsWalk(ent)
			phaseEnd(&physicsWalkMS)
		default:
			panic(fmt.Sprintf("SV_Physics: bad movetype %d", int(ent.Vars.MoveType)))
		}

		if clientForPostThink != nil && !ent.Free {
			phaseBegin()
			s.runClientQCThinkWithMode(clientForPostThink, "PlayerPostThink", false)
			phaseEnd(&postThinkMS)
		}

		phaseBegin()
		ent.SendInterval = false
		if !ent.Free && ent.Vars.NextThink > s.Time &&
			(moveType == MoveTypeStep || moveType == MoveTypeWalk || ent.Vars.Frame != ent.OldFrame) {
			// Encode the interval to next think as a byte (0-255).
			// Values 25 and 26 are close enough to 0.1 (the client default)
			// that sending them would be redundant.
			j := int(math.Round(float64((ent.Vars.NextThink - ent.OldThinkTime) * 255)))
			if j >= 0 && j < 256 && j != 25 && j != 26 {
				ent.SendInterval = true
			}
		}
		phaseEnd(&bookkeepingMS)
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
	phaseBegin()
	active := 0
	for i := 0; i < s.NumEdicts; i++ {
		if s.Edicts[i] != nil && !s.Edicts[i].Free {
			active++
		}
	}
	s.recordDevStatsEdicts(active)
	phaseEnd(&devStatsMS)

	if hostSpeeds {
		slog.Info("physics_speeds",
			"time", s.Time,
			"startframe_ms", startFrameMS,
			"force_retouch_ms", forceRetouchMS,
			"prethink_ms", preThinkMS,
			"postthink_ms", postThinkMS,
			"push_ms", physicsPushMS,
			"push_count", physicsPushCount,
			"none_ms", physicsNoneMS,
			"none_count", physicsNoneCount,
			"noclip_ms", physicsNoClipMS,
			"noclip_count", physicsNoClipCount,
			"step_ms", physicsStepMS,
			"step_count", physicsStepCount,
			"toss_ms", physicsTossMS,
			"toss_count", physicsTossCount,
			"walk_ms", physicsWalkMS,
			"walk_count", physicsWalkCount,
			"bookkeeping_ms", bookkeepingMS,
			"devstats_ms", devStatsMS,
			"total_ms", float64(time.Since(physicsStart))/float64(time.Millisecond),
		)
	}
}

// PeakEdicts returns the highest active edict count seen by Physics.
func (s *Server) PeakEdicts() int {
	if s == nil {
		return 0
	}
	return s.peakEdicts
}
