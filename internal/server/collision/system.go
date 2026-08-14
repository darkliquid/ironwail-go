// system.go implements the standalone Collision System component.
package collision

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// WorldProvider provides access to BSP world geometry and tree structures.
type WorldProvider interface {
	GetWorldModel() srvtypes.CollisionModel
	GetWorldTree() *bsp.Tree
}

// TouchProvider handles trigger touch callbacks during spatial edict linking.
type TouchProvider interface {
	TouchLinks(ent *srvtypes.Edict)
}

// System encapsulates BSP collision detection, ray/box sweep tracing, and spatial partitioning.
type System struct {
	world        WorldProvider
	store        srvtypes.EntityStore
	touch        TouchProvider
	sh           srvtypes.ServerHandle
	areanodes    []AreaNode
	numAreaNodes int
}

// NewSystem creates a new collision System with injected dependencies.
func NewSystem(
	world WorldProvider,
	store srvtypes.EntityStore,
	touch TouchProvider,
	sh srvtypes.ServerHandle,
) *System {
	sys := &System{
		world: world,
		store: store,
		touch: touch,
		sh:    sh,
	}
	sys.ClearWorld()
	return sys
}

// NumAreaNodes returns the count of active spatial area nodes.
func (c *System) NumAreaNodes() int {
	return c.numAreaNodes
}

// SV_Move performs a sweep test of a bounding box along a ray through the BSP world.
func (c *System) SV_Move(start, mins, maxs, end qtypes.Vec3, moveType srvtypes.MoveType, passedict *srvtypes.Edict) srvtypes.TraceResult {
	return c.Move(start, mins, maxs, end, moveType, passedict)
}

// SV_TestEntityPosition checks if an edict collides with any solid entity at its position.
func (c *System) SV_TestEntityPosition(ent *srvtypes.Edict) *srvtypes.Edict {
	return c.TestEntityPosition(ent)
}

// SV_HullForEntity retrieves the collision hull for world geometry or brush models.
func (c *System) SV_HullForEntity(ent *srvtypes.Edict, mins, maxs qtypes.Vec3) (*model.Hull, qtypes.Vec3) {
	var offset qtypes.Vec3
	h := c.hullForEntity(ent, mins, maxs, &offset)
	return h, offset
}

// ClipMoveToEntity clips a move against a single entity.
func (c *System) ClipMoveToEntity(ent *srvtypes.Edict, start, mins, maxs, end qtypes.Vec3) srvtypes.TraceResult {
	return c.clipMoveToEntity(ent, start, mins, maxs, end)
}
