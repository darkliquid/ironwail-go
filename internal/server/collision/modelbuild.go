// modelbuild.go adapts parsed BSP tree data into the runtime model.Model used
// by engine subsystems, and builds movement hulls/clipnodes so SV_Move can
// trace against map geometry. Extracted from server_net_main.go; these are
// pure functions with no server dependency.
package collision

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
)

// brushHullClipBounds holds the clip bounds for the standard movement hulls
// (box, head, and large step offsets used by Quake's player collision).
var brushHullClipBounds = [model.MaxMapHulls]struct {
	mins [3]float32
	maxs [3]float32
}{
	0: {},
	1: {mins: [3]float32{-16, -16, -24}, maxs: [3]float32{16, 16, 32}},
	2: {mins: [3]float32{-32, -32, -24}, maxs: [3]float32{32, 32, 64}},
}

// WorldModelFromBSPTree adapts a parsed BSP tree into the runtime model.Model
// expected by engine subsystems.
func WorldModelFromBSPTree(modelName string, tree *bsp.Tree) *model.Model {
	m := &model.Model{
		Name:      modelName,
		Type:      model.ModBrush,
		NumLeafs:  len(tree.Leafs),
		NumNodes:  len(tree.Nodes),
		Entities:  string(tree.Entities),
		NumPlanes: len(tree.Planes),
	}

	if len(tree.Models) > 0 {
		m.Mins = tree.Models[0].BoundsMin
		m.Maxs = tree.Models[0].BoundsMax
		m.ClipMins = m.Mins
		m.ClipMaxs = m.Maxs
		m.ClipBox = true
	}
	m.NumSubModels = len(tree.Models)
	m.SubModels = append([]bsp.DModel(nil), tree.Models...)

	m.Planes = make([]model.MPlane, len(tree.Planes))
	for i, p := range tree.Planes {
		m.Planes[i] = model.MPlane{
			Normal: p.Normal,
			Dist:   p.Dist,
			Type:   uint8(p.Type),
		}
	}

	m.Nodes = make([]model.MNode, len(tree.Nodes))
	for i, n := range tree.Nodes {
		m.Nodes[i] = model.MNode{
			Contents: int(bsp.ContentsEmpty),
			MinMaxs: [6]float32{
				n.BoundsMin[0], n.BoundsMin[1], n.BoundsMin[2],
				n.BoundsMax[0], n.BoundsMax[1], n.BoundsMax[2],
			},
			FirstSurface: n.FirstFace,
			NumSurfaces:  n.NumFaces,
		}
		if int(n.PlaneNum) >= 0 && int(n.PlaneNum) < len(m.Planes) {
			m.Nodes[i].Plane = &m.Planes[n.PlaneNum]
		}
	}

	for i, n := range tree.Nodes {
		for side := 0; side < 2; side++ {
			child := n.Children[side]
			if !child.IsLeaf && child.Index >= 0 && child.Index < len(m.Nodes) {
				m.Nodes[i].Children[side] = &m.Nodes[child.Index]
			}
		}
	}

	for i := range m.Hulls {
		m.Hulls[i].FirstClipNode = -1
		m.Hulls[i].LastClipNode = -1
	}

	return m
}

// PopulateWorldModelCollision builds movement hulls/clipnodes so SV_Move can
// trace against map geometry.
func PopulateWorldModelCollision(m *model.Model, tree *bsp.Tree, file *bsp.File) {
	if m == nil || tree == nil || len(m.Planes) == 0 || len(tree.Models) == 0 {
		return
	}

	m.Hulls[0] = BuildNodeHull(tree, m.Planes, int(tree.Models[0].HeadNode[0]))

	clipNodes := BSPClipNodesToModel(file)
	if len(clipNodes) == 0 {
		return
	}

	m.ClipNodes = clipNodes
	for hullNum := 1; hullNum <= 2; hullNum++ {
		headNode := int(tree.Models[0].HeadNode[hullNum])
		if headNode < 0 {
			continue
		}
		m.Hulls[hullNum] = model.Hull{
			ClipNodes:     clipNodes,
			Planes:        m.Planes,
			FirstClipNode: headNode,
			LastClipNode:  len(clipNodes) - 1,
			ClipMins:      brushHullClipBounds[hullNum].mins,
			ClipMaxs:      brushHullClipBounds[hullNum].maxs,
		}
	}
}

// BuildNodeHull converts BSP nodes/leaves into a hull clipnode graph for
// player/world collision tracing.
func BuildNodeHull(tree *bsp.Tree, planes []model.MPlane, headNode int) model.Hull {
	if tree == nil || len(tree.Nodes) == 0 || headNode < 0 || headNode >= len(tree.Nodes) {
		return model.Hull{FirstClipNode: -1, LastClipNode: -1}
	}

	clipNodes := make([]model.MClipNode, len(tree.Nodes))
	for i, node := range tree.Nodes {
		clipNodes[i].PlaneNum = int(node.PlaneNum)
		for side, child := range node.Children {
			if child.IsLeaf {
				if child.Index >= 0 && child.Index < len(tree.Leafs) {
					clipNodes[i].Children[side] = int(tree.Leafs[child.Index].Contents)
				} else {
					clipNodes[i].Children[side] = bsp.ContentsSolid
				}
				continue
			}
			clipNodes[i].Children[side] = child.Index
		}
	}

	return model.Hull{
		ClipNodes:     clipNodes,
		Planes:        planes,
		FirstClipNode: headNode,
		LastClipNode:  len(clipNodes) - 1,
	}
}

// BSPClipNodesToModel normalizes BSP clipnode lump variants into the runtime
// model.MClipNode format.
func BSPClipNodesToModel(file *bsp.File) []model.MClipNode {
	if file == nil {
		return nil
	}

	switch clipNodes := file.Clipnodes.(type) {
	case []bsp.DSClipNode:
		out := make([]model.MClipNode, len(clipNodes))
		for i, node := range clipNodes {
			out[i] = model.MClipNode{
				PlaneNum: int(node.PlaneNum),
				Children: [2]int{int(node.Children[0]), int(node.Children[1])},
			}
		}
		return out
	case []bsp.DLClipNode:
		out := make([]model.MClipNode, len(clipNodes))
		for i, node := range clipNodes {
			out[i] = model.MClipNode{
				PlaneNum: int(node.PlaneNum),
				Children: [2]int{int(node.Children[0]), int(node.Children[1])},
			}
		}
		return out
	default:
		return nil
	}
}
