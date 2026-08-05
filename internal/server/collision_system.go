// collision_system.go re-exports collision.System for package server.
package server

import (
	"github.com/darkliquid/ironwail-go/internal/server/collision"
)

// CollisionSystem wraps collision.System.
type CollisionSystem = collision.System

// NewCollisionSystem creates a new collision.System instance.
func NewCollisionSystem(s *Server) *CollisionSystem {
	return collision.NewSystem(s, s, s, s)
}
