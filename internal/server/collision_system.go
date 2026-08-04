// collision_system.go implements the CollisionWorld interface as a standalone component.

// This file belongs to the Physics/Collision subsystem: collision detection, spatial queries, movement, and per-entity physics simulation.
package server

import (
	"github.com/darkliquid/ironwail-go/internal/model"
)

// CollisionSystem encapsulates BSP collision detection, ray/box sweep tracing, and spatial partitioning.
type CollisionSystem struct {
	server *Server
}

// NewCollisionSystem creates a new CollisionSystem wrapping spatial queries and tracing.
func NewCollisionSystem(s *Server) *CollisionSystem {
	return &CollisionSystem{
		server: s,
	}
}

// SV_Move performs a sweep test of a bounding box along a ray through the BSP world.
func (c *CollisionSystem) SV_Move(start, mins, maxs, end [3]float32, moveType MoveType, passedict *Edict) TraceResult {
	return c.server.Move(start, mins, maxs, end, moveType, passedict)
}

// SV_TestEntityPosition checks if an edict collides with any solid entity at its position.
func (c *CollisionSystem) SV_TestEntityPosition(ent *Edict) *Edict {
	return c.server.TestEntityPosition(ent)
}

// SV_HullForEntity retrieves the collision hull for world geometry or brush models.
func (c *CollisionSystem) SV_HullForEntity(ent *Edict, mins, maxs [3]float32) (*model.Hull, [3]float32) {
	return c.server.SV_HullForEntity(ent, mins, maxs)
}

// LinkEdict inserts an entity into the spatial AreaNode tree for collision and touch queries.
func (c *CollisionSystem) LinkEdict(ent *Edict, touchTriggers bool) {
	c.server.LinkEdict(ent, touchTriggers)
}

// PointContents returns the content flags (empty, solid, water, slime, lava) for a 3D coordinate.
func (c *CollisionSystem) PointContents(p [3]float32) int {
	return c.server.PointContents(p)
}
