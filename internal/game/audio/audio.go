// Package audio implements pure, testable helpers for the game layer's audio
// subsystem: underwater detection, leaf lookup, and static-sound key building.
// These were extracted from game_audio.go so they can run without a live Game.
package audio

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// UnderwaterIntensity maps a BSP leaf contents value to an underwater audio
// intensity (1 for liquid, 0 otherwise), matching the C audio/water logic.
func UnderwaterIntensity(contents int32) float32 {
	switch contents {
	case bsp.ContentsWater, bsp.ContentsSlime, bsp.ContentsLava:
		return 1
	default:
		return 0
	}
}

// PointInTreeLeaf walks the BSP node tree to find the leaf containing point.
// Returns (leaf, true) on success, or (zero leaf, false) on invalid trees /
// out-of-range indices.
func PointInTreeLeaf(tree *bsp.Tree, point [3]float32) (bsp.TreeLeaf, bool) {
	if tree == nil || len(tree.Nodes) == 0 || len(tree.Planes) == 0 || len(tree.Leafs) == 0 {
		return bsp.TreeLeaf{}, false
	}

	nodeIndex := 0
	for {
		if nodeIndex < 0 || nodeIndex >= len(tree.Nodes) {
			return bsp.TreeLeaf{}, false
		}
		node := tree.Nodes[nodeIndex]
		if int(node.PlaneNum) < 0 || int(node.PlaneNum) >= len(tree.Planes) {
			return bsp.TreeLeaf{}, false
		}
		plane := tree.Planes[node.PlaneNum]
		dist := point[0]*plane.Normal[0] + point[1]*plane.Normal[1] + point[2]*plane.Normal[2] - plane.Dist
		side := 0
		if dist < 0 {
			side = 1
		}

		child := node.Children[side]
		if child.IsLeaf {
			if child.Index < 0 || child.Index >= len(tree.Leafs) {
				return bsp.TreeLeaf{}, false
			}
			return tree.Leafs[child.Index], true
		}
		nodeIndex = child.Index
	}
}
