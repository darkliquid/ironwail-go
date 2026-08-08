// leafs.go implements the per-entity physics leaf algorithms (FlyMove,
// PushMove, PhysicsWalk, PhysicsToss, Impact, SV_WalkMove, ...) as System
// methods driven by injected dependencies. Mirrors SV_Physics,
// SV_WalkMove, SV_FlyMove, SV_PushMove, and SV_PushRotate in sv_phys.c.
//
// The leafs previously lived as *Server methods in server_physics.go and
// server_physics_walk.go; they now run against the injected CollisionWorld,
// EntityStore, ServerHandle, and PhysicsFacade so they can be unit-tested in
// isolation.
package physics

import (
	"log/slog"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	srvdebug "github.com/darkliquid/ironwail-go/internal/server/debug"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

const maxClipPlanes = 5

// ============================================================================
// Velocity / gravity / water
// ============================================================================

// CheckVelocity clamps an entity's velocity components to MaxVelocity bounds.
func (s *System) CheckVelocity(ent *srvtypes.Edict) {
	checkVelocity(s.facade, ent, s.sh)
}

func checkVelocity(cfg srvtypes.PhysicsConfig, ent *srvtypes.Edict, sh srvtypes.ServerHandle) {
	vel := ent.Velocity(sh)
	orig := ent.Origin(sh)
	changedVel := false
	changedOrig := false
	maxVel := cfg.GetMaxVelocity()
	for i := 0; i < 3; i++ {
		if math.IsNaN(float64(vel[i])) {
			// C prints "Got a NaN velocity on %s" before zeroing — surface the
			// same warning instead of silently fixing. Where in C:
			// SV_CheckVelocity sv_phys.c:87-110.
			slog.Warn("Got a NaN velocity", "entity", ent.Num, "classname", ent.ClassName(sh))
			vel[i] = 0
			changedVel = true
		}
		if math.IsNaN(float64(orig[i])) {
			slog.Warn("Got a NaN origin", "entity", ent.Num, "classname", ent.ClassName(sh))
			orig[i] = 0
			changedOrig = true
		}
		if vel[i] > maxVel {
			vel[i] = maxVel
			changedVel = true
		} else if vel[i] < -maxVel {
			vel[i] = -maxVel
			changedVel = true
		}
	}
	if changedVel {
		ent.SetVelocity(sh, vel)
	}
	if changedOrig {
		ent.SetOrigin(sh, orig)
	}
}

// AddGravity applies frame gravity acceleration to an entity's Z velocity.
func (s *System) AddGravity(ent *srvtypes.Edict) {
	addGravity(s.facade, s.facade, ent, s.sh)
}

func addGravity(cfg srvtypes.PhysicsConfig, timing srvtypes.FrameTiming, ent *srvtypes.Edict, sh srvtypes.ServerHandle) {
	entGravity := float32(1)
	if fg := fieldGravity(sh, ent); fg >= 0 {
		if vm := sh.GetVM(); vm != nil && fg < vm.EdictSize/4 {
			if g := vm.EFloat(ent.Num, fg); g != 0 {
				entGravity = g
			}
		}
	}
	vel := ent.Velocity(sh)
	vel[2] -= entGravity * cfg.GetGravity() * timing.GetFrameTime()
	ent.SetVelocity(sh, vel)
}

// fieldGravity returns the QC "gravity" field offset, or -1 if unavailable.
func fieldGravity(sh srvtypes.ServerHandle, ent *srvtypes.Edict) int {
	type gravityProvider interface{ GetFieldGravity() int }
	if gp, ok := sh.(gravityProvider); ok {
		if fg := gp.GetFieldGravity(); fg >= 0 {
			return fg
		}
	}
	return -1
}

// SV_CheckWater checks if an entity is submerged in liquid.
func (s *System) SV_CheckWater(ent *srvtypes.Edict) bool {
	return svCheckWater(s.col, ent, s.sh)
}

func svCheckWater(col srvtypes.CollisionWorld, ent *srvtypes.Edict, sh srvtypes.ServerHandle) bool {
	orig := ent.Origin(sh)
	mins := ent.Mins(sh)
	maxs := ent.Maxs(sh)
	viewOfs := ent.ViewOfs(sh)

	var point [3]float32
	point[0] = orig[0]
	point[1] = orig[1]
	point[2] = orig[2] + mins[2] + 1

	ent.SetWaterLevel(sh, 0)
	ent.SetWaterType(sh, float32(bsp.ContentsEmpty))
	cont := col.PointContents(point)
	if cont <= bsp.ContentsWater {
		ent.SetWaterType(sh, float32(cont))
		ent.SetWaterLevel(sh, 1)
		point[2] = orig[2] + (mins[2]+maxs[2])*0.5
		cont = col.PointContents(point)
		if cont <= bsp.ContentsWater {
			ent.SetWaterLevel(sh, 2)
			point[2] = orig[2] + viewOfs[2]
			cont = col.PointContents(point)
			if cont <= bsp.ContentsWater {
				ent.SetWaterLevel(sh, 3)
			}
		}
	}

	return ent.WaterLevel(sh) > 1
}

// CheckWaterTransition plays the splash sound and updates water type when an
// entity crosses a liquid boundary.
func (s *System) CheckWaterTransition(ent *srvtypes.Edict) {
	orig := ent.Origin(s.sh)
	wtype := ent.WaterType(s.sh)
	cont := s.col.PointContents(orig)

	if wtype == 0 { // just spawned here
		ent.SetWaterType(s.sh, float32(cont))
		ent.SetWaterLevel(s.sh, 1)
		return
	}

	if cont <= bsp.ContentsWater {
		if wtype == float32(bsp.ContentsEmpty) {
			// just crossed into water
			s.facade.StartSound(ent, 0, "misc/h2ohit1.wav", 255, 1)
		}
		ent.SetWaterType(s.sh, float32(cont))
		ent.SetWaterLevel(s.sh, 1)
	} else {
		if wtype != float32(bsp.ContentsEmpty) {
			// just crossed out of water
			s.facade.StartSound(ent, 0, "misc/h2ohit1.wav", 255, 1)
		}
		ent.SetWaterType(s.sh, float32(bsp.ContentsEmpty))
		ent.SetWaterLevel(s.sh, float32(cont))
	}
}

// ClipVelocity slides off an impacting surface.
func ClipVelocity(in, normal [3]float32, overbounce float32) [3]float32 {
	backoff := srvtypes.VecDot(in, normal) * overbounce

	var out [3]float32
	for i := 0; i < 3; i++ {
		change := normal[i] * backoff
		out[i] = in[i] - change
		if out[i] > -srvtypes.StopEpsilon && out[i] < srvtypes.StopEpsilon {
			out[i] = 0
		}
	}

	return out
}

// ============================================================================
// Movement primitives
// ============================================================================

// FlyMove integrates velocity across up to 4 sliding planes.
func (s *System) FlyMove(ent *srvtypes.Edict, time float32, steptrace *srvtypes.TraceResult) int {
	blocked := 0
	entVel := ent.Velocity(s.sh)
	entOrig := ent.Origin(s.sh)
	entMins := ent.Mins(s.sh)
	entMaxs := ent.Maxs(s.sh)
	originalVelocity := entVel
	primalVelocity := entVel
	numPlanes := 0
	var planes [maxClipPlanes][3]float32
	timeLeft := time

	for bumpCount := 0; bumpCount < 4; bumpCount++ {
		if entVel[0] == 0 && entVel[1] == 0 && entVel[2] == 0 {
			break
		}

		end := srvtypes.VecAdd(entOrig, srvtypes.VecScale(entVel, timeLeft))
		trace := s.col.SV_Move(entOrig, entMins, entMaxs, end, srvtypes.MoveNormal, ent)

		if trace.AllSolid {
			ent.SetVelocity(s.sh, [3]float32{})
			return 3
		}

		if trace.Fraction > 0 {
			entOrig = trace.EndPos
			ent.SetOrigin(s.sh, entOrig)
			originalVelocity = entVel
			numPlanes = 0
		}

		if trace.Fraction == 1 {
			break
		}

		if trace.PlaneNormal[2] > 0.7 {
			blocked |= 1
			if trace.Entity != nil && int(trace.Entity.Solid(s.sh)) == int(srvtypes.SolidBSP) {
				ent.SetFlags(s.sh, float32(uint32(ent.Flags(s.sh))|srvtypes.FlagOnGround))
				ent.SetGroundEntity(s.sh, int32(s.facade.NumForEdict(trace.Entity)))
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
			ent.SetVelocity(s.sh, [3]float32{})
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
					if srvtypes.VecDot(newVelocity, planes[j]) < 0 {
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
			ent.SetVelocity(s.sh, entVel)
		} else {
			// go along the crease
			if numPlanes != 2 {
				ent.SetVelocity(s.sh, [3]float32{})
				return 7
			}
			dir := srvtypes.VecCross(planes[0], planes[1])
			d := srvtypes.VecDot(dir, entVel)
			entVel = srvtypes.VecScale(dir, d)
			ent.SetVelocity(s.sh, entVel)
		}
		if srvtypes.VecDot(entVel, primalVelocity) <= 0 {
			ent.SetVelocity(s.sh, [3]float32{})
			return blocked
		}
	}

	return blocked
}

// PushEntity moves an entity by a push vector, clipping against solid geometry.
func (s *System) PushEntity(ent *srvtypes.Edict, push [3]float32) srvtypes.TraceResult {
	orig := ent.Origin(s.sh)
	mins := ent.Mins(s.sh)
	maxs := ent.Maxs(s.sh)
	mt := ent.MoveType(s.sh)
	solid := ent.Solid(s.sh)

	end := [3]float32{
		orig[0] + push[0],
		orig[1] + push[1],
		orig[2] + push[2],
	}

	moveType := srvtypes.MoveNormal
	if srvtypes.MoveType(mt) == srvtypes.MoveTypeFlyMissile {
		moveType = srvtypes.MoveMissile
	} else if int(solid) == int(srvtypes.SolidTrigger) || int(solid) == int(srvtypes.SolidNot) {
		moveType = srvtypes.MoveNoMonsters
	}

	trace := s.col.SV_Move(orig, mins, maxs, end, moveType, ent)
	ent.SetOrigin(s.sh, trace.EndPos)
	s.col.LinkEdict(ent, true)

	if trace.Entity != nil {
		s.Impact(ent, trace.Entity)
	}

	return trace
}

// ============================================================================
// Think / impact / QC callbacks
// ============================================================================

// RunThink executes the entity's think function if its nextthink time has been
// reached. Returns false if the entity was freed by its think.
func (s *System) RunThink(ent *srvtypes.Edict) bool {
	facade := s.facade
	sh := s.sh
	thinkTime := ent.NextThink(sh)
	if thinkTime <= 0 || thinkTime > facade.GetTime()+facade.GetFrameTime() {
		return true
	}

	if thinkTime < facade.GetTime() {
		thinkTime = facade.GetTime()
	}

	ent.OldThinkTime = thinkTime
	ent.OldFrame = ent.Frame(sh)
	ent.SetNextThink(sh, 0)

	entNum := facade.NumForEdict(ent)
	thinkFn := ent.Think(sh)
	telemetryEnabled := facade.EventsEnabled()
	if telemetryEnabled {
		facade.LogEventf(srvdebug.DebugEventThink, facade.GetVM(), entNum, ent,
			"runthink begin think_time=%.3f fn=%d", thinkTime, thinkFn)
	}

	facade.SetQCTimeGlobal(thinkTime)
	if vm := facade.GetVM(); vm != nil {
		vm.SetGlobal("self", entNum)
		vm.SetGlobal("other", 0)
	}
	if thinkFn != 0 {
		prevNumEdicts := facade.GetNumEdicts()
		if err := facade.ExecuteQCFunction(int(thinkFn)); err == nil {
			facade.SyncSpawnedEdictsFromQCVM(prevNumEdicts)
		}
	}
	if telemetryEnabled {
		facade.LogEventf(srvdebug.DebugEventThink, facade.GetVM(), entNum, ent,
			"runthink end think_time=%.3f freed=%t", thinkTime, ent.Free)
	}

	return !ent.Free
}

// Impact runs touch functions for two entities that have collided.
func (s *System) Impact(e1, e2 *srvtypes.Edict) {
	facade := s.facade
	if facade == nil || facade.GetVM() == nil || facade.SuppressTouchQC() {
		return
	}
	ctx := facade.CaptureExecutionContext()
	defer facade.RestoreExecutionContext(ctx)
	e1Num := facade.NumForEdict(e1)
	e2Num := facade.NumForEdict(e2)
	telemetryEnabled := facade.EventsEnabled()

	facade.SetQCTimeGlobal(facade.GetTime())

	e1Touch := e1.Touch(s.sh)
	e1Solid := e1.Solid(s.sh)
	if e1Touch != 0 && e1Solid != float32(srvtypes.SolidNot) {
		prevNumEdicts := facade.GetNumEdicts()
		if telemetryEnabled {
			facade.LogEventf(srvdebug.DebugEventTouch, facade.GetVM(), e1Num, e1,
				"impact touch begin other=%d fn=%d", e2Num, e1Touch)
		}
		facade.DebugTriggerTouch("impact", e1, e2)
		facade.GetVM().SetGlobal("self", e1Num)
		facade.GetVM().SetGlobal("other", e2Num)
		if err := facade.ExecuteQCFunction(int(e1Touch)); err == nil {
			facade.SyncSpawnedEdictsFromQCVM(prevNumEdicts)
		}
		if telemetryEnabled {
			facade.LogEventf(srvdebug.DebugEventTouch, facade.GetVM(), e1Num, e1,
				"impact touch end other=%d fn=%d", e2Num, e1Touch)
		}
	}

	e2Touch := e2.Touch(s.sh)
	e2Solid := e2.Solid(s.sh)
	if e2Touch != 0 && e2Solid != float32(srvtypes.SolidNot) {
		prevNumEdicts := facade.GetNumEdicts()
		if telemetryEnabled {
			facade.LogEventf(srvdebug.DebugEventTouch, facade.GetVM(), e2Num, e2,
				"impact touch begin other=%d fn=%d", e1Num, e2Touch)
		}
		facade.DebugTriggerTouch("impact", e2, e1)
		facade.GetVM().SetGlobal("self", e2Num)
		facade.GetVM().SetGlobal("other", e1Num)
		if err := facade.ExecuteQCFunction(int(e2Touch)); err == nil {
			facade.SyncSpawnedEdictsFromQCVM(prevNumEdicts)
		}
		if telemetryEnabled {
			facade.LogEventf(srvdebug.DebugEventTouch, facade.GetVM(), e2Num, e2,
				"impact touch end other=%d fn=%d", e2Num, e2Touch)
		}
	}
}

// ============================================================================
// PushMove
// ============================================================================

// PushMove moves all entities riding or overlapping a pusher, restoring
// origins and blocking QC callbacks when the pusher is blocked.
func (s *System) PushMove(pusher *srvtypes.Edict, movetime float32) {
	facade := s.facade
	sh := s.sh
	col := s.col

	pusherNum := facade.NumForEdict(pusher)
	pusherVel := pusher.Velocity(sh)
	pusherLTime := pusher.LTime(sh)
	pusherAbsMin := pusher.AbsMin(sh)
	pusherAbsMax := pusher.AbsMax(sh)
	pusherOrig := pusher.Origin(sh)

	if pusherVel[0] == 0 && pusherVel[1] == 0 && pusherVel[2] == 0 {
		srvdebug.SvdbgPushLogf("pusher=%d velocity=0 — early return (ltime += %.4f), no entities pushed", pusherNum, movetime)
		pusher.SetLTime(sh, pusherLTime+movetime)
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
	pusher.SetOrigin(sh, srvtypes.VecAdd(pusherOrig, move))
	pusher.SetLTime(sh, pusherLTime+movetime)
	col.LinkEdict(pusher, false)

	// Reuse the origin-restore buffers across calls: resetting the length
	// keeps the backing arrays hot and avoids reallocating NumEdicts-sized
	// slices on every pusher every frame (major per-frame alloc source on
	// maps with many moving platforms, e.g. qbj3_softi).
	movedEdicts, movedFrom := facade.PushMoveScratch()
	*movedEdicts = (*movedEdicts)[:0]
	*movedFrom = (*movedFrom)[:0]

	// sv_debug_push logging is the only varargs-allocation site in the
	// per-edict scan below; hoist the gate so the hot loop does not build
	// []any argument slices when diagnostics are disabled.
	pushLogEnabled := srvdebug.SvDebugPushEnabled()

	for e := 1; e < facade.GetNumEdicts(); e++ {
		check := s.store.EdictNum(e)
		if check == nil || check.Free {
			continue
		}

		checkMoveType := check.MoveType(sh)
		movemask := 1 << int(checkMoveType)
		if movemask&((1<<int(srvtypes.MoveTypePush))|(1<<int(srvtypes.MoveTypeNone))|(1<<int(srvtypes.MoveTypeNoClip))) != 0 {
			continue
		}

		checkFlags := check.Flags(sh)
		checkGroundEnt := check.GroundEntity(sh)
		checkAbsMin := check.AbsMin(sh)
		checkAbsMax := check.AbsMax(sh)

		riding := (uint32(checkFlags)&srvtypes.FlagOnGround) != 0 && s.store.EdictNum(int(checkGroundEnt)) == pusher
		if !riding {
			if checkAbsMin[0] >= maxs[0] ||
				checkAbsMin[1] >= maxs[1] ||
				checkAbsMin[2] >= maxs[2] ||
				checkAbsMax[0] <= mins[0] ||
				checkAbsMax[1] <= mins[1] ||
				checkAbsMax[2] <= mins[2] {
				if pushLogEnabled {
					srvdebug.SvdbgPushLogfAt(2, "pusher=%d check=%d riding=false aabb_miss=true — skipped", pusherNum, e)
				}
				continue
			}

			if col.SV_TestEntityPosition(check) == nil {
				if pushLogEnabled {
					srvdebug.SvdbgPushLogfAt(2, "pusher=%d check=%d riding=false aabb_hit=true testpos=nil — skipped (not stuck)", pusherNum, e)
				}
				continue
			}
			if pushLogEnabled {
				srvdebug.SvdbgPushLogf("pusher=%d check=%d riding=false aabb_hit=true testpos=stuck — pushing (overlapping non-rider)", pusherNum, e)
			}
		} else if pushLogEnabled {
			srvdebug.SvdbgPushLogf("pusher=%d check=%d riding=true — pushing (groundentity=%d onground=1)", pusherNum, e, int(checkGroundEnt))
		}

		if srvtypes.MoveType(checkMoveType) != srvtypes.MoveTypeWalk {
			check.SetFlags(sh, float32(uint32(checkFlags)&^srvtypes.FlagOnGround))
		}

		entorig := check.Origin(sh)
		*movedEdicts = append(*movedEdicts, check)
		*movedFrom = append(*movedFrom, entorig)

		pusherSolid := pusher.Solid(sh)
		if int(pusherSolid) == int(srvtypes.SolidBSP) || int(pusherSolid) == int(srvtypes.SolidBBox) || int(pusherSolid) == int(srvtypes.SolidSlideBox) {
			pusher.SetSolid(sh, float32(srvtypes.SolidNot))
			s.PushEntity(check, move)
			pusher.SetSolid(sh, pusherSolid)
		} else {
			s.PushEntity(check, move)
		}

		// sv_debug_push: log post-push position so we can see where the player
		// ended up after being pushed by the lift, and whether they're blocked.
		postPushOrig := check.Origin(sh)
		postPushAbsMin := check.AbsMin(sh)
		postPushAbsMax := check.AbsMax(sh)
		srvdebug.SvdbgPushLogfAt(2, "pusher=%d check=%d post-push origin=(%.1f %.1f %.1f) absmin=(%.1f %.1f %.1f) absmax=(%.1f %.1f %.1f)",
			pusherNum, e,
			postPushOrig[0], postPushOrig[1], postPushOrig[2],
			postPushAbsMin[0], postPushAbsMin[1], postPushAbsMin[2],
			postPushAbsMax[0], postPushAbsMax[1], postPushAbsMax[2])

		block := col.SV_TestEntityPosition(check)
		if block == nil {
			srvdebug.SvdbgPushLogfAt(2, "pusher=%d check=%d post-push testpos=nil (not blocked)", pusherNum, e)
			continue
		}

		checkMins := check.Mins(sh)
		checkMaxs := check.Maxs(sh)
		if checkMins[0] == checkMaxs[0] {
			continue
		}

		checkSolid := check.Solid(sh)
		if int(checkSolid) == int(srvtypes.SolidNot) || int(checkSolid) == int(srvtypes.SolidTrigger) {
			check.SetMins(sh, [3]float32{})
			check.SetMaxs(sh, [3]float32{})
			continue
		}

		// Elevator gameplay fix: if entity is riding the pusher and blocked
		// by it, try nudging upward by DIST_EPSILON to prevent crushing.
		// Matches C sv_phys.c:552-578 (sv_gameplayfix_elevators).
		fixLevel := facade.FloatValue("sv_gameplayfix_elevators")
		if riding && block == pusher &&
			(fixLevel >= 2 || (fixLevel > 0 && e <= facade.MaxClients())) {
			postPushOrig[2] += srvtypes.DistEpsilon
			check.SetOrigin(sh, postPushOrig)
			if col.SV_TestEntityPosition(check) == nil {
				slog.Debug("elevator fix nudged entity",
					"entity", e, "pusher", facade.NumForEdict(pusher))
				continue
			}
		}

		check.SetOrigin(sh, entorig)
		col.LinkEdict(check, true)

		pusher.SetOrigin(sh, pushorig)
		col.LinkEdict(pusher, false)
		pusher.SetLTime(sh, pusher.LTime(sh)-movetime)

		pusherBlocked := pusher.Blocked(sh)
		if pusherBlocked != 0 && facade.GetVM() != nil {
			pusherNum := facade.NumForEdict(pusher)
			checkNum := facade.NumForEdict(check)
			ctx := facade.CaptureExecutionContext()
			telemetryEnabled := facade.EventsEnabled()
			if telemetryEnabled {
				facade.LogEventf(srvdebug.DebugEventBlocked, facade.GetVM(), pusherNum, pusher,
					"pushmove blocked by=%d callback=%d movetime=%.3f", checkNum, pusherBlocked, movetime)
			}
			prevNumEdicts := facade.GetNumEdicts()
			facade.GetVM().SetGlobal("self", pusherNum)
			facade.GetVM().SetGlobal("other", checkNum)
			if err := facade.ExecuteQCFunction(int(pusherBlocked)); err == nil {
				facade.SyncSpawnedEdictsFromQCVM(prevNumEdicts)
			}
			facade.RestoreExecutionContext(ctx)
			if telemetryEnabled {
				facade.LogEventf(srvdebug.DebugEventBlocked, facade.GetVM(), pusherNum, pusher,
					"pushmove blocked callback done by=%d callback=%d", checkNum, pusherBlocked)
			}
		}

		for i, moved := range *movedEdicts {
			moved.SetOrigin(sh, (*movedFrom)[i])
			col.LinkEdict(moved, false)
		}

		// Retain the underlying arrays for the next call.
		*movedEdicts = (*movedEdicts)[:0]
		*movedFrom = (*movedFrom)[:0]

		return
	}
}

// ============================================================================
// Movetype dispatchers
// ============================================================================

// PhysicsNone handles static entities (only runs think functions).
func (s *System) PhysicsNone(ent *srvtypes.Edict) {
	s.RunThink(ent)
}

// PhysicsNoClip handles noclip movement.
func (s *System) PhysicsNoClip(ent *srvtypes.Edict) {
	if !s.RunThink(ent) {
		return
	}
	sh := s.sh
	angles := ent.Angles(sh)
	avel := ent.AVelocity(sh)
	orig := ent.Origin(sh)
	vel := ent.Velocity(sh)
	frameTime := s.facade.GetFrameTime()

	for i := 0; i < 3; i++ {
		angles[i] += avel[i] * frameTime
		orig[i] += vel[i] * frameTime
	}

	ent.SetAngles(sh, angles)
	ent.SetOrigin(sh, orig)
	s.col.LinkEdict(ent, false)
}

// PhysicsPusher moves a pusher by its velocity and runs its think when due.
func (s *System) PhysicsPusher(ent *srvtypes.Edict) {
	facade := s.facade
	sh := s.sh
	entNum := facade.NumForEdict(ent)
	telemetryEnabled := facade.EventsEnabled()
	oldLTime := ent.LTime(sh)
	// Snapshot nextthink once, like C: both the movetime derivation and the
	// post-push think gate must use the SAME original thinktime. Re-reading
	// nextthink after PushMove lets a blocked/use callback's re-arm (a new
	// nextthink landing inside the original window) fire the pusher think
	// twice (or eat the re-arm). Where in C: SV_Physics_Pusher in sv_phys.c.
	thinkTime := ent.NextThink(sh)
	movetime := facade.GetFrameTime()

	if thinkTime < oldLTime+facade.GetFrameTime() {
		movetime = thinkTime - oldLTime
		if movetime < 0 {
			movetime = 0
		}
	}
	if telemetryEnabled {
		facade.LogEventf(srvdebug.DebugEventPhysics, facade.GetVM(), entNum, ent,
			"physicspusher movetime=%.3f think_time=%.3f ltime=%.3f", movetime, thinkTime, oldLTime)
	}

	if movetime != 0 {
		s.PushMove(ent, movetime)
	}

	// Gate on the ORIGINAL thinkTime, exactly like C:
	// `if (thinktime > oldltime && thinktime <= ent->v.ltime)`. curLTime is
	// the post-push ltime; a new nextthink written by a blocked callback stays
	// armed for a later frame instead of firing twice in one frame.
	thinkFn := ent.Think(sh)
	curLTime := ent.LTime(sh)
	if thinkTime > oldLTime && thinkTime <= curLTime {
		ent.SetNextThink(sh, 0)
		if facade.GetVM() != nil && thinkFn != 0 {
			if telemetryEnabled {
				facade.LogEventf(srvdebug.DebugEventThink, facade.GetVM(), entNum, ent,
					"physicspusher think begin fn=%d", thinkFn)
			}
			facade.SetQCTimeGlobal(facade.GetTime())
			facade.GetVM().SetGlobal("self", entNum)
			facade.GetVM().SetGlobal("other", 0)
			prevNumEdicts := facade.GetNumEdicts()
			if err := facade.ExecuteQCFunction(int(thinkFn)); err == nil {
				facade.SyncSpawnedEdictsFromQCVM(prevNumEdicts)
			}
			if telemetryEnabled {
				facade.LogEventf(srvdebug.DebugEventThink, facade.GetVM(), entNum, ent,
					"physicspusher think end fn=%d freed=%t", thinkFn, ent.Free)
			}
		}
	}
}

// PhysicsStep handles step movement with gravity and landing sounds.
func (s *System) PhysicsStep(ent *srvtypes.Edict) {
	sh := s.sh
	flags := uint32(ent.Flags(sh))
	if flags&(srvtypes.FlagOnGround|srvtypes.FlagFly|srvtypes.FlagSwim) == 0 {
		vel := ent.Velocity(sh)
		hitSound := vel[2] < -0.1*s.facade.GetGravity()
		s.AddGravity(ent)
		s.CheckVelocity(ent)
		s.FlyMove(ent, s.facade.GetFrameTime(), nil)
		s.col.LinkEdict(ent, true)
		if uint32(ent.Flags(sh))&srvtypes.FlagOnGround != 0 && hitSound {
			s.facade.StartSound(ent, 0, "demon/dland2.wav", 255, 1)
		}
	}

	s.RunThink(ent)
	s.CheckWaterTransition(ent)
}

// SV_CheckAllEnts warns about entities in invalid positions.
func (s *System) SV_CheckAllEnts() {
	sh := s.sh
	for i := 1; i < s.store.GetNumEdicts(); i++ {
		ent := s.store.EdictNum(i)
		if ent == nil || ent.Free {
			continue
		}
		mt := srvtypes.MoveType(ent.MoveType(sh))
		if mt == srvtypes.MoveTypePush || mt == srvtypes.MoveTypeNone || mt == srvtypes.MoveTypeNoClip {
			continue
		}

		if s.col.SV_TestEntityPosition(ent) != nil {
			// Mirrors SV_CheckAllEnts in the C reference: the original
			// emits a Con_Printf warning; we surface it through slog.
			slog.Debug("sv_checkallents: entity in invalid position",
				"index", i,
				"classname", ent.ClassName(sh))
		}
	}
}

// SV_TryUnstick attempts to unstick an entity by nudging in 8 directions.
func (s *System) SV_TryUnstick(ent *srvtypes.Edict, oldVel [3]float32) int {
	sh := s.sh
	oldOrg := ent.Origin(sh)
	var dir [3]float32

	for i := 0; i < 8; i++ {
		dir = [3]float32{}
		switch i {
		case 0:
			dir[0] = 2
		case 1:
			dir[1] = 2
		case 2:
			dir[0] = -2
		case 3:
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
		ent.SetVelocity(sh, [3]float32{oldVel[0], oldVel[1], 0})
		blocked := s.FlyMove(ent, 0.1, nil)

		curOrg := ent.Origin(sh)
		if math.Abs(float64(oldOrg[0]-curOrg[0])) > 4 ||
			math.Abs(float64(oldOrg[1]-curOrg[1])) > 4 {
			return blocked
		}

		// go back to the original pos and try again
		ent.SetOrigin(sh, oldOrg)
	}

	ent.SetVelocity(sh, [3]float32{})
	return 7 // still not moving
}

// ============================================================================
// Walking movement
// ============================================================================

// SV_WalkMove implements the walk-move stair-step algorithm.
func (s *System) SV_WalkMove(ent *srvtypes.Edict) {
	sh := s.sh
	facade := s.facade
	flags := uint32(ent.Flags(sh))
	oldonground := flags & srvtypes.FlagOnGround
	ent.SetFlags(sh, float32(flags&^srvtypes.FlagOnGround))

	oldorg := ent.Origin(sh)
	oldvel := ent.Velocity(sh)

	var steptrace srvtypes.TraceResult
	clip := s.FlyMove(ent, facade.GetFrameTime(), &steptrace)

	if clip&2 == 0 {
		return // move didn't block on a step
	}

	if oldonground == 0 && ent.WaterLevel(sh) == 0 {
		return // don't stair up while jumping
	}

	if srvtypes.MoveType(ent.MoveType(sh)) != srvtypes.MoveTypeWalk {
		return // gibbed by a trigger
	}

	if facade.BoolValue("sv_nostep") {
		return
	}

	if uint32(ent.Flags(sh))&srvtypes.FlagWaterJump != 0 {
		return
	}

	nosteporg := ent.Origin(sh)
	nostepvel := ent.Velocity(sh)

	// back to start pos
	ent.SetOrigin(sh, oldorg)

	// step up
	upmove := [3]float32{0, 0, 18}
	s.PushEntity(ent, upmove)

	// move forward with zeroed Z velocity
	ent.SetVelocity(sh, [3]float32{oldvel[0], oldvel[1], 0})
	clip = s.FlyMove(ent, facade.GetFrameTime(), &steptrace)

	// check for stuckness
	if clip != 0 {
		if srvtypes.WalkMoveNeedsUnstick(oldorg, ent.Origin(sh)) {
			clip = s.SV_TryUnstick(ent, oldvel)
		}
	}

	// extra friction based on view angle
	if clip&2 != 0 {
		s.SV_WallFriction(ent, &steptrace)
	}

	// move down
	downmove := [3]float32{0, 0, -18 + oldvel[2]*facade.GetFrameTime()}
	downtrace := s.PushEntity(ent, downmove)

	if downtrace.PlaneNormal[2] > 0.7 {
		if downtrace.Entity != nil && int(downtrace.Entity.Solid(sh)) == int(srvtypes.SolidBSP) {
			ent.SetFlags(sh, float32(uint32(ent.Flags(sh))|srvtypes.FlagOnGround))
			ent.SetGroundEntity(sh, int32(facade.NumForEdict(downtrace.Entity)))
		}
	} else {
		// if the push down didn't end up on good ground, use the move without the step up
		ent.SetOrigin(sh, nosteporg)
		ent.SetVelocity(sh, nostepvel)
	}
}

// SV_WallFriction applies extra friction based on view angle against a wall.
func (s *System) SV_WallFriction(ent *srvtypes.Edict, trace *srvtypes.TraceResult) {
	sh := s.sh
	var forward, right, up [3]float32
	srvtypes.AngleVectors(ent.VAngle(sh), &forward, &right, &up)

	d := srvtypes.VecDot(trace.PlaneNormal, forward)
	d += 0.5
	if d >= 0 {
		return
	}

	// cut the tangential velocity
	vel := ent.Velocity(sh)
	i := srvtypes.VecDot(trace.PlaneNormal, vel)
	into := srvtypes.VecScale(trace.PlaneNormal, i)
	side := srvtypes.VecSub(vel, into)

	vel[0] = side[0] * (1 + d)
	vel[1] = side[1] * (1 + d)
	ent.SetVelocity(sh, vel)
}

// PhysicsWalk handles walking physics with client pre/post think.
func (s *System) PhysicsWalk(ent *srvtypes.Edict) {
	sh := s.sh
	facade := s.facade
	PlayerClient := facade.PlayerClient(ent)
	wasUnderwater := ent.WaterLevel(sh) >= 3
	if PlayerClient != nil {
		facade.RunClientQCThinkWithMode(PlayerClient, "PlayerPreThink", false)
		if ent.Free {
			return
		}
	}

	s.CheckVelocity(ent)

	if !s.RunThink(ent) {
		return
	}

	if !s.SV_CheckWater(ent) && uint32(ent.Flags(sh))&srvtypes.FlagWaterJump == 0 {
		s.AddGravity(ent)
	}

	s.SV_CheckStuck(ent)
	s.SV_WalkMove(ent)

	s.col.LinkEdict(ent, true)
	if PlayerClient != nil {
		facade.RunClientQCThinkWithMode(PlayerClient, "PlayerPostThink", false)
		if ent.Free {
			return
		}
		forceUnderwater := !wasUnderwater && ent.WaterLevel(sh) >= 3
		if forceUnderwater != ent.ForceWater {
			ent.ForceWater = forceUnderwater
			ent.SendForceWater = true
		}
	}
}

// SV_CheckStuck checks and unsticks an entity that overlaps solid.
func (s *System) SV_CheckStuck(ent *srvtypes.Edict) {
	sh := s.sh
	orig := ent.Origin(sh)
	if s.col.SV_TestEntityPosition(ent) == nil {
		ent.SetOldOrigin(sh, orig)
		return
	}

	oldOrg := ent.OldOrigin(sh)
	ent.SetOrigin(sh, oldOrg)
	if s.col.SV_TestEntityPosition(ent) == nil {
		// Unstuck.
		s.col.LinkEdict(ent, true)
		return
	}

	for z := float32(0); z < 18; z++ {
		for i := float32(-1); i <= 1; i++ {
			for j := float32(-1); j <= 1; j++ {
				cand := [3]float32{orig[0] + i, orig[1] + j, orig[2] + z}
				ent.SetOrigin(sh, cand)
				if s.col.SV_TestEntityPosition(ent) == nil {
					// Unstuck.
					s.col.LinkEdict(ent, true)
					return
				}
			}
		}
	}

	ent.SetOrigin(sh, orig)
	// player is stuck
}

// PhysicsToss handles ballistic/toss movement with bounce backoff.
func (s *System) PhysicsToss(ent *srvtypes.Edict) {
	sh := s.sh
	facade := s.facade
	if !s.RunThink(ent) {
		return
	}

	flags := uint32(ent.Flags(sh))
	if flags&srvtypes.FlagOnGround != 0 {
		return
	}

	s.CheckVelocity(ent)

	mt := srvtypes.MoveType(ent.MoveType(sh))
	if mt != srvtypes.MoveTypeFly && mt != srvtypes.MoveTypeFlyMissile {
		s.AddGravity(ent)
	}

	angles := ent.Angles(sh)
	avel := ent.AVelocity(sh)
	for i := 0; i < 3; i++ {
		angles[i] += avel[i] * facade.GetFrameTime()
	}
	ent.SetAngles(sh, angles)

	vel := ent.Velocity(sh)
	move := srvtypes.VecScale(vel, facade.GetFrameTime())
	trace := s.PushEntity(ent, move)
	if trace.Fraction == 1 || ent.Free {
		return
	}

	backoff := float32(1)
	if mt == srvtypes.MoveTypeBounce {
		backoff = 1.5
	}

	newVel := ClipVelocity(vel, trace.PlaneNormal, backoff)
	ent.SetVelocity(sh, newVel)

	if trace.PlaneNormal[2] > 0.7 {
		if newVel[2] < 60 || mt != srvtypes.MoveTypeBounce {
			ent.SetFlags(sh, float32(uint32(ent.Flags(sh))|srvtypes.FlagOnGround))
			ent.SetGroundEntity(sh, int32(facade.NumForEdict(trace.Entity)))
			ent.SetVelocity(sh, [3]float32{})
			ent.SetAVelocity(sh, [3]float32{})
		}
	}

	s.CheckWaterTransition(ent)
}
