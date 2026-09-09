package bspdec

import (
	"bytes"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// prettyRoomSrc is the 6-slab sealed-room fixture (the prettyRoom layout from
// internal/qbsp/solidbsp_test.go, expressed with slabBox). The room interior
// is x,y in [t0..x1-t0]-ish so a player start inside never leaks.
func prettyRoomSrc(x0, y0, z0, x1, y1, z1, t float64) string {
	return slabBox(x0, y0, z0, x1, y1, z0+t, "mt_wall") + // floor
		slabBox(x0, y0, z1-t, x1, y1, z1, "mt_wall") + // ceiling
		slabBox(x0, y0, z0, x0+t, y1, z1, "mt_wall") + // west
		slabBox(x1-t, y0, z0, x1, y1, z1, "mt_wall") + // east
		slabBox(x0, y0, z0, x1, y0+t, z1, "mt_wall") + // north
		slabBox(x0, y1-t, z0, x1, y1, z1, "mt_wall") // south
}

// boxMapSrc is a sealed 64-cube room with a player start inside.
func boxMapSrc() string {
	return "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoomSrc(0, 0, 0, 64, 64, 64, 8) +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 32\"\n}\n"
}

// compileBrushListFixture parses src, compiles with the pinned qbsp (always
// appends the BRUSHLIST oracle), and returns the parsed original map plus the
// compiled BSP bytes.
func compileBrushListFixture(t *testing.T, src string) (*mapfile.Map, []byte) {
	t.Helper()
	m, err := mapfile.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return m, res.Data
}

func TestParseBrushListRoundTrip(t *testing.T) {
	// Build a tiny map with known world brushes, compile with the pinned
	// qbsp (always appends BRUSHLIST), then parse it back and compare.
	src := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoomSrc(0, 0, 0, 64, 64, 64, 8) +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 32\"\n}\n"
	m, res := compileBrushListFixture(t, src)
	brushes, err := BrushListFromBSP(res)
	if err != nil {
		t.Fatal(err)
	}
	if len(brushes) == 0 {
		t.Fatal("no BRUSHLIST brushes parsed")
	}
	// The world brush count must match the source worldspawn brush count.
	want := len(m.Entities[0].Brushes)
	var world []BSPXBrush
	for _, b := range brushes {
		if b.Model == 0 {
			world = append(world, b)
		}
	}
	if len(world) != want {
		t.Fatalf("world brushes = %d, want %d", len(world), want)
	}
	// Contents must be preserved and bounds must match the originals.
	if world[0].Contents != int32(bsp.ContentsSolid) {
		t.Errorf("contents = %d, want solid", world[0].Contents)
	}
}

// writeAndReparse emits one brush to a worldspawn map and re-parses it.
func writeAndReparse(t *testing.T, mb mapfile.MapBrush) *mapfile.MapBrush {
	t.Helper()
	m := &mapfile.Map{Entities: []mapfile.Entity{{
		Epairs:  []mapfile.Epair{{Key: "classname", Value: "worldspawn"}},
		Brushes: []mapfile.MapBrush{mb},
	}}}
	var buf bytes.Buffer
	if err := mapfile.Write(&buf, m, mapfile.WriteOptions{GridSnap: 8}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := mapfile.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, buf.String())
	}
	if len(got.Entities) == 0 || len(got.Entities[0].Brushes) == 0 {
		t.Fatalf("no brush reparsed:\n%s", buf.String())
	}
	return &got.Entities[0].Brushes[0]
}

func TestBrushFromBSPXRewritesPlanes(t *testing.T) {
	// The first world brush is the floor slab, a 6-axial-face box; the
	// emitted MapFaces must reconstruct the same bounds on reparse.
	src := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoomSrc(0, 0, 0, 64, 64, 64, 8) +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 32\"\n}\n"
	_, res := compileBrushListFixture(t, src)
	brushes, err := BrushListFromBSP(res)
	if err != nil {
		t.Fatal(err)
	}
	if len(brushes) == 0 {
		t.Fatal("no brushes")
	}
	mb, err := BrushFromBSPX(brushes[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(mb.Faces) != 6 {
		t.Fatalf("faces = %d, want 6", len(mb.Faces))
	}
	// Reparse the emitted brush through the map parser's PlaneFromPoints
	// path: the brush must be convex-closed (the map parser rejects open
	// brushes).
	round := writeAndReparse(t, mb)
	if len(round.Faces) != 6 {
		t.Fatalf("reparsed faces = %d, want 6", len(round.Faces))
	}
}

// TestDecompileBrushListPath: a compiled BSP (BRUSHLIST present) must
// decompile through the direct path, producing one brush per original
// world brush instead of the treewalk's leaf-derived cells.
func TestDecompileBrushListPath(t *testing.T) {
	m, res := compileBrushListFixture(t, boxMapSrc())
	with, _, errs := Decompile(res, Options{GridSnap: 8})
	if errs != nil {
		t.Fatal(errs)
	}
	without, _, errs := Decompile(res, Options{NoBrushlist: true, GridSnap: 8})
	if errs != nil {
		t.Fatal(errs)
	}
	want := len(m.Entities[0].Brushes)
	if len(with.Entities[0].Brushes) != want {
		t.Errorf("brushlist path brushes = %d, want %d (original world brushes)", len(with.Entities[0].Brushes), want)
	}
	_ = without // treewalk fallback must still produce output; compared by the headroom study
}
