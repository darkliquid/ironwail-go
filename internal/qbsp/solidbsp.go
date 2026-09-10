package qbsp

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// splitPolicy selects the split-plane heuristic (classic qbsp AUTO/FAST/
// PRECISE, simplified): FAST prefers the first plane that splits the list,
// AUTO/PRECISE score planes by balance with an axial preference.
type splitPolicy int

const (
	splitAuto    splitPolicy = iota
	splitFast                // brush entities / hull passes
	splitPrecise             // world passes
)

// boundPlane is one oriented bounding halfspace of a region: the region is
// the intersection of dot(b.p.Normal, x) <= b.p.Dist over all bounds.
// pi < 0 marks a root AABB plane (the void boundary).
type boundPlane struct {
	pi int
	p  plane
}

// leafRegion is a tree leaf's exact convex region, expressed as oriented
// bounding halfspaces (the six root-box planes plus every split plane on
// the leaf's path).
type leafRegion struct {
	bs []boundPlane
}

// facetGeom is one facet of a leaf region: the polygon and the table plane
// it lies on (pi < 0 = root-box/void facet).
type facetGeom struct {
	pi int
	p  plane // oriented so the leaf interior is dot(p.Normal, x) <= p.Dist
	w  winding
}

// facets enumerates the region's boundary facets, each clipped to the
// exact region by all other bounds. The seed box must contain the region
// (the root AABB always does).
func (r *leafRegion) facets(bounds [2]vec3) []facetGeom {
	var out []facetGeom
	for _, b := range r.bs {
		// Seed on the bound plane within the root AABB (which contains the
		// region); the other bounds trim it to the exact facet.
		w := windingFromBoxPlane(b.p, bounds[0], bounds[1])
		if w == nil {
			continue
		}
		ok := true
		for _, other := range r.bs {
			clipped, cok := clipWindingKeepBack(w, other.p)
			if !cok {
				ok = false
				break
			}
			w = clipped
		}
		if !ok {
			continue
		}
		w = windingRemoveColinear(w)
		if len(w) < 3 || windingIsTiny(w) || windingArea(w) < 0.1 {
			continue
		}
		w = windingOrientTo(w, b.p.Normal)
		out = append(out, facetGeom{pi: b.pi, p: b.p, w: w})
	}
	return out
}

// addFront returns the child region on the FRONT side of split plane p
// (dot(p.Normal, x) >= p.Dist), i.e. bounded by the negated plane.
func (r *leafRegion) addFront(pn int, p plane) leafRegion {
	np := p.Normal.Neg()
	out := leafRegion{bs: make([]boundPlane, 0, len(r.bs)+1)}
	out.bs = append(out.bs, r.bs...)
	out.bs = append(out.bs, boundPlane{pi: pn, p: plane{Normal: np, Dist: -p.Dist}})
	return out
}

// addBack returns the child region on the BACK side of p.
func (r *leafRegion) addBack(pn int, p plane) leafRegion {
	out := leafRegion{bs: make([]boundPlane, 0, len(r.bs)+1)}
	out.bs = append(out.bs, r.bs...)
	out.bs = append(out.bs, boundPlane{pi: pn, p: p})
	return out
}

// rootRegion builds the initial region: the six root AABB faces (empty pi).
func rootRegion(bounds [2]vec3) leafRegion {
	return leafRegion{bs: []boundPlane{
		{pi: -1, p: plane{Normal: v3(1, 0, 0), Dist: bounds[1].X}},   // x <= max
		{pi: -1, p: plane{Normal: v3(-1, 0, 0), Dist: -bounds[0].X}}, // x >= min
		{pi: -1, p: plane{Normal: v3(0, 1, 0), Dist: bounds[1].Y}},
		{pi: -1, p: plane{Normal: v3(0, -1, 0), Dist: -bounds[0].Y}},
		{pi: -1, p: plane{Normal: v3(0, 0, 1), Dist: bounds[1].Z}},
		{pi: -1, p: plane{Normal: v3(0, 0, -1), Dist: -bounds[0].Z}},
	}}
}

// --- tree building ---

// pathStep records one tree step on a leaf's root-to-leaf path.
type pathStep struct {
	node int // index into nodes
	side int // 0 = front child, 1 = back child
}

// leafPaths aligned with leafs (pre-renumber): the node chain to each leaf.
// Reconstructed from outLeaf.parent/side by walking up; kept for O(1)
// neighbour queries after renumbering.

