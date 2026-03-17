package server

import (
	"github.com/ironwail/ironwail-go/internal/model"
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
	hull.Planes[0] = model.MPlane{Normal: [3]float32{0, 0, 1}, Dist: 0, Type: 2}
	hull.ClipNodes[0] = model.MClipNode{PlaneNum: 0, Children: [2]int{-1, -2}}

	hull.FirstClipNode = 0
	hull.LastClipNode = 0
	hull.ClipMins = [3]float32{-512, -512, 0}
	hull.ClipMaxs = [3]float32{512, 512, 512}

	m.Hulls[0] = hull
	m.Mins = [3]float32{-512, -512, 0}
	m.Maxs = [3]float32{512, 512, 512}
	m.ClipBox = true
	m.ClipMins = m.Mins
	m.ClipMaxs = m.Maxs

	return m
}
