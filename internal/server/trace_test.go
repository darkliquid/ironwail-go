package server

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
)

func TestMoveAgainstBoxWorld(t *testing.T) {
	s := newMovementTestServer()

	// Configure the world edict as a simple box from -10..-10..-10 to 10..10..10
	world := s.Edicts[0]
	world.Vars.Mins = [3]float32{-10, -10, -10}
	world.Vars.Maxs = [3]float32{10, 10, 10}
	// Use non-SolidBSP so hullForEntity falls back to box hull
	world.Vars.Solid = float32(SolidBBox)

	// Move from outside towards the box center
	start := [3]float32{-20, 0, 0}
	end := [3]float32{0, 0, 0}
	trace := s.Move(start, [3]float32{}, [3]float32{}, end, MoveNormal, nil)

	if trace.Fraction == 1 {
		t.Fatalf("expected collision fraction < 1, got 1 (no collision)")
	}
	if trace.Entity == nil {
		t.Fatalf("expected entity collision, got nil")
	}
}

func TestMoveThroughEmptySpace(t *testing.T) {
	s := newMovementTestServer()

	// World still exists but set it to a distant box so path is empty
	world := s.Edicts[0]
	world.Vars.Mins = [3]float32{1000, 1000, 1000}
	world.Vars.Maxs = [3]float32{1010, 1010, 1010}
	world.Vars.Solid = float32(SolidBBox)

	start := [3]float32{0, 0, 0}
	end := [3]float32{16, 0, 0}
	trace := s.Move(start, [3]float32{}, [3]float32{}, end, MoveNormal, nil)

	if trace.Fraction != 1 {
		t.Fatalf("expected no collision (fraction==1), got %v", trace.Fraction)
	}
}

func TestRecursiveHullCheckTracksInOpen(t *testing.T) {
	hull := &model.Hull{
		ClipNodes:     []model.MClipNode{{PlaneNum: 0, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}}},
		Planes:        []model.MPlane{{Normal: [3]float32{1, 0, 0}, Type: 0}},
		FirstClipNode: 0,
		LastClipNode:  0,
	}
	trace := TraceResult{AllSolid: true}
	if !recursiveHullCheck(hull, 0, 0, 1, [3]float32{1, 0, 0}, [3]float32{2, 0, 0}, &trace) {
		t.Fatal("recursiveHullCheck returned false")
	}
	if !trace.InOpen {
		t.Fatal("expected trace to record open-space traversal")
	}
	if trace.InWater {
		t.Fatal("unexpected in-water flag for empty-space trace")
	}
}

func TestRecursiveHullCheckTracksInWater(t *testing.T) {
	hull := &model.Hull{
		ClipNodes:     []model.MClipNode{{PlaneNum: 0, Children: [2]int{bsp.ContentsWater, bsp.ContentsSolid}}},
		Planes:        []model.MPlane{{Normal: [3]float32{1, 0, 0}, Type: 0}},
		FirstClipNode: 0,
		LastClipNode:  0,
	}
	trace := TraceResult{AllSolid: true}
	if !recursiveHullCheck(hull, 0, 0, 1, [3]float32{1, 0, 0}, [3]float32{2, 0, 0}, &trace) {
		t.Fatal("recursiveHullCheck returned false")
	}
	if !trace.InWater {
		t.Fatal("expected trace to record water traversal")
	}
	if trace.InOpen {
		t.Fatal("unexpected in-open flag for water trace")
	}
}

func TestHullPointContentsUsesDoublePrecisionForNonAxialPlanes(t *testing.T) {
	hull := &model.Hull{
		ClipNodes:     []model.MClipNode{{PlaneNum: 0, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}}},
		Planes:        []model.MPlane{{Normal: [3]float32{0.9193270206451416, 1.595353126525879, 0.7359357476234436}, Dist: -71107.78125, Type: 3}},
		FirstClipNode: 0,
		LastClipNode:  0,
	}
	point := [3]float32{-2785.728515625, -39929.87890625, -6582.81640625}

	if got := hullPointContents(hull, 0, point); got != bsp.ContentsSolid {
		t.Fatalf("hullPointContents() = %d, want %d (double-precision non-axial classification)", got, bsp.ContentsSolid)
	}
}

func TestRecursiveHullCheckKeepsNonAxialFarSideSolid(t *testing.T) {
	point := [3]float32{-2785.728515625, -39929.87890625, -6582.81640625}
	hull := &model.Hull{
		ClipNodes: []model.MClipNode{
			{PlaneNum: 0, Children: [2]int{bsp.ContentsEmpty, 1}},
			{PlaneNum: 1, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
		},
		Planes: []model.MPlane{
			{Normal: [3]float32{0, 1, 0}, Dist: point[1] - DistEpsilon, Type: 1},
			{Normal: [3]float32{0.9193270206451416, 1.595353126525879, 0.7359357476234436}, Dist: -71107.78125, Type: 3},
		},
		FirstClipNode: 0,
		LastClipNode:  1,
	}
	start := [3]float32{point[0], point[1] + 1, point[2]}
	end := [3]float32{point[0], point[1] - 1, point[2]}
	trace := TraceResult{Fraction: 1, AllSolid: true, EndPos: end}

	recursiveHullCheck(hull, hull.FirstClipNode, 0, 1, start, end, &trace)

	if trace.StartSolid {
		t.Fatal("recursiveHullCheck reported startsolid after non-axial far side rounded to zero")
	}
	if trace.Fraction >= 1 {
		t.Fatalf("trace fraction = %v, want collision before entering far side", trace.Fraction)
	}
	if trace.EndPos != point {
		t.Fatalf("trace end = %v, want %v", trace.EndPos, point)
	}
}
