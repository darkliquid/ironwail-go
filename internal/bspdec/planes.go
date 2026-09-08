package bspdec

import (
	"math"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// planeMatchEpsilon covers float32 BSP plane dists vs float64 brush math.
const planeMatchEpsilon = 0.05

// planesMatch reports whether a and b are the same oriented plane.
func planesMatch(a, b mapfile.Plane) bool {
	return v3Dot(a.Normal, b.Normal) > 1-1e-4 && math.Abs(a.Dist-b.Dist) < planeMatchEpsilon
}

// planesOpposite reports whether a and b are the same plane, opposite sides.
func planesOpposite(a, b mapfile.Plane) bool {
	return v3Dot(a.Normal, b.Normal) < -(1-1e-4) && math.Abs(a.Dist+b.Dist) < planeMatchEpsilon
}

// removeRedundantPlanes drops sides that contribute no area to the brush.
//
// Where in C: RemoveRedundantPlanes in ericw-tools common/decompile.cc — a
// plane whose base winding vanishes when clipped by all other planes is not
// part of the brush boundary.
func removeRedundantPlanes(b *Brush) {
	rebuildWindings(b)
	kept := b.Sides[:0]
	for _, s := range b.Sides {
		if s.Winding != nil && len(s.Winding.Points) >= 3 {
			kept = append(kept, s)
		}
	}
	b.Sides = kept
	// recompute survivors' windings without the dropped planes so areas are exact
	rebuildWindings(b)
}

// planeSetCount returns the number of distinct oriented planes across brushes
// (the PlanesUsed stat).
func planeSetCount(brushes []*Brush) int {
	var planes []mapfile.Plane
	for _, b := range brushes {
		for _, s := range b.Sides {
			seen := false
			for _, p := range planes {
				if planesMatch(s.Plane, p) {
					seen = true
					break
				}
			}
			if !seen {
				planes = append(planes, s.Plane)
			}
		}
	}
	return len(planes)
}