// outNode is a BSP node in compiler terms (extended: parent/side links and
// the oriented split plane for point descent).
type outNode struct {
	planenum int
	splitN   vec3
	splitD   float64
	children [2]childRef
	bounds   [2]vec3
	parent   int // node index, -1 for root
	side     int // which child of parent (0/1), -1 for root
	// firstface/numfaces filled at assembly time by plane grouping.
	firstface int
	numfaces  int
}

// outLeaf is a compiler BSP leaf.
type outLeaf struct {
	content     int32
	mins, maxs  vec3
	marksurface []int
	region      leafRegion
	parent      int // node index
	side        int
}

// treeBuild accumulates nodes/leafs during the solidbsp recursion.
type treeBuild struct {
	nodes []outNode
	leafs []outLeaf
	// register returns the table index for a geometric plane, registering
	// it in the compiler's plane lump when new (deduped, normalized).
	register func(p plane) int
}

// contentsOf returns the leaf content of a (possibly empty) brush list:
// highest precedence is solid, then liquid types, sky, or empty.
func contentsOf(brushes []*bspBrush) int32 {
	if len(brushes) == 0 {
		return bsp.ContentsEmpty
	}
	hasEmpty := false
	hasWater := false
	hasSlime := false
	hasLava := false
	hasSky := false
	for _, b := range brushes {
		switch b.content {
		case bsp.ContentsSolid:
			return bsp.ContentsSolid
		case bsp.ContentsLava:
			hasLava = true
		case bsp.ContentsSlime:
			hasSlime = true
		case bsp.ContentsWater:
			hasWater = true
		case bsp.ContentsSky:
			hasSky = true
		default:
			hasEmpty = true
		}
	}
	switch {
	case hasLava:
		return bsp.ContentsLava
	case hasSlime:
		return bsp.ContentsSlime
	case hasWater:
		return bsp.ContentsWater
	case hasSky:
		return bsp.ContentsSky
	case hasEmpty:
		return bsp.ContentsEmpty
	default:
		return bsp.ContentsEmpty
	}
}

// planeSplitsBounds reports whether the plane crosses the AABB (both sides
// strictly non-empty), the classic CheckPlaneAgainstVolume test: a split
// plane must actually partition the node region, otherwise the recursion
// would carve empty space with a plane flush against a wall.
func planeSplitsBounds(bounds [2]vec3, p plane) bool {
	minD, maxD := math.Inf(1), math.Inf(-1)
	for i := 0; i < 8; i++ {
		pt := vec3{
			X: bounds[i&1].X,
			Y: bounds[(i>>1)&1].Y,
			Z: bounds[(i>>2)&1].Z,
		}
		d := p.Normal.Dot(pt) - p.Dist
		if d < minD {
			minD = d
		}
		if d > maxD {
			maxD = d
		}
	}
	return maxD > splitEpsilon && minD < -splitEpsilon
}

