// physics_system.go re-exports physics.System for package server.
package server

import (
	"github.com/darkliquid/ironwail-go/internal/server/physics"
)

// PhysicsSystem wraps physics.System.
type PhysicsSystem = physics.System

// NewPhysicsSystem creates a new physics.System instance.
func NewPhysicsSystem(col CollisionWorld, store EntityStore, s *Server) *PhysicsSystem {
	return physics.NewSystem(col, store, s)
}
