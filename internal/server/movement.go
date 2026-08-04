// This file belongs to the Physics/Collision subsystem: collision detection, spatial queries, movement, and per-entity physics simulation.

package server

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

const (
	stepSize = 18
	diNoDir  = -1
)

func (s *Server) SV_Move(start, mins, maxs, end [3]float32, moveType MoveType, passedict *Edict) TraceResult {
	return s.Move(start, mins, maxs, end, moveType, passedict)
}

func (s *Server) SV_HullForEntity(ent *Edict, mins, maxs [3]float32) (*model.Hull, [3]float32) {
	var offset [3]float32
	h := s.hullForEntity(ent, mins, maxs, &offset)
	return h, offset
}

func (s *Server) SV_TestEntityPosition(ent *Edict) *Edict {
	return s.TestEntityPosition(ent)
}

func (s *Server) changeYaw(ent *Edict) {
	current := types.AngleMod(ent.Angles(s)[1])
	ideal := ent.IdealYaw(s)
	speed := ent.YawSpeed(s)

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

	angles := ent.Angles(s)
	angles[1] = types.AngleMod(current + move)
	ent.SetAngles(s, angles)
}

func (s *Server) CheckBottom(ent *Edict) bool {
	result := s.checkBottom(ent)
	if result {
		checkBottomYes++
	} else {
		checkBottomNo++
	}
	return result
}

// checkBottomYes and checkBottomNo track CheckBottom results for debug stats.
// Matches C sv_move.c c_yes/c_no counters.
var (
	checkBottomYes int
	checkBottomNo  int
)

// CheckBottomStats returns the c_yes/c_no debug counters.
func CheckBottomStats() (yes, no int) {
	return checkBottomYes, checkBottomNo
}

// ResetCheckBottomStats clears the c_yes/c_no debug counters.
func ResetCheckBottomStats() {
	checkBottomYes = 0
	checkBottomNo = 0
}

func (s *Server) checkBottom(ent *Edict) bool {
	origin := ent.Origin(s)
	mins := VecAdd(origin, ent.Mins(s))
	maxs := VecAdd(origin, ent.Maxs(s))

	var start [3]float32
	var stop [3]float32

	start[2] = mins[2] - 1
	for x := 0; x <= 1; x++ {
		for y := 0; y <= 1; y++ {
			if x == 1 {
				start[0] = maxs[0]
			} else {
				start[0] = mins[0]
			}
			if y == 1 {
				start[1] = maxs[1]
			} else {
				start[1] = mins[1]
			}
			if s.PointContents(start) != bsp.ContentsSolid {
				goto realcheck
			}
		}
	}

	return true

realcheck:
	start[2] = mins[2]
	start[0] = (mins[0] + maxs[0]) * 0.5
	start[1] = (mins[1] + maxs[1]) * 0.5
	stop = start
	stop[2] = start[2] - 2*stepSize

	trace := s.Move(start, [3]float32{}, [3]float32{}, stop, MoveType(MoveNoMonsters), ent)
	if trace.Fraction == 1 {
		return false
	}

	mid := trace.EndPos[2]
	bottom := trace.EndPos[2]

	for x := 0; x <= 1; x++ {
		for y := 0; y <= 1; y++ {
			if x == 1 {
				start[0], stop[0] = maxs[0], maxs[0]
			} else {
				start[0], stop[0] = mins[0], mins[0]
			}
			if y == 1 {
				start[1], stop[1] = maxs[1], maxs[1]
			} else {
				start[1], stop[1] = mins[1], mins[1]
			}

			trace = s.Move(start, [3]float32{}, [3]float32{}, stop, MoveType(MoveNoMonsters), ent)
			if trace.Fraction != 1 && trace.EndPos[2] > bottom {
				bottom = trace.EndPos[2]
			}
			if trace.Fraction == 1 || mid-trace.EndPos[2] > stepSize {
				return false
			}
		}
	}

	return true
}

func (s *Server) MoveStep(ent *Edict, move [3]float32, relink bool) bool {
	oldorg := ent.Origin(s)
	neworg := VecAdd(ent.Origin(s), move)
	flags := uint32(ent.Flags(s))

	if flags&(FlagSwim|FlagFly) != 0 {
		for i := 0; i < 2; i++ {
			neworg = VecAdd(ent.Origin(s), move)
			enemy := s.EdictNum(int(ent.Enemy(s)))
			if i == 0 && enemy != nil && len(s.Edicts) > 0 && enemy != s.Edicts[0] {
				dz := ent.Origin(s)[2] - enemy.Origin(s)[2]
				if dz > 40 {
					neworg[2] -= 8
				}
				if dz < 30 {
					neworg[2] += 8
				}
			}

			trace := s.Move(ent.Origin(s), ent.Mins(s), ent.Maxs(s), neworg, MoveType(MoveNormal), ent)
			if trace.Fraction == 1 {
				if flags&FlagSwim != 0 && s.PointContents(trace.EndPos) == bsp.ContentsEmpty {
					return false
				}
				ent.SetOrigin(s, trace.EndPos)
				if relink {
					s.LinkEdict(ent, true)
				}
				return true
			}

			if enemy == nil || (len(s.Edicts) > 0 && enemy == s.Edicts[0]) {
				break
			}
		}

		return false
	}

	neworg[2] += stepSize
	end := neworg
	end[2] -= stepSize * 2

	trace := s.Move(neworg, ent.Mins(s), ent.Maxs(s), end, MoveType(MoveNormal), ent)
	if trace.AllSolid {
		return false
	}

	if trace.StartSolid {
		neworg[2] -= stepSize
		trace = s.Move(neworg, ent.Mins(s), ent.Maxs(s), end, MoveType(MoveNormal), ent)
		if trace.AllSolid || trace.StartSolid {
			return false
		}
	}

	if trace.Fraction == 1 {
		if flags&FlagPartialGround != 0 {
			ent.SetOrigin(s, VecAdd(ent.Origin(s), move))
			if relink {
				s.LinkEdict(ent, true)
			}
			ent.SetFlags(s, float32(flags&^FlagOnGround))
			return true
		}

		return false
	}

	ent.SetOrigin(s, trace.EndPos)
	if !s.CheckBottom(ent) {
		if flags&FlagPartialGround != 0 {
			if relink {
				s.LinkEdict(ent, true)
			}
			return true
		}
		ent.SetOrigin(s, oldorg)
		return false
	}

	if flags&FlagPartialGround != 0 {
		ent.SetFlags(s, float32(flags&^FlagPartialGround))
	}
	if trace.Entity != nil {
		ent.SetGroundEntity(s, int32(s.NumForEdict(trace.Entity)))
	} else {
		ent.SetGroundEntity(s, 0)
	}

	if relink {
		s.LinkEdict(ent, true)
	}
	return true
}

