package bspdec

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// slabBox returns one QuakeEd-format box brush string. The face winding
// pattern is copied from internal/qbsp/mapfile_test.go's slabBrush, which the
// qbsp suite compiles successfully, so orientation is compile-proven.
func slabBox(x0, y0, z0, x1, y1, z1 float64, tex string) string {
	format := func(p1, p2, p3 [3]float64) string {
		return fmt.Sprintf("( %g %g %g ) ( %g %g %g ) ( %g %g %g ) %s 0 0 0 1 1\n",
			p1[0], p1[1], p1[2], p2[0], p2[1], p2[2], p3[0], p3[1], p3[2], tex)
	}
	mins := [3]float64{x0, y0, z0}
	maxs := [3]float64{x1, y1, z1}
	return "{\n" +
		format([3]float64{maxs[0], mins[1], mins[2]}, [3]float64{maxs[0], mins[1], maxs[2]}, [3]float64{maxs[0], maxs[1], mins[2]}) +
		format([3]float64{mins[0], maxs[1], mins[2]}, [3]float64{mins[0], maxs[1], maxs[2]}, [3]float64{mins[0], mins[1], maxs[2]}) +
		format([3]float64{mins[0], maxs[1], mins[2]}, [3]float64{maxs[0], maxs[1], mins[2]}, [3]float64{mins[0], maxs[1], maxs[2]}) +
		format([3]float64{mins[0], mins[1], mins[2]}, [3]float64{mins[0], mins[1], maxs[2]}, [3]float64{maxs[0], mins[1], mins[2]}) +
		format([3]float64{mins[0], mins[1], maxs[2]}, [3]float64{mins[0], maxs[1], maxs[2]}, [3]float64{maxs[0], mins[1], maxs[2]}) +
		format([3]float64{mins[0], mins[1], mins[2]}, [3]float64{maxs[0], mins[1], mins[2]}, [3]float64{mins[0], maxs[1], mins[2]}) +
		"}\n"
}

// roomMap is a sealed room: interior x,y in [0,256], z in [0,192], 64-unit
// walls, with floor/wall/ceiling textured differently for the texturing
// tasks. info_player_start is required: qbsp leak detection needs an entity
// inside the empty region.
func roomMap() string {
	return "{\n\"classname\" \"worldspawn\"\n" +
		slabBox(-64, -64, -64, 0, 320, 256, "wwall") +
		slabBox(256, -64, -64, 320, 320, 256, "wwall") +
		slabBox(0, -64, -64, 256, 0, 256, "wwall") +
		slabBox(0, 256, -64, 256, 320, 256, "wwall") +
		slabBox(0, 0, -64, 256, 256, 0, "ffloor") +
		slabBox(0, 0, 192, 256, 256, 256, "cceil") +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"128 128 64\"\n}\n"
}

// compileFixture compiles mapSrc with the in-repo qbsp and loads the BSP
// tree; the compile always appends a BRUSHLIST BSPX oracle.
func compileFixture(t *testing.T, mapSrc string) (*bsp.Tree, []byte) {
	t.Helper()
	m, err := qbsp.ParseMap(strings.NewReader(mapSrc))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.Leaked {
		t.Fatalf("fixture leaks; fix the fixture")
	}
	tree, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("LoadTree: %v", err)
	}
	return tree, res.Data
}

func TestBoxBrushHasSixClosedSides(t *testing.T) {
	b := boxBrush(vc(0, 0, 0), vc(64, 64, 64))
	if len(b.Sides) != 6 {
		t.Fatalf("sides = %d, want 6", len(b.Sides))
	}
	for i, s := range b.Sides {
		if s.Winding == nil || len(s.Winding.Points) != 4 {
			t.Fatalf("side %d winding not a closed quad", i)
		}
		if a := s.Winding.Area(); a < 64*64-1 {
			t.Fatalf("side %d area = %v, want 4096", i, a)
		}
	}
}

func TestSplitBrushProducesTwoHalves(t *testing.T) {
	b := boxBrush(vc(0, 0, 0), vc(64, 64, 64))
	front, back := splitBrush(b, mapfile.Plane{Normal: vc(1, 0, 0), Dist: 32})
	if front == nil || back == nil {
		t.Fatalf("front=%v back=%v, want both non-nil", front != nil, back != nil)
	}
	for name, half := range map[string]*Brush{"front": front, "back": back} {
		if len(half.Sides) != 6 {
			t.Fatalf("%s sides = %d, want 6", name, len(half.Sides))
		}
		vol := 0.0
		for _, s := range half.Sides {
			if s.Winding == nil {
				t.Fatalf("%s has nil winding", name)
			}
			vol += s.Winding.Area()
		}
		if vol < 6*64*32-64 {
			t.Fatalf("%s total area %v implausibly small", name, vol)
		}
	}
}

func TestDecompileModelEmitsSolidCells(t *testing.T) {
	tree, data := compileFixture(t, roomMap())
	counts, err := qbsp.ReadBSPXBrushList(data)
	if err != nil {
		t.Fatalf("ReadBSPXBrushList: %v", err)
	}
	if len(counts) == 0 || counts[0] != 6 {
		t.Fatalf("BRUSHLIST oracle = %v, want [6]", counts)
	}
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	brushes, err := d.decompileModel(0)
	if err != nil {
		t.Fatalf("decompileModel: %v", err)
	}
	// CSG splits can only increase cell count vs the original 6 brushes.
	if len(brushes) < counts[0] {
		t.Fatalf("brushes = %d, want >= %d (oracle)", len(brushes), counts[0])
	}
	if d.leavesSolid == 0 {
		t.Fatal("no solid leaves visited")
	}
	for i, b := range brushes {
		if b.Contents == bsp.ContentsEmpty {
			t.Fatalf("brush %d emitted with empty contents", i)
		}
		if len(b.Sides) < 4 {
			t.Fatalf("brush %d has %d sides", i, len(b.Sides))
		}
		for _, s := range b.Sides {
			if s.Winding == nil || len(s.Winding.Points) < 3 {
				t.Fatalf("brush %d has degenerate side", i)
			}
		}
	}
}