// interfaces.go defines the core subsystem contracts (Collision, Entities, Physics, QC).
package types

import (
	"github.com/darkliquid/ironwail-go/internal/model"
)

const (
	DistEpsilon = float32(0.03125)
	AreaDepth   = 4
	AreaNodes   = 2 << AreaDepth
)

// CollisionModel abstracts the collision-relevant aspects of a BSP world model.
type CollisionModel interface {
	ModelType() int
	NumHulls() int
	Hull(index int) model.Hull
	CollisionClipNodes() []model.MClipNode
	CollisionPlanes() []model.MPlane
	IsClipBox() bool
	CollisionClipMins() [3]float32
	CollisionClipMaxs() [3]float32
}

// CollisionWorld defines the contract for BSP collision queries, trace sweeps, and spatial entity partitioning.
type CollisionWorld interface {
	SV_Move(start, mins, maxs, end [3]float32, moveType MoveType, passedict *Edict) TraceResult
	SV_TestEntityPosition(ent *Edict) *Edict
	SV_HullForEntity(ent *Edict, mins, maxs [3]float32) (*model.Hull, [3]float32)
	LinkEdict(ent *Edict, touchTriggers bool)
	PointContents(p [3]float32) int
}

// EntityStore defines edict allocation, retrieval, and lifetime bounds.
type EntityStore interface {
	EdictNum(num int) *Edict
	AllocEdict() *Edict
	FreeEdict(ed *Edict)
}

// PhysicsConfig defines dynamic server CVars affecting entity physics.
type PhysicsConfig interface {
	GetGravity() float32
	GetMaxVelocity() float32
	GetFriction() float32
	GetStopSpeed() float32
}

// FrameTiming defines frame timing parameters.
type FrameTiming interface {
	GetTime() float32
	GetFrameTime() float32
}

// ThinkExecutor executes entity think functions and QuakeC bytecode functions.
type ThinkExecutor interface {
	RunThink(ent *Edict) bool
	ExecuteQCFunction(funcIdx int) error
}

// PhysicsEngine encapsulates per-entity physics simulation.
type PhysicsEngine interface {
	CheckVelocity(ent *Edict)
	AddGravity(ent *Edict)
	SV_CheckWater(ent *Edict) bool
	PushEntity(ent *Edict, push [3]float32) TraceResult
}

// MovementEngine encapsulates monster navigation and pathfinding.
type MovementEngine interface {
	CheckBottom(ent *Edict) bool
	MoveStep(ent *Edict, move [3]float32, relink bool) bool
	StepDirection(ent *Edict, yaw, dist float32) bool
	MoveToGoal(ent *Edict, dist float32) bool
}

// NetworkBroadcaster defines the contract for network packet broadcasts and sound events.
type NetworkBroadcaster interface {
	StartParticle(org, dir [3]float32, color, count int)
	StartSound(ent *Edict, channel int, sample string, volume int, attenuation float32)
}
