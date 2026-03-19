package server

import (
	"log/slog"
	"math"

	"github.com/ironwail/ironwail-go/internal/cvar"
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
		if startFrame := s.QCVM.FindFunction("StartFrame"); startFrame >= 0 {
			if telemetryActive {
				s.DebugTelemetry.LogEventf(DebugEventFrame, s.QCVM, 0, s.EdictNum(0),
					"startframe begin function=%d", startFrame)
			}
			s.QCVM.SetGlobal("self", 0)
			s.QCVM.SetGlobal("other", 0)
			s.QCVM.Time = float64(s.Time)
			_ = s.executeQCFunction(startFrame)
			if telemetryActive {
				s.DebugTelemetry.LogEventf(DebugEventFrame, s.QCVM, 0, s.EdictNum(0),
					"startframe end function=%d", startFrame)
			}
		}
	}

	for i := 0; i < s.NumEdicts; i++ {
		ent := s.Edicts[i]
		if ent == nil || ent.Free {
			continue
		}
		if cv := cvar.Get("sv_freezenonclients"); cv != nil && cv.Bool() {
			isClientEnt := s.Static != nil && i > 0 && i <= s.Static.MaxClients
			if !isClientEnt {
				continue
			}
		}

		if s.QCVM != nil {
			if forceRetouch := s.QCVM.GetGlobalInt("force_retouch"); forceRetouch != 0 {
				s.LinkEdict(ent, true)
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
		if forceRetouch := s.QCVM.GetGlobalInt("force_retouch"); forceRetouch > 0 {
			s.QCVM.SetGlobalInt("force_retouch", forceRetouch-1)
		}
	}

	s.Time += s.FrameTime

	// Track active edict count and warn if exceeding standard limit of 600.
	// Matches C host.c dev_stats/dev_peakstats tracking.
	active := 0
	for i := 0; i < s.NumEdicts; i++ {
		if s.Edicts[i] != nil && !s.Edicts[i].Free {
			active++
		}
	}
	if active > 600 && s.peakEdicts <= 600 {
		slog.Warn("edict count exceeds standard limit",
			"active", active, "limit", 600, "max", s.MaxEdicts)
	}
	if active > s.peakEdicts {
		s.peakEdicts = active
	}
}
