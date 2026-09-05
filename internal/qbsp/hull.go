package qbsp

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// outClipNode is a clipnode in compiler terms; children are node indices
// (>= 0) or negative BSP contents for leafs.
type outClipNode struct {
	plane    int
	children [2]int32
}

// clipHullExtents pairs describe the classic movement hull boxes applied
// to brush planes for the clip trees: [0] is the expansion below the
// origin (±x, ±y, -z) and [1] above. Hull 1 is the player box
// (±16x±16x(-24..32)) which the Go engine's world collision traces
// against (FirstClipNode=0 in modelbuild.go, with box offsets driving
// larger entity sizes). Hull 2 is the large box (±32x±32x(-24..64)) used
// for submodel clip trees (the engine uses HeadNode[1]/[2] for those).
var hull1Extents = [2]vec3{{16, 16, 24}, {16, 16, 32}}
var hull2Extents = [2]vec3{{32, 32, 24}, {32, 32, 64}}

// expandSolidBrushes builds the clip-hull brush list: every solid world
// brush with its planes shifted outward by the hull extents projection
// (liquids/sky are passable in the clip hulls and are skipped, matching
// classic qbsp hull semantics). Expanded planes are registered in the
// compiler's main plane table (clip nodes reference it), deduped against
// existing entries.
func (c *compiler) expandSolidBrushes(world []*bspBrush, bounds [2]vec3, ext [2]vec3) []*bspBrush {
	var out []*bspBrush
	for _, b := range world {
		if b.content != bsp.ContentsSolid {
			continue // liquids/sky passable in clip hulls
		}
		faces := make([]brushFace, 0, len(b.sides))
		for _, s := range b.sides {
			// Per-axis hull-box projection: planes shift by the box half
			// span along their own normal axis only (adding the z term to
			// x/y faces would inflate walls by the player height).
			n := s.n
			shift := 0.0
			for i := 0; i < 3; i++ {
				if n[i] > 0 {
					shift += ext[1][i]
				} else if n[i] < 0 {
					shift += ext[0][i]
				}
			}
			p := plane{Normal: n, Dist: snapPlaneDist(s.d + shift)}
			faces = append(faces, brushFace{p: p, pn: c.addPlaneIndex(p)})
		}
		eb := buildBspBrushFaces(faces, bounds)
		if eb == nil {
			continue
		}
		eb.content = bsp.ContentsSolid
		eb.sortKey = b.sortKey
		out = append(out, eb)
	}
	return out
}

// addPlaneIndex finds or creates a normalized plane-table entry (used for
// hull clip planes, which share the main plane lump).
func (c *compiler) addPlaneIndex(p plane) int {
	p = normalizePlane(p)
	p.Dist = snapPlaneDist(p.Dist)
	for i, existing := range c.planes {
		if planeEqualNear(p, existing) {
			return i
		}
	}
	c.planes = append(c.planes, p)
	return len(c.planes) - 1
}

// buildHullClipNodes compiles the clip-hull tree (hulls 1/2 shared root at
// clipnode 0) from the expanded solid brushes using the solidbsp recursion.
func (c *compiler) buildHullClipNodes(hulls []*bspBrush, bounds [2]vec3) []outClipNode {
	tb := &treeBuild{}
	tb.register = c.addPlaneIndex
	root := tb.build(bounds, rootRegion(bounds), -1, -1, hulls, splitFast)
	if root.isLeaf {
		// Empty clip tree: a single EMPTY clipnode keeps headnode valid.
		var content int32 = bsp.ContentsEmpty
		if len(hulls) > 0 {
			content = hulls[0].content
		}
		return []outClipNode{{plane: 0, children: [2]int32{content, content}}}
	}
	leafContent := make([]int32, len(tb.leafs))
	for i := range tb.leafs {
		leafContent[i] = tb.leafs[i].content
	}
	clip := make([]outClipNode, len(tb.nodes))
	for i, nd := range tb.nodes {
		cn := outClipNode{plane: nd.planenum}
		for c := 0; c < 2; c++ {
			ch := nd.children[c]
			if ch.isLeaf {
				cn.children[c] = leafContent[ch.idx]
			} else {
				cn.children[c] = int32(ch.idx)
			}
		}
		clip[i] = cn
	}
	return clip
}
