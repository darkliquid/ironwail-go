// system.go implements the standalone Physics System component.
package physics

import (
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// System encapsulates physics, collision resolution, and monster movement.
type System struct {
	col    srvtypes.CollisionWorld
	store  srvtypes.EntityStore
	sh     srvtypes.ServerHandle
	facade srvtypes.PhysicsFacade
}

// NewSystem creates a new physics System with injected dependencies.
func NewSystem(
	col srvtypes.CollisionWorld,
	store srvtypes.EntityStore,
	sh srvtypes.ServerHandle,
) *System {
	return &System{
		col:   col,
		store: store,
		sh:    sh,
	}
}

// NewSystemWithFacade creates a physics System that can also drive the
// per-entity leaf algorithms (FlyMove, PushMove, PhysicsWalk, ...) which need
// the server facade (QC callbacks, telemetry, sounds, scratch buffers).
func NewSystemWithFacade(
	col srvtypes.CollisionWorld,
	store srvtypes.EntityStore,
	sh srvtypes.ServerHandle,
	facade srvtypes.PhysicsFacade,
) *System {
	return &System{
		col:    col,
		store:  store,
		sh:     sh,
		facade: facade,
	}
}

// CheckBottom checks if an entity is supported on solid ground.
func (s *System) CheckBottom(ent *srvtypes.Edict) bool {
	return checkBottom(s.col, s.store, ent, s.sh)
}

// MoveStep attempts to step an entity forward in a direction.
func (s *System) MoveStep(ent *srvtypes.Edict, move [3]float32, relink bool) bool {
	return moveStep(s.col, s.store, ent, move, relink, s.sh)
}

// StepDirection turns and steps an entity toward a specific yaw angle.
func (s *System) StepDirection(ent *srvtypes.Edict, yaw, dist float32) bool {
	return stepDirection(s.col, s.store, ent, yaw, dist, s.sh)
}

// MoveToGoal moves a monster entity toward its current goal or enemy.
func (s *System) MoveToGoal(ent *srvtypes.Edict, dist float32) bool {
	return moveToGoal(s.col, s.store, ent, dist, s.sh)
}

// NewChaseDir drives monster chase direction selection.
func (s *System) NewChaseDir(actor, enemy *srvtypes.Edict, dist float32) {
	NewChaseDir(s.col, s.store, actor, enemy, dist, s.sh)
}
