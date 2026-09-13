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
var hull1Extents = [2]vec3{{X: 16, Y: 16, Z: 24}, {X: 16, Y: 16, Z: 32}}
var hull2Extents = [2]vec3{{X: 32, Y: 32, Z: 24}, {X: 32, Y: 32, Z: 64}}

// expandSolidBrushes builds the clip-hull brush list: every solid world
// brush with its planes shifted outward by the hull extents projection
// (liquids/sky are passable in the clip hulls and are skipped, matching
// classic qbsp hull semantics). Expanded planes are registered in the
// compiler's main plane table (clip nodes reference it), deduped against
// existing entries.
func (u *treeUnit) expandSolidBrushes(world []brushRef, bounds [2]vec3, ext [2]vec3) []brushRef {
	var out []brushRef
	for _, b := range world {
		if u.ba.brushes[b].content != bsp.ContentsSolid {
			continue // liquids/sky passable in clip hulls
		}
		faces := make([]brushFace, 0, len(u.ba.sidesOf(b)))
		for i := range u.ba.sidesOf(b) {
			s := &u.ba.sidesOf(b)[i]
			// Per-axis hull-box projection: planes shift by the box half
			// span along their own normal axis only (adding the z term to
			// x/y faces would inflate walls by the player height).
			n := s.n
			shift := 0.0
			if n.X > 0 {
				shift += ext[1].X
			} else if n.X < 0 {
				shift += ext[0].X
			}
			if n.Y > 0 {
				shift += ext[1].Y
			} else if n.Y < 0 {
				shift += ext[0].Y
			}
			if n.Z > 0 {
				shift += ext[1].Z
			} else if n.Z < 0 {
				shift += ext[0].Z
			}
			p := plane{Normal: n, Dist: snapPlaneDist(n, s.d+shift)}
			faces = append(faces, brushFace{p: p, pn: u.register(p)})
		}
		eb := buildBspBrushFaces(u.ba, faces, bounds)
		if eb == -1 {
			continue
		}
		u.ba.brushes[eb].content = bsp.ContentsSolid
		u.ba.brushes[eb].sortKey = u.ba.brushes[b].sortKey
		out = append(out, eb)
	}
	return out
}

// addPlaneIndex finds or creates a normalized plane-table entry (used for
// hull clip planes, which share the main plane lump).
func (c *compiler) addPlaneIndex(p plane) int {
	p = normalizePlane(p)
	p.Dist = snapPlaneDist(p.Normal, p.Dist)
	c.planeMu.Lock()
	defer c.planeMu.Unlock()
	if i := c.lookupPlaneIndexLocked(p); i >= 0 {
		return i
	}
	c.planes = append(c.planes, p)
	c.indexPlane(len(c.planes)-1, p)
	c.planeKeys[orientedPlaneKeyOf(p)] = len(c.planes) - 1
	return len(c.planes) - 1
}

// buildHullClipNodes compiles the clip-hull tree (hulls 1/2 shared root at
// clipnode 0) from the expanded solid brushes using the solidbsp recursion.
func (u *treeUnit) buildHullClipNodes(hulls []brushRef, bounds [2]vec3, ext [2]vec3) []outClipNode {
	// parentUnit=nil: the root unit's locals are already merged into the
	// shared table at this point, so hull units intern straight into it.
	// Expansion runs on the hull unit too (the root's local table is
	// closed after mergeUp).
	unit := &treeUnit{}
	unit.init(u.shared, nil, u.maxNodeSize, nil)
	hulls = unit.adoptList(u.ba, hulls)
	expanded := unit.expandSolidBrushes(hulls, bounds, ext)
	root := unit.build(bounds, rootRegion(bounds), -1, -1, expanded, splitFast, 0)
	unit.mergeUp()
	if root.isLeaf {
		// Empty clip tree: a single EMPTY clipnode keeps headnode valid.
		var content int32 = bsp.ContentsEmpty
		if len(hulls) > 0 {
			content = u.ba.brushes[hulls[0]].content
		}
		return []outClipNode{{plane: 0, children: [2]int32{content, content}}}
	}
	leafContent := make([]int32, len(unit.leafs))
	for i := range unit.leafs {
		leafContent[i] = unit.leafs[i].content
	}
	clip := make([]outClipNode, len(unit.nodes))
	for i, nd := range unit.nodes {
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
