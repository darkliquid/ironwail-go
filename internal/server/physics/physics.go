// physics.go implements per-entity physics simulation, dispatching movement and collision for all entity movetypes.
package physics

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// CheckVelocity clamps an entity's velocity components to MaxVelocity bounds.
func CheckVelocity(cfg srvtypes.PhysicsConfig, ent *srvtypes.Edict, sh srvtypes.ServerHandle) {
	vel := ent.Velocity(sh)
	orig := ent.Origin(sh)
	changedVel := false
	changedOrig := false
	maxVel := cfg.GetMaxVelocity()
	for i := 0; i < 3; i++ {
		if math.IsNaN(float64(vel[i])) {
			vel[i] = 0
			changedVel = true
		}
		if math.IsNaN(float64(orig[i])) {
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
func AddGravity(cfg srvtypes.PhysicsConfig, timing srvtypes.FrameTiming, ent *srvtypes.Edict, sh srvtypes.ServerHandle) {
	entGravity := ent.Gravity(sh)
	if entGravity == 0 {
		entGravity = 1.0
	}
	vel := ent.Velocity(sh)
	vel[2] -= entGravity * cfg.GetGravity() * timing.GetFrameTime()
	ent.SetVelocity(sh, vel)
}

// SV_CheckWater checks if an entity is submerged in liquid.
func SV_CheckWater(col srvtypes.CollisionWorld, ent *srvtypes.Edict, sh srvtypes.ServerHandle) bool {
	point := ent.Origin(sh)
	point[2] += ent.Mins(sh)[2] + 1.0
	ent.SetWaterLevel(sh, 0)
	ent.SetWaterType(sh, float32(bsp.ContentsEmpty))
	cont := col.PointContents(point)

	if cont <= bsp.ContentsWater {
		ent.SetWaterType(sh, float32(cont))
		ent.SetWaterLevel(sh, 1)

		point[2] = ent.Origin(sh)[2] + (ent.Mins(sh)[2]+ent.Maxs(sh)[2])*0.5
		cont = col.PointContents(point)

		if cont <= bsp.ContentsWater {
			ent.SetWaterLevel(sh, 2)

			point[2] = ent.Origin(sh)[2] + ent.ViewOfs(sh)[2]
			cont = col.PointContents(point)

			if cont <= bsp.ContentsWater {
				ent.SetWaterLevel(sh, 3)
			}
		}
	}

	return ent.WaterLevel(sh) > 0
}

// PushEntity moves an entity by a push vector, clipping against solid geometry.
func PushEntity(col srvtypes.CollisionWorld, ent *srvtypes.Edict, push [3]float32, sh srvtypes.ServerHandle) srvtypes.TraceResult {
	start := ent.Origin(sh)
	end := srvtypes.VecAdd(start, push)
	moveType := srvtypes.MoveNormal

	if ent.Solid(sh) == float32(srvtypes.SolidTrigger) {
		moveType = srvtypes.MoveNoMonsters
	}

	trace := col.SV_Move(start, ent.Mins(sh), ent.Maxs(sh), end, moveType, ent)
	ent.SetOrigin(sh, trace.EndPos)
	col.LinkEdict(ent, true)

	if trace.Entity != nil {
		Impact(col, ent, trace.Entity, &trace, sh)
	}

	return trace
}

// Impact handles collision impact events between two entities.
func Impact(col srvtypes.CollisionWorld, e1, e2 *srvtypes.Edict, trace *srvtypes.TraceResult, sh srvtypes.ServerHandle) {
	if e1 == nil || e2 == nil {
		return
	}
	col.LinkEdict(e1, true)
	col.LinkEdict(e2, true)
}

// ClipVelocity redirects a velocity vector off a surface normal.
func ClipVelocity(in, normal [3]float32, out *[3]float32, overbounce float32) int {
	backoff := srvtypes.VecDot(in, normal) * overbounce
	for i := 0; i < 3; i++ {
		change := normal[i] * backoff
		out[i] = in[i] - change
		if out[i] > -srvtypes.StopEpsilon && out[i] < srvtypes.StopEpsilon {
			out[i] = 0
		}
	}

	blocked := 0
	if normal[2] < 0.7 {
		blocked |= 1
	}
	if normal[2] == 0 {
		blocked |= 2
	}
	return blocked
}

// FlyMove integrates velocity across up to 4 sliding planes.
func FlyMove(col srvtypes.CollisionWorld, cfg srvtypes.PhysicsConfig, timing srvtypes.FrameTiming, ent *srvtypes.Edict, time float32, sh srvtypes.ServerHandle) int {
	bumpcount := 0
	numplanes := 0
	var planes [5][3]float32
	blocked := 0
	timeLeft := time

	entOrig := ent.Origin(sh)
	entVel := ent.Velocity(sh)

	for bumpcount < 4 {
		if entVel[0] == 0 && entVel[1] == 0 && entVel[2] == 0 {
			break
		}

		end := srvtypes.VecAdd(entOrig, srvtypes.VecScale(entVel, timeLeft))
		trace := col.SV_Move(entOrig, ent.Mins(sh), ent.Maxs(sh), end, srvtypes.MoveNormal, ent)

		if trace.AllSolid {
			ent.SetVelocity(sh, [3]float32{0, 0, 0})
			return 3
		}

		if trace.Fraction > 0 {
			entOrig = trace.EndPos
			ent.SetOrigin(sh, entOrig)
			numplanes = 0
		}

		if trace.Fraction == 1.0 {
			break
		}

		if trace.Entity != nil {
			Impact(col, ent, trace.Entity, &trace, sh)
		}

		timeLeft -= timeLeft * trace.Fraction

		if numplanes >= 5 {
			ent.SetVelocity(sh, [3]float32{0, 0, 0})
			return 3
		}

		planes[numplanes] = trace.PlaneNormal
		numplanes++

		if numplanes == 1 {
			var newVel [3]float32
			ClipVelocity(entVel, planes[0], &newVel, 1.0)
			entVel = newVel
		} else {
			var newVel [3]float32
			for i := 0; i < numplanes; i++ {
				ClipVelocity(entVel, planes[i], &newVel, 1.0)
				for j := 0; j < numplanes; j++ {
					if i != j {
						if srvtypes.VecDot(newVel, planes[j]) < 0 {
							break
						}
					}
				}
				if i < numplanes {
					break
				}
			}
			entVel = newVel
		}

		bumpcount++
	}

	ent.SetVelocity(sh, entVel)
	return blocked
}

// PhysicsWalk handles walking physics for an entity.
func PhysicsWalk(col srvtypes.CollisionWorld, store srvtypes.EntityStore, cfg srvtypes.PhysicsConfig, timing srvtypes.FrameTiming, exec srvtypes.ThinkExecutor, ent *srvtypes.Edict, sh srvtypes.ServerHandle) {
	if !exec.RunThink(ent) {
		return
	}

	CheckVelocity(cfg, ent, sh)

	wasOnGround := (uint32(ent.Flags(sh)) & srvtypes.FlagOnGround) != 0

	if wasOnGround {
		vel := ent.Velocity(sh)
		vel[2] = 0
		ent.SetVelocity(sh, vel)
	} else {
		AddGravity(cfg, timing, ent, sh)
	}

	FlyMove(col, cfg, timing, ent, timing.GetFrameTime(), sh)

	CheckVelocity(cfg, ent, sh)
	col.LinkEdict(ent, true)

	SV_CheckWater(col, ent, sh)
}

// PhysicsToss handles ballistic/toss physics for an entity.
func PhysicsToss(col srvtypes.CollisionWorld, store srvtypes.EntityStore, cfg srvtypes.PhysicsConfig, timing srvtypes.FrameTiming, exec srvtypes.ThinkExecutor, ent *srvtypes.Edict, sh srvtypes.ServerHandle) {
	if !exec.RunThink(ent) {
		return
	}

	CheckVelocity(cfg, ent, sh)

	moveType := srvtypes.MoveType(ent.MoveType(sh))
	if moveType == srvtypes.MoveTypeToss || moveType == srvtypes.MoveTypeBounce {
		AddGravity(cfg, timing, ent, sh)
	}

	FlyMove(col, cfg, timing, ent, timing.GetFrameTime(), sh)

	CheckVelocity(cfg, ent, sh)
	col.LinkEdict(ent, true)
}

// PhysicsNoclip handles noclip movement.
func PhysicsNoclip(cfg srvtypes.PhysicsConfig, timing srvtypes.FrameTiming, exec srvtypes.ThinkExecutor, ent *srvtypes.Edict, sh srvtypes.ServerHandle) {
	if !exec.RunThink(ent) {
		return
	}
	CheckVelocity(cfg, ent, sh)
	orig := ent.Origin(sh)
	vel := ent.Velocity(sh)
	for i := 0; i < 3; i++ {
		orig[i] += vel[i] * timing.GetFrameTime()
	}
	ent.SetOrigin(sh, orig)
}

// PhysicsNone handles static entities (only runs think functions).
func PhysicsNone(exec srvtypes.ThinkExecutor, ent *srvtypes.Edict, sh srvtypes.ServerHandle) {
	exec.RunThink(ent)
}
