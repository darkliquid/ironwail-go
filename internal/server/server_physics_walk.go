// This file belongs to the Physics/Collision subsystem: collision detection, spatial queries, movement, and per-entity physics simulation.
//
// The walking/tossing movement algorithms (SV_WalkMove, SV_WallFriction,
// PhysicsWalk, SV_CheckStuck) have moved to internal/server/physics
// (physics.System). This file now provides thin *Server delegators.
package server

// SV_WalkMove implements the walk-move stair-step algorithm.
func (s *Server) SV_WalkMove(ent *Edict) {
	s.ensurePhysicsSys()
	s.PhysicsSys.SV_WalkMove(ent)
}

// SV_WallFriction applies extra friction based on view angle against a wall.
func (s *Server) SV_WallFriction(ent *Edict, trace *TraceResult) {
	s.ensurePhysicsSys()
	s.PhysicsSys.SV_WallFriction(ent, trace)
}

// PhysicsWalk handles walking physics with client pre/post think.
func (s *Server) PhysicsWalk(ent *Edict) {
	s.ensurePhysicsSys()
	s.PhysicsSys.PhysicsWalk(ent)
}

// SV_CheckStuck checks and unsticks an entity that overlaps solid.
func (s *Server) SV_CheckStuck(ent *Edict) {
	s.ensurePhysicsSys()
	s.PhysicsSys.SV_CheckStuck(ent)
}
