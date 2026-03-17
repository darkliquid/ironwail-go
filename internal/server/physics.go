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
func ClipVelocity(in, normal [3]float32, overbounce float32) [3]float32 {
	backoff := VecDot(in, normal) * overbounce

	var out [3]float32
	for i := 0; i < 3; i++ {
		change := normal[i] * backoff
		out[i] = in[i] - change
		if out[i] > -StopEpsilon && out[i] < StopEpsilon {
			out[i] = 0
		}
	}

	return out
}

const maxClipPlanes = 5

func (s *Server) AddGravity(ent *Edict) {
	entGravity := float32(1)
	ent.Vars.Velocity[2] -= entGravity * s.Gravity * s.FrameTime
}

func (s *Server) SV_CheckWater(ent *Edict) bool {
	var point [3]float32
	point[0] = ent.Vars.Origin[0]
	point[1] = ent.Vars.Origin[1]
	point[2] = ent.Vars.Origin[2] + ent.Vars.Mins[2] + 1

	ent.Vars.WaterLevel = 0
	ent.Vars.WaterType = float32(bsp.ContentsEmpty)
	cont := s.PointContents(point)
	if cont <= bsp.ContentsWater {
		ent.Vars.WaterType = float32(cont)
		ent.Vars.WaterLevel = 1
		point[2] = ent.Vars.Origin[2] + (ent.Vars.Mins[2]+ent.Vars.Maxs[2])*0.5
		cont = s.PointContents(point)
		if cont <= bsp.ContentsWater {
			ent.Vars.WaterLevel = 2
			point[2] = ent.Vars.Origin[2] + ent.Vars.ViewOfs[2]
			cont = s.PointContents(point)
			if cont <= bsp.ContentsWater {
				ent.Vars.WaterLevel = 3
			}
		}
	}

	return ent.Vars.WaterLevel > 1
}

func (s *Server) CheckWaterTransition(ent *Edict) {
	cont := s.PointContents(ent.Vars.Origin)

	if ent.Vars.WaterType == 0 { // just spawned here
		ent.Vars.WaterType = float32(cont)
		ent.Vars.WaterLevel = 1
		return
	}

	if cont <= bsp.ContentsWater {
		if ent.Vars.WaterType == float32(bsp.ContentsEmpty) {
			// just crossed into water
			s.StartSound(ent, 0, "misc/h2ohit1.wav", 255, 1)
		}
		ent.Vars.WaterType = float32(cont)
		ent.Vars.WaterLevel = 1
	} else {
		if ent.Vars.WaterType != float32(bsp.ContentsEmpty) {
			// just crossed out of water
			s.StartSound(ent, 0, "misc/h2ohit1.wav", 255, 1)
		}
		ent.Vars.WaterType = float32(bsp.ContentsEmpty)
		ent.Vars.WaterLevel = 0
	}
}