// selectSplitPlane picks the best split plane from the brush side planes:
// the plane must split the region bounds (volume test) and the brush list;
// scoring follows ericw SelectSplitPlane: prefer fewer splits, balanced
// front/back, and axial planes. FAST takes the first valid plane.
func selectSplitPlane(brushes []*bspBrush, policy splitPolicy, region leafRegion, bounds [2]vec3) (plane, bool) {
	if len(brushes) == 0 {
		return plane{}, false
	}
	// Skip candidates whose geometric plane already bounds the region
	// (coplanar with an ancestor split: the volume test would reject them
	// anyway, this just avoids dead metrics).
	hasPlane := func(p plane) bool {
		for _, b := range region.bs {
			if planeEqualOriented(b.p, p) {
				return true
			}
		}
		return false
	}

	found := false
	var bestPlane plane
	bestValue := -99999
	for _, b := range brushes {
		for _, s := range b.sides {
			if hasPlane(s.sidePlane()) {
				continue
			}
			// Normalize to the table-plane orientation (positive axial
			// normals): node children must align with the engine's
			// PointInLeaf (children[0] = front of the stored plane).
			p := normalizePlane(s.sidePlane())
			if !planeSplitsBounds(bounds, p) {
				continue
			}
			fronts, backs, splits, facing := 0, 0, 0, 0
			for _, b2 := range brushes {
				bits := classifyBrush(b2, p)
				if bits&psideFront != 0 {
					fronts++
				}
				if bits&psideBack != 0 {
					backs++
				}
				if bits&psideFront != 0 && bits&psideBack != 0 {
					splits++
				}
				if bits&psideFacing != 0 {
					facing++
				}
			}
			// ericw SelectSplitPlane metric: facing brushes (whose face lies on
			// the candidate plane) are strongly preferred; without the facing
			// bonus, boundary-sealing planes (wall faces, trim edges) lose to
			// balanced mid-map planes, small brushes get swallowed into larger
			// leaves, and phantom-solid leaves create false leaks.
			value := 5*facing - 5*splits - absInt(fronts-backs)
			if isAxial(s.n) {
				value += 5
			}
			if policy == splitFast {
				return p, true
			}
			if value > bestValue {
				bestValue = value
				bestPlane = p
				found = true
			}
		}
	}
	if !found {
		return plane{}, false
	}
	return bestPlane, true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func isAxial(n vec3) bool {
	return (n.X == 1 || n.X == -1) || (n.Y == 1 || n.Y == -1) || (n.Z == 1 || n.Z == -1)
}

// splitBrushList partitions brushes by plane p, splitting those that
// straddle it.
func splitBrushList(brushes []*bspBrush, pn int, p plane) ([]*bspBrush, []*bspBrush) {
	var front, back []*bspBrush
	for _, b := range brushes {
		bits := classifyBrush(b, p)
		if bits&psideFront != 0 && bits&psideBack != 0 {
			f, bk := splitBrush(b, pn, p)
			if f != nil {
				front = append(front, f)
			}
			if bk != nil {
				back = append(back, bk)
			}
			continue
		}
		if bits&psideFront != 0 {
			front = append(front, b)
			continue
		}
		back = append(back, b)
	}
	return front, back
}

// childBounds refines the region AABB for an axial split plane (classic
// qbsp: clamp the axis; non-axial keeps the parent bounds).
func childBounds(bounds [2]vec3, p plane) ([2]vec3, [2]vec3) {
	fb, bb := bounds, bounds
	switch {
	case p.Normal.X == 1:
		fb[0].X = p.Dist
		bb[1].X = p.Dist
		return fb, bb
	case p.Normal.X == -1:
		fb[1].X = -p.Dist
		bb[0].X = -p.Dist
		return fb, bb
	case p.Normal.Y == 1:
		fb[0].Y = p.Dist
		bb[1].Y = p.Dist
		return fb, bb
	case p.Normal.Y == -1:
		fb[1].Y = -p.Dist
		bb[0].Y = -p.Dist
		return fb, bb
	case p.Normal.Z == 1:
		fb[0].Z = p.Dist
		bb[1].Z = p.Dist
		return fb, bb
	case p.Normal.Z == -1:
		fb[1].Z = -p.Dist
		bb[0].Z = -p.Dist
		return fb, bb
	}
	return bounds, bounds
}

// build runs the solidbsp recursion over brushes within bounds and returns
// the tree root.
func (t *treeBuild) build(bounds [2]vec3, region leafRegion, parent, side int, brushes []*bspBrush, policy splitPolicy) childRef {
	p, ok := selectSplitPlane(brushes, policy, region, bounds)
	if !ok {
		idx := len(t.leafs)
		t.leafs = append(t.leafs, outLeaf{
			content: contentsOf(brushes),
			region:  region,
			parent:  parent,
			side:    side,
		})
		return childRef{isLeaf: true, idx: idx}
	}
	pn := t.register(p)
	front, back := splitBrushList(brushes, pn, p)
	idx := len(t.nodes)
	t.nodes = append(t.nodes, outNode{
		planenum: pn,
		splitN:   p.Normal,
		splitD:   p.Dist,
		bounds:   bounds,
		parent:   parent,
		side:     side,
	})
	fb, bb := childBounds(bounds, p)
	ch0 := t.build(fb, region.addFront(pn, p), idx, 0, front, policy)
	ch1 := t.build(bb, region.addBack(pn, p), idx, 1, back, policy)
	t.nodes[idx].children = [2]childRef{ch0, ch1}
	return childRef{isLeaf: false, idx: idx}
}

// solidBrushDef defines a solid brush's bounding box and outward face planes
// used for surface-based leaf solidity verification.
type solidBrushDef struct {
	bounds [2]vec3
	planes []plane
}

// finalize computes each leaf's exact facets (and bounds) from its region,
// and enforces surface-based leaf solidity: any leaf whose region is
// geometrically contained within the volume of a solid brush is marked solid,
// restoring solidity lost when open or degenerate brushes drop sliver pieces
// during tree splitting.
func (t *treeBuild) finalize(rootBounds [2]vec3, solidBrushes []solidBrushDef) {
	for i := range t.leafs {
		fs := t.leafs[i].region.facets(rootBounds)
		mins, maxs := rootBounds[0], rootBounds[1]
		first := true
		var sum vec3
		count := 0
		for _, f := range fs {
			m, x := windingBounds(f.w)
			if first {
				mins, maxs = m, x
				first = false
			} else {
				if m.X < mins.X {
					mins.X = m.X
				}
				if x.X > maxs.X {
					maxs.X = x.X
				}
				if m.Y < mins.Y {
					mins.Y = m.Y
				}
				if x.Y > maxs.Y {
					maxs.Y = x.Y
				}
				if m.Z < mins.Z {
					mins.Z = m.Z
				}
				if x.Z > maxs.Z {
					maxs.Z = x.Z
				}
			}
			for _, v := range f.w {
				sum = sum.Add(v)
				count++
			}
		}
		if !first {
			t.leafs[i].mins, t.leafs[i].maxs = mins, maxs
		} else {
			t.leafs[i].mins, t.leafs[i].maxs = rootBounds[0], rootBounds[1]
		}

		if t.leafs[i].content != bsp.ContentsSolid && count > 0 && len(solidBrushes) > 0 {
			for _, sb := range solidBrushes {
				// Fast rejection: leaf must be contained within the brush's AABB (with epsilon tolerance).
				const eps = 0.5
				if t.leafs[i].mins.X < sb.bounds[0].X-eps || t.leafs[i].maxs.X > sb.bounds[1].X+eps ||
					t.leafs[i].mins.Y < sb.bounds[0].Y-eps || t.leafs[i].maxs.Y > sb.bounds[1].Y+eps ||
					t.leafs[i].mins.Z < sb.bounds[0].Z-eps || t.leafs[i].maxs.Z > sb.bounds[1].Z+eps {
					continue
				}

				// Leaf centroid must be inside all brush planes.
				centroid := sum.Scale(1.0 / float64(count))
				centroidInside := true
				for _, p := range sb.planes {
					if p.Normal.Dot(centroid)-p.Dist > 0.01 {
						centroidInside = false
						break
					}
				}
				if !centroidInside {
					continue
				}

				// Leaf centroid is inside; verify all vertices of all leaf facets
				// lie on or behind all planes of the brush.
				allInside := true
				for _, f := range fs {
					for _, v := range f.w {
						for _, p := range sb.planes {
							if p.Normal.Dot(v)-p.Dist > 0.01 {
								allInside = false
								break
							}
						}
						if !allInside {
							break
						}
					}
					if !allInside {
						break
					}
				}
				if allInside {
					t.leafs[i].content = bsp.ContentsSolid
					break
				}
			}
		}
	}
}

// pathToLeaf reconstructs the root-to-leaf node chain for leaf li (using
// pre-renumber nodes/leafs).
func pathToLeaf(nodes []outNode, leafs []outLeaf, li int) []pathStep {
	var rev []pathStep
	cur := leafs[li].parent
	side := leafs[li].side
	for cur >= 0 {
		rev = append(rev, pathStep{node: cur, side: side})
		side = nodes[cur].side
		cur = nodes[cur].parent
	}
	// reverse
	n := len(rev)
	out := make([]pathStep, n)
	for i := 0; i < n; i++ {
		out[i] = rev[n-1-i]
	}
	return out
}

// pointInLeaf descends the tree to the leaf containing p (nodes only; the
// root box is implied by the region construction).
func pointInLeaf(nodes []outNode, root childRef, p vec3) (int, bool) {
	ref := root
	for !ref.isLeaf {
		if ref.idx < 0 || ref.idx >= len(nodes) {
			return 0, false
		}
		nd := &nodes[ref.idx]
		if v3Dot(nd.splitN, p)-nd.splitD > 0 {
			ref = nd.children[0]
		} else {
			ref = nd.children[1]
		}
	}
	return ref.idx, true
}
