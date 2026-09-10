package bspdec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// seamRichWorld is a sealed room with coplanar-adjacent original brushes
// (two floor slabs meeting at x=32 plus an 8x8 trim on the floor top).
func seamRichWorld(t *testing.T) (src string) {
	t.Helper()
	return "{\n\"classname\" \"worldspawn\"\n" +
		slabBox(0, 0, 0, 32, 64, 8, "mt_floor") +
		slabBox(32, 0, 0, 64, 64, 8, "mt_floor") +
		slabBox(8, 8, 8, 16, 16, 16, "mt_rock") +
		slabBox(0, 0, 8, 64, 8, 64, "mt_wall") +
		slabBox(0, 56, 8, 64, 64, 64, "mt_wall") +
		slabBox(0, 0, 8, 8, 64, 64, "mt_wall") +
		slabBox(56, 0, 8, 64, 64, 64, "mt_wall") +
		slabBox(0, 0, 56, 64, 64, 64, "mt_floor") +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 32\"\n}\n"
}

func TestSeamCandidatesFixture(t *testing.T) {
	src := seamRichWorld(t)
	tree, _ := compileFixture(t, src)
	orig := mustParseMapString(t, src)
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	cells, err := d.decompileModel(0)
	if err != nil {
		t.Fatal(err)
	}
	// run the merge+cleanup the label path uses, then build geometries
	for _, b := range cells {
		removeRedundantPlanes(b)
	}
	d.textureBrushes(cells)
	var split []*Brush
	for _, b := range cells {
		split = append(split, d.splitDifferentTextures(b)...)
	}
	cells = mergeConvex(split)
	canonicalizeBrush(cells)
	out := attachBrushesForTest(orig, [][]*Brush{cells})
	geoms, err := FacePolygons(out)
	if err != nil {
		t.Fatal(err)
	}
	cands := SeamCandidates(geoms, OriginalBrushPlanes(orig))
	if len(cands) == 0 {
		t.Fatal("no candidates")
	}
	pos, neg := 0, 0
	for _, c := range cands {
		if len(c.Features) != SeamFeatureCount {
			t.Fatalf("feature length %d, want %d", len(c.Features), SeamFeatureCount)
		}
		if c.Seam {
			pos++
		} else {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		t.Fatalf("pos/neg = %d/%d, want both present", pos, neg)
	}
	// the floor-slab junction must be among the seam candidates
	found := false
	for _, c := range cands {
		if c.Seam && nearX(c.Edge[0], 32) && nearX(c.Edge[1], 32) {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing seam candidate on x=32 (%d candidates, %d seams)", len(cands), pos)
	}
}

// attachBrushesForTest routes cells onto a worldspawn entity for FacePolygons.
func attachBrushesForTest(ent *mapfile.Map, perModel [][]*Brush) *mapfile.Map {
	out := &mapfile.Map{}
	first := ent.Entities[0]
	first.Brushes = nil
	for _, b := range perModel[0] {
		first.Brushes = append(first.Brushes, toMapBrush(b))
	}
	out.Entities = append(out.Entities, first)
	return out
}

func TestSeamCandidatesSingleSlabNoSeams(t *testing.T) {
	// one original brush (a solid box): no coplanar join exists, so no seam
	// candidate. Built without qbsp because an open solid box leaks.
	orig := mustParseMapString(t, "{\n\"classname\" \"worldspawn\"\n"+
		slabBox(0, 0, 0, 64, 64, 8, "brick")+"}\n")
	b := boxBrush(vc(0, 0, 0), vc(64, 64, 8))
	rebuildWindings(b)
	out := &mapfile.Map{Entities: []mapfile.Entity{{
		Epairs:  []mapfile.Epair{{Key: "classname", Value: "worldspawn"}},
		Brushes: []mapfile.MapBrush{toMapBrush(b)},
	}}}
	geoms, err := FacePolygons(out)
	if err != nil {
		t.Fatal(err)
	}
	cands := SeamCandidates(geoms, OriginalBrushPlanes(orig))
	for _, c := range cands {
		if c.Seam {
			t.Fatalf("single slab produced a seam candidate: %+v", c)
		}
	}
}

func TestLoadSeamModel(t *testing.T) {
	if _, err := LoadSeamModel(t.TempDir()); err == nil {
		t.Fatal("expected error for missing model dir")
	}
	dir := filepath.Join(t.TempDir(), "route-a-v0.1.0")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	art := SeamModel{
		Schema:  "route-a-features-v1",
		Weights: make([]float64, SeamFeatureCount),
		Bias:    0.5,
		Mean:    make([]float64, SeamFeatureCount),
		Std:     make([]float64, SeamFeatureCount),
	}
	for i := range art.Std {
		art.Std[i] = 1
	}
	b, _ := json.Marshal(art)
	if err := os.WriteFile(filepath.Join(dir, "model.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := LoadSeamModel(filepath.Dir(dir))
	if err != nil {
		t.Fatalf("LoadSeamModel: %v", err)
	}
	f := make([]float64, SeamFeatureCount)
	if p := m.Score(f); p != 0.6224593312018546 {
		t.Fatalf("score = %v, want sigmoid(0.5)", p)
	}
}