func (s *Server) FlyMove(ent *Edict, time float32) int {
	blocked := 0
	originalVelocity := ent.Vars.Velocity
	primalVelocity := ent.Vars.Velocity
	numPlanes := 0
	var planes [maxClipPlanes][3]float32
	timeLeft := time

	for bumpCount := 0; bumpCount < 4; bumpCount++ {
		if ent.Vars.Velocity[0] == 0 && ent.Vars.Velocity[1] == 0 && ent.Vars.Velocity[2] == 0 {
			break
		}

		end := VecAdd(ent.Vars.Origin, VecScale(ent.Vars.Velocity, timeLeft))
		trace := s.Move(ent.Vars.Origin, ent.Vars.Mins, ent.Vars.Maxs, end, MoveNormal, ent)

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

		if trace.PlaneNormal[2] > 0.7 {
			blocked |= 1
			if trace.Entity != nil && int(trace.Entity.Vars.Solid) == int(SolidBSP) {
				ent.Vars.Flags = float32(uint32(ent.Vars.Flags) | FlagOnGround)
				ent.Vars.GroundEntity = int32(s.NumForEdict(trace.Entity))
			}
		}
		if trace.PlaneNormal[2] == 0 {
			blocked |= 2
		}

		// Run touch functions
		if trace.Entity != nil {
			s.Impact(ent, trace.Entity)
			if ent.Free {
				break
			}
		}

		timeLeft -= timeLeft * trace.Fraction

		if numPlanes >= maxClipPlanes {
			ent.Vars.Velocity = [3]float32{}
			return 3
		}

		planes[numPlanes] = trace.PlaneNormal
		numPlanes++

		// Standard Quake recursive plane clipping
		var newVelocity [3]float32
		for i := 0; i < numPlanes; i++ {
			newVelocity = ClipVelocity(originalVelocity, planes[i], 1.001) // overbounce for precision
			j := 0
			for ; j < numPlanes; j++ {
				if j == i {
					continue
				}
				if VecDot(newVelocity, planes[j]) < 0 {
					break
				}
			}
			if j == numPlanes {
				break
			}
		}

		if numPlanes >= 2 {
			// Slide along intersection of two planes
			dir := VecCross(planes[0], planes[1])
			d := VecDot(dir, ent.Vars.Velocity)
			newVelocity = VecScale(dir, d)

			if numPlanes >= 3 {
				// Stuck in a corner
				ent.Vars.Velocity = [3]float32{}
				return blocked
			}
		}

		ent.Vars.Velocity = newVelocity
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

func (s *Server) SV_CheckAllEnts() {
	for i := 1; i < s.NumEdicts; i++ {
		ent := s.Edicts[i]
		if ent == nil || ent.Free {
			continue
		}
		mt := MoveType(ent.Vars.MoveType)
		if mt == MoveTypePush || mt == MoveTypeNone || mt == MoveTypeNoClip {
			continue
		}

		if s.TestEntityPosition(ent) != nil {
			// entity in invalid position
		}
	}
}

func (s *Server) SV_TryUnstick(ent *Edict, oldVel [3]float32) int {
	oldOrg := ent.Vars.Origin
	var dir [3]float32

	for i := 0; i < 8; i++ {
		dir = [3]float32{}
		switch i {
		case 0:
			dir[0] = 2
			dir[1] = 0
		case 1:
			dir[0] = 0
			dir[1] = 2
		case 2:
			dir[0] = -2
			dir[1] = 0
		case 3:
			dir[0] = 0
			dir[1] = -2
		case 4:
			dir[0] = 2
			dir[1] = 2
		case 5:
			dir[0] = -2
			dir[1] = 2
		case 6:
			dir[0] = 2
			dir[1] = -2
		case 7:
			dir[0] = -2
			dir[1] = -2
		}

		s.PushEntity(ent, dir)

		// retry the original move
		ent.Vars.Velocity[0] = oldVel[0]
		ent.Vars.Velocity[1] = oldVel[1]
		ent.Vars.Velocity[2] = 0
		blocked := s.FlyMove(ent, 0.1)

		if math.Abs(float64(oldOrg[0]-ent.Vars.Origin[0])) > 4 ||
			math.Abs(float64(oldOrg[1]-ent.Vars.Origin[1])) > 4 {
			return blocked
		}

		// go back to the original pos and try again
		ent.Vars.Origin = oldOrg
	}

	ent.Vars.Velocity = [3]float32{}
	return 7 // still not moving
}

func (s *Server) SV_StepMove(ent *Edict, move [3]float32, relink bool) bool {
	// Try moving at current height
	trace := s.Move(ent.Vars.Origin, ent.Vars.Mins, ent.Vars.Maxs, VecAdd(ent.Vars.Origin, move), MoveType(MoveNormal), ent)
	if trace.Fraction == 1 {
		ent.Vars.Origin = trace.EndPos
		if relink {
			s.LinkEdict(ent, true)
		}
		return true
	}

	// Try stepping up
	originalOrigin := ent.Vars.Origin
	oldVel := ent.Vars.Velocity

	// Raise up
	up := [3]float32{0, 0, 18} // Quake step size
	trace = s.Move(ent.Vars.Origin, ent.Vars.Mins, ent.Vars.Maxs, VecAdd(ent.Vars.Origin, up), MoveType(MoveNormal), ent)
	ent.Vars.Origin = trace.EndPos

	// Move forward
	trace = s.Move(ent.Vars.Origin, ent.Vars.Mins, ent.Vars.Maxs, VecAdd(ent.Vars.Origin, move), MoveType(MoveNormal), ent)
	ent.Vars.Origin = trace.EndPos

	if trace.Fraction < 1 {
		if math.Abs(float64(originalOrigin[0]-ent.Vars.Origin[0])) < 0.03125 &&
			math.Abs(float64(originalOrigin[1]-ent.Vars.Origin[1])) < 0.03125 {
			s.SV_TryUnstick(ent, oldVel)
		}
	}

	if trace.Fraction < 1 && trace.PlaneNormal[2] == 0 {
		s.SV_WallFriction(ent, &trace)
	}
	// Push back down
	down := [3]float32{0, 0, -18}
	trace = s.Move(ent.Vars.Origin, ent.Vars.Mins, ent.Vars.Maxs, VecAdd(ent.Vars.Origin, down), MoveType(MoveNormal), ent)
	ent.Vars.Origin = trace.EndPos

	// If we didn't move at all, or we're in a worse spot, revert
	if trace.AllSolid || trace.StartSolid || trace.Fraction == 0 {
		ent.Vars.Origin = originalOrigin
		return false
	}

	if trace.PlaneNormal[2] > 0.7 {
		if trace.Entity != nil && int(trace.Entity.Vars.Solid) == int(SolidBSP) {
			ent.Vars.Flags = float32(uint32(ent.Vars.Flags) | FlagOnGround)
			ent.Vars.GroundEntity = int32(s.NumForEdict(trace.Entity))
		}
	} else {
		// if the push down didn't end up on good ground, use the move without the step up.
		ent.Vars.Origin = originalOrigin
		return false
	}

	if relink {
		s.LinkEdict(ent, true)
	}
	return true
}

func (s *Server) SV_WallFriction(ent *Edict, trace *TraceResult) {
	var forward, right, up [3]float32
	AngleVectors(ent.Vars.VAngle, &forward, &right, &up)

	d := VecDot(trace.PlaneNormal, forward)
	d += 0.5
	if d >= 0 {
		return
	}

	// cut the tangential velocity
	i := VecDot(trace.PlaneNormal, ent.Vars.Velocity)
	into := VecScale(trace.PlaneNormal, i)
	side := VecSub(ent.Vars.Velocity, into)

	ent.Vars.Velocity[0] = side[0] * (1 + d)
	ent.Vars.Velocity[1] = side[1] * (1 + d)
}

func (s *Server) PhysicsWalk(ent *Edict) {
	playerClient := s.playerClient(ent)
	if playerClient != nil {
		s.runClientQCThink(playerClient, "PlayerPreThink")
		if ent.Free {
			return
		}
	}

	if !s.RunThink(ent) {
		return
	}

	flags := uint32(ent.Vars.Flags)
	if flags&FlagOnGround != 0 && !s.CheckBottom(ent) {
		flags &^= FlagOnGround
		ent.Vars.Flags = float32(flags)
		ent.Vars.GroundEntity = 0
	}

	if !s.SV_CheckWater(ent) && flags&FlagWaterJump == 0 && flags&(FlagOnGround|FlagFly|FlagSwim) == 0 {
		s.AddGravity(ent)
	}

	// Player jump processing (Server side fallback/parity)
	if playerClient != nil && (flags&FlagOnGround != 0) && ent.Vars.Button2 != 0 {
		ent.Vars.Flags = float32(flags &^ FlagOnGround)
		ent.Vars.GroundEntity = 0
		ent.Vars.Velocity[2] += 270 // Standard Quake jump speed
	}

	s.CheckVelocity(ent)
	s.SV_CheckStuck(ent)

	if flags&FlagOnGround != 0 {
		move := VecScale(ent.Vars.Velocity, s.FrameTime)
		if move[0] != 0 || move[1] != 0 || move[2] != 0 {
			s.SV_StepMove(ent, move, true)
		}
	} else {
		s.FlyMove(ent, s.FrameTime)
	}

	s.LinkEdict(ent, true)
	s.CheckWaterTransition(ent)
	if playerClient != nil {
		s.runClientQCThink(playerClient, "PlayerPostThink")
	}
}

func (s *Server) SV_CheckStuck(ent *Edict) {
	if s.TestEntityPosition(ent) == nil {
		ent.Vars.OldOrigin = ent.Vars.Origin
		return
	}

	org := ent.Vars.Origin
	ent.Vars.Origin = ent.Vars.OldOrigin
	if s.TestEntityPosition(ent) == nil {
		// Unstuck.
		s.LinkEdict(ent, true)
		return
	}

	for z := float32(0); z < 18; z++ {
		for i := float32(-1); i <= 1; i++ {
			for j := float32(-1); j <= 1; j++ {
				ent.Vars.Origin[0] = org[0] + i
				ent.Vars.Origin[1] = org[1] + j
				ent.Vars.Origin[2] = org[2] + z
				if s.TestEntityPosition(ent) == nil {
					// Unstuck.
					s.LinkEdict(ent, true)
					return
				}
			}
		}
	}

	ent.Vars.Origin = org
	// player is stuck
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

	newVel := ClipVelocity(ent.Vars.Velocity, trace.PlaneNormal, backoff)
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
