package server

import (
	"math"

	"github.com/ironwail/ironwail-go/internal/bsp"
)

// CheckVelocity ensures entity velocity is within valid bounds.
func (s *Server) CheckVelocity(ent *Edict) {
	for i := 0; i < 3; i++ {
		if math.IsNaN(float64(ent.Vars.Velocity[i])) {
			ent.Vars.Velocity[i] = 0
		}
		if math.IsNaN(float64(ent.Vars.Origin[i])) {
			ent.Vars.Origin[i] = 0
		}
		if ent.Vars.Velocity[i] > s.MaxVelocity {
			ent.Vars.Velocity[i] = s.MaxVelocity
		} else if ent.Vars.Velocity[i] < -s.MaxVelocity {
			ent.Vars.Velocity[i] = -s.MaxVelocity
		}
	}
}

// RunThink executes the entity's think function if its nextthink time has been reached.
func (s *Server) RunThink(ent *Edict) bool {
	thinkTime := ent.Vars.NextThink
	if thinkTime <= 0 || thinkTime > s.Time+s.FrameTime {
		return true
	}

	if thinkTime < s.Time {
		thinkTime = s.Time
	}

	ent.OldThinkTime = thinkTime
	ent.OldFrame = ent.Vars.Frame
	ent.Vars.NextThink = 0

	s.QCVM.Time = float64(thinkTime)
	s.QCVM.SetGlobal("self", s.NumForEdict(ent))
	s.QCVM.SetGlobal("other", 0)
	if ent.Vars.Think != 0 {
		s.QCVM.ExecuteFunction(int(ent.Vars.Think))
	}

	return !ent.Free
}

// Impact runs touch functions for two entities that have collided.
func (s *Server) Impact(e1, e2 *Edict) {
	oldSelf := s.QCVM.GetGlobalInt("self")
	oldOther := s.QCVM.GetGlobalInt("other")

	s.QCVM.Time = float64(s.Time)

	if e1.Vars.Touch != 0 && e1.Vars.Solid != float32(SolidNot) {
		s.QCVM.SetGlobal("self", s.NumForEdict(e1))
		s.QCVM.SetGlobal("other", s.NumForEdict(e2))
		s.QCVM.ExecuteFunction(int(e1.Vars.Touch))
	}

	if e2.Vars.Touch != 0 && e2.Vars.Solid != float32(SolidNot) {
		s.QCVM.SetGlobal("self", s.NumForEdict(e2))
		s.QCVM.SetGlobal("other", s.NumForEdict(e1))
		s.QCVM.ExecuteFunction(int(e2.Vars.Touch))
	}

	s.QCVM.SetGlobalInt("self", oldSelf)
	s.QCVM.SetGlobalInt("other", oldOther)
}

// ClipVelocity slides off an impacting surface.
func ClipVelocity(in, normal [3]float32, overbounce float32) ([3]float32, int) {
	var backoff float32
	var change float32
	blocked := 0

	if normal[2] > 0 {
		blocked |= 1
	}
	if normal[2] == 0 {
		blocked |= 2
	}

	backoff = float32(float64(in[0])*float64(normal[0])+
		float64(in[1])*float64(normal[1])+
		float64(in[2])*float64(normal[2])) * overbounce

	var out [3]float32
	for i := 0; i < 3; i++ {
		change = normal[i] * backoff
		out[i] = in[i] - change
		if out[i] > -StopEpsilon && out[i] < StopEpsilon {
			out[i] = 0
		}
	}

	return out, blocked
}

const maxClipPlanes = 5

func (s *Server) AddGravity(ent *Edict) {
	entGravity := float32(1)
	ent.Vars.Velocity[2] -= entGravity * s.Gravity * s.FrameTime
}

func (s *Server) CheckWaterTransition(ent *Edict) {
	cont := s.PointContents(ent.Vars.Origin)

	if ent.Vars.WaterType == 0 {
		ent.Vars.WaterType = float32(cont)
		ent.Vars.WaterLevel = 1
		return
	}

	if cont <= bsp.ContentsWater {
		ent.Vars.WaterType = float32(cont)
		ent.Vars.WaterLevel = 1
	} else {
		ent.Vars.WaterType = float32(bsp.ContentsEmpty)
		ent.Vars.WaterLevel = float32(cont)
	}
}

