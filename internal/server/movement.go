// This file belongs to the Physics/Collision subsystem: collision detection, spatial queries, movement, and per-entity physics simulation.

package server

import (
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

func (s *Server) ensurePhysicsSys() {
	if s.PhysicsSys == nil {
		if s.CollisionSys == nil {
			s.CollisionSys = NewCollisionSystem(s)
		}
		s.PhysicsSys = NewPhysicsSystem(s.CollisionSys, s, s)
	}
}

func (s *Server) CheckBottom(ent *Edict) bool {
	s.ensurePhysicsSys()
	result := s.PhysicsSys.CheckBottom(ent)
	if result {
		checkBottomYes++
	} else {
		checkBottomNo++
	}
	return result
}

var (
	checkBottomYes int
	checkBottomNo  int
)

func CheckBottomStats() (yes, no int) {
	return checkBottomYes, checkBottomNo
}

func ResetCheckBottomStats() {
	checkBottomYes = 0
	checkBottomNo = 0
}

func (s *Server) MoveStep(ent *Edict, move [3]float32, relink bool) bool {
	s.ensurePhysicsSys()
	return s.PhysicsSys.MoveStep(ent, move, relink)
}

func (s *Server) StepDirection(ent *Edict, yaw, dist float32) bool {
	s.ensurePhysicsSys()
	return s.PhysicsSys.StepDirection(ent, yaw, dist)
}

func (s *Server) MoveToGoal(ent *Edict, dist float32) bool {
	s.ensurePhysicsSys()
	return s.PhysicsSys.MoveToGoal(ent, dist)
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

func (s *Server) CloseEnough(ent, goal *Edict, dist float32) bool {
	entAbsMin := ent.AbsMin(s)
	entAbsMax := ent.AbsMax(s)
	goalAbsMin := goal.AbsMin(s)
	goalAbsMax := goal.AbsMax(s)
	for i := 0; i < 3; i++ {
		if goalAbsMin[i] > entAbsMax[i]+dist || goalAbsMax[i] < entAbsMin[i]-dist {
			return false
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

func (s *Server) NewChaseDir(actor, enemy *Edict, dist float32) {
	s.ensurePhysicsSys()
	s.PhysicsSys.NewChaseDir(actor, enemy, dist)
}
