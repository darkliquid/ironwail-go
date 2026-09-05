package qbsp

import (
	"bytes"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// clipPointContents walks a clipnode tree (engine hull semantics) from a
// root clipnode index and returns the contents at p: negative child values
// are contents, non-negative are clipnode indices. Planes come from the
// BSP plane lump.
func clipPointContents(clip []bsp.DSClipNode, planes []bsp.DPlane, root int, p [3]float32) int32 {
	if root < 0 || root >= len(clip) {
		return bsp.ContentsSolid
	}
	for {
		node := clip[root]
		if node.PlaneNum < 0 || int(node.PlaneNum) >= len(planes) {
			return bsp.ContentsSolid
		}
		pl := &planes[node.PlaneNum]
		var d float32
		switch pl.Type {
		case 0:
			d = p[0] - pl.Dist
		case 1:
			d = p[1] - pl.Dist
		case 2:
			d = p[2] - pl.Dist
		default:
			n := pl.Normal
			d = p[0]*n.X + p[1]*n.Y + p[2]*n.Z - pl.Dist
		}
		child := node.Children[0]
		if d < 0 {
			child = node.Children[1]
		}
		if child < 0 {
			return child
		}
		root = int(child)
	}
}

// TestClipHullInteriorAir is AC1: with the player-box (±16) clip hull a
// point trace through an 8-thick wall room stays in air in the interior
// and through a door opening, while points inside the (expanded) walls are
// solid. The old ±32 expansion collapsed the interior to a single solid
// clipnode.
func TestClipHullInteriorAir(t *testing.T) {
	// AC1 core: with the player-box (+-16) clip hull, a point trace through
	// an 8-thick wall room stays in air in the interior and is solid inside
	// the (expanded) walls. The old expansion added the z hull term to x/y
	// planes and used the +-32 large box, collapsing thin-wall interiors to
	// a single solid clipnode.
	src := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoom(0, 0, 0, 64, 64, 128, 8) +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 64\"\n}\n"
	m, err := ParseMap(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(m, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Leaked {
		t.Fatal("sealed room leaked")
	}
	file, err := bsp.Load(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("bsp.Load: %v", err)
	}
	clip, ok := file.Clipnodes.([]bsp.DSClipNode)
	if !ok {
		t.Fatalf("clipnodes type %T, want []DSClipNode", file.Clipnodes)
	}
	// The world hull-1 tree must be a real tree, not the single-solid
	// collapse the old expansion produced.
	if len(clip) < 8 {
		t.Fatalf("world clipnodes = %d, want a real tree (>= 8)", len(clip))
	}
	// Hull 1 (player box +-16) starts at clipnode 0 (the engine's contract).
	const hull1Root = 0

	check := []struct {
		name string
		p    [3]float32
		want int32
	}{
		{"room center", [3]float32{32, 32, 64}, bsp.ContentsEmpty},
		{"wall interior", [3]float32{20, 32, 64}, bsp.ContentsSolid},
		{"floor slab", [3]float32{32, 32, 4}, bsp.ContentsSolid},
	}
	for _, c := range check {
		got := clipPointContents(clip, file.Planes, hull1Root, c.p)
		if got != c.want {
			t.Errorf("%s: hull1 point = %d, want %d", c.name, got, c.want)
		}
	}
	// The world model records a separate hull-2 root (headnode[2]).
	world := file.Models[0]
	if int(world.HeadNode[2]) < 0 || int(world.HeadNode[2]) >= len(clip) {
		t.Errorf("world hull2 root %d out of range (%d clipnodes)", world.HeadNode[2], len(clip))
	}
}

func TestSubmodelHullRoots(t *testing.T) {
	m, err := ParseMap(strings.NewReader(boxMapWithFuncWall()))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(m, Options{})
	if err != nil {
		t.Fatal(err)
	}
	file, err := bsp.Load(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatal(err)
	}
	clip, ok := file.Clipnodes.([]bsp.DSClipNode)
	if !ok {
		t.Fatalf("clipnodes type %T", file.Clipnodes)
	}
	sub := file.Models[1]
	if int(sub.HeadNode[1]) >= len(clip) || int(sub.HeadNode[1]) < 0 {
		t.Fatalf("submodel hull1 root %d out of range (%d clipnodes)", sub.HeadNode[1], len(clip))
	}
	if int(sub.HeadNode[2]) >= len(clip) || int(sub.HeadNode[2]) < 0 {
		t.Fatalf("submodel hull2 root %d out of range", sub.HeadNode[2])
	}
	if sub.HeadNode[1] == sub.HeadNode[2] && int(sub.HeadNode[1]) != 0 {
		t.Errorf("hull roots share a node: %d", sub.HeadNode[1])
	}
	// A point inside the func_wall slab (28..36 box) is solid in hull 1.
	got := clipPointContents(clip, file.Planes, int(sub.HeadNode[1]), [3]float32{32, 32, 28})
	if got != bsp.ContentsSolid {
		t.Errorf("submodel interior point = %d, want solid", got)
	}
}
