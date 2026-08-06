// modelbuild_test.go verifies the BSP-to-model converters in isolation using
// a minimal synthetic BSP tree.
package collision

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
)

func TestWorldModelFromBSPTreeBuildsNodes(t *testing.T) {
	tree := &bsp.Tree{
		Planes: []bsp.DPlane{
			{Normal: [3]float32{0, 0, 1}, Dist: 0},
		},
		Nodes: []bsp.TreeNode{
			{PlaneNum: 0, Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 0}, {IsLeaf: false, Index: 1}}},
			{PlaneNum: 0},
		},
		Leafs: []bsp.TreeLeaf{{Contents: int32(bsp.ContentsEmpty)}},
		Models: []bsp.DModel{
			{
				BoundsMin: [3]float32{-100, -100, 0},
				BoundsMax: [3]float32{100, 100, 128},
				HeadNode:  [4]int32{0, -1, -1, -1},
			},
		},
	}

	m := WorldModelFromBSPTree("testmap", tree)

	if m.Type != model.ModBrush {
		t.Errorf("Type = %v, want ModBrush", m.Type)
	}
	if len(m.Nodes) != 2 {
		t.Errorf("Nodes = %d, want 2", len(m.Nodes))
	}
	if m.Nodes[0].Plane == nil {
		t.Error("node 0 plane = nil, want set")
	}
	if !m.ClipBox {
		t.Error("ClipBox = false, want true (world model)")
	}
}

func TestBuildNodeHullTracksLeafContents(t *testing.T) {
	tree := &bsp.Tree{
		Nodes: []bsp.TreeNode{
			{PlaneNum: 0, Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 0}, {IsLeaf: false, Index: 1}}},
			{PlaneNum: 0},
		},
		Leafs: []bsp.TreeLeaf{{Contents: int32(bsp.ContentsSolid)}},
	}
	planes := []model.MPlane{{}}

	hull := BuildNodeHull(tree, planes, 0)

	if hull.FirstClipNode != 0 || hull.LastClipNode != 1 {
		t.Errorf("clipnode range = [%d %d], want [0 1]", hull.FirstClipNode, hull.LastClipNode)
	}
	// Child 0 is a leaf index 0 -> mapped to its contents (Solid).
	// Child 1 is node 1 -> mapped to its index.
	if hull.ClipNodes[0].Children[0] != bsp.ContentsSolid {
		t.Errorf("leaf child %d = %d, want ContentsSolid", 0, hull.ClipNodes[0].Children[0])
	}
	if hull.ClipNodes[0].Children[1] != 1 {
		t.Errorf("node child = %d, want 1", hull.ClipNodes[0].Children[1])
	}
}

func TestPopulateWorldModelCollisionSetsHull0(t *testing.T) {
	tree := &bsp.Tree{
		Planes: []bsp.DPlane{{Normal: [3]float32{0, 0, 1}}},
		Nodes:  []bsp.TreeNode{{PlaneNum: 0}},
		Leafs:  []bsp.TreeLeaf{{Contents: int32(bsp.ContentsEmpty)}},
		Models: []bsp.DModel{
			{BoundsMin: [3]float32{0, 0, 0}, BoundsMax: [3]float32{1, 1, 1}, HeadNode: [4]int32{0, -1, -1, -1}},
		},
	}
	file := &bsp.File{}

	m := WorldModelFromBSPTree("m", tree)
	PopulateWorldModelCollision(m, tree, file)

	if m.Hulls[0].FirstClipNode != 0 {
		t.Errorf("Hull0 FirstClipNode = %d, want 0 (head node)", m.Hulls[0].FirstClipNode)
	}
}