func (s *Server) FlyMove(ent *Edict, time float32) int {
	numbumps := 4
	blocked := 0
	originalVelocity := ent.Vars.Velocity
	primalVelocity := ent.Vars.Velocity
	numPlanes := 0
	var planes [maxClipPlanes][3]float32
	timeLeft := time

	for bumpCount := 0; bumpCount < numbumps; bumpCount++ {
		if ent.Vars.Velocity[0] == 0 && ent.Vars.Velocity[1] == 0 && ent.Vars.Velocity[2] == 0 {
			break
		}

		end := [3]float32{
			ent.Vars.Origin[0] + timeLeft*ent.Vars.Velocity[0],
			ent.Vars.Origin[1] + timeLeft*ent.Vars.Velocity[1],
			ent.Vars.Origin[2] + timeLeft*ent.Vars.Velocity[2],
		}

		trace := s.Move(ent.Vars.Origin, ent.Vars.Mins, ent.Vars.Maxs, end, MoveType(MoveNormal), ent)
		if trace.AllSolid {
			ent.Vars.Velocity = [3]float32{}
			return 3
		}

		if trace.Fraction > 0 {
			ent.Vars.Origin = trace.EndPos
			originalVelocity = ent.Vars.Velocity
			numPlanes = 0
		}

		if trace.Fraction == 1 {
			break
		}

		if trace.Entity == nil {
			break
		}

		if trace.PlaneNormal[2] > 0.7 {
			blocked |= 1
			if int(trace.Entity.Vars.Solid) == int(SolidBSP) {
				ent.Vars.Flags = float32(uint32(ent.Vars.Flags) | FlagOnGround)
				ent.Vars.GroundEntity = int32(s.NumForEdict(trace.Entity))
			}
		}
		if trace.PlaneNormal[2] == 0 {
			blocked |= 2
		}

		s.Impact(ent, trace.Entity)
		if ent.Free {
			break
		}

		timeLeft -= timeLeft * trace.Fraction

		if numPlanes >= maxClipPlanes {
			ent.Vars.Velocity = [3]float32{}
			return 3
		}

		planes[numPlanes] = trace.PlaneNormal
		numPlanes++

		clipped := false
		for i := 0; i < numPlanes; i++ {
			newVelocity, _ := ClipVelocity(originalVelocity, planes[i], 1)

			ok := true
			for j := 0; j < numPlanes; j++ {
				if j == i {
					continue
				}
				if VecDot(newVelocity, planes[j]) < 0 {
					ok = false
					break
				}
			}

			if ok {
				ent.Vars.Velocity = newVelocity
				clipped = true
				break
			}
		}

		if !clipped {
			if numPlanes != 2 {
				ent.Vars.Velocity = [3]float32{}
				return 7
			}

			dir := VecCross(planes[0], planes[1])
			d := VecDot(dir, ent.Vars.Velocity)
			ent.Vars.Velocity = VecScale(dir, d)
		}

		if VecDot(ent.Vars.Velocity, primalVelocity) <= 0 {
			ent.Vars.Velocity = [3]float32{}
			return blocked
		}
	}

	return blocked
}

func (s *Server) PushEntity(ent *Edict, push [3]float32) TraceResult {
	end := [3]float32{
		ent.Vars.Origin[0] + push[0],
		ent.Vars.Origin[1] + push[1],
		ent.Vars.Origin[2] + push[2],
	}

	moveType := MoveNormal
	if MoveType(ent.Vars.MoveType) == MoveTypeFlyMissile {
		moveType = MoveMissile
	} else if int(ent.Vars.Solid) == int(SolidTrigger) || int(ent.Vars.Solid) == int(SolidNot) {
		moveType = MoveNoMonsters
	}

	trace := s.Move(ent.Vars.Origin, ent.Vars.Mins, ent.Vars.Maxs, end, MoveType(moveType), ent)
	ent.Vars.Origin = trace.EndPos
	s.LinkEdict(ent, true)

	if trace.Entity != nil {
		s.Impact(ent, trace.Entity)
	}

	return trace
}

