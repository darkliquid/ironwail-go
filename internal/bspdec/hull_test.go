package bspdec

import (
	"bytes"
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

func TestDecompileHull1ClippedCells(t *testing.T) {
	// M0 contract (recorded limitation, see plan Task 10): hull output is the
	// un-expanded solid clip cells capped to the model's render bounds. It
	// approximates the collision volume but does not recover exact brush
	// planes from this qbsp's hull CSG structure (interior bisect planes,
	// mixed hull offsets), so no wall-plane recovery is asserted here.
	tree, data := compileFixture(t, roomMap())
	f, err := bsp.Load(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("bsp.Load: %v", err)
	}
	nodes, err := clipnodesOf(f)
	if err != nil {
		t.Fatalf("clipnodesOf: %v", err)
	}
	d := newDecompiler(tree, Options{GridSnap: 8, TextureFallback: "nearest"})
	brushes := d.decompileHull(0, 1, nodes)
	if len(brushes) == 0 {
		t.Fatal("hull 1 produced no brushes")
	}
	for _, b := range brushes {
		if b.Contents != bsp.ContentsClip {
			t.Fatalf("hull brush contents = %d, want ContentsClip", b.Contents)
		}
		if len(b.Sides) < 4 {
			t.Fatalf("hull brush has %d sides", len(b.Sides))
		}
		for _, s := range b.Sides {
			if s.TexName != "clip" {
				t.Fatalf("hull side texture = %q, want clip", s.TexName)
			}
			if s.Winding == nil || len(s.Winding.Points) < 3 {
				t.Fatal("hull side lost its winding")
			}
			// un-expansion sanity: axial planes land on the integer lattice
			// within the model bounds
			if math.Abs(s.Plane.Normal.X) > 0.99 {
				if math.Abs(s.Plane.Dist-math.Round(s.Plane.Dist)) > 0.05 {
					t.Fatalf("unexpanded axial dist %v not on lattice", s.Plane.Dist)
				}
			}
		}
	}
}

func TestDecompileHullMissing(t *testing.T) {
	tree, data := compileFixture(t, roomMap())
	f, err := bsp.Load(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("bsp.Load: %v", err)
	}
	nodes, err := clipnodesOf(f)
	if err != nil {
		t.Fatalf("clipnodesOf: %v", err)
	}
	d := newDecompiler(tree, Options{})
	// hull 3 is unused by Quake; a missing/zero headnode must not panic
	_ = d.decompileHull(0, 3, nodes)
}