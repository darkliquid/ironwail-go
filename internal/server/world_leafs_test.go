package server

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
)

func TestFindTouchedLeafsSkipsSolidLeafZero(t *testing.T) {
	s := NewServer()
	s.WorldTree = &bsp.Tree{
		Leafs: []bsp.TreeLeaf{
			{Contents: bsp.ContentsSolid},
			{Contents: 0},
		},
	}

	ent := &Edict{Vars: &EntVars{}}

	// Solid leaf 0 should be excluded entirely.
	s.findTouchedLeafs(ent, bsp.TreeChild{IsLeaf: true, Index: 0})
	if ent.NumLeafs != 0 {
		t.Fatalf("NumLeafs = %d after solid leaf 0, want 0", ent.NumLeafs)
	}

	// BSP leaf 1 should be stored as visleaf 0.
	s.findTouchedLeafs(ent, bsp.TreeChild{IsLeaf: true, Index: 1})
	if ent.NumLeafs != 1 {
		t.Fatalf("NumLeafs = %d after leaf 1, want 1", ent.NumLeafs)
	}
	if got := ent.LeafNums[0]; got != 0 {
		t.Fatalf("LeafNums[0] = %d, want visleaf index 0", got)
	}
}

func TestFindTouchedLeafsUsesBoxOnPlaneSideForNonAxialPlanes(t *testing.T) {
	s := NewServer()
	invSqrt2 := float32(1 / math.Sqrt2)
	s.WorldTree = &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: [3]float32{invSqrt2, -invSqrt2, 0}, Dist: 0, Type: 3}},
		Nodes: []bsp.TreeNode{{
			PlaneNum: 0,
			Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 1}, {IsLeaf: true, Index: 2}},
		}},
		Leafs: []bsp.TreeLeaf{
			{Contents: bsp.ContentsSolid},
			{Contents: 0},
			{Contents: 0},
		},
	}
	s.WorldModel = &model.Model{
		Planes: []model.MPlane{{Normal: [3]float32{invSqrt2, -invSqrt2, 0}, Dist: 0, Type: 3}},
	}

	ent := &Edict{Vars: &EntVars{
		AbsMin: [3]float32{2, 0, -1},
		AbsMax: [3]float32{4, 3, 1},
	}}

	s.findTouchedLeafs(ent, bsp.TreeChild{Index: 0, IsLeaf: false})

	if ent.NumLeafs != 2 {
		t.Fatalf("NumLeafs = %d, want 2 for box crossing both sides of non-axial plane", ent.NumLeafs)
	}
	if ent.LeafNums[0] != 0 || ent.LeafNums[1] != 1 {
		t.Fatalf("LeafNums = %v, want [0 1]", ent.LeafNums[:ent.NumLeafs])
	}
}
