package server

func (s *Server) Physics() {
	if startFrame := s.QCVM.FindFunction("StartFrame"); startFrame >= 0 {
		s.QCVM.SetGlobal("self", 0)
		s.QCVM.SetGlobal("other", 0)
		s.QCVM.Time = float64(s.Time)
		s.QCVM.ExecuteFunction(startFrame)
	}

	for i := 0; i < s.NumEdicts; i++ {
		ent := s.Edicts[i]
		if ent.Free {
			continue
		}

		if forceRetouch := s.QCVM.GetGlobalInt("force_retouch"); forceRetouch != 0 {
			s.LinkEdict(ent, true)
		}

		mt := MoveType(ent.Vars.MoveType)

		switch mt {
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
			s.RunThink(ent)
		}

		if !ent.Free && ent.Vars.NextThink > s.Time {
			ent.SendInterval = true
		}
	}

	if forceRetouch := s.QCVM.GetGlobalInt("force_retouch"); forceRetouch > 0 {
		s.QCVM.SetGlobalInt("force_retouch", forceRetouch-1)
	}

	s.Time += s.FrameTime
}

func (s *Server) PhysicsNone(ent *Edict) {
	s.RunThink(ent)
}

func (s *Server) PhysicsNoClip(ent *Edict) {
	if !s.RunThink(ent) {
		return
	}

	for i := 0; i < 3; i++ {
		ent.Vars.Origin[i] += ent.Vars.Velocity[i] * s.FrameTime
	}

	s.LinkEdict(ent, false)
}

func (s *Server) PhysicsPusher(ent *Edict) {
	if !s.RunThink(ent) {
		return
	}

	if ent.Vars.AVelocity[0] != 0 || ent.Vars.AVelocity[1] != 0 || ent.Vars.AVelocity[2] != 0 {
		for i := 0; i < 3; i++ {
			ent.Vars.Angles[i] += ent.Vars.AVelocity[i] * s.FrameTime
		}
		s.LinkEdict(ent, true)
	}
}

func (s *Server) PhysicsStep(ent *Edict) {
	if !s.RunThink(ent) {
		return
	}

	s.CheckVelocity(ent)

	flags := uint32(ent.Vars.Flags)
	if flags&FlagOnGround == 0 {
		ent.Vars.Velocity[2] -= s.Gravity * s.FrameTime
	}

	s.CheckVelocity(ent)

	s.PushEntity(ent, ent.Vars.Velocity[0]*s.FrameTime,
		ent.Vars.Velocity[1]*s.FrameTime,
		ent.Vars.Velocity[2]*s.FrameTime)

	s.RunThink(ent)
}

func (s *Server) PhysicsToss(ent *Edict) {
	if !s.RunThink(ent) {
		return
	}

	s.CheckVelocity(ent)

	mt := MoveType(ent.Vars.MoveType)
	if mt == MoveTypeToss || mt == MoveTypeGib || mt == MoveTypeBounce {
		ent.Vars.Velocity[2] -= s.Gravity * s.FrameTime
	}

	s.CheckVelocity(ent)

	s.PushEntity(ent, ent.Vars.Velocity[0]*s.FrameTime,
		ent.Vars.Velocity[1]*s.FrameTime,
		ent.Vars.Velocity[2]*s.FrameTime)

	s.RunThink(ent)
}








func (s *Server) PushEntity(ent *Edict, dx, dy, dz float32) {
	var end [3]float32
	end[0] = ent.Vars.Origin[0] + dx
	end[1] = ent.Vars.Origin[1] + dy
	end[2] = ent.Vars.Origin[2] + dz

	ent.Vars.Origin[0] = end[0]
	ent.Vars.Origin[1] = end[1]
	ent.Vars.Origin[2] = end[2]

	s.LinkEdict(ent, true)
}
