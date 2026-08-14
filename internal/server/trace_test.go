// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
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
	s.Edicts[0].SetSolid(s, float32(SolidBSP))
	s.ClearWorld()

	owner := s.AllocEdict()
	projectile := s.AllocEdict()
	if owner == nil || projectile == nil {
		t.Fatal("failed to allocate edicts")
	}
	ownerNum := s.NumForEdict(owner)

	owner.SetOrigin(s, qtypes.Vec3{X: 0, Y: 0, Z: 128})
	owner.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -16})
	owner.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	owner.SetSolid(s, float32(SolidBBox))
	s.LinkEdict(owner, false)

	projectile.SetOrigin(s, qtypes.Vec3{X: -64, Y: 0, Z: 128})
	projectile.SetMins(s, qtypes.Vec3{})
	projectile.SetMaxs(s, qtypes.Vec3{})
	projectile.SetSolid(s, float32(SolidBBox))
	projectile.SetMoveType(s, float32(MoveTypeFlyMissile))
	s.LinkEdict(projectile, false)

	return s, owner, projectile, ownerNum
}

func TestMoveAgainstBoxWorld(t *testing.T) {
	s := newMovementTestServer()

	// Configure the world edict as a simple box from -10..-10..-10 to 10..10..10
	world := s.Edicts[0]
	world.SetMins(s, qtypes.Vec3{X: -10, Y: -10, Z: -10})
	world.SetMaxs(s, qtypes.Vec3{X: 10, Y: 10, Z: 10})
	// Use non-SolidBSP so hullForEntity falls back to box hull
	world.SetSolid(s, float32(SolidBBox))

	// Move from outside towards the box center
	start := qtypes.Vec3{X: -20, Y: 0, Z: 0}
	end := qtypes.Vec3{}
	trace := s.Move(start, qtypes.Vec3{}, qtypes.Vec3{}, end, MoveNormal, nil)

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
	world.SetMins(s, qtypes.Vec3{X: 1000, Y: 1000, Z: 1000})
	world.SetMaxs(s, qtypes.Vec3{X: 1010, Y: 1010, Z: 1010})
	world.SetSolid(s, float32(SolidBBox))

	start := qtypes.Vec3{}
	end := qtypes.Vec3{X: 16, Y: 0, Z: 0}
	trace := s.Move(start, qtypes.Vec3{}, qtypes.Vec3{}, end, MoveNormal, nil)

	if trace.Fraction != 1 {
		t.Fatalf("expected no collision (fraction==1), got %v", trace.Fraction)
	}
}

func TestMoveMissileSkipsOwnerEdictRef(t *testing.T) {
	s, owner, projectile, ownerNum := newOwnerSkipTraceServer(t)
	projectile.SetOwner(s, int32(ownerNum))

	trace := s.Move(projectile.Origin(s), projectile.Mins(s), projectile.Maxs(s), owner.Origin(s), MoveMissile, projectile)
	if trace.Entity == owner {
		t.Fatal("missile move clipped against owner with edict-number owner ref")
	}
}

func TestMoveMissileSkipsOwnerQCOffsetRef(t *testing.T) {
	s, owner, projectile, ownerNum := newOwnerSkipTraceServer(t)
	s.QCVM.EdictSize = 223
	projectile.SetOwner(s, int32(ownerNum*s.QCVM.EdictSize))

	trace := s.Move(projectile.Origin(s), projectile.Mins(s), projectile.Maxs(s), owner.Origin(s), MoveMissile, projectile)
	if trace.Entity == owner {
		t.Fatal("missile move clipped against owner with QC offset owner ref")
	}
}

