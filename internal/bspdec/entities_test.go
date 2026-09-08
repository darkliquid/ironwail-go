package bspdec

import (
	"testing"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// roomWithDoorMap adds a func_door bmodel inside the room.
func roomWithDoorMap() string {
	return "{\n\"classname\" \"worldspawn\"\n" +
		slabBox(-64, -64, -64, 0, 320, 256, "wwall") +
		slabBox(256, -64, -64, 320, 320, 256, "wwall") +
		slabBox(0, -64, -64, 256, 0, 256, "wwall") +
		slabBox(0, 256, -64, 256, 320, 256, "wwall") +
		slabBox(0, 0, -64, 256, 256, 0, "ffloor") +
		slabBox(0, 0, 192, 256, 256, 256, "cceil") +
		"}\n" +
		"{\n\"classname\" \"func_door\"\n\"targetname\" \"d1\"\n\"sounds\" \"2\"\n" +
		slabBox(96, 96, 16, 160, 160, 112, "ddoor") +
		"}\n" +
		"{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 64\"\n}\n"
}

func TestAttachBrushesPreservesEntities(t *testing.T) {
	tree, data := compileFixture(t, roomWithDoorMap())
	counts, err := qbsp.ReadBSPXBrushList(data)
	if err != nil {
		t.Fatalf("ReadBSPXBrushList: %v", err)
	}
	if len(counts) != 2 || counts[0] != 6 || counts[1] != 1 {
		t.Fatalf("BRUSHLIST oracle = %v, want [6 1]", counts)
	}
	d := newDecompiler(tree, Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	perModel := make([][]*Brush, len(tree.Models))
	for mi := range tree.Models {
		brushes, err := d.decompileModel(mi)
		if err != nil {
			t.Fatalf("decompileModel(%d): %v", mi, err)
		}
		for _, b := range brushes {
			removeRedundantPlanes(b)
		}
		d.textureBrushes(brushes)
		perModel[mi] = brushes
	}
	ents, err := parseEntities(tree)
	if err != nil {
		t.Fatalf("parseEntities: %v", err)
	}
	out := attachBrushes(ents, perModel, tree)

	// worldspawn got model-0 brushes
	ws := out.Entities[0]
	if v, _ := ws.Value("classname"); v != "worldspawn" {
		t.Fatalf("entity 0 classname = %q", v)
	}
	if len(ws.Brushes) < counts[0] {
		t.Fatalf("worldspawn brushes = %d, want >= %d", len(ws.Brushes), counts[0])
	}

	// the door entity keeps its epairs verbatim and gains model-1 brushes
	var door *mapfile.Entity
	for i := range out.Entities {
		if v, _ := out.Entities[i].Value("classname"); v == "func_door" {
			door = &out.Entities[i]
		}
	}
	if door == nil {
		t.Fatal("func_door entity lost")
	}
	if v, ok := door.Value("targetname"); !ok || v != "d1" {
		t.Fatalf("door targetname = %q, %v", v, ok)
	}
	if v, ok := door.Value("sounds"); !ok || v != "2" {
		t.Fatalf("door sounds = %q, %v (epairs must be verbatim)", v, ok)
	}
	if v, ok := door.Value("model"); !ok || v != "*1" {
		t.Fatalf("door model = %q, %v, want *1 (kept from the lump)", v, ok)
	}
	if len(door.Brushes) < counts[1] {
		t.Fatalf("door brushes = %d, want >= %d", len(door.Brushes), counts[1])
	}
	// door geometry stays at its authored position (96..160)
	if len(perModel[1]) == 0 {
		t.Fatal("model 1 produced no brushes")
	}
	found := false
	for _, s := range perModel[1][0].Sides {
		if s.Plane.Normal == (mapfile.Vec3{X: 0, Y: 0, Z: 1}) {
			found = true
		}
	}
	if !found {
		t.Fatal("door brush has no +z side")
	}
	for _, f := range door.Brushes[0].Faces {
		for _, p := range f.Points {
			if p.X < 96-72 || p.X > 160+72 {
				t.Fatalf("door point %v far from authored bounds", p)
			}
		}
	}
}

func TestPick3PointsNonCollinear(t *testing.T) {
	w := &Winding{Points: []mapfile.Vec3{vc(0, 0, 0), vc(32, 0, 0), vc(64, 0, 0), vc(64, 64, 0), vc(0, 64, 0)}}
	pts := pick3Points(w)
	p, length := mapfile.PlaneFromPoints(pts[0], pts[1], pts[2])
	if length < 0.01 {
		t.Fatalf("degenerate triple %v", pts)
	}
	_ = p
}