func (s *Server) StepDirection(ent *Edict, yaw, dist float32) bool {
	ent.SetIdealYaw(s, yaw)
	s.changeYaw(ent)

	rad := float64(yaw) * math.Pi * 2 / 360
	move := [3]float32{float32(math.Cos(rad)) * dist, float32(math.Sin(rad)) * dist, 0}

	oldorigin := ent.Origin(s)
	if s.MoveStep(ent, move, false) {
		angles := ent.Angles(s)
		delta := angles[1] - ent.IdealYaw(s)
		if delta > 45 && delta < 315 {
			ent.SetOrigin(s, oldorigin)
		}
		s.LinkEdict(ent, true)
		return true
	}

	s.LinkEdict(ent, true)
	return false
}

func (s *Server) FixCheckBottom(ent *Edict) {
	ent.SetFlags(s, float32(uint32(ent.Flags(s))|FlagPartialGround))
}

func (s *Server) CloseEnough(ent, goal *Edict, dist float32) bool {
	if ent == nil || goal == nil {
		return false
	}

	for i := 0; i < 3; i++ {
		if goal.AbsMin(s)[i] > ent.AbsMax(s)[i]+dist {
			return false
		}
		if goal.AbsMax(s)[i] < ent.AbsMin(s)[i]-dist {
			return false
		}
	}

	return true
}

func (s *Server) NewChaseDir(actor, enemy *Edict, dist float32) {
	if actor == nil || enemy == nil {
		return
	}

	olddir := types.AngleMod(float32(int(actor.IdealYaw(s)/45)) * 45)
	turnaround := types.AngleMod(olddir - 180)

	deltax := enemy.Origin(s)[0] - actor.Origin(s)[0]
	deltay := enemy.Origin(s)[1] - actor.Origin(s)[1]

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

		if tdir != turnaround && s.StepDirection(actor, tdir, dist) {
			return
		}
	}

	if ((s.compatRand()&3)&1) != 0 || int(math.Abs(float64(deltay))) > int(math.Abs(float64(deltax))) {
		d[1], d[2] = d[2], d[1]
	}

	if d[1] != diNoDir && d[1] != turnaround && s.StepDirection(actor, d[1], dist) {
		return
	}
	if d[2] != diNoDir && d[2] != turnaround && s.StepDirection(actor, d[2], dist) {
		return
	}

	if olddir != diNoDir && s.StepDirection(actor, olddir, dist) {
		return
	}

	if s.compatRand()&1 != 0 {
		for tdir := float32(0); tdir <= 315; tdir += 45 {
			if tdir != turnaround && s.StepDirection(actor, tdir, dist) {
				return
			}
		}
	} else {
		for tdir := float32(315); tdir >= 0; tdir -= 45 {
			if tdir != turnaround && s.StepDirection(actor, tdir, dist) {
				return
			}
		}
	}

	if turnaround != diNoDir && s.StepDirection(actor, turnaround, dist) {
		return
	}

	actor.SetIdealYaw(s, olddir)
	if !s.CheckBottom(actor) {
		s.FixCheckBottom(actor)
	}
}

func (s *Server) MoveToGoal(ent *Edict, dist float32) bool {
	if ent == nil {
		return false
	}

	flags := uint32(ent.Flags(s))
	if flags&(FlagOnGround|FlagFly|FlagSwim) == 0 {
		return false
	}

	goal := s.EdictNum(int(ent.GoalEntity(s)))
	enemy := s.EdictNum(int(ent.Enemy(s)))
	if goal != nil && len(s.Edicts) > 0 && enemy != nil && enemy != s.Edicts[0] && s.CloseEnough(ent, goal, dist) {
		return true
	}

	if (s.compatRand()&3) == 1 || !s.StepDirection(ent, ent.IdealYaw(s), dist) {
		if goal != nil {
			s.NewChaseDir(ent, goal, dist)
		}
	}

	return true
}

func (s *Server) compatRand() int32 {
	if s.compatRNG == nil {
		s.SetCompatRNG(nil)
	}
	return s.compatRNG.Int()
}