func TestRecursiveHullCheckTracksInOpen(t *testing.T) {
	hull := &model.Hull{
		ClipNodes:     []model.MClipNode{{PlaneNum: 0, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}}},
		Planes:        []model.MPlane{{Normal: qtypes.Vec3{X: 1, Y: 0, Z: 0}, Type: 0}},
		FirstClipNode: 0,
		LastClipNode:  0,
	}
	trace := TraceResult{AllSolid: true}
	if !recursiveHullCheck(hull, 0, 0, 1, qtypes.Vec3{X: 1, Y: 0, Z: 0}, qtypes.Vec3{X: 2, Y: 0, Z: 0}, &trace) {
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
		Planes:        []model.MPlane{{Normal: qtypes.Vec3{X: 1, Y: 0, Z: 0}, Type: 0}},
		FirstClipNode: 0,
		LastClipNode:  0,
	}
	trace := TraceResult{AllSolid: true}
	if !recursiveHullCheck(hull, 0, 0, 1, qtypes.Vec3{X: 1, Y: 0, Z: 0}, qtypes.Vec3{X: 2, Y: 0, Z: 0}, &trace) {
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
		Planes:        []model.MPlane{{Normal: qtypes.Vec3{X: 0.9193270206451416, Y: 1.595353126525879, Z: 0.7359357476234436}, Dist: -71107.78125, Type: 3}},
		FirstClipNode: 0,
		LastClipNode:  0,
	}
	point := qtypes.Vec3{X: -2785.728515625, Y: -39929.87890625, Z: -6582.81640625}

	if got := hullPointContents(hull, 0, point); got != bsp.ContentsSolid {
		t.Fatalf("hullPointContents() = %d, want %d (double-precision non-axial classification)", got, bsp.ContentsSolid)
	}
}

func TestRecursiveHullCheckKeepsNonAxialFarSideSolid(t *testing.T) {
	point := qtypes.Vec3{X: -2785.728515625, Y: -39929.87890625, Z: -6582.81640625}
	hull := &model.Hull{
		ClipNodes: []model.MClipNode{
			{PlaneNum: 0, Children: [2]int{bsp.ContentsEmpty, 1}},
			{PlaneNum: 1, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
		},
		Planes: []model.MPlane{
			{Normal: qtypes.Vec3{X: 0, Y: 1, Z: 0}, Dist: point.Y - DistEpsilon, Type: 1},
			{Normal: qtypes.Vec3{X: 0.9193270206451416, Y: 1.595353126525879, Z: 0.7359357476234436}, Dist: -71107.78125, Type: 3},
		},
		FirstClipNode: 0,
		LastClipNode:  1,
	}
	start := qtypes.Vec3{X: point.X, Y: point.Y + 1, Z: point.Z}
	end := qtypes.Vec3{X: point.X, Y: point.Y - 1, Z: point.Z}
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
			{Normal: qtypes.Vec3{X: 0, Y: 0, Z: 1}, Dist: -1.8989416, Type: 2},
			{Normal: qtypes.Vec3{X: 0, Y: 0, Z: 1}, Dist: -2.3453076, Type: 2},
			{Normal: qtypes.Vec3{X: 0.70710677, Y: 0, Z: 0.70710677}, Dist: 2.5941012, Type: 3},
			{Normal: qtypes.Vec3{X: 1, Y: 0, Z: 0}, Dist: -1.5072697, Type: 0},
			{Normal: qtypes.Vec3{X: 0.70710677, Y: 0, Z: -0.70710677}, Dist: 2.7501428, Type: 3},
			{Normal: qtypes.Vec3{X: 0.4472136, Y: 0, Z: 0.8944272}, Dist: -1.713885, Type: 3},
		},
		FirstClipNode: 0,
		LastClipNode:  5,
	}
	start := qtypes.Vec3{X: 2, Y: 0, Z: 3}
	end := qtypes.Vec3{X: 2, Y: 0, Z: -3}

	sawOpen := false
	for i := 0; i <= 256; i++ {
		frac := float32(i) / 256
		point := qtypes.Vec3{
			X: start.X + (end.X-start.X)*frac,
			Y: start.Y + (end.Y-start.Y)*frac,
			Z: start.Z + (end.Z-start.Z)*frac,
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
