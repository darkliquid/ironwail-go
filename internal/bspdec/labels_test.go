package bspdec

import (
	"strings"
	"testing"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// twoSlabWorldMap is the original-map ground truth for the seam test: two
// 32-wide slabs meeting at x=32 (compile text generated from slabBox, the
// same winding pattern qbsp validates).
func twoSlabWorldMap() string {
	return "{\n\"classname\" \"worldspawn\"\n" +
		slabBox(0, 0, 0, 32, 64, 64, "brick") +
		slabBox(32, 0, 0, 64, 64, 64, "brick") +
		"}\n"
}

func TestLabelCellsRoomAssignment(t *testing.T) {
	tree, _ := compileFixture(t, roomMap())
	orig := mustParseMapString(t, roomMap())
	labels, seams, err := LabelCells(tree, orig, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		t.Fatalf("LabelCells: %v", err)
	}
	if len(labels) == 0 {
		t.Fatal("no cell labels")
	}
	sawAssigned, sawNone := false, false
	for _, l := range labels {
		switch l.Confidence {
		case "assigned":
			sawAssigned = true
		case "none":
			sawNone = true
		case "multi":
			// boundary cells legitimately touch two slabs
		default:
			t.Fatalf("unexpected confidence %q", l.Confidence)
		}
		if l.OriginalBrush < -1 {
			t.Fatalf("bad brush index %d", l.OriginalBrush)
		}
	}
	if !sawAssigned {
		t.Fatal("no cell assigned to an original brush")
	}
	if sawNone {
		// the +8 seed shell can produce outside-fill cells; tolerate a few
		// but they must be a minority
		none := 0
		for _, l := range labels {
			if l.Confidence == "none" {
				none++
			}
		}
		if none > len(labels)/2 {
			t.Fatalf("too many unassigned cells: %d of %d", none, len(labels))
		}
	}
	_ = seams
}

func TestLabelCellsSeamAtSlabBoundary(t *testing.T) {
	tree := twoTextureTopTree()
	orig := mustParseMapString(t, twoSlabWorldMap())
	// seams live on the merged coplanar face, i.e. the pre-split brush: the
	// top side (z=64) carries faces of both slabs meeting at x=32
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	b := boxBrush(vc(0, 0, 0), vc(64, 64, 64))
	d.textureBrush(b)
	seams := SeamEdges(d, []*Brush{b}, brushPlaneSets(originalWorldBrushes(orig)))
	found := false
	for _, s := range seams {
		if !s.Seam {
			continue
		}
		if nearX(s.Edge[0], 32) && nearX(s.Edge[1], 32) {
			found = true
		}
	}
	if !found {
		t.Fatalf("no seam edge on x=32; got %d seam labels", len(seams))
	}
}

func nearX(p mapfile.Vec3, x float64) bool {
	return p.X > x-0.5 && p.X < x+0.5
}

// mustParseMapString is a test helper for ground-truth maps.
func mustParseMapString(t *testing.T, src string) *mapfile.Map {
	t.Helper()
	m, err := mapfile.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return m
}
func TestLabelCoverageCounts(t *testing.T) {
	labels := []CellLabel{
		{Confidence: "assigned"},
		{Confidence: "multi"},
		{Confidence: "none"},
	}
	labeled, total := LabelCoverage(labels)
	if labeled != 2 || total != 3 {
		t.Fatalf("coverage = %d/%d, want 2/3", labeled, total)
	}
}

func TestNearestBrushFallback(t *testing.T) {
	// one unit box at the origin and a second shifted +16 x; a point just
	// outside the first (float-drift case) must snap to it, not the far one.
	box := func(x0 float64) []mapfile.Plane {
		return []mapfile.Plane{
			{Normal: vc(1, 0, 0), Dist: x0 + 1}, {Normal: vc(-1, 0, 0), Dist: -x0},
			{Normal: vc(0, 1, 0), Dist: 1}, {Normal: vc(0, -1, 0), Dist: 0},
			{Normal: vc(0, 0, 1), Dist: 1}, {Normal: vc(0, 0, -1), Dist: 0},
		}
	}
	// unit box at the origin and a second shifted +16 x
	sets := [][]mapfile.Plane{box(0), box(16)}
	if got := nearestBrush(vc(0.5, 0.5, 0.5), sets, 8); got != 0 {
		t.Fatalf("inside point snapped to brush %d, want 0", got)
	}
	// just outside brush 0 (at x = 1.4): closer than brush 1 at 16
	if got := nearestBrush(vc(1.4, 0.5, 0.5), sets, 8); got != 0 {
		t.Fatalf("drift point snapped to brush %d, want 0", got)
	}
	// beyond the max-distance window: no brush
	if got := nearestBrush(vc(40, 0.5, 0.5), sets, 8); got != -1 {
		t.Fatalf("distant point snapped to brush %d, want -1", got)
	}
}
