package qbsp

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// outClipNode is a clipnode in compiler terms; children are node indices
// (>= 0) or negative BSP contents for leafs.
type outClipNode struct {
	plane    int
	children [2]int32
}

// clipHullExtents is the expansion applied to world brush planes for the
// clip hull tree. The Go engine uses a single clip tree (starting at
// clipnode 0) for movement hulls 1 and 2 with bounds
// ±16x±16x(-24..32) (player) and ±32x±32x(-24..64) (large), so the tree
// must be expanded for the largest box.
var clipHullExtents = [2]vec3{{32, 32, 24}, {32, 32, 64}}

// buildHullClipNodes expands the solid world brushes by the clip-hull box
// and builds a single clip tree (root at index 0) shared by hulls 1 and 2.
// Liquids and sky are passable in the clip hulls (treated as empty), and
// skip/hint brushes are ignored, matching classic qbsp hull semantics.
func (c *compiler) buildHullClipNodes(world []worldBrush) []outClipNode {
	// 1. Expanded plane table.
	var planes []plane
	addPlane := func(p plane) int {
		p = normalizePlane(p)
		for i, existing := range planes {
			if planeEqualNear(p, existing) {
				return i
			}
		}
		planes = append(planes, p)
		return len(planes) - 1
	}

	type expandedBrush struct {
		planes []int
		volume bool
	}
	var expBrushes []expandedBrush
	for _, b := range world {
		if b.content != bsp.ContentsSolid {
			continue // liquids/sky passable in clip hulls
		}
		eb := expandedBrush{}
		for _, pi := range b.planes {
			p := c.planes[pi]
			// Shift the inward-facing-solid plane outward by the hull box
			// projection so a point trace (hull centre) stays clear.
			shift := math.Abs(p.Normal[0])*clipHullExtents[1][0] +
				math.Abs(p.Normal[1])*clipHullExtents[1][1]
			if p.Normal[2] >= 0 {
				shift += clipHullExtents[1][2]
			} else {
				shift += clipHullExtents[0][2]
			}
			p.Dist = snapPlaneDist(p.Dist + shift)
			eb.planes = append(eb.planes, addPlane(p))
		}
		if len(eb.planes) > 0 {
			eb.volume = true
			expBrushes = append(expBrushes, eb)
		}
	}

	// 2. Box planes.
	np := len(planes)
	bp := boxPlanes(c.wbounds[0], c.wbounds[1])
	for _, p := range bp {
		planes = append(planes, p)
	}

	// 3. Arrangement + contents: solid iff inside any expanded brush.
	arr := buildArrangement(planes, c.wbounds)
	for ci := range arr.cells {
		center := arr.cellCenter(ci)
		solid := false
		for _, eb := range expBrushes {
			inside := true
			for _, pi := range eb.planes {
				if v3Dot(planes[pi].Normal, center)-planes[pi].Dist > 0.01 {
					inside = false
					break
				}
			}
			if inside {
				solid = true
				break
			}
		}
		if solid {
			arr.cells[ci].content = bsp.ContentsSolid
		} else {
			arr.cells[ci].content = bsp.ContentsEmpty
		}
	}

	// 4. Clip tree.
	cells := make([]int, len(arr.cells))
	for i := range arr.cells {
		cells[i] = i
	}
	var clip []outClipNode
	var build func(cells []int) int32
	build = func(sub []int) int32 {
		content := arr.cells[sub[0]].content
		homogeneous := true
		for _, ci := range sub[1:] {
			if arr.cells[ci].content != content {
				homogeneous = false
				break
			}
		}
		if homogeneous || len(sub) == 1 {
			return content // negative = leaf
		}
		bestPlane, bestScore := -1, 0
		for pi := 0; pi < np; pi++ {
			front, back := 0, 0
			for _, ci := range sub {
				if hsFront(arr.cells[ci], pi) {
					front++
				} else {
					back++
				}
			}
			score := front
			if back < score {
				score = back
			}
			if score > 0 && score > bestScore {
				bestScore = score
				bestPlane = pi
			}
		}
		if bestPlane < 0 {
			return content
		}
		var front, back []int
		for _, ci := range sub {
			if hsFront(arr.cells[ci], bestPlane) {
				front = append(front, ci)
			} else {
				back = append(back, ci)
			}
		}
		idx := int32(len(clip))
		clip = append(clip, outClipNode{plane: bestPlane})
		clip[idx].children[0] = build(front)
		clip[idx].children[1] = build(back)
		return idx
	}
	root := build(cells)
	_ = root // always 0 for a non-empty tree
	return clip
}