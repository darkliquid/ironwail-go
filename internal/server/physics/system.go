// system.go implements the standalone Physics System component.
package physics

import (
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// System encapsulates physics, collision resolution, and monster movement.
type System struct {
	col    srvtypes.CollisionWorld
	store  srvtypes.EntityStore
	cfg    srvtypes.PhysicsConfig
	timing srvtypes.FrameTiming
	exec   srvtypes.ThinkExecutor
	sh     srvtypes.ServerHandle
}

// NewSystem creates a new physics System with injected dependencies.
func NewSystem(
	col srvtypes.CollisionWorld,
	store srvtypes.EntityStore,
	cfg srvtypes.PhysicsConfig,
	timing srvtypes.FrameTiming,
	exec srvtypes.ThinkExecutor,
	sh srvtypes.ServerHandle,
) *System {
	return &System{
		col:    col,
		store:  store,
		cfg:    cfg,
		timing: timing,
		exec:   exec,
		sh:     sh,
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

// CheckVelocity clamps an entity's velocity components to MaxVelocity bounds.
func (s *System) CheckVelocity(ent *srvtypes.Edict) {
	CheckVelocity(s.cfg, ent, s.sh)
}

// AddGravity applies frame gravity acceleration to an entity's Z velocity.
func (s *System) AddGravity(ent *srvtypes.Edict) {
	AddGravity(s.cfg, s.timing, ent, s.sh)
}

// SV_CheckWater checks if an entity is submerged in liquid.
func (s *System) SV_CheckWater(ent *srvtypes.Edict) bool {
	return SV_CheckWater(s.col, ent, s.sh)
}

// PushEntity moves an entity by a push vector, clipping against solid geometry.
func (s *System) PushEntity(ent *srvtypes.Edict, push [3]float32) srvtypes.TraceResult {
	return PushEntity(s.col, ent, push, s.sh)
}

// FlyMove integrates velocity across sliding planes for an entity over time.
func (s *System) FlyMove(ent *srvtypes.Edict, time float32) int {
	return FlyMove(s.col, s.cfg, s.timing, ent, time, s.sh)
}
