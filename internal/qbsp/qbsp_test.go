package qbsp

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// compileMapString parses and compiles a map, failing the test on error.
func compileMapString(t *testing.T, src string) *CompileResult {
	t.Helper()
	m, err := ParseMap(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := Compile(m, Options{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return res
}

func loadTreeResult(t *testing.T, res *CompileResult) *bsp.Tree {
	t.Helper()
	tree, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("LoadTree: %v\nlogs:\n%v", err, res.Log)
	}
	return tree
}

func TestCompileBoxRoundTrip(t *testing.T) {
	res := compileMapString(t, boxMap())
	if res.Leaked {
		t.Fatal("sealed box reported as leaking")
	}
	tree := loadTreeResult(t, res)

	if len(tree.Models) == 0 {
		t.Fatal("no world model")
	}
	m := tree.Models[0]
	if m.HeadNode[0] < 0 {
		t.Errorf("world headnode[0] = %d", m.HeadNode[0])
	}

	if len(tree.Nodes) == 0 {
		t.Fatal("no nodes")
	}
	if len(tree.Leafs) < 2 {
		t.Errorf("leafs = %d, want >= 2 (solid + empty)", len(tree.Leafs))
	}
	if len(tree.Faces) < 6 {
		t.Errorf("faces = %d, want >= 6 (box exterior)", len(tree.Faces))
	}
	if len(tree.Planes) < 12 {
		t.Errorf("planes = %d, want >= 12 (brush + box)", len(tree.Planes))
	}

	// Structural invariants.
	for i, f := range tree.Faces {
		if f.NumEdges < 3 {
			t.Errorf("face %d has %d edges", i, f.NumEdges)
		}
		if f.FirstEdge < 0 || int(f.FirstEdge)+int(f.NumEdges) > len(tree.Surfedges) {
			t.Errorf("face %d edge range out of bounds", i)
		}
	}
	for i, se := range tree.Surfedges {
		e := int(se)
		if e < 0 {
			e = -e - 1
		}
		if e >= len(tree.Edges) {
			t.Errorf("surfedge %d -> edge %d out of bounds", i, e)
		}
	}
	for i, e := range tree.Edges {
		for _, v := range e.V {
			if int(v) >= len(tree.Vertexes) {
				t.Errorf("edge %d vertex %d out of bounds", i, v)
			}
		}
	}

	// Tree walk from the world head node.
	solid, empty := 0, 0
	walk := func(node bsp.TreeChild) { /* closure below */ }
	_ = walk
	var walkAll func(n int)
	walkAll = func(n int) {
		node := tree.Nodes[n]
		for _, ch := range node.Children {
			if ch.IsLeaf {
				if ch.Index < 0 || ch.Index >= len(tree.Leafs) {
					t.Fatalf("leaf child %d out of bounds", ch.Index)
				}
				c := tree.Leafs[ch.Index].Contents
				switch c {
				case bsp.ContentsSolid:
					solid++
				case bsp.ContentsEmpty:
					empty++
				}
			} else {
				if ch.Index < 0 || ch.Index >= len(tree.Nodes) {
					t.Fatalf("node child %d out of bounds", ch.Index)
				}
				walkAll(ch.Index)
			}
		}
	}
	walkAll(int(m.HeadNode[0]))
	if solid == 0 {
		t.Error("no solid leaf reached from world head node")
	}
	if empty == 0 {
		t.Error("no empty leaf reached from world head node")
	}
}

func TestCompileBoxClipnodes(t *testing.T) {
	res := compileMapString(t, boxMap())
	file, err := bsp.Load(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("bsp.Load: %v", err)
	}
	clip, ok := file.Clipnodes.([]bsp.DSClipNode)
	if !ok {
		t.Fatalf("clipnodes type %T, want []DSClipNode", file.Clipnodes)
	}
	if len(clip) == 0 {
		t.Fatal("no clipnodes")
	}
	for i, cn := range clip {
		if cn.PlaneNum < 0 || int(cn.PlaneNum) >= len(file.Planes) {
			t.Errorf("clipnode %d plane %d out of range", i, cn.PlaneNum)
		}
		for _, ch := range cn.Children {
			if ch < 0 {
				if ch < bsp.ContentsSky {
					t.Errorf("clipnode %d child %d: unknown contents", i, ch)
				}
			} else if int(ch) >= len(clip) {
				t.Errorf("clipnode %d child %d out of range", i, ch)
			}
		}
	}
}

// boxBrush renders a six-face brush for an axis-aligned box with outward
// windings, using the same convention as boxMap()'s faces.
func boxBrush(name string, mins, maxs [3]float64, tex string) string {
	format := func(p1, p2, p3 [3]float64) string {
		return fmt.Sprintf("( %g %g %g ) ( %g %g %g ) ( %g %g %g ) %s 0 0 0 1 1\n",
			p1[0], p1[1], p1[2], p2[0], p2[1], p2[2], p3[0], p3[1], p3[2], tex)
	}
	_ = name
	// outward windings (verified convention): +x, -x, +y, -y, +z, -z
	return "{\n" +
		format([3]float64{maxs[0], mins[1], mins[2]}, [3]float64{maxs[0], mins[1], maxs[2]}, [3]float64{maxs[0], maxs[1], mins[2]}) +
		format([3]float64{mins[0], maxs[1], mins[2]}, [3]float64{mins[0], maxs[1], maxs[2]}, [3]float64{mins[0], mins[1], maxs[2]}) +
		format([3]float64{mins[0], maxs[1], mins[2]}, [3]float64{maxs[0], maxs[1], mins[2]}, [3]float64{mins[0], maxs[1], maxs[2]}) +
		format([3]float64{mins[0], mins[1], mins[2]}, [3]float64{mins[0], mins[1], maxs[2]}, [3]float64{maxs[0], mins[1], mins[2]}) +
		format([3]float64{mins[0], mins[1], maxs[2]}, [3]float64{mins[0], maxs[1], maxs[2]}, [3]float64{maxs[0], mins[1], maxs[2]}) +
		format([3]float64{mins[0], mins[1], mins[2]}, [3]float64{maxs[0], mins[1], mins[2]}, [3]float64{mins[0], maxs[1], mins[2]}) +
		"}\n"
}

func TestCompileWaterContents(t *testing.T) {
	// Solid box with an interior water pool: the pool's cells get WATER
	// (later brushes override), the pool is sealed by the solid box (no
	// leak), and a water leaf exists.
	src := boxMap()
	// Insert the water brush after the world brush: worldspawn entity
	// contains both brushes.
	src = strings.Replace(src, "}\n}\n{\n\"classname\" \"info_player_start\"",
		"}\n"+boxBrush("pool", [3]float64{16, 16, 16}, [3]float64{48, 48, 48}, "*water1")+
			"}\n{\n\"classname\" \"info_player_start\"", 1)
	res := compileMapString(t, src)
	if res.Leaked {
		t.Fatal("water pool box leaked")
	}
	tree := loadTreeResult(t, res)
	water := false
	for _, l := range tree.Leafs {
		if l.Contents == bsp.ContentsWater {
			water = true
			break
		}
	}
	if !water {
		t.Error("no water leaf found")
	}
}

func TestCompileLeakDetection(t *testing.T) {
	// A hollow room with NO floor slab: the interior is reachable from the
	// void below → leak.
	open := "{\n\"classname\" \"worldspawn\"\n" +
		strings.Replace(hollowRoom(0, 0, 0, 64, 64, 64, 8),
			slabBrush(0, 0, 0, 64, 64, 8, "mt_floor"), "", 1) +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"32 32 32\"\n}\n"
	res := compileMapString(t, open)
	if !res.Leaked {
		t.Fatal("expected leak with an open floor")
	}
	if len(res.LeakPath) < 1 {
		t.Errorf("leak trail has %d points, want >= 1", len(res.LeakPath))
	}
}

func TestCompileDeterministic(t *testing.T) {
	a := compileMapString(t, boxMap())
	b := compileMapString(t, boxMap())
	if !bytes.Equal(a.Data, b.Data) {
		t.Error("compile output is not deterministic")
	}
}

func TestCompileBSP2(t *testing.T) {
	m, err := ParseMap(strings.NewReader(boxMap()))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(m, Options{BSP2: true})
	if err != nil {
		t.Fatalf("Compile BSP2: %v", err)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatalf("LoadTree (BSP2): %v", err)
	}
	if tree.Version != bsp.BSP2Version_BSP2 {
		t.Errorf("version = %x, want BSP2", tree.Version)
	}
	if len(tree.Nodes) == 0 || len(tree.Faces) < 6 {
		t.Errorf("BSP2 tree empty: nodes %d faces %d", len(tree.Nodes), len(tree.Faces))
	}
}

func TestCompileCorridor(t *testing.T) {
	// Two rooms joined by a door tube. Brush planes bound halfspaces, so
	// each room is a 5-sided box with its shared-wall side omitted, and the
	// doorway is a separate 6-faced box tween the openings.
	roomA := boxBrush("roomA", [3]float64{0, 0, 0}, [3]float64{64, 64, 64}, "mt_wall")
	roomB := boxBrush("roomB", [3]float64{80, 0, 0}, [3]float64{144, 64, 64}, "mt_wall")
	door := boxBrush("door", [3]float64{64, 24, 16}, [3]float64{80, 40, 48}, "mt_wall")

	// Remove roomA's +x face and roomB's -x face (the openings).
	stripFace := func(brush, p1 string) string {
		idx := strings.Index(brush, p1)
		if idx < 0 {
			return brush
		}
		end := strings.Index(brush[idx:], "\n")
		return brush[:idx] + brush[idx+end+1:]
	}
	roomA = stripFace(roomA, "( 64 0 0 ) ( 64 0 64 ) ( 64 64 0 )")
	roomB = stripFace(roomB, "( 80 64 0 ) ( 80 64 64 ) ( 80 0 64 )")

	src := "{\n\"classname\" \"worldspawn\"\n" + roomA + roomB + door + "}\n" +
		"{\n\"classname\" \"info_player_start\"\n\"origin\" \"72 32 32\"\n}\n"
	res := compileMapString(t, src)
	tree := loadTreeResult(t, res)
	if len(tree.Nodes) == 0 {
		t.Fatal("no nodes")
	}
	if len(tree.Faces) < 12 {
		t.Errorf("faces = %d, want >= 12 (two rooms + door tube)", len(tree.Faces))
	}
}