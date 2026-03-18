package server

import (
	"math"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

func (s *Server) Physics() {
	if s.QCVM != nil {
		if startFrame := s.QCVM.FindFunction("StartFrame"); startFrame >= 0 {
			s.QCVM.SetGlobal("self", 0)
			s.QCVM.SetGlobal("other", 0)
			s.QCVM.Time = float64(s.Time)
			s.QCVM.ExecuteFunction(startFrame)
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
}
