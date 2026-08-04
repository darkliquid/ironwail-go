// movement.go implements monster movement and pathfinding.
package physics

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

const (
	stepSize = 18
	diNoDir  = -1
)

func changeYaw(ent *srvtypes.Edict, sh srvtypes.ServerHandle) {
	current := srvtypes.AngleMod(ent.Angles(sh)[1])
	ideal := ent.IdealYaw(sh)
	speed := ent.YawSpeed(sh)

	if current == ideal {
		return
	}

	move := ideal - current
	if ideal > current {
		if move >= 180 {
			move -= 360
		}
	} else if move <= -180 {
		move += 360
	}

	if move > 0 {
		if move > speed {
			move = speed
		}
	} else if move < -speed {
		move = -speed
	}

	angles := ent.Angles(sh)
	angles[1] = srvtypes.AngleMod(current + move)
	ent.SetAngles(sh, angles)
}

func checkBottom(col srvtypes.CollisionWorld, store srvtypes.EntityStore, ent *srvtypes.Edict, sh srvtypes.ServerHandle) bool {
	origin := ent.Origin(sh)
	mins := srvtypes.VecAdd(origin, ent.Mins(sh))
	maxs := srvtypes.VecAdd(origin, ent.Maxs(sh))

	var start [3]float32
	var stop [3]float32

	start[2] = mins[2] - 1
	stop[2] = start[2] - 2

	for x := 0; x <= 1; x++ {
		for y := 0; y <= 1; y++ {
			if x == 0 {
				start[0] = mins[0]
			} else {
				start[0] = maxs[0]
			}
			if y == 0 {
				start[1] = mins[1]
			} else {
				start[1] = maxs[1]
			}
			stop[0] = start[0]
			stop[1] = start[1]

			contents := col.PointContents(start)
			if contents != bsp.ContentsSolid {
				return false
			}
		}
	}

	return true
}

func fixCheckBottom(ent *srvtypes.Edict, sh srvtypes.ServerHandle) {
	ent.SetFlags(sh, float32(uint32(ent.Flags(sh))|srvtypes.FlagPartialGround))
}

func moveStep(col srvtypes.CollisionWorld, store srvtypes.EntityStore, ent *srvtypes.Edict, move [3]float32, relink bool, sh srvtypes.ServerHandle) bool {
	flags := uint32(ent.Flags(sh))
	oldorg := ent.Origin(sh)
	neworg := srvtypes.VecAdd(ent.Origin(sh), move)

	if flags&(srvtypes.FlagSwim|srvtypes.FlagFly) != 0 {
		for i := 0; i < 2; i++ {

			neworg = srvtypes.VecAdd(ent.Origin(sh), move)

			if i == 0 && ent.Enemy(sh) != 0 {
				enemy := store.EdictNum(int(ent.Enemy(sh)))
				if enemy != nil {
					dz := ent.Origin(sh)[2] - enemy.Origin(sh)[2]
					if dz > 40 {
						neworg[2] -= 8
					} else if dz < 30 {
						neworg[2] += 8
					}
				}
			}

			trace := col.SV_Move(ent.Origin(sh), ent.Mins(sh), ent.Maxs(sh), neworg, srvtypes.MoveNormal, ent)

			if trace.Fraction == 1.0 {
				if flags&srvtypes.FlagSwim != 0 && col.PointContents(trace.EndPos) == bsp.ContentsEmpty {
					return false
				}

				ent.SetOrigin(sh, trace.EndPos)
				if relink {
					col.LinkEdict(ent, true)
				}
				return true
			}

			if ent.Enemy(sh) == 0 {
				return false
			}
		}

		return false
	}

	neworg[2] += stepSize
	end := neworg
	end[2] -= stepSize * 2

	trace := col.SV_Move(neworg, ent.Mins(sh), ent.Maxs(sh), end, srvtypes.MoveNormal, ent)

	if trace.AllSolid {
		return false
	}

	if trace.StartSolid {
		neworg[2] -= stepSize
		trace = col.SV_Move(neworg, ent.Mins(sh), ent.Maxs(sh), end, srvtypes.MoveNormal, ent)
		if trace.AllSolid || trace.StartSolid {
			return false
		}
	}

	if trace.Fraction == 1.0 {
		if flags&srvtypes.FlagPartialGround != 0 {
			ent.SetOrigin(sh, srvtypes.VecAdd(ent.Origin(sh), move))
			if relink {
				col.LinkEdict(ent, true)
			}
			ent.SetFlags(sh, float32(flags|srvtypes.FlagOnGround))
			return true
		}
		return false
	}

	ent.SetOrigin(sh, trace.EndPos)
	if !checkBottom(col, store, ent, sh) {
		if flags&srvtypes.FlagPartialGround != 0 {
			if relink {
				col.LinkEdict(ent, true)
			}
			return true
		}
		ent.SetOrigin(sh, oldorg)
		return false
	}

	ent.SetFlags(sh, float32(flags&^srvtypes.FlagPartialGround))

	if trace.Entity != nil {
		ent.SetGroundEntity(sh, int32(trace.Entity.Num))
	} else {
		ent.SetGroundEntity(sh, 0)
	}

	if relink {
		col.LinkEdict(ent, true)
	}

	return true
}

