package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
)

// TestRecursiveHullCheckTransitionsFromSolidToOpen tests the recursive hull collision check algorithm with synthetic data.
// It verifying the correctness of the core BSP collision logic by ensuring it correctly identifies transitions from solid to open space.
// Where in C: SV_RecursiveHullCheck in sv_phys.c
func TestRecursiveHullCheckTransitionsFromSolidToOpen(t *testing.T) {
	hull := &model.Hull{
		FirstClipNode: 0,
		LastClipNode:  0,
		ClipNodes: []model.MClipNode{
			{PlaneNum: 0, Children: [2]int{bsp.ContentsSolid, bsp.ContentsEmpty}},
		},
		Planes: []model.MPlane{
			{Normal: [3]float32{0, 0, 1}, Dist: 1, Type: 2}, // z = 1
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
	if !trace.StartSolid {
		t.Fatal("recursiveHullCheck should mark trace startsolid when the ray begins in solid")
	}
	if trace.Fraction != 1 {
		t.Fatalf("trace fraction = %v, want 1 when the ray exits solid without a later impact", trace.Fraction)
	}
}
