// physics_system.go implements the PhysicsEngine and MovementEngine interfaces
// as a standalone, dependency-injected component.

// This file belongs to the Physics/Collision subsystem: collision detection, spatial queries, movement, and per-entity physics simulation.
package server

// PhysicsSystem encapsulates per-frame physics execution and entity movement.
type PhysicsSystem struct {
	collision CollisionWorld
	entities  EntityStore
	config    PhysicsConfig
	timing    FrameTiming
	executor  ThinkExecutor
	server    *Server
}

// NewPhysicsSystem creates a new PhysicsSystem with injected dependencies.
func NewPhysicsSystem(col CollisionWorld, store EntityStore, cfg PhysicsConfig, timing FrameTiming, exec ThinkExecutor, s *Server) *PhysicsSystem {
	return &PhysicsSystem{
		collision: col,
		entities:  store,
		config:    cfg,
		timing:    timing,
		executor:  exec,
		server:    s,
	}
}

// CheckBottom delegates ground checking to the movement subsystem.
func (p *PhysicsSystem) CheckBottom(ent *Edict) bool {
	return checkBottom(p.collision, ent, p.server)
}

// MoveStep delegates step movement to the movement subsystem.
func (p *PhysicsSystem) MoveStep(ent *Edict, move [3]float32, relink bool) bool {
	return moveStep(p.collision, p.entities, ent, move, relink, p.server)
}

// StepDirection delegates direction stepping to the movement subsystem.
func (p *PhysicsSystem) StepDirection(ent *Edict, yaw, dist float32) bool {
	return stepDirection(p.collision, p.entities, ent, yaw, dist, p.server)
}

// MoveToGoal delegates monster goal movement to the movement subsystem.
func (p *PhysicsSystem) MoveToGoal(ent *Edict, dist float32) bool {
	return moveToGoal(p.collision, p.entities, ent, dist, p.server)
}
