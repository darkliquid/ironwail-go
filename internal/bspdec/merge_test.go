package bspdec

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

func newTexturedBox(mins, maxs mapfile.Vec3, tex string, contents int32) *Brush {
	b := boxBrush(mins, maxs)
	b.Contents = contents
	for _, s := range b.Sides {
		s.TexName = tex
	}
	rebuildWindings(b)
	return b
}

func TestMergeConvexJoinsAdjacentBoxes(t *testing.T) {
	a := newTexturedBox(vc(0, 0, 0), vc(32, 64, 64), "brick", bsp.ContentsSolid)
	b := newTexturedBox(vc(32, 0, 0), vc(64, 64, 64), "brick", bsp.ContentsSolid)
	out := mergeConvex([]*Brush{a, b})
	if len(out) != 1 {
		t.Fatalf("brushes = %d, want 1", len(out))
	}
	if len(out[0].Sides) != 6 {
		t.Fatalf("merged sides = %d, want 6", len(out[0].Sides))
	}
	for _, s := range out[0].Sides {
		if s.Winding == nil {
			t.Fatal("merged brush has nil winding")
		}
	}
}

func TestMergeConvexRejectsDifferentContents(t *testing.T) {
	a := newTexturedBox(vc(0, 0, 0), vc(32, 64, 64), "brick", bsp.ContentsSolid)
	b := newTexturedBox(vc(32, 0, 0), vc(64, 64, 64), "*water", bsp.ContentsWater)
	if out := mergeConvex([]*Brush{a, b}); len(out) != 2 {
		t.Fatalf("brushes = %d, want 2 (contents differ)", len(out))
	}
}

func TestMergeConvexRejectsTextureBoundary(t *testing.T) {
	// same contents, but the coplanar outer sides carry different textures:
	// merging would lose the boundary the splitter created
	a := newTexturedBox(vc(0, 0, 0), vc(32, 64, 64), "brick", bsp.ContentsSolid)
	b := newTexturedBox(vc(32, 0, 0), vc(64, 64, 64), "brick", bsp.ContentsSolid)
	for _, s := range b.Sides {
		if s.Plane.Normal == (mapfile.Vec3{X: 0, Y: 0, Z: 1}) {
			s.TexName = "stone"
		}
	}
	if out := mergeConvex([]*Brush{a, b}); len(out) != 2 {
		t.Fatalf("brushes = %d, want 2 (texture boundary)", len(out))
	}
}

func TestMergeConvexRejectsNonConvexUnion(t *testing.T) {
	// L shape: b taller than a — the union is non-convex because a's top
	// plane cuts b
	a := newTexturedBox(vc(0, 0, 0), vc(32, 64, 32), "brick", bsp.ContentsSolid)
	b := newTexturedBox(vc(32, 0, 0), vc(64, 64, 64), "brick", bsp.ContentsSolid)
	if out := mergeConvex([]*Brush{a, b}); len(out) != 2 {
		t.Fatalf("brushes = %d, want 2 (non-convex union)", len(out))
	}
}

func TestMergeConvexRunsOnDecompiledRoom(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	brushes, err := d.decompileModel(0)
	if err != nil {
		t.Fatalf("decompileModel: %v", err)
	}
	for _, b := range brushes {
		removeRedundantPlanes(b)
	}
	d.textureBrushes(brushes)
	before := len(brushes)
	merged := mergeConvex(brushes)
	// The minimal room already decompiles to 6 non-mergeable convex cells
	// (reduction is exercised by TestMergeConvexJoinsAdjacentBoxes); what must
	// hold is that merging never invalidates or grows the output.
	if len(merged) > before {
		t.Fatalf("merge grew brush count: before=%d after=%d", before, len(merged))
	}
	for i, b := range merged {
		if len(b.Sides) < 4 {
			t.Fatalf("merged brush %d has %d sides", i, len(b.Sides))
		}
	}
}