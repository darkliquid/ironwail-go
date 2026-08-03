// physics.go implements Quake's per-entity physics simulation, dispatching
// movement and collision for all entity movetypes each server frame.
//
// # Movetypes
//
// Quake entities have a movetype field that determines how they move:
//
//   - MOVETYPE_NONE:     No movement (static entities)
//   - MOVETYPE_WALK:     Standard walking movement with gravity and step-up
//                        (players, monsters). Uses SV_WalkMove.
//   - MOVETYPE_FLY:      Free-flight movement without gravity (spectators)
//   - MOVETYPE_TOSS:     Ballistic trajectory with gravity and bouncing
//                        (gibs, dropped items). Uses SV_FlyMove.
//   - MOVETYPE_PUSH:     Pushed by a moving platform (doors, elevators,
//                        trains). Uses SV_PushMove with riding detection.
//   - MOVETYPE_NOCLIP:   No collision, flies through walls (debug)
//   - MOVETYPE_FLYMISSILE: Like TOSS but no gravity (rockets, nails)
//   - MOVETYPE_BOUNCE:   Like TOSS with coefficient-of-restitution bouncing
//   - MOVETYPE_BOUNCEMISSILE: Like BOUNCE but no gravity
//
// # Collision Detection
//
// All movetypes eventually call SV_FlyMove (for ballistic/free movement)
// or SV_WalkMove (for walking movement), which perform sweep tests against
// the BSP hulls. The trace functions (SV_Trace, SV_MoveTrace) in world.go
// perform the actual recursive hull traversal.
//
// SV_FlyMove uses a 4-iteration bump loop: on each iteration, it traces
// the entity's velocity and slides along any planes it hits. Up to 4
// planes are tracked per frame (max 2 per iteration). This handles
// corner cases like sliding along two walls simultaneously.
//
// # C Lineage
//
// Mirrors SV_Physics, SV_WalkMove, SV_FlyMove, SV_PushMove, and
// SV_PushRotate in sv_phys.c. The C version used global state
// (sv_maxvelocity, sv_gravity); the Go version reads these from
// the server struct.

package server

import (
	"log/slog"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// CheckVelocity ensures entity velocity is within valid bounds.
func (s *Server) CheckVelocity(ent *Edict) {
	vel := ent.Velocity(s)
	orig := ent.Origin(s)
	changedVel := false
	changedOrig := false
	for i := 0; i < 3; i++ {
		if math.IsNaN(float64(vel[i])) {
			vel[i] = 0
			changedVel = true
		}
		if math.IsNaN(float64(orig[i])) {
			orig[i] = 0
			changedOrig = true
		}
		if vel[i] > s.MaxVelocity {
			vel[i] = s.MaxVelocity
			changedVel = true
		} else if vel[i] < -s.MaxVelocity {
			vel[i] = -s.MaxVelocity
			changedVel = true
		}
	}
	if changedVel {
		ent.SetVelocity(s, vel)
	}
	if changedOrig {
		ent.SetOrigin(s, orig)
	}
}

// RunThink executes the entity's think function if its nextthink time has been reached.
func (s *Server) RunThink(ent *Edict) bool {
	thinkTime := ent.NextThink(s)
	if thinkTime <= 0 || thinkTime > s.Time+s.FrameTime {
		return true
	}

	if thinkTime < s.Time {
		thinkTime = s.Time
	}

	ent.OldThinkTime = thinkTime
	ent.OldFrame = ent.Frame(s)
	ent.SetNextThink(s, 0)

	entNum := s.NumForEdict(ent)
	thinkFn := ent.Think(s)
	telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
	if telemetryEnabled {
		s.DebugTelemetry.LogEventf(DebugEventThink, s.QCVM, entNum, ent,
			"runthink begin think_time=%.3f fn=%d", thinkTime, thinkFn)
	}

	s.setQCTimeGlobal(thinkTime)
	s.QCVM.SetGlobal("self", entNum)
	s.QCVM.SetGlobal("other", 0)
	if thinkFn != 0 {
		prevNumEdicts := s.NumEdicts
		if err := s.executeQCFunction(int(thinkFn)); err == nil {
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
	if s == nil || s.QCVM == nil || s.suppressTouchQC {
		return
	}
	ctx := captureQCExecutionContext(s.QCVM)
	defer restoreQCExecutionContext(s.QCVM, ctx)
	e1Num := s.NumForEdict(e1)
	e2Num := s.NumForEdict(e2)
	telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()

	s.setQCTimeGlobal(s.Time)

	e1Touch := e1.Touch(s)
	e1Solid := e1.Solid(s)
	if e1Touch != 0 && e1Solid != float32(SolidNot) {
		prevNumEdicts := s.NumEdicts
		if telemetryEnabled {
			s.DebugTelemetry.LogEventf(DebugEventTouch, s.QCVM, e1Num, e1,
				"impact touch begin other=%d fn=%d", e2Num, e1Touch)
		}
		s.debugTriggerTouch("impact", e1, e2)
		s.QCVM.SetGlobal("self", e1Num)
		s.QCVM.SetGlobal("other", e2Num)
		if err := s.executeQCFunction(int(e1Touch)); err == nil {
			s.syncSpawnedEdictsFromQCVM(prevNumEdicts)
		}
		if telemetryEnabled {
			s.DebugTelemetry.LogEventf(DebugEventTouch, s.QCVM, e1Num, e1,
				"impact touch end other=%d fn=%d", e2Num, e1Touch)
		}
	}

	e2Touch := e2.Touch(s)
	e2Solid := e2.Solid(s)
	if e2Touch != 0 && e2Solid != float32(SolidNot) {
		prevNumEdicts := s.NumEdicts
		if telemetryEnabled {
			s.DebugTelemetry.LogEventf(DebugEventTouch, s.QCVM, e2Num, e2,
				"impact touch begin other=%d fn=%d", e1Num, e2Touch)
		}
		s.debugTriggerTouch("impact", e2, e1)
		s.QCVM.SetGlobal("self", e2Num)
		s.QCVM.SetGlobal("other", e1Num)
		if err := s.executeQCFunction(int(e2Touch)); err == nil {
			s.syncSpawnedEdictsFromQCVM(prevNumEdicts)
		}
		if telemetryEnabled {
			s.DebugTelemetry.LogEventf(DebugEventTouch, s.QCVM, e2Num, e2,
				"impact touch end other=%d fn=%d", e2Num, e2Touch)
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
	vel := ent.Velocity(s)
	vel[2] -= entGravity * s.Gravity * s.FrameTime
	ent.SetVelocity(s, vel)
}

func (s *Server) SV_CheckWater(ent *Edict) bool {
	orig := ent.Origin(s)
	mins := ent.Mins(s)
	maxs := ent.Maxs(s)
	viewOfs := ent.ViewOfs(s)

	var point [3]float32
	point[0] = orig[0]
	point[1] = orig[1]
	point[2] = orig[2] + mins[2] + 1

	ent.SetWaterLevel(s, 0)
	ent.SetWaterType(s, float32(bsp.ContentsEmpty))
	cont := s.PointContents(point)
	if cont <= bsp.ContentsWater {
		ent.SetWaterType(s, float32(cont))
		ent.SetWaterLevel(s, 1)
		point[2] = orig[2] + (mins[2]+maxs[2])*0.5
		cont = s.PointContents(point)
		if cont <= bsp.ContentsWater {
			ent.SetWaterLevel(s, 2)
			point[2] = orig[2] + viewOfs[2]
			cont = s.PointContents(point)
			if cont <= bsp.ContentsWater {
				ent.SetWaterLevel(s, 3)
			}
		}
	}

	return ent.WaterLevel(s) > 1
}

func (s *Server) CheckWaterTransition(ent *Edict) {
	orig := ent.Origin(s)
	wtype := ent.WaterType(s)
	cont := s.PointContents(orig)

	if wtype == 0 { // just spawned here
		ent.SetWaterType(s, float32(cont))
		ent.SetWaterLevel(s, 1)
		return
	}

	if cont <= bsp.ContentsWater {
		if wtype == float32(bsp.ContentsEmpty) {
			// just crossed into water
			s.StartSound(ent, 0, "misc/h2ohit1.wav", 255, 1)
		}
		ent.SetWaterType(s, float32(cont))
		ent.SetWaterLevel(s, 1)
	} else {
		if wtype != float32(bsp.ContentsEmpty) {
			// just crossed out of water
			s.StartSound(ent, 0, "misc/h2ohit1.wav", 255, 1)
		}
		ent.SetWaterType(s, float32(bsp.ContentsEmpty))
		ent.SetWaterLevel(s, float32(cont))
	}
}

func (s *Server) FlyMove(ent *Edict, time float32, steptrace *TraceResult) int {
	blocked := 0
	entVel := ent.Velocity(s)
	entOrig := ent.Origin(s)
	entMins := ent.Mins(s)
	entMaxs := ent.Maxs(s)
	originalVelocity := entVel
	primalVelocity := entVel
	numPlanes := 0
	var planes [maxClipPlanes][3]float32
	timeLeft := time

	for bumpCount := 0; bumpCount < 4; bumpCount++ {
		if entVel[0] == 0 && entVel[1] == 0 && entVel[2] == 0 {
			break
		}

		end := VecAdd(entOrig, VecScale(entVel, timeLeft))
		trace := s.Move(entOrig, entMins, entMaxs, end, MoveNormal, ent)

		if trace.AllSolid {
			ent.SetVelocity(s, [3]float32{})
			return 3
		}

		if trace.Fraction > 0 {
			entOrig = trace.EndPos
			ent.SetOrigin(s, entOrig)
			originalVelocity = entVel
			numPlanes = 0
		}

		if trace.Fraction == 1 {
			break
		}

		if trace.PlaneNormal[2] > 0.7 {
			blocked |= 1
			if trace.Entity != nil && int(trace.Entity.Solid(s)) == int(SolidBSP) {
				ent.SetFlags(s, float32(uint32(ent.Flags(s))|FlagOnGround))
				ent.SetGroundEntity(s, int32(s.NumForEdict(trace.Entity)))
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
			ent.SetVelocity(s, [3]float32{})
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
			entVel = newVelocity
			ent.SetVelocity(s, entVel)
		} else {
			// go along the crease
			if numPlanes != 2 {
				ent.SetVelocity(s, [3]float32{})
				return 7
			}
			dir := VecCross(planes[0], planes[1])
			d := VecDot(dir, entVel)
			entVel = VecScale(dir, d)
			ent.SetVelocity(s, entVel)
		}
		if VecDot(entVel, primalVelocity) <= 0 {
			ent.SetVelocity(s, [3]float32{})
			return blocked
		}
	}

	return blocked
}

func (s *Server) PushEntity(ent *Edict, push [3]float32) TraceResult {
	orig := ent.Origin(s)
	mins := ent.Mins(s)
	maxs := ent.Maxs(s)
	mt := ent.MoveType(s)
	solid := ent.Solid(s)

	end := [3]float32{
		orig[0] + push[0],
		orig[1] + push[1],
		orig[2] + push[2],
	}

	moveType := MoveNormal
	if MoveType(mt) == MoveTypeFlyMissile {
		moveType = MoveMissile
	} else if int(solid) == int(SolidTrigger) || int(solid) == int(SolidNot) {
		moveType = MoveNoMonsters
	}

	trace := s.Move(orig, mins, maxs, end, MoveType(moveType), ent)
	ent.SetOrigin(s, trace.EndPos)
	s.LinkEdict(ent, true)

	if trace.Entity != nil {
		s.Impact(ent, trace.Entity)
	}

	return trace
}

func (s *Server) PushMove(pusher *Edict, movetime float32) {
	pusherNum := s.NumForEdict(pusher)
	pusherVel := pusher.Velocity(s)
	pusherLTime := pusher.LTime(s)
	pusherAbsMin := pusher.AbsMin(s)
	pusherAbsMax := pusher.AbsMax(s)
	pusherOrig := pusher.Origin(s)

	if pusherVel[0] == 0 && pusherVel[1] == 0 && pusherVel[2] == 0 {
		SvdbgPushLogf("pusher=%d velocity=0 — early return (ltime += %.4f), no entities pushed", pusherNum, movetime)
		pusher.SetLTime(s, pusherLTime+movetime)
		return
	}

	move := [3]float32{
		pusherVel[0] * movetime,
		pusherVel[1] * movetime,
		pusherVel[2] * movetime,
	}
	mins := [3]float32{
		pusherAbsMin[0] + move[0],
		pusherAbsMin[1] + move[1],
		pusherAbsMin[2] + move[2],
	}
	maxs := [3]float32{
		pusherAbsMax[0] + move[0],
		pusherAbsMax[1] + move[1],
		pusherAbsMax[2] + move[2],
	}

	pushorig := pusherOrig
	pusher.SetOrigin(s, VecAdd(pusherOrig, move))
	pusher.SetLTime(s, pusherLTime+movetime)
	s.LinkEdict(pusher, false)

	movedEdicts := make([]*Edict, 0, s.NumEdicts)
	movedFrom := make([][3]float32, 0, s.NumEdicts)

	for e := 1; e < s.NumEdicts; e++ {
		check := s.Edicts[e]
		if check == nil || check.Free {
			continue
		}

		checkMoveType := check.MoveType(s)
		movemask := 1 << int(checkMoveType)
		if movemask&((1<<int(MoveTypePush))|(1<<int(MoveTypeNone))|(1<<int(MoveTypeNoClip))) != 0 {
			continue
		}

		checkFlags := check.Flags(s)
		checkGroundEnt := check.GroundEntity(s)
		checkAbsMin := check.AbsMin(s)
		checkAbsMax := check.AbsMax(s)

		riding := (uint32(checkFlags)&FlagOnGround) != 0 && s.EdictNum(int(checkGroundEnt)) == pusher
		if !riding {
			if checkAbsMin[0] >= maxs[0] ||
				checkAbsMin[1] >= maxs[1] ||
				checkAbsMin[2] >= maxs[2] ||
				checkAbsMax[0] <= mins[0] ||
				checkAbsMax[1] <= mins[1] ||
				checkAbsMax[2] <= mins[2] {
				SvdbgPushLogfAt(2, "pusher=%d check=%d riding=false aabb_miss=true — skipped", pusherNum, e)
				continue
			}

			if s.TestEntityPosition(check) == nil {
				SvdbgPushLogfAt(2, "pusher=%d check=%d riding=false aabb_hit=true testpos=nil — skipped (not stuck)", pusherNum, e)
				continue
			}
			SvdbgPushLogf("pusher=%d check=%d riding=false aabb_hit=true testpos=stuck — pushing (overlapping non-rider)", pusherNum, e)
		} else {
			SvdbgPushLogf("pusher=%d check=%d riding=true — pushing (groundentity=%d onground=1)", pusherNum, e, int(checkGroundEnt))
		}

		if MoveType(checkMoveType) != MoveTypeWalk {
			check.SetFlags(s, float32(uint32(checkFlags)&^FlagOnGround))
		}

		entorig := check.Origin(s)
		movedEdicts = append(movedEdicts, check)
		movedFrom = append(movedFrom, entorig)

		pusherSolid := pusher.Solid(s)
		if int(pusherSolid) == int(SolidBSP) || int(pusherSolid) == int(SolidBBox) || int(pusherSolid) == int(SolidSlideBox) {
			pusher.SetSolid(s, float32(SolidNot))
			s.PushEntity(check, move)
			pusher.SetSolid(s, pusherSolid)
		} else {
			s.PushEntity(check, move)
		}

		// sv_debug_push: log post-push position so we can see where the player
		// ended up after being pushed by the lift, and whether they're blocked.
		postPushOrig := check.Origin(s)
		postPushAbsMin := check.AbsMin(s)
		postPushAbsMax := check.AbsMax(s)
		SvdbgPushLogfAt(2, "pusher=%d check=%d post-push origin=(%.1f %.1f %.1f) absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f)",
			pusherNum, e,
			postPushOrig[0], postPushOrig[1], postPushOrig[2],
			postPushAbsMin[0], postPushAbsMin[1], postPushAbsMin[2],
			postPushAbsMax[0], postPushAbsMax[1], postPushAbsMax[2])

		block := s.TestEntityPosition(check)
		if block == nil {
			SvdbgPushLogfAt(2, "pusher=%d check=%d post-push testpos=nil (not blocked)", pusherNum, e)
			continue
		}

		checkMins := check.Mins(s)
		checkMaxs := check.Maxs(s)
		if checkMins[0] == checkMaxs[0] {
			continue
		}

		checkSolid := check.Solid(s)
		if int(checkSolid) == int(SolidNot) || int(checkSolid) == int(SolidTrigger) {
			check.SetMins(s, [3]float32{})
			check.SetMaxs(s, [3]float32{})
			continue
		}

		// Elevator gameplay fix: if entity is riding the pusher and blocked
		// by it, try nudging upward by DIST_EPSILON to prevent crushing.
		// Matches C sv_phys.c:552-578 (sv_gameplayfix_elevators).
		fixLevel := s.CVar.FloatValue("sv_gameplayfix_elevators")
		if riding && block == pusher &&
			(fixLevel >= 2 || (fixLevel > 0 && e <= s.MaxClients())) {
			postPushOrig[2] += DistEpsilon
			check.SetOrigin(s, postPushOrig)
			if s.TestEntityPosition(check) == nil {
				slog.Debug("elevator fix nudged entity",
					"entity", e, "pusher", s.NumForEdict(pusher))
				continue
			}
		}

		check.SetOrigin(s, entorig)
		s.LinkEdict(check, true)

		pusher.SetOrigin(s, pushorig)
		s.LinkEdict(pusher, false)
		pusher.SetLTime(s, pusher.LTime(s)-movetime)

		pusherBlocked := pusher.Blocked(s)
		if pusherBlocked != 0 && s.QCVM != nil {
			pusherNum := s.NumForEdict(pusher)
			checkNum := s.NumForEdict(check)
			ctx := captureQCExecutionContext(s.QCVM)
			telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventBlocked, s.QCVM, pusherNum, pusher,
					"pushmove blocked by=%d callback=%d movetime=%.3f", checkNum, pusherBlocked, movetime)
			}
			prevNumEdicts := s.NumEdicts
			s.QCVM.SetGlobal("self", pusherNum)
			s.QCVM.SetGlobal("other", checkNum)
			if err := s.executeQCFunction(int(pusherBlocked)); err == nil {
				s.syncSpawnedEdictsFromQCVM(prevNumEdicts)
			}
			restoreQCExecutionContext(s.QCVM, ctx)
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventBlocked, s.QCVM, pusherNum, pusher,
					"pushmove blocked callback done by=%d callback=%d", checkNum, pusherBlocked)
			}
		}

		for i, moved := range movedEdicts {
			moved.SetOrigin(s, movedFrom[i])
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

	angles := ent.Angles(s)
	avel := ent.AVelocity(s)
	orig := ent.Origin(s)
	vel := ent.Velocity(s)

	for i := 0; i < 3; i++ {
		angles[i] += avel[i] * s.FrameTime
		orig[i] += vel[i] * s.FrameTime
	}

	ent.SetAngles(s, angles)
	ent.SetOrigin(s, orig)
	s.LinkEdict(ent, false)
}

func (s *Server) PhysicsPusher(ent *Edict) {
	entNum := s.NumForEdict(ent)
	telemetryEnabled := s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
	oldLTime := ent.LTime(s)
	thinkTime := ent.NextThink(s)
	movetime := s.FrameTime

	if thinkTime < oldLTime+s.FrameTime {
		movetime = thinkTime - oldLTime
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

	thinkTime = ent.NextThink(s)
	thinkFn := ent.Think(s)
	curLTime := ent.LTime(s)
	if thinkTime > oldLTime && thinkTime <= curLTime {
		ent.SetNextThink(s, 0)
		if s.QCVM != nil && thinkFn != 0 {
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventThink, s.QCVM, entNum, ent,
					"physicspusher think begin fn=%d", thinkFn)
			}
			s.setQCTimeGlobal(s.Time)
			s.QCVM.SetGlobal("self", entNum)
			s.QCVM.SetGlobal("other", 0)
			prevNumEdicts := s.NumEdicts
			if err := s.executeQCFunction(int(thinkFn)); err == nil {
				s.syncSpawnedEdictsFromQCVM(prevNumEdicts)
			}
			if telemetryEnabled {
				s.DebugTelemetry.LogEventf(DebugEventThink, s.QCVM, entNum, ent,
					"physicspusher think end fn=%d freed=%t", thinkFn, ent.Free)
			}
		}
	}
}

func (s *Server) PhysicsStep(ent *Edict) {
	flags := uint32(ent.Flags(s))
	if flags&(FlagOnGround|FlagFly|FlagSwim) == 0 {
		vel := ent.Velocity(s)
		hitSound := vel[2] < -0.1*s.Gravity
		s.AddGravity(ent)
		s.CheckVelocity(ent)
		s.FlyMove(ent, s.FrameTime, nil)
		s.LinkEdict(ent, true)
		if uint32(ent.Flags(s))&FlagOnGround != 0 && hitSound {
			s.StartSound(ent, 0, "demon/dland2.wav", 255, 1)
		}
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
		mt := MoveType(ent.MoveType(s))
		if mt == MoveTypePush || mt == MoveTypeNone || mt == MoveTypeNoClip {
			continue
		}

		if s.TestEntityPosition(ent) != nil {
			// Mirrors SV_CheckAllEnts in the C reference: the original
			// emits a Con_Printf warning; we surface it through slog.
			slog.Debug("sv_checkallents: entity in invalid position",
				"index", i,
				"classname", ent.ClassName(s))
		}
	}
}

func (s *Server) SV_TryUnstick(ent *Edict, oldVel [3]float32) int {
	oldOrg := ent.Origin(s)
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
		ent.SetVelocity(s, [3]float32{oldVel[0], oldVel[1], 0})
		blocked := s.FlyMove(ent, 0.1, nil)

		curOrg := ent.Origin(s)
		if math.Abs(float64(oldOrg[0]-curOrg[0])) > 4 ||
			math.Abs(float64(oldOrg[1]-curOrg[1])) > 4 {
			return blocked
		}

		// go back to the original pos and try again
		ent.SetOrigin(s, oldOrg)
	}

	ent.SetVelocity(s, [3]float32{})
	return 7 // still not moving
}

func walkMoveNeedsUnstick(oldOrg, newOrg [3]float32) bool {
	return math.Abs(float64(oldOrg[0]-newOrg[0])) < DistEpsilon &&
		math.Abs(float64(oldOrg[1]-newOrg[1])) < DistEpsilon
}

func (s *Server) SV_WalkMove(ent *Edict) {
	flags := uint32(ent.Flags(s))
	oldonground := flags & FlagOnGround
	ent.SetFlags(s, float32(flags&^FlagOnGround))

	oldorg := ent.Origin(s)
	oldvel := ent.Velocity(s)

	var steptrace TraceResult
	clip := s.FlyMove(ent, s.FrameTime, &steptrace)

	if clip&2 == 0 {
		return // move didn't block on a step
	}

	if oldonground == 0 && ent.WaterLevel(s) == 0 {
		return // don't stair up while jumping
	}

	if MoveType(ent.MoveType(s)) != MoveTypeWalk {
		return // gibbed by a trigger
	}

	if s.CVar.BoolValue("sv_nostep") {
		return
	}

	if uint32(ent.Flags(s))&FlagWaterJump != 0 {
		return
	}

	nosteporg := ent.Origin(s)
	nostepvel := ent.Velocity(s)

	// back to start pos
	ent.SetOrigin(s, oldorg)

	// step up
	upmove := [3]float32{0, 0, 18}
	s.PushEntity(ent, upmove)

	// move forward with zeroed Z velocity
	ent.SetVelocity(s, [3]float32{oldvel[0], oldvel[1], 0})
	clip = s.FlyMove(ent, s.FrameTime, &steptrace)

	// check for stuckness
	if clip != 0 {
		if walkMoveNeedsUnstick(oldorg, ent.Origin(s)) {
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
		if downtrace.Entity != nil && int(downtrace.Entity.Solid(s)) == int(SolidBSP) {
			ent.SetFlags(s, float32(uint32(ent.Flags(s))|FlagOnGround))
			ent.SetGroundEntity(s, int32(s.NumForEdict(downtrace.Entity)))
		}
	} else {
		// if the push down didn't end up on good ground, use the move without the step up
		ent.SetOrigin(s, nosteporg)
		ent.SetVelocity(s, nostepvel)
	}
}

func (s *Server) SV_WallFriction(ent *Edict, trace *TraceResult) {
	var forward, right, up [3]float32
	AngleVectors(ent.VAngle(s), &forward, &right, &up)

	d := VecDot(trace.PlaneNormal, forward)
	d += 0.5
	if d >= 0 {
		return
	}

	// cut the tangential velocity
	vel := ent.Velocity(s)
	i := VecDot(trace.PlaneNormal, vel)
	into := VecScale(trace.PlaneNormal, i)
	side := VecSub(vel, into)

	vel[0] = side[0] * (1 + d)
	vel[1] = side[1] * (1 + d)
	ent.SetVelocity(s, vel)
}

func (s *Server) PhysicsWalk(ent *Edict) {
	playerClient := s.playerClient(ent)
	wasUnderwater := ent.WaterLevel(s) >= 3
	if playerClient != nil {
		s.runClientQCThinkWithMode(playerClient, "PlayerPreThink", false)
		if ent.Free {
			return
		}
	}

	s.CheckVelocity(ent)

	if !s.RunThink(ent) {
		return
	}

	if !s.SV_CheckWater(ent) && uint32(ent.Flags(s))&FlagWaterJump == 0 {
		s.AddGravity(ent)
	}

	s.SV_CheckStuck(ent)
	s.SV_WalkMove(ent)

	s.LinkEdict(ent, true)
	if playerClient != nil {
		s.runClientQCThinkWithMode(playerClient, "PlayerPostThink", false)
		if ent.Free {
			return
		}
		forceUnderwater := !wasUnderwater && ent.WaterLevel(s) >= 3
		if forceUnderwater != ent.ForceWater {
			ent.ForceWater = forceUnderwater
			ent.SendForceWater = true
		}
	}
}

func (s *Server) SV_CheckStuck(ent *Edict) {
	orig := ent.Origin(s)
	if s.TestEntityPosition(ent) == nil {
		ent.SetOldOrigin(s, orig)
		return
	}

	oldOrg := ent.OldOrigin(s)
	ent.SetOrigin(s, oldOrg)
	if s.TestEntityPosition(ent) == nil {
		// Unstuck.
		s.LinkEdict(ent, true)
		return
	}

	for z := float32(0); z < 18; z++ {
		for i := float32(-1); i <= 1; i++ {
			for j := float32(-1); j <= 1; j++ {
				cand := [3]float32{orig[0] + i, orig[1] + j, orig[2] + z}
				ent.SetOrigin(s, cand)
				if s.TestEntityPosition(ent) == nil {
					// Unstuck.
					s.LinkEdict(ent, true)
					return
				}
			}
		}
	}

	ent.SetOrigin(s, orig)
	// player is stuck
}

func (s *Server) PhysicsToss(ent *Edict) {
	if !s.RunThink(ent) {
		return
	}

	flags := uint32(ent.Flags(s))
	if flags&FlagOnGround != 0 {
		return
	}

	s.CheckVelocity(ent)

	mt := MoveType(ent.MoveType(s))
	if mt != MoveTypeFly && mt != MoveTypeFlyMissile {
		s.AddGravity(ent)
	}

	angles := ent.Angles(s)
	avel := ent.AVelocity(s)
	for i := 0; i < 3; i++ {
		angles[i] += avel[i] * s.FrameTime
	}
	ent.SetAngles(s, angles)

	vel := ent.Velocity(s)
	move := VecScale(vel, s.FrameTime)
	trace := s.PushEntity(ent, move)
	if trace.Fraction == 1 || ent.Free {
		return
	}

	backoff := float32(1)
	if mt == MoveTypeBounce {
		backoff = 1.5
	}

	newVel := ClipVelocity(vel, trace.PlaneNormal, backoff)
	ent.SetVelocity(s, newVel)

	if trace.PlaneNormal[2] > 0.7 {
		if newVel[2] < 60 || mt != MoveTypeBounce {
			ent.SetFlags(s, float32(uint32(ent.Flags(s))|FlagOnGround))
			ent.SetGroundEntity(s, int32(s.NumForEdict(trace.Entity)))
			ent.SetVelocity(s, [3]float32{})
			ent.SetAVelocity(s, [3]float32{})
		}
	}

	s.CheckWaterTransition(ent)
}
