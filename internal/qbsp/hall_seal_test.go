package qbsp

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

func slabFace(x0, y0, z0, x1, y1, z1 float64, tex string) string {
	f := func(p1, p2, p3 [3]float64) string {
		return fmt.Sprintf("( %g %g %g ) ( %g %g %g ) ( %g %g %g ) %s 0 0 0 1 1\n",
			p1[0], p1[1], p1[2], p2[0], p2[1], p2[2], p3[0], p3[1], p3[2], tex)
	}
	mins := [3]float64{x0, y0, z0}
	maxs := [3]float64{x1, y1, z1}
	return "{\n" +
		f([3]float64{maxs[0], mins[1], mins[2]}, [3]float64{maxs[0], mins[1], maxs[2]}, [3]float64{maxs[0], maxs[1], mins[2]}) +
		f([3]float64{mins[0], maxs[1], mins[2]}, [3]float64{mins[0], maxs[1], maxs[2]}, [3]float64{mins[0], mins[1], maxs[2]}) +
		f([3]float64{mins[0], maxs[1], mins[2]}, [3]float64{maxs[0], maxs[1], mins[2]}, [3]float64{mins[0], maxs[1], maxs[2]}) +
		f([3]float64{mins[0], mins[1], mins[2]}, [3]float64{mins[0], mins[1], maxs[2]}, [3]float64{maxs[0], mins[1], mins[2]}) +
		f([3]float64{mins[0], mins[1], maxs[2]}, [3]float64{mins[0], maxs[1], maxs[2]}, [3]float64{maxs[0], mins[1], maxs[2]}) +
		f([3]float64{mins[0], mins[1], mins[2]}, [3]float64{maxs[0], mins[1], mins[2]}, [3]float64{mins[0], maxs[1], mins[2]}) +
		"}\n"
}

// TestFloorSurvivesSplittingWall reproduces the id1-map risk class behind
// ironwail-go-aeh: a hall floor slab split by an intersecting wall plane
// (x=632) must keep its east slice solid, and the hall must compile sealed.
// The smaller construct sealed in the investigation; e1m1's equivalent slice
// still loses solidity east of x~632 — this locks the simple interaction.
func TestFloorSurvivesSplittingWall(t *testing.T) {
	src := "{\n\"classname\" \"worldspawn\"\n" +
		slabFace(320, 0, 176, 704, 40, 208, "mt_floor") +
		slabFace(632, 0, 208, 640, 40, 272, "mt_wall") +
		slabFace(320, -16, 176, 704, 0, 272, "mt_wall") + // north wall of the hall
		slabFace(320, 40, 176, 704, 56, 272, "mt_wall") + // south wall
		slabFace(320, -16, 208, 704, 56, 240, "mt_floor") + // ceiling slab
		slabFace(300, -30, 160, 724, 70, 288, "mt_rock") + // outer shell block
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"400 20 190\"\n}\n"
	m, err := mapfile.Parse(bytes.NewReader([]byte(src)))
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(m, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Leaked {
		t.Fatalf("sealed hall leaked (%d trail points)", len(res.LeakPath))
	}
	tr, err := bsp.LoadTree(bytes.NewReader(res.Data))
	if err != nil {
		t.Fatal(err)
	}
	// the floor slab interior (z=200) and the east slice (x>632) stay solid
	for _, pt := range [][3]float64{{700, 20, 200}, {680, 20, 210}, {660, 20, 250}, {500, 20, 200}} {
		if v := contentsAt2(tr, vec3{X: pt[0], Y: pt[1], Z: pt[2]}); v == bsp.ContentsEmpty {
			t.Fatalf("point (%v %v %v) lost its solidity", pt[0], pt[1], pt[2])
		}
	}
}

func contentsAt2(tr *bsp.Tree, p vec3) int32 {
	idx := 0
	for {
		if idx < 0 || idx >= len(tr.Nodes) {
			return -2
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
