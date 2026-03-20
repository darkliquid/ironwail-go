package server

import (
	"math"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/pkg/types"
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
	current := types.AngleMod(ent.Vars.Angles[1])
	ideal := ent.Vars.IdealYaw
	speed := ent.Vars.YawSpeed

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

	ent.Vars.Angles[1] = types.AngleMod(current + move)
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

func (s *Server) checkBottom(ent *Edict) bool {
	mins := VecAdd(ent.Vars.Origin, ent.Vars.Mins)
	maxs := VecAdd(ent.Vars.Origin, ent.Vars.Maxs)

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
	oldorg := ent.Vars.Origin
	neworg := VecAdd(ent.Vars.Origin, move)
	flags := uint32(ent.Vars.Flags)

	if flags&(FlagSwim|FlagFly) != 0 {
		for i := 0; i < 2; i++ {
			neworg = VecAdd(ent.Vars.Origin, move)
			enemy := s.EdictNum(int(ent.Vars.Enemy))
			if i == 0 && enemy != nil && len(s.Edicts) > 0 && enemy != s.Edicts[0] {
				dz := ent.Vars.Origin[2] - enemy.Vars.Origin[2]
				if dz > 40 {
					neworg[2] -= 8
				}
				if dz < 30 {
					neworg[2] += 8
				}
			}

			trace := s.Move(ent.Vars.Origin, ent.Vars.Mins, ent.Vars.Maxs, neworg, MoveType(MoveNormal), ent)
			if trace.Fraction == 1 {
				if flags&FlagSwim != 0 && s.PointContents(trace.EndPos) == bsp.ContentsEmpty {
					return false
				}
				ent.Vars.Origin = trace.EndPos
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

	trace := s.Move(neworg, ent.Vars.Mins, ent.Vars.Maxs, end, MoveType(MoveNormal), ent)
	if trace.AllSolid {
		return false
	}

	if trace.StartSolid {
		neworg[2] -= stepSize
		trace = s.Move(neworg, ent.Vars.Mins, ent.Vars.Maxs, end, MoveType(MoveNormal), ent)
		if trace.AllSolid || trace.StartSolid {
			return false
		}
	}

	if trace.Fraction == 1 {
		if flags&FlagPartialGround != 0 {
			ent.Vars.Origin = VecAdd(ent.Vars.Origin, move)
			if relink {
				s.LinkEdict(ent, true)
			}
			ent.Vars.Flags = float32(flags &^ FlagOnGround)
			return true
		}

		return false
	}

	ent.Vars.Origin = trace.EndPos
	if !s.CheckBottom(ent) {
		if flags&FlagPartialGround != 0 {
			if relink {
				s.LinkEdict(ent, true)
			}
			return true
		}
		ent.Vars.Origin = oldorg
		return false
	}

	if flags&FlagPartialGround != 0 {
		ent.Vars.Flags = float32(flags &^ FlagPartialGround)
	}
	if trace.Entity != nil {
		ent.Vars.GroundEntity = int32(s.NumForEdict(trace.Entity))
	} else {
		ent.Vars.GroundEntity = 0
	}

	if relink {
		s.LinkEdict(ent, true)
	}
	return true
}

func (s *Server) StepDirection(ent *Edict, yaw, dist float32) bool {
	ent.Vars.IdealYaw = yaw
	s.changeYaw(ent)

	rad := float64(yaw) * math.Pi * 2 / 360
	move := [3]float32{float32(math.Cos(rad)) * dist, float32(math.Sin(rad)) * dist, 0}

	oldorigin := ent.Vars.Origin
	if s.MoveStep(ent, move, false) {
		delta := ent.Vars.Angles[1] - ent.Vars.IdealYaw
		if delta > 45 && delta < 315 {
			ent.Vars.Origin = oldorigin
		}
		s.LinkEdict(ent, true)
		return true
	}

	s.LinkEdict(ent, true)
	return false
}

func (s *Server) FixCheckBottom(ent *Edict) {
	ent.Vars.Flags = float32(uint32(ent.Vars.Flags) | FlagPartialGround)
}

func (s *Server) CloseEnough(ent, goal *Edict, dist float32) bool {
	if ent == nil || goal == nil {
		return false
	}

	for i := 0; i < 3; i++ {
		if goal.Vars.AbsMin[i] > ent.Vars.AbsMax[i]+dist {
			return false
		}
		if goal.Vars.AbsMax[i] < ent.Vars.AbsMin[i]-dist {
			return false
		}
	}

	return true
}

func (s *Server) NewChaseDir(actor, enemy *Edict, dist float32) {
	if actor == nil || enemy == nil {
		return
	}

	olddir := types.AngleMod(float32(int(actor.Vars.IdealYaw/45)) * 45)
	turnaround := types.AngleMod(olddir - 180)

	deltax := enemy.Vars.Origin[0] - actor.Vars.Origin[0]
	deltay := enemy.Vars.Origin[1] - actor.Vars.Origin[1]

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
			tdir = 225
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

	actor.Vars.IdealYaw = olddir
	if !s.CheckBottom(actor) {
		s.FixCheckBottom(actor)
	}
}

func (s *Server) MoveToGoal(ent *Edict, dist float32) bool {
	if ent == nil {
		return false
	}

	flags := uint32(ent.Vars.Flags)
	if flags&(FlagOnGround|FlagFly|FlagSwim) == 0 {
		return false
	}

	goal := s.EdictNum(int(ent.Vars.GoalEntity))
	enemy := s.EdictNum(int(ent.Vars.Enemy))
	if goal != nil && len(s.Edicts) > 0 && enemy != nil && enemy != s.Edicts[0] && s.CloseEnough(ent, goal, dist) {
		return true
	}

	if (s.compatRand()&3) == 1 || !s.StepDirection(ent, ent.Vars.IdealYaw, dist) {
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
