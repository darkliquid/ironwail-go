// interfaces.go defines the core subsystem contracts (Collision, Entities, Physics, QC)
// that decouple the monolithic Server struct into standalone, mockable components,
// along with the Server receiver methods implementing the contract getters.

// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.
package server

import (
	"github.com/darkliquid/ironwail-go/internal/model"
)

// CollisionWorld defines the contract for BSP collision queries, trace sweeps, and spatial entity partitioning.
type CollisionWorld interface {
	// SV_Move performs a sweep test of a bounding box along a ray.
	SV_Move(start, mins, maxs, end [3]float32, moveType MoveType, passedict *Edict) TraceResult

	// SV_TestEntityPosition checks if an edict collides with any solid entity at its current position.
	SV_TestEntityPosition(ent *Edict) *Edict

	// SV_HullForEntity retrieves the collision hull for world geometry or brush models.
	SV_HullForEntity(ent *Edict, mins, maxs [3]float32) (*model.Hull, [3]float32)

	// LinkEdict inserts an entity into the spatial AreaNode tree for collision/touch testing.
	LinkEdict(ent *Edict, touchTriggers bool)

	// PointContents returns the content flags for a point in the world.
	PointContents(p [3]float32) int
}

// EntityStore defines edict allocation, retrieval, and lifetime bounds.
type EntityStore interface {
	// EdictNum returns the entity at index num (0 = worldspawn).
	EdictNum(num int) *Edict

	// AllocEdict allocates a new edict.
	AllocEdict() *Edict

	// FreeEdict releases an edict back to the free list.
	FreeEdict(ed *Edict)
}

// PhysicsConfig provides environmental physics constants.
type PhysicsConfig interface {
	GetGravity() float32
	GetMaxVelocity() float32
	GetFriction() float32
	GetStopSpeed() float32
}

// FrameTiming provides frame timing state.
type FrameTiming interface {
	GetTime() float32
	GetFrameTime() float32
}

// ThinkExecutor abstracts QuakeC function execution and entity think routines.
type ThinkExecutor interface {
	// RunThink executes an entity's .nextthink function if due.
	RunThink(ent *Edict) bool

	// ExecuteQCFunction calls a QuakeC bytecode function index in the VM.
	ExecuteQCFunction(funcIdx int) error
}

// PhysicsEngine defines the contract for frame-level entity simulation.
type PhysicsEngine interface {
	// Physics runs per-frame physics for all active edicts.
	Physics()
	// PhysicsWalk handles walking physics for an entity.
	PhysicsWalk(ent *Edict)
	// PhysicsToss handles ballistic/toss physics for an entity.
	PhysicsToss(ent *Edict)
	// PhysicsPusher handles platform/door movement and riding entity pushes.
	PhysicsPusher(ent *Edict)
}

// MovementEngine defines the contract for entity movement and pathfinding.
type MovementEngine interface {
	// CheckBottom checks if an entity is supported on solid ground.
	CheckBottom(ent *Edict) bool
	// MoveStep attempts to step an entity forward in a direction.
	MoveStep(ent *Edict, move [3]float32, relink bool) bool
	// StepDirection turns and steps an entity toward a specific yaw angle.
	StepDirection(ent *Edict, yaw, dist float32) bool
	// MoveToGoal moves a monster entity toward its current goal or enemy.
	MoveToGoal(ent *Edict, dist float32) bool
}

// NetworkBroadcaster defines the contract for network packet broadcasts and sound events.
type NetworkBroadcaster interface {
	StartParticle(org, dir [3]float32, color, count int)
	StartSound(ent *Edict, channel int, sample string, volume int, attenuation float32)
}




// Interface getters for PhysicsConfig and FrameTiming.

func (s *Server) GetGravity() float32     { return s.Gravity }
func (s *Server) GetMaxVelocity() float32 { return s.MaxVelocity }
func (s *Server) GetFriction() float32    { return s.Friction }
func (s *Server) GetStopSpeed() float32   { return s.StopSpeed }
func (s *Server) GetTime() float32        { return s.Time }
func (s *Server) GetFrameTime() float32   { return s.FrameTime }

// ExecuteQCFunction calls a QuakeC bytecode function index in the VM.
func (s *Server) ExecuteQCFunction(funcIdx int) error {
	return s.executeQCFunction(funcIdx)
}
