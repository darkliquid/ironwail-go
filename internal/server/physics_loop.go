package server

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

		if !ent.Free && ent.Vars.NextThink > s.Time {
			ent.SendInterval = true
		}
	}

	if s.QCVM != nil {
		if forceRetouch := s.QCVM.GetGlobalInt("force_retouch"); forceRetouch > 0 {
			s.QCVM.SetGlobalInt("force_retouch", forceRetouch-1)
		}
	}

	s.Time += s.FrameTime
}