func (s *Server) PushMove(pusher *Edict, movetime float32) {
	if pusher.Vars.Velocity[0] == 0 && pusher.Vars.Velocity[1] == 0 && pusher.Vars.Velocity[2] == 0 {
		pusher.Vars.LTime += movetime
		return
	}

	move := [3]float32{
		pusher.Vars.Velocity[0] * movetime,
		pusher.Vars.Velocity[1] * movetime,
		pusher.Vars.Velocity[2] * movetime,
	}
	mins := [3]float32{
		pusher.Vars.AbsMin[0] + move[0],
		pusher.Vars.AbsMin[1] + move[1],
		pusher.Vars.AbsMin[2] + move[2],
	}
	maxs := [3]float32{
		pusher.Vars.AbsMax[0] + move[0],
		pusher.Vars.AbsMax[1] + move[1],
		pusher.Vars.AbsMax[2] + move[2],
	}

	pushorig := pusher.Vars.Origin
	pusher.Vars.Origin = VecAdd(pusher.Vars.Origin, move)
	pusher.Vars.LTime += movetime
	s.LinkEdict(pusher, false)

	movedEdicts := make([]*Edict, 0, s.NumEdicts)
	movedFrom := make([][3]float32, 0, s.NumEdicts)

	for e := 1; e < s.NumEdicts; e++ {
		check := s.Edicts[e]
		if check == nil || check.Free {
			continue
		}

		movemask := 1 << int(check.Vars.MoveType)
		if movemask&((1<<int(MoveTypePush))|(1<<int(MoveTypeNone))|(1<<int(MoveTypeNoClip))) != 0 {
			continue
		}

		riding := (uint32(check.Vars.Flags)&FlagOnGround) != 0 && s.EdictNum(int(check.Vars.GroundEntity)) == pusher
		if !riding {
			if check.Vars.AbsMin[0] >= maxs[0] ||
				check.Vars.AbsMin[1] >= maxs[1] ||
				check.Vars.AbsMin[2] >= maxs[2] ||
				check.Vars.AbsMax[0] <= mins[0] ||
				check.Vars.AbsMax[1] <= mins[1] ||
				check.Vars.AbsMax[2] <= mins[2] {
				continue
			}

			if s.TestEntityPosition(check) == nil {
				continue
			}
		}

		if MoveType(check.Vars.MoveType) != MoveTypeWalk {
			check.Vars.Flags = float32(uint32(check.Vars.Flags) &^ FlagOnGround)
		}

		entorig := check.Vars.Origin
		movedEdicts = append(movedEdicts, check)
		movedFrom = append(movedFrom, entorig)

		solidBackup := pusher.Vars.Solid
		if int(pusher.Vars.Solid) == int(SolidBSP) || int(pusher.Vars.Solid) == int(SolidBBox) || int(pusher.Vars.Solid) == int(SolidSlideBox) {
			pusher.Vars.Solid = float32(SolidNot)
			s.PushEntity(check, move)
			pusher.Vars.Solid = float32(solidBackup)
		} else {
			s.PushEntity(check, move)
		}

		block := s.TestEntityPosition(check)
		if block == nil {
			continue
		}

		if check.Vars.Mins[0] == check.Vars.Maxs[0] {
			continue
		}

		if int(check.Vars.Solid) == int(SolidNot) || int(check.Vars.Solid) == int(SolidTrigger) {
			check.Vars.Mins[0], check.Vars.Mins[1], check.Vars.Mins[2] = 0, 0, 0
			check.Vars.Maxs = check.Vars.Mins
			continue
		}

		check.Vars.Origin = entorig
		s.LinkEdict(check, true)

		pusher.Vars.Origin = pushorig
		s.LinkEdict(pusher, false)
		pusher.Vars.LTime -= movetime

		if pusher.Vars.Blocked != 0 && s.QCVM != nil {
			s.QCVM.SetGlobal("self", s.NumForEdict(pusher))
			s.QCVM.SetGlobal("other", s.NumForEdict(check))
			s.QCVM.ExecuteFunction(int(pusher.Vars.Blocked))
		}

		for i, moved := range movedEdicts {
			moved.Vars.Origin = movedFrom[i]
			s.LinkEdict(moved, false)
		}

		return
	}
}

