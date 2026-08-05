// interfaces.go defines the core subsystem contracts (Collision, Entities, Physics, QC)
// that decouple the monolithic Server struct into standalone, mockable components,
// along with the Server receiver methods implementing the contract getters.

// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.
package server

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

type (
	CollisionWorld     = srvtypes.CollisionWorld
	EntityStore        = srvtypes.EntityStore
	PhysicsConfig      = srvtypes.PhysicsConfig
	FrameTiming        = srvtypes.FrameTiming
	ThinkExecutor      = srvtypes.ThinkExecutor
	PhysicsEngine      = srvtypes.PhysicsEngine
	MovementEngine     = srvtypes.MovementEngine
	NetworkBroadcaster = srvtypes.NetworkBroadcaster
)

func (s *Server) GetGravity() float32     { return s.Gravity }
func (s *Server) GetMaxVelocity() float32 { return s.MaxVelocity }
func (s *Server) GetFriction() float32    { return s.Friction }
func (s *Server) GetStopSpeed() float32   { return s.StopSpeed }
func (s *Server) GetTime() float32        { return s.Time }
func (s *Server) GetFrameTime() float32   { return s.FrameTime }

func (s *Server) GetVM() *qc.VM {
	if s == nil {
		return nil
	}
	return s.QCVM
}

func (s *Server) GetFieldAlpha() int {
	if s == nil {
		return -1
	}
	return s.QCFieldAlpha
}
func (s *Server) GetFieldScale() int {
	if s == nil {
		return -1
	}
	return s.QCFieldScale
}
func (s *Server) GetFieldGravity() int {
	if s == nil {
		return -1
	}
	return s.QCFieldGravity
}
func (s *Server) GetFieldItems2() int {
	if s == nil {
		return -1
	}
	return s.QCFieldItems2
}
func (s *Server) GetFieldState() int {
	if s == nil {
		return -1
	}
	return s.QCFieldState
}
func (s *Server) GetFieldWait() int {
	if s == nil {
		return -1
	}
	return s.QCFieldWait
}
func (s *Server) GetFieldSpeed() int {
	if s == nil {
		return -1
	}
	return s.QCFieldSpeed
}
func (s *Server) GetFieldCustomFlags() int {
	if s == nil {
		return -1
	}
	return s.QCFieldCustomFlags
}
func (s *Server) GetFieldThCheckAttack() int {
	if s == nil {
		return -1
	}
	return s.QCFieldThCheckAttack
}
func (s *Server) GetFieldMap() int {
	if s == nil {
		return -1
	}
	return s.QCFieldMap
}

func (s *Server) ExecuteQCFunction(funcIdx int) error {
	if s == nil {
		return nil
	}
	return s.executeQCFunction(funcIdx)
}
