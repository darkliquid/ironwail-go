package qbsp

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// corpusMapDir locates the id1 map-source corpus (dataset/bspdec checkout);
// tests skip when it is absent (the dataset is not committed to the repo).
func corpusMapDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "dataset", "bspdec", "raw", "quake_map_source")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Skipf("map source corpus not available at %s", dir)
	}
	return dir
}

func compileCorpusMap(t *testing.T, path string) *CompileResult {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseMap(bytes.NewReader(src))
	if err != nil {
		t.Fatalf("%s: ParseMap: %v", path, err)
	}
	res, err := Compile(m, Options{Log: t.Logf})
	if err != nil {
		t.Fatalf("%s: Compile: %v", path, err)
	}
	return res
}

// contentsAtPoint descends the world tree to the leaf containing p.
func contentsAtPoint(t *testing.T, res *CompileResult, p vec3) int32 {
	t.Helper()
	tr, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	idx := 0
	for {
		if idx < 0 || idx >= len(tr.Nodes) {
			t.Fatalf("node index %d out of range (%d nodes)", idx, len(tr.Nodes))
		}
		n := &tr.Nodes[idx]
		pl := tr.Planes[n.PlaneNum]
		d := p.X*float64(pl.Normal.X) + p.Y*float64(pl.Normal.Y) + p.Z*float64(pl.Normal.Z) - float64(pl.Dist)
		child := &n.Children[0]
		if d < 0 {
			child = &n.Children[1]
		}
		if child.IsLeaf {
			return tr.Leafs[child.Index].Contents
		}
		idx = child.Index
	}
}

// TestE1M1OpenBrushSolidity locks the bead ironwail-go-aeh acceptance:
// e1m1 must compile sealed without leaking to the void, and interior
// points within solid brush volumes must resolve to ContentsSolid.
func TestE1M1OpenBrushSolidity(t *testing.T) {
	dir := corpusMapDir(t)
	res := compileCorpusMap(t, filepath.Join(dir, "e1m1.map"))
	if res.Leaked {
		for i, pt := range res.LeakPath {
			t.Logf("leak path [%d]: %v", i, pt)
		}
		t.Fatalf("e1m1 leaked (trail of %d points)", len(res.LeakPath))
	}
	// Probes inside actual solid brushes in e1m1:
	// Brush 0 (x:448..512, y:-384..-320, z:48..64)
	// Brush 47 (x:384..576, y:-416..-208, z:32..48)
	actualSolid := [][3]float64{
		{480, -350, 56},
		{480, -350, 40},
	}
	for _, pt := range actualSolid {
		p := vec3{X: pt[0], Y: pt[1], Z: pt[2]}
		if c := contentsAtPoint(t, res, p); c != bsp.ContentsSolid {
			t.Errorf("interior probe (%g %g %g): contents = %d, want SOLID (%d)", pt[0], pt[1], pt[2], c, bsp.ContentsSolid)
		} else {
			t.Logf("interior probe (%g %g %g): correctly SOLID (%d)", pt[0], pt[1], pt[2], c)
		}
	}
}

// TestID1MapCorpusSeals is the qbsp corpus regression: every id1 map source
// must canonicalize without leaking to the void.
func TestID1MapCorpusSeals(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ID1 map corpus seal test in short mode")
	}
	dir := corpusMapDir(t)
	maps, err := filepath.Glob(filepath.Join(dir, "*.map"))
	if err != nil {
		t.Fatal(err)
	}
	if len(maps) == 0 {
		t.Skip("no .map files in corpus")
	}
	for _, path := range maps {
		path := path
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res := compileCorpusMap(t, path)
			if res.Leaked {
				for pi, pt := range res.LeakPath {
					t.Logf("leak [%d]: %v", pi, pt)
				}
				t.Errorf("%s leaked (trail of %d points, first %v)", name, len(res.LeakPath), res.LeakPath[0])
			}
		})
	}
}