func stepDirection(col srvtypes.CollisionWorld, store srvtypes.EntityStore, ent *srvtypes.Edict, yaw, dist float32, sh srvtypes.ServerHandle) bool {
	ent.SetIdealYaw(sh, yaw)
	changeYaw(ent, sh)

	yawRad := yaw * (math.Pi / 180.0)
	move := [3]float32{
		dist * float32(math.Cos(float64(yawRad))),
		dist * float32(math.Sin(float64(yawRad))),
		0,
	}

	oldorigin := ent.Origin(sh)
	if moveStep(col, store, ent, move, false, sh) {
		delta := ent.Angles(sh)[1] - yaw
		if delta > 45 || delta < -45 {
			col.LinkEdict(ent, true)
		}
		return true
	}

	col.LinkEdict(ent, true)
	ent.SetOrigin(sh, oldorigin)
	return false
}

func newChaseDir(col srvtypes.CollisionWorld, store srvtypes.EntityStore, actor, enemy *srvtypes.Edict, dist float32, sh srvtypes.ServerHandle) {
	if actor == nil || enemy == nil {
		return
	}

	olddir := srvtypes.AngleMod(float32(int(actor.IdealYaw(sh)/45)) * 45)
	turnaround := srvtypes.AngleMod(olddir - 180)

	deltax := enemy.Origin(sh)[0] - actor.Origin(sh)[0]
	deltay := enemy.Origin(sh)[1] - actor.Origin(sh)[1]

	d := [3]float32{diNoDir, diNoDir, diNoDir}
	if deltax > 10 {
		d[1] = 0
	} else if deltax < -10 {
		d[1] = 180
	}

	if deltay < -10 {
		d[2] = 270
	} else if deltay > 10 {
		d[2] = 90
	}

	if d[1] != diNoDir && d[2] != diNoDir {
		tdir := float32(0)
		if d[1] == 0 {
			if d[2] == 90 {
				tdir = 45
			} else {
				tdir = 315
			}
		} else if d[2] == 90 {
			tdir = 135
		} else {
			tdir = 215
		}

		if tdir != turnaround && stepDirection(col, store, actor, tdir, dist, sh) {
			return
		}
	}

	if ((randInt()&3)&1) != 0 || int(math.Abs(float64(deltay))) > int(math.Abs(float64(deltax))) {
		d[1], d[2] = d[2], d[1]
	}

	if d[1] != diNoDir && d[1] != turnaround && stepDirection(col, store, actor, d[1], dist, sh) {
		return
	}
	if d[2] != diNoDir && d[2] != turnaround && stepDirection(col, store, actor, d[2], dist, sh) {
		return
	}

	if olddir != diNoDir && stepDirection(col, store, actor, olddir, dist, sh) {
		return
	}

	if randInt()&1 != 0 {
		for tdir := float32(0); tdir <= 315; tdir += 45 {
			if tdir != turnaround && stepDirection(col, store, actor, tdir, dist, sh) {
				return
			}
		}
	} else {
		for tdir := float32(315); tdir >= 0; tdir -= 45 {
			if tdir != turnaround && stepDirection(col, store, actor, tdir, dist, sh) {
				return
			}
		}
	}

	if turnaround != diNoDir && stepDirection(col, store, actor, turnaround, dist, sh) {
		return
	}

	actor.SetIdealYaw(sh, olddir)
	if !checkBottom(col, store, actor, sh) {
		fixCheckBottom(actor, sh)
	}
}

var prRandSeed int = 1

func randInt() int {
	prRandSeed = (prRandSeed*214013 + 2531011) & 0x7FFFFFFF
	return (prRandSeed >> 16) & 0x7FFF
}

func moveToGoal(col srvtypes.CollisionWorld, store srvtypes.EntityStore, ent *srvtypes.Edict, dist float32, sh srvtypes.ServerHandle) bool {
	if ent == nil {
		return false
	}

	flags := uint32(ent.Flags(sh))
	if flags&(srvtypes.FlagOnGround|srvtypes.FlagFly|srvtypes.FlagSwim) == 0 {
		return false
	}

	goalNum := ent.GoalEntity(sh)
	enemyNum := ent.Enemy(sh)

	var goal *srvtypes.Edict
	if goalNum != 0 {
		goal = store.EdictNum(int(goalNum))
	} else if enemyNum != 0 {
		goal = store.EdictNum(int(enemyNum))
	}

	if goal != nil {
		newChaseDir(col, store, ent, goal, dist, sh)
	}

	return true
}

func NewChaseDir(col srvtypes.CollisionWorld, store srvtypes.EntityStore, actor, enemy *srvtypes.Edict, dist float32, sh srvtypes.ServerHandle) {
	newChaseDir(col, store, actor, enemy, dist, sh)
}
