// This file belongs to the Physics/Collision subsystem: collision detection, spatial queries, movement, and per-entity physics simulation.
//
// The walking/tossing movement algorithms (SV_WalkMove, SV_WallFriction,
// PhysicsWalk, SV_CheckStuck, PhysicsToss) have moved to
// internal/server/physics (physics.System). This file now provides thin
// *Server delegators.
package server

import (
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// WalkMoveNeedsUnstick reports whether a forward move was obstructed by a low
// step, mirroring the C Ironwail SV_WalkMove unstick heuristic.
func WalkMoveNeedsUnstick(oldOrg, newOrg [3]float32) bool {
	return srvtypes.WalkMoveNeedsUnstick(oldOrg, newOrg)
}

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

// PhysicsToss handles ballistic/toss movement with bounce backoff.
func (s *Server) PhysicsToss(ent *Edict) {
	s.ensurePhysicsSys()
	s.PhysicsSys.PhysicsToss(ent)
}
