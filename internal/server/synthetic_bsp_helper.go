// This file belongs to the Physics/Collision subsystem: collision detection, spatial queries, movement, and per-entity physics simulation.

package server

import (
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// CreateSyntheticWorldModel returns a tiny world model with a single
// horizontal plane at z=0 (points with z>=0 are empty, below are solid).
// This is sufficient for deterministic movement/trace unit tests.
func CreateSyntheticWorldModel() *model.Model {
	m := &model.Model{}

	var hull model.Hull
	hull.Planes = make([]model.MPlane, 1)
	hull.ClipNodes = make([]model.MClipNode, 1)

	// Plane: z >= 0 is front (empty); z < 0 is back (solid)
	hull.Planes[0] = model.MPlane{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 0, Type: 2}
	hull.ClipNodes[0] = model.MClipNode{PlaneNum: 0, Children: [2]int{-1, -2}}

	hull.FirstClipNode = 0
	hull.LastClipNode = 0
	hull.ClipMins = types.Vec3{X: -512, Y: -512, Z: 0}
	hull.ClipMaxs = types.Vec3{X: 512, Y: 512, Z: 512}

	m.Hulls[0] = hull
	m.Mins = types.Vec3{X: -512, Y: -512, Z: 0}
	m.Maxs = types.Vec3{X: 512, Y: 512, Z: 512}
	m.ClipBox = true
	m.ClipMins = m.Mins
	m.ClipMaxs = m.Maxs

	return m
}
