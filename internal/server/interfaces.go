// interfaces.go defines the core subsystem contracts (Collision, Entities, Physics, QC)
// that decouple the monolithic Server struct into standalone, mockable components,
// along with the Server receiver methods implementing the contract getters.

// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.
package server

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
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
	ClientThinker      = srvtypes.ClientThinker
	FrameDriver        = srvtypes.FrameDriver
	CVarReader         = srvtypes.CVarReader
	TelemetrySink      = srvtypes.TelemetrySink
	CvarHandle         = srvtypes.CvarHandle
	PhysicsFacade      = srvtypes.PhysicsFacade
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

// TelemetrySink implementation: forwards to the DebugTelemetry engine.
func (s *Server) EventsEnabled() bool {
	return s != nil && s.DebugTelemetry != nil && s.DebugTelemetry.EventsEnabled()
}

// CVarReader implementation: forwards to the CVar registry.
func (s *Server) BoolValue(name string) bool {
	if s == nil || s.CVar == nil {
		return false
	}
	return s.CVar.BoolValue(name)
}

func (s *Server) Get(name string) CvarHandle {
	if s == nil || s.CVar == nil {
		return nil
	}
	cv := s.CVar.Get(name)
	if cv == nil {
		return nil
	}
	return cv
}

func (s *Server) BeginFrame(serverTime, frameTime float32) {
	if s != nil && s.DebugTelemetry != nil {
		s.DebugTelemetry.BeginFrame(serverTime, frameTime)
	}
}

func (s *Server) EndFrame() {
	if s != nil && s.DebugTelemetry != nil {
		s.DebugTelemetry.EndFrame()
	}
}

func (s *Server) LogEventf(kind DebugEventKind, vm *qc.VM, entNum int, ent *Edict, format string, args ...any) bool {
	if s == nil || s.DebugTelemetry == nil {
		return false
	}
	return s.DebugTelemetry.LogEventf(kind, vm, entNum, ent, format, args...)
}

// PhysicsFacade implementation: thin forwards to server state.
func (s *Server) FloatValue(name string) float64 {
	if s == nil || s.CVar == nil {
		return 0
	}
	return s.CVar.FloatValue(name)
}

func (s *Server) SuppressTouchQC() bool {
	return s != nil && s.suppressTouchQC
}

func (s *Server) DebugTriggerTouch(source string, touch, other *Edict) {
	if s != nil {
		s.debugTriggerTouch(source, touch, other)
	}
}

func (s *Server) PushMoveScratch() (moved *[]*Edict, from *[]qtypes.Vec3) {
	if s == nil {
		return nil, nil
	}
	return &s.pushMoveMoved, &s.pushMoveFrom
}

func (s *Server) CaptureExecutionContext() any {
	if s == nil || s.QCVM == nil {
		return nil
	}
	return captureQCExecutionContext(s.QCVM)
}

func (s *Server) RestoreExecutionContext(ctx any) {
	if s == nil || s.QCVM == nil || ctx == nil {
		return
	}
	if c, ok := ctx.(qcExecutionContext); ok {
		restoreQCExecutionContext(s.QCVM, c)
	}
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
