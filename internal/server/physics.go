package server

import (
	"log/slog"
	"math"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/cvar"
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

	entNum := s.NumForEdict(ent)
	telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
	if telemetryEnabled {
		s.DebugTelemetry.LogEventf(DebugEventThink, s.QCVM, entNum, ent,
			"runthink begin think_time=%.3f fn=%d", thinkTime, ent.Vars.Think)
	}

	s.setQCTimeGlobal(thinkTime)
	s.QCVM.SetGlobal("self", entNum)
	s.QCVM.SetGlobal("other", 0)
	if ent.Vars.Think != 0 {
		prevNumEdicts := s.NumEdicts
		syncEdictToQCVM(s.QCVM, entNum, ent)
		if err := s.executeQCFunction(int(ent.Vars.Think)); err == nil {
			syncEdictFromQCVM(s.QCVM, entNum, ent)
			s.syncSpawnedEdictsFromQCVM(prevNumEdicts)
		}
	}
	if telemetryEnabled {
		s.DebugTelemetry.LogEventf(DebugEventThink, s.QCVM, entNum, ent,
			"runthink end think_time=%.3f freed=%t", thinkTime, ent.Free)
	}

	return !ent.Free
}

// Impact runs touch functions for two entities that have collided.
func (s *Server) Impact(e1, e2 *Edict) {
	ctx := captureQCExecutionContext(s.QCVM)
	defer restoreQCExecutionContext(s.QCVM, ctx)
	e1Num := s.NumForEdict(e1)
	e2Num := s.NumForEdict(e2)
	telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()

	s.setQCTimeGlobal(s.Time)

	if e1.Vars.Touch != 0 && e1.Vars.Solid != float32(SolidNot) {
		prevNumEdicts := s.NumEdicts
		if telemetryEnabled {
			s.DebugTelemetry.LogEventf(DebugEventTouch, s.QCVM, e1Num, e1,
				"impact touch begin other=%d fn=%d", e2Num, e1.Vars.Touch)
		}
		syncEdictToQCVM(s.QCVM, e1Num, e1)
		syncEdictToQCVM(s.QCVM, e2Num, e2)
		s.QCVM.SetGlobal("self", e1Num)
		s.QCVM.SetGlobal("other", e2Num)
		if err := s.executeQCFunction(int(e1.Vars.Touch)); err == nil {
			syncEdictFromQCVM(s.QCVM, e1Num, e1)
			syncEdictFromQCVM(s.QCVM, e2Num, e2)
			s.syncSpawnedEdictsFromQCVM(prevNumEdicts)
		}
		if telemetryEnabled {
			s.DebugTelemetry.LogEventf(DebugEventTouch, s.QCVM, e1Num, e1,
				"impact touch end other=%d fn=%d", e2Num, e1.Vars.Touch)
		}
	}

	if e2.Vars.Touch != 0 && e2.Vars.Solid != float32(SolidNot) {
		prevNumEdicts := s.NumEdicts
		if telemetryEnabled {
			s.DebugTelemetry.LogEventf(DebugEventTouch, s.QCVM, e2Num, e2,
				"impact touch begin other=%d fn=%d", e1Num, e2.Vars.Touch)
		}
		syncEdictToQCVM(s.QCVM, e2Num, e2)
		syncEdictToQCVM(s.QCVM, e1Num, e1)
		s.QCVM.SetGlobal("self", e2Num)
		s.QCVM.SetGlobal("other", e1Num)
		if err := s.executeQCFunction(int(e2.Vars.Touch)); err == nil {
			syncEdictFromQCVM(s.QCVM, e2Num, e2)
			syncEdictFromQCVM(s.QCVM, e1Num, e1)
			s.syncSpawnedEdictsFromQCVM(prevNumEdicts)
		}
		if telemetryEnabled {
			s.DebugTelemetry.LogEventf(DebugEventTouch, s.QCVM, e2Num, e2,
				"impact touch end other=%d fn=%d", e1Num, e2.Vars.Touch)
		}
	}

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
	// Check for per-entity gravity multiplier (used by mods for flying
	// monsters, low-gravity areas, etc). Matches C GetEdictFieldValueByName.
	if s.QCFieldGravity >= 0 && s.QCVM != nil {
		if g := s.QCVM.EFloat(s.NumForEdict(ent), s.QCFieldGravity); g != 0 {
			entGravity = g
		}
	}
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

func (s *Server) FlyMove(ent *Edict, time float32, steptrace *TraceResult) int {
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
			if steptrace != nil {
				*steptrace = trace
			}
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

		// modify original_velocity so it parallels all of the clip planes
		var newVelocity [3]float32
		i := 0
		for i = 0; i < numPlanes; i++ {
			newVelocity = ClipVelocity(originalVelocity, planes[i], 1)
			j := 0
			for j = 0; j < numPlanes; j++ {
				if j != i {
					if VecDot(newVelocity, planes[j]) < 0 {
						break
					}
				}
			}
			if j == numPlanes {
				break
			}
		}

		if i != numPlanes {
			// go along this plane
			ent.Vars.Velocity = newVelocity
		} else {
			// go along the crease
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

		// Elevator gameplay fix: if entity is riding the pusher and blocked
		// by it, try nudging upward by DIST_EPSILON to prevent crushing.
		// Matches C sv_phys.c:552-578 (sv_gameplayfix_elevators).
		fixLevel := cvar.FloatValue("sv_gameplayfix_elevators")
		if riding && block == pusher &&
			(fixLevel >= 2 || (fixLevel > 0 && e <= s.GetMaxClients())) {
			check.Vars.Origin[2] += DistEpsilon
			if s.TestEntityPosition(check) == nil {
				slog.Debug("elevator fix nudged entity",
					"entity", e, "pusher", s.NumForEdict(pusher))
				continue
			}
		}

		check.Vars.Origin = entorig
		s.LinkEdict(check, true)

		pusher.Vars.Origin = pushorig
		s.LinkEdict(pusher, false)
		pusher.Vars.LTime -= movetime

		if pusher.Vars.Blocked != 0 && s.QCVM != nil {
			pusherNum := s.NumForEdict(pusher)
			checkNum := s.NumForEdict(check)
			ctx := captureQCExecutionContext(s.QCVM)
			telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventBlocked, s.QCVM, pusherNum, pusher,
					"pushmove blocked by=%d callback=%d movetime=%.3f", checkNum, pusher.Vars.Blocked, movetime)
			}
			s.QCVM.SetGlobal("self", pusherNum)
			s.QCVM.SetGlobal("other", checkNum)
			_ = s.executeQCFunction(int(pusher.Vars.Blocked))
			restoreQCExecutionContext(s.QCVM, ctx)
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventBlocked, s.QCVM, pusherNum, pusher,
					"pushmove blocked callback done by=%d callback=%d", checkNum, pusher.Vars.Blocked)
			}
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
	entNum := s.NumForEdict(ent)
	telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
	oldLTime := ent.Vars.LTime
	thinkTime := ent.Vars.NextThink
	movetime := s.FrameTime

	if thinkTime < ent.Vars.LTime+s.FrameTime {
		movetime = thinkTime - ent.Vars.LTime
		if movetime < 0 {
			movetime = 0
		}
	}
	if telemetryEnabled {
		s.DebugTelemetry.LogEventf(DebugEventPhysics, s.QCVM, entNum, ent,
			"physicspusher movetime=%.3f think_time=%.3f ltime=%.3f", movetime, thinkTime, oldLTime)
	}

	if movetime != 0 {
		s.PushMove(ent, movetime)
	}

	if thinkTime > oldLTime && thinkTime <= ent.Vars.LTime {
		ent.Vars.NextThink = 0
		if s.QCVM != nil && ent.Vars.Think != 0 {
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventThink, s.QCVM, entNum, ent,
					"physicspusher think begin fn=%d", ent.Vars.Think)
			}
			s.setQCTimeGlobal(s.Time)
			s.QCVM.SetGlobal("self", entNum)
			s.QCVM.SetGlobal("other", 0)
			prevNumEdicts := s.NumEdicts
			s.syncPushersToQCVM()
			if err := s.executeQCFunction(int(ent.Vars.Think)); err == nil {
				s.syncPushersFromQCVM()
				s.syncSpawnedEdictsFromQCVM(prevNumEdicts)
			}
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventThink, s.QCVM, entNum, ent,
					"physicspusher think end fn=%d freed=%t", ent.Vars.Think, ent.Free)
			}
		}
	}
}

