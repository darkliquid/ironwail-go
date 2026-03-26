package server

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
)

func newOwnerSkipTraceServer(t *testing.T) (*Server, *Edict, *Edict, int) {
	t.Helper()
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.WorldModel = CreateSyntheticWorldModel()
	if len(s.Edicts) == 0 || s.Edicts[0] == nil {
		t.Fatal("missing world edict")
	}
	s.Edicts[0].Vars.Solid = float32(SolidBSP)
	s.ClearWorld()

	owner := s.AllocEdict()
	projectile := s.AllocEdict()
	if owner == nil || projectile == nil {
		t.Fatal("failed to allocate edicts")
	}
	ownerNum := s.NumForEdict(owner)

	owner.Vars.Origin = [3]float32{0, 0, 128}
	owner.Vars.Mins = [3]float32{-16, -16, -16}
	owner.Vars.Maxs = [3]float32{16, 16, 16}
	owner.Vars.Solid = float32(SolidBBox)
	s.LinkEdict(owner, false)

	projectile.Vars.Origin = [3]float32{-64, 0, 128}
	projectile.Vars.Mins = [3]float32{}
	projectile.Vars.Maxs = [3]float32{}
	projectile.Vars.Solid = float32(SolidBBox)
	projectile.Vars.MoveType = float32(MoveTypeFlyMissile)
	s.LinkEdict(projectile, false)

	return s, owner, projectile, ownerNum
}

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

func TestMoveMissileSkipsOwnerEdictRef(t *testing.T) {
	s, owner, projectile, ownerNum := newOwnerSkipTraceServer(t)
	projectile.Vars.Owner = int32(ownerNum)

	trace := s.Move(projectile.Vars.Origin, projectile.Vars.Mins, projectile.Vars.Maxs, owner.Vars.Origin, MoveMissile, projectile)
	if trace.Entity == owner {
		t.Fatal("missile move clipped against owner with edict-number owner ref")
	}
}

func TestMoveMissileSkipsOwnerQCOffsetRef(t *testing.T) {
	s, owner, projectile, ownerNum := newOwnerSkipTraceServer(t)
	s.QCVM.EdictSize = 223
	projectile.Vars.Owner = int32(ownerNum * s.QCVM.EdictSize)

	trace := s.Move(projectile.Vars.Origin, projectile.Vars.Mins, projectile.Vars.Maxs, owner.Vars.Origin, MoveMissile, projectile)
	if trace.Entity == owner {
		t.Fatal("missile move clipped against owner with QC offset owner ref")
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

func TestRecursiveHullCheckUsesFarSideMidpointForNestedSolid(t *testing.T) {
	hull := &model.Hull{
		ClipNodes: []model.MClipNode{
			{PlaneNum: 0, Children: [2]int{bsp.ContentsSolid, 3}},
			{PlaneNum: 1, Children: [2]int{bsp.ContentsEmpty, 2}},
			{PlaneNum: 2, Children: [2]int{4, bsp.ContentsEmpty}},
			{PlaneNum: 3, Children: [2]int{4, bsp.ContentsEmpty}},
			{PlaneNum: 4, Children: [2]int{5, bsp.ContentsSolid}},
			{PlaneNum: 5, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
		},
		Planes: []model.MPlane{
			{Normal: [3]float32{0, 0, 1}, Dist: -1.8989416, Type: 2},
			{Normal: [3]float32{0, 0, 1}, Dist: -2.3453076, Type: 2},
			{Normal: [3]float32{0.70710677, 0, 0.70710677}, Dist: 2.5941012, Type: 3},
			{Normal: [3]float32{1, 0, 0}, Dist: -1.5072697, Type: 0},
			{Normal: [3]float32{0.70710677, 0, -0.70710677}, Dist: 2.7501428, Type: 3},
			{Normal: [3]float32{0.4472136, 0, 0.8944272}, Dist: -1.713885, Type: 3},
		},
		FirstClipNode: 0,
		LastClipNode:  5,
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
