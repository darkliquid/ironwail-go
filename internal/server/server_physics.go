// This file belongs to the Physics/Collision subsystem: collision detection, spatial queries, movement, and per-entity physics simulation.
//
// The per-entity physics leaf algorithms (velocity checks, gravity, water,
// FlyMove, PushEntity, PushMove, movetype dispatchers, unstick helpers) have
// moved to internal/server/physics (physics.System). This file now provides
// thin *Server delegators so existing call sites compile unchanged.
package server

// CheckVelocity clamps an entity's velocity components to MaxVelocity bounds.
func (s *Server) CheckVelocity(ent *Edict) {
	s.ensurePhysicsSys()
	s.PhysicsSys.CheckVelocity(ent)
}

// RunThink executes the entity's think function if its nextthink time has been
// reached. Returns false if the entity was freed by its think.
func (s *Server) RunThink(ent *Edict) bool {
	s.ensurePhysicsSys()
	return s.PhysicsSys.RunThink(ent)
}

// Impact runs touch functions for two entities that have collided.
func (s *Server) Impact(e1, e2 *Edict) {
	s.ensurePhysicsSys()
	s.PhysicsSys.Impact(e1, e2)
}

// AddGravity applies frame gravity acceleration to an entity's Z velocity.
func (s *Server) AddGravity(ent *Edict) {
	s.ensurePhysicsSys()
	s.PhysicsSys.AddGravity(ent)
}

// SV_CheckWater checks if an entity is submerged in liquid.
func (s *Server) SV_CheckWater(ent *Edict) bool {
	s.ensurePhysicsSys()
	return s.PhysicsSys.SV_CheckWater(ent)
}

// CheckWaterTransition plays the splash sound and updates water type when an
// entity crosses a liquid boundary.
func (s *Server) CheckWaterTransition(ent *Edict) {
	s.ensurePhysicsSys()
	s.PhysicsSys.CheckWaterTransition(ent)
}

// FlyMove integrates velocity across up to 4 sliding planes.
func (s *Server) FlyMove(ent *Edict, time float32, steptrace *TraceResult) int {
	s.ensurePhysicsSys()
	return s.PhysicsSys.FlyMove(ent, time, steptrace)
}

// PushEntity moves an entity by a push vector, clipping against solid geometry.
func (s *Server) PushEntity(ent *Edict, push [3]float32) TraceResult {
	s.ensurePhysicsSys()
	return s.PhysicsSys.PushEntity(ent, push)
}

// PushMove moves all entities riding or overlapping a pusher.
func (s *Server) PushMove(pusher *Edict, movetime float32) {
	s.ensurePhysicsSys()
	s.PhysicsSys.PushMove(pusher, movetime)
}

// PhysicsNone, PhysicsNoClip, and PhysicsToss delegators were removed: the
// per-entity movetype dispatch now lives entirely inside physics.System
// (StepFrame calls its own leafs directly), so the Server-level wrappers had
// no remaining callers.

// PhysicsPusher moves a pusher by its velocity and runs its think when due.
func (s *Server) PhysicsPusher(ent *Edict) {
	s.ensurePhysicsSys()
	s.PhysicsSys.PhysicsPusher(ent)
}

// PhysicsStep handles step movement with gravity and landing sounds.
func (s *Server) PhysicsStep(ent *Edict) {
	s.ensurePhysicsSys()
	s.PhysicsSys.PhysicsStep(ent)
}

// SV_CheckAllEnts warns about entities in invalid positions.
func (s *Server) SV_CheckAllEnts() {
	s.ensurePhysicsSys()
	s.PhysicsSys.SV_CheckAllEnts()
}

// SV_TryUnstick attempts to unstick an entity by nudging in 8 directions.
func (s *Server) SV_TryUnstick(ent *Edict, oldVel [3]float32) int {
	s.ensurePhysicsSys()
	return s.PhysicsSys.SV_TryUnstick(ent, oldVel)
}