func (s *Server) PhysicsStep(ent *Edict) {
	flags := uint32(ent.Vars.Flags)
	if flags&(FlagOnGround|FlagFly|FlagSwim) == 0 {
		s.AddGravity(ent)
		s.CheckVelocity(ent)
		s.FlyMove(ent, s.FrameTime, nil)
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
		blocked := s.FlyMove(ent, 0.1, nil)

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

func (s *Server) SV_WalkMove(ent *Edict) {
	oldonground := uint32(ent.Vars.Flags) & FlagOnGround
	ent.Vars.Flags = float32(uint32(ent.Vars.Flags) &^ FlagOnGround)

	oldorg := ent.Vars.Origin
	oldvel := ent.Vars.Velocity

	var steptrace TraceResult
	clip := s.FlyMove(ent, s.FrameTime, &steptrace)

	if clip&2 == 0 {
		return // move didn't block on a step
	}

	if oldonground == 0 && ent.Vars.WaterLevel == 0 {
		return // don't stair up while jumping
	}

	if MoveType(ent.Vars.MoveType) != MoveTypeWalk {
		return // gibbed by a trigger
	}

	if uint32(ent.Vars.Flags)&FlagWaterJump != 0 {
		return
	}

	nosteporg := ent.Vars.Origin
	nostepvel := ent.Vars.Velocity

	// back to start pos
	ent.Vars.Origin = oldorg

	// step up
	upmove := [3]float32{0, 0, 18}
	s.PushEntity(ent, upmove)

	// move forward with zeroed Z velocity
	ent.Vars.Velocity[0] = oldvel[0]
	ent.Vars.Velocity[1] = oldvel[1]
	ent.Vars.Velocity[2] = 0
	clip = s.FlyMove(ent, s.FrameTime, &steptrace)

	// check for stuckness
	if clip != 0 {
		if math.Abs(float64(oldorg[0]-ent.Vars.Origin[0])) < 0.03125 &&
			math.Abs(float64(oldorg[1]-ent.Vars.Origin[1])) < 0.03125 {
			clip = s.SV_TryUnstick(ent, oldvel)
		}
	}

	// extra friction based on view angle
	if clip&2 != 0 {
		s.SV_WallFriction(ent, &steptrace)
	}

	// move down
	downmove := [3]float32{0, 0, -18 + oldvel[2]*s.FrameTime}
	downtrace := s.PushEntity(ent, downmove)

	if downtrace.PlaneNormal[2] > 0.7 {
		if downtrace.Entity != nil && int(downtrace.Entity.Vars.Solid) == int(SolidBSP) {
			ent.Vars.Flags = float32(uint32(ent.Vars.Flags) | FlagOnGround)
			ent.Vars.GroundEntity = int32(s.NumForEdict(downtrace.Entity))
		}
	} else {
		// if the push down didn't end up on good ground, use the move without the step up
		ent.Vars.Origin = nosteporg
		ent.Vars.Velocity = nostepvel
	}
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
	wasUnderwater := ent.Vars.WaterLevel >= 3
	if playerClient != nil {
		s.runClientQCThink(playerClient, "PlayerPreThink")
		if ent.Free {
			return
		}
	}

	s.CheckVelocity(ent)

	if !s.RunThink(ent) {
		return
	}

	if !s.SV_CheckWater(ent) && uint32(ent.Vars.Flags)&FlagWaterJump == 0 {
		s.AddGravity(ent)
	}

	s.SV_CheckStuck(ent)
	s.SV_WalkMove(ent)

	s.LinkEdict(ent, true)
	if playerClient != nil {
		s.runClientQCThink(playerClient, "PlayerPostThink")
		if ent.Free {
			return
		}
		forceUnderwater := !wasUnderwater && ent.Vars.WaterLevel >= 3
		if forceUnderwater != ent.ForceWater {
			ent.ForceWater = forceUnderwater
			ent.SendForceWater = true
		}
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
