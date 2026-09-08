package bspdec

import (
	"testing"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

func TestRemoveRedundantPlanes(t *testing.T) {
	b := boxBrush(vc(0, 0, 0), vc(64, 64, 64))
	// a distant plane that cuts nothing is redundant
	b.Sides = append(b.Sides, &Side{Plane: mapfile.Plane{Normal: vc(1, 0, 0), Dist: 1000}})
	removeRedundantPlanes(b)
	if len(b.Sides) != 6 {
		t.Fatalf("sides = %d, want 6", len(b.Sides))
	}
	for i, s := range b.Sides {
		if s.Winding == nil || s.Winding.Area() < 64*64-1 {
			t.Fatalf("side %d lost area after pruning", i)
		}
	}
	// and a real plane is never redundant
	removeRedundantPlanes(b)
	if len(b.Sides) != 6 {
		t.Fatalf("second pass changed sides = %d", len(b.Sides))
	}
}

func TestPlanesMatch(t *testing.T) {
	a := mapfile.Plane{Normal: vc(1, 0, 0), Dist: 64}
	if !planesMatch(a, mapfile.Plane{Normal: vc(1, 0, 0), Dist: 64.01}) {
		t.Fatal("near-identical planes should match")
	}
	if planesMatch(a, mapfile.Plane{Normal: vc(1, 0, 0), Dist: 65}) {
		t.Fatal("distinct dists should not match")
	}
	if planesMatch(a, negatePlane(a)) {
		t.Fatal("opposite planes should not match")
	}
	if !planesOpposite(a, negatePlane(a)) {
		t.Fatal("negated plane should be opposite")
	}
}

func TestDecompiledCellsSurvivePruning(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	brushes, err := d.decompileModel(0)
	if err != nil {
		t.Fatalf("decompileModel: %v", err)
	}
	// A minimal sealed-room BSP carries only real boundary planes, so pruning
	// may legitimately remove nothing (redundancy is covered by
	// TestRemoveRedundantPlanes); what must hold is post-prune validity.
	for _, b := range brushes {
		removeRedundantPlanes(b)
		if len(b.Sides) < 4 {
			t.Fatalf("brush pruned below 4 sides: %d", len(b.Sides))
		}
		for _, s := range b.Sides {
			if s.Winding == nil || len(s.Winding.Points) < 3 {
				t.Fatal("side lost its winding to pruning")
			}
		}
	}
}