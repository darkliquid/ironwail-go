package synth

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// onLattice reports whether v sits on the 8-unit lattice: either a unit axis
// component (a plane normal, snapping to {-1,0,1}) or a multiple of 8 (an
// axial plane dist).
func onLattice(v float64, tol float64) bool {
	axis := math.Round(v)
	if math.Abs(v-axis) <= tol && math.Abs(axis) <= 1 {
		return true
	}
	return math.Abs(math.Round(v/grid)*grid-v) <= tol
}

// brushSetsEqual compares two brush lists by their derived face planes.
func brushSetsEqual(a, b []mapfile.MapBrush) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i].Faces) != len(b[i].Faces) {
			return false
		}
		for j := range a[i].Faces {
			pa, la := mapfile.PlaneFromPoints(a[i].Faces[j].Points[0], a[i].Faces[j].Points[1], a[i].Faces[j].Points[2])
			pb, lb := mapfile.PlaneFromPoints(b[i].Faces[j].Points[0], b[i].Faces[j].Points[1], b[i].Faces[j].Points[2])
			if la < 0.01 || lb < 0.01 {
				return false
			}
			if math.Abs(pa.Dist-pb.Dist) > 1e-9 ||
				math.Abs(pa.Normal.X-pb.Normal.X) > 1e-9 ||
				math.Abs(pa.Normal.Y-pb.Normal.Y) > 1e-9 ||
				math.Abs(pa.Normal.Z-pb.Normal.Z) > 1e-9 {
				return false
			}
		}
	}
	return true
}

func TestGeneratorDeterministic(t *testing.T) {
	a := NewGenerator(42, 3)
	b := NewGenerator(42, 3)
	m1 := a.GenMap(1)
	m2 := b.GenMap(1)
	if len(m1.Entities) == 0 {
		t.Fatal("no entities")
	}
	if len(m1.Entities[0].Brushes) == 0 {
		t.Fatal("no world brushes")
	}
	if !brushSetsEqual(m1.Entities[0].Brushes, m2.Entities[0].Brushes) {
		t.Fatal("same seed produced different maps")
	}
	// All coordinates must lie on the 8-unit lattice.
	for _, br := range m1.Entities[0].Brushes {
		for _, f := range br.Faces {
			if !onLattice(f.Plane().Normal.X, 0.6) || !onLattice(f.Plane().Normal.Y, 0.6) || !onLattice(f.Plane().Normal.Z, 0.6) ||
				!onLattice(f.Plane().Dist, 0.6) {
				t.Fatalf("face (n=%v d=%v) off the 8-unit lattice", f.Plane().Normal, f.Plane().Dist)
			}
		}
	}
	// Different seeds differ.
	m3 := NewGenerator(7, 3).GenMap(1)
	if brushSetsEqual(m1.Entities[0].Brushes, m3.Entities[0].Brushes) {
		t.Fatal("different seeds produced identical maps")
	}
}

func TestGenMapRoomsSealed(t *testing.T) {
	for seed := int64(0); seed < 10; seed++ {
		g := NewGenerator(seed, 1)
		m := g.GenMap(0)
		// Every brush must be a closed convex 6-face box on the lattice.
		for bi, br := range m.Entities[0].Brushes {
			if len(br.Faces) != 6 {
				t.Fatalf("seed %d brush %d faces = %d, want 6", seed, bi, len(br.Faces))
			}
			for _, f := range br.Faces {
				if !onLattice(f.Plane().Dist, 0.6) {
					t.Fatalf("seed %d brush %d face dist %v off lattice", seed, bi, f.Plane().Dist)
				}
			}
		}
	}
}

// TestGrammarHelpersLatticeValid checks each grammar helper's brushes are
// closed 6-face boxes on the lattice (Step 9's per-helper unit coverage).
func TestGrammarHelpersLatticeValid(t *testing.T) {
	g := NewGenerator(99, 1)
	rm := g.randomRoom(0, 0, 0)
	nb := func(name string, brs []mapfile.MapBrush) {
		t.Helper()
		if len(brs) == 0 {
			t.Fatalf("%s produced no brushes", name)
		}
		for bi, br := range brs {
			if len(br.Faces) != 6 {
				t.Fatalf("%s brush %d faces = %d, want 6", name, bi, len(br.Faces))
			}
			for _, f := range br.Faces {
				if !onLattice(f.Plane().Dist, 0.6) {
					t.Fatalf("%s brush %d face dist %v off lattice", name, bi, f.Plane().Dist)
				}
			}
		}
	}
	nb("shell", g.shellFor(rm, false, false))
	nb("connectRooms", g.connectRooms(rm, g.randomRoom(rm.x1+rm.wallT+128, 0, 0)))
	nb("placeStairs", g.placeStairs(rm))
	nb("placeTrims", g.placeTrims(rm))
	nb("placeDetails", g.placeDetails(rm))
	nb("placeLiquids", g.placeLiquids(rm))
}

func TestSynthPairCompilesClean(t *testing.T) {
	dir := t.TempDir()
	g := NewGenerator(1234, 5)
	for n := 0; n < 5; n++ {
		m := g.GenMap(n)
		var buf bytes.Buffer
		if err := g.Emit(m, &buf); err != nil {
			t.Fatal(err)
		}
		src := filepath.Join(dir, fmt.Sprintf("s%d.map", n))
		if err := os.WriteFile(src, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := eval.CompileMapPair(src, dir)
		if err != nil {
			t.Fatalf("map %d fails to compile: %v", n, err)
		}
		data, err := os.ReadFile(p.BSPPath)
		if err != nil {
			t.Fatal(err)
		}
		brushes, err := bspdec.BrushListFromBSP(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(brushes) < 6 {
			t.Fatalf("map %d: BRUSHLIST brushes = %d, want >= 6", n, len(brushes))
		}
	}
}