func (s *Server) PhysicsNone(ent *Edict) {
	s.RunThink(ent)
}

func (s *Server) PhysicsNoClip(ent *Edict) {
	if !s.RunThink(ent) {
		return
	}

	for i := 0; i < 3; i++ {
		ent.Vars.Angles[i] += ent.Vars.AVelocity[i] * s.FrameTime
		ent.Vars.Origin[i] += ent.Vars.Velocity[i] * s.FrameTime
	}

	s.LinkEdict(ent, false)
}

func (s *Server) PhysicsPusher(ent *Edict) {
	oldLTime := ent.Vars.LTime
	thinkTime := ent.Vars.NextThink
	movetime := s.FrameTime

	if thinkTime < ent.Vars.LTime+s.FrameTime {
		movetime = thinkTime - ent.Vars.LTime
		if movetime < 0 {
			movetime = 0
		}
	}

	if movetime != 0 {
		s.PushMove(ent, movetime)
	}

	if thinkTime > oldLTime && thinkTime <= ent.Vars.LTime {
		ent.Vars.NextThink = 0
		if s.QCVM != nil && ent.Vars.Think != 0 {
			s.QCVM.Time = float64(s.Time)
			s.QCVM.SetGlobal("self", s.NumForEdict(ent))
			s.QCVM.SetGlobal("other", 0)
			s.QCVM.ExecuteFunction(int(ent.Vars.Think))
		}
	}
}

func (s *Server) PhysicsStep(ent *Edict) {
	flags := uint32(ent.Vars.Flags)
	if flags&(FlagOnGround|FlagFly|FlagSwim) == 0 {
		s.AddGravity(ent)
		s.CheckVelocity(ent)
		s.FlyMove(ent, s.FrameTime)
		s.LinkEdict(ent, true)
	}

	s.RunThink(ent)
	s.CheckWaterTransition(ent)
}

func (s *Server) PhysicsWalk(ent *Edict) {
	if !s.RunThink(ent) {
		return
	}

	flags := uint32(ent.Vars.Flags)
	if flags&FlagWaterJump == 0 && ent.Vars.WaterLevel <= 1 && flags&(FlagOnGround|FlagFly|FlagSwim) == 0 {
		s.AddGravity(ent)
	}

	s.CheckVelocity(ent)
	s.FlyMove(ent, s.FrameTime)
	s.LinkEdict(ent, true)
	s.CheckWaterTransition(ent)
}

func (s *Server) PhysicsToss(ent *Edict) {
	if !s.RunThink(ent) {
		return
	}

	if uint32(ent.Vars.Flags)&FlagOnGround != 0 {
		return
	}

	s.CheckVelocity(ent)

	mt := MoveType(ent.Vars.MoveType)
	if mt != MoveTypeFly && mt != MoveTypeFlyMissile {
		s.AddGravity(ent)
	}

	for i := 0; i < 3; i++ {
		ent.Vars.Angles[i] += ent.Vars.AVelocity[i] * s.FrameTime
	}

	move := VecScale(ent.Vars.Velocity, s.FrameTime)
	trace := s.PushEntity(ent, move)
	if trace.Fraction == 1 || ent.Free {
		return
	}

	backoff := float32(1)
	if mt == MoveTypeBounce {
		backoff = 1.5
	}

	newVel, _ := ClipVelocity(ent.Vars.Velocity, trace.PlaneNormal, backoff)
	ent.Vars.Velocity = newVel

	if trace.PlaneNormal[2] > 0.7 {
		if ent.Vars.Velocity[2] < 60 || mt != MoveTypeBounce {
			ent.Vars.Flags = float32(uint32(ent.Vars.Flags) | FlagOnGround)
			ent.Vars.GroundEntity = int32(s.NumForEdict(trace.Entity))
			ent.Vars.Velocity = [3]float32{}
			ent.Vars.AVelocity = [3]float32{}
		}
	}

	s.CheckWaterTransition(ent)
}
