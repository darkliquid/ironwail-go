package server

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
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
