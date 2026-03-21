package server

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
)

// TestRecursiveHullCheckTransitionsFromSolidToOpen tests the recursive hull collision check algorithm with synthetic data.
// It verifying the correctness of the core BSP collision logic by ensuring it correctly identifies transitions from solid to open space.
// Where in C: SV_RecursiveHullCheck in sv_phys.c
func TestRecursiveHullCheckTransitionsFromSolidToOpen(t *testing.T) {
	hull := &bsp.Hull{
		FirstClipNode: 0,
		ClipNodes: []bsp.DSClipNode{
			{PlaneNum: 0, Children: [2]bsp.HullChild{{Index: 1}, {Index: bsp.ContentsSolid, IsLeaf: true}}},
			{PlaneNum: 1, Children: [2]bsp.HullChild{{Index: bsp.ContentsEmpty, IsLeaf: true}, {Index: bsp.ContentsSolid, IsLeaf: true}}},
		},
		Planes: []bsp.DSPlane{
			{Normal: [3]float32{0, 0, 1}, Dist: 1},  // z = 1
			{Normal: [3]float32{0, 0, 1}, Dist: -1}, // z = -1
		},
	}

	start := [3]float32{2, 0, 3}
	end := [3]float32{2, 0, -3}

	sawOpen := false
	for i := 0; i <= 256; i++ {
		frac := float32(i) / 256
		point := [3]float32{
			start[0] + (end[0]-start[0])*frac,
			start[1] + (end[1]-start[1])*frac,
			start[2] + (end[2]-start[2])*frac,
		}
		if got := hullPointContents(hull, hull.FirstClipNode, point); got != bsp.ContentsSolid {
			sawOpen = true
			break
		}
	}
	if !sawOpen {
		t.Fatal("test hull never transitions out of solid along the sample ray")
	}

	trace := TraceResult{Fraction: 1, AllSolid: true, EndPos: end}
	recursiveHullCheck(hull, hull.FirstClipNode, 0, 1, start, end, &trace)

	if trace.AllSolid {
		t.Fatal("recursiveHullCheck left trace allsolid despite open space on the ray")
	}
	if trace.Fraction >= 1 {
		t.Fatalf("trace fraction = %v, want collision before the end point", trace.Fraction)
	}
}
