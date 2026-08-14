package audio

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestUnderwaterIntensity(t *testing.T) {
	cases := []struct {
		contents int32
		want     float32
	}{
		{bsp.ContentsWater, 1},
		{bsp.ContentsSlime, 1},
		{bsp.ContentsLava, 1},
		{bsp.ContentsSolid, 0},
		{bsp.ContentsEmpty, 0},
	}
	for _, c := range cases {
		if got := UnderwaterIntensity(c.contents); got != c.want {
			t.Errorf("UnderwaterIntensity(%d) = %v, want %v", c.contents, got, c.want)
		}
	}
}

func TestPointInTreeLeafSingleNode(t *testing.T) {
	tree := &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 0}},
		Nodes:  []bsp.TreeNode{{PlaneNum: 0, Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 0}, {IsLeaf: true, Index: 1}}}},
		Leafs: []bsp.TreeLeaf{
			{Contents: int32(bsp.ContentsEmpty)},
			{Contents: int32(bsp.ContentsSolid)},
		},
	}

	// Below z=0 plane -> side 1 -> leaf 1 (solid).
	leaf, ok := PointInTreeLeaf(tree, types.Vec3{X: 0, Y: 0, Z: -10})
	if !ok {
		t.Fatal("PointInTreeLeaf failed on valid tree")
	}
	if leaf.Contents != int32(bsp.ContentsSolid) {
		t.Errorf("leaf contents = %d, want ContentsSolid", leaf.Contents)
	}

	// Above z=0 plane -> side 0 -> leaf 0 (empty).
	leaf, ok = PointInTreeLeaf(tree, types.Vec3{X: 0, Y: 0, Z: 10})
	if !ok {
		t.Fatal("PointInTreeLeaf failed on upper side")
	}
	if leaf.Contents != int32(bsp.ContentsEmpty) {
		t.Errorf("leaf contents = %d, want ContentsEmpty", leaf.Contents)
	}
}

func TestPointInTreeLeafInvalidTree(t *testing.T) {
	if _, ok := PointInTreeLeaf(nil, types.Vec3{}); ok {
		t.Error("PointInTreeLeaf(nil tree) = ok, want false")
	}
	tree := &bsp.Tree{Planes: []bsp.DPlane{{}}, Nodes: []bsp.TreeNode{{Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 99}}}}}
	if _, ok := PointInTreeLeaf(tree, types.Vec3{}); ok {
		t.Error("PointInTreeLeaf(out-of-range leaf) = ok, want false")
	}
}
