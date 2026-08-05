// physics_walk.go implements walking and bouncing entity movement plus the
// walk-move unstick helpers.
//
// This file was split out of physics.go to keep that file under the
// project-wide 1,000-line ceiling. The functions here are the
// MOVETYPE_WALK / MOVETYPE_TOSS simulation logic and the shared
// SV_CheckStuck / SV_TryUnstick helpers.
package server

import "math"

// walkMoveNeedsUnstick reports whether the player's forward move was
// obstructed by a low step (only X/Y drift while Z stayed put), meaning the
// attempted stair-step should be retried from the original position.
// Mirrors C Ironwail SV_WalkMove's unstick heuristic.
func walkMoveNeedsUnstick(oldOrg, newOrg [3]float32) bool {
	return math.Abs(float64(oldOrg[0]-newOrg[0])) < float64(DistEpsilon) &&
		math.Abs(float64(oldOrg[1]-newOrg[1])) < float64(DistEpsilon)
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
	PlayerClient := s.PlayerClient(ent)
	wasUnderwater := ent.WaterLevel(s) >= 3
	if PlayerClient != nil {
		s.RunClientQCThinkWithMode(PlayerClient, "PlayerPreThink", false)
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
	if PlayerClient != nil {
		s.RunClientQCThinkWithMode(PlayerClient, "PlayerPostThink", false)
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
