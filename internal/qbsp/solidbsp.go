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
// (the root AABB always does). Facet windings are allocated from the arena
// when non-nil (they outlive the tree build into surface generation).
// clipWindingKeepBack clips w to the BACK side (dot <= d) of p, keeping
// on-plane points (slice currency for the region-facet path).
func clipWindingKeepBack(a *windingArena, w winding, p plane) (winding, bool) {
	return clipWinding(a, w, negPlane(p))
}

func (r *leafRegion) facets(ar *windingArena, bounds [2]vec3) []facetGeom {
	var out []facetGeom
	for _, b := range r.bs {
		// Seed on the bound plane within the root AABB (which contains the
		// region); the other bounds trim it to the exact facet.
		w := windingFromBoxPlane(ar, b.p, bounds[0], bounds[1])
		if w == nil {
			continue
		}
		ok := true
		for _, other := range r.bs {
			clipped, cok := clipWindingKeepBack(ar, w, other.p)
			if !cok {
				ok = false
				break
			}
			w = clipped
		}
		if !ok {
			continue
		}
		w = windingRemoveColinear(ar, w)
		if len(w) < 3 || windingIsTiny(w) || windingArea(w) < 0.1 {
			continue
		}
		w = windingOrientTo(ar, w, b.p.Normal)
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

// contentsOf returns the leaf content of a (possibly empty) brush list:
// highest precedence is solid, then liquid types, sky, or empty.
func contentsOf(ba *brushArena, brushes []brushRef) int32 {
	if len(brushes) == 0 {
		return bsp.ContentsEmpty
	}
	hasEmpty := false
	hasWater := false
	hasSlime := false
	hasLava := false
	hasSky := false
	for _, b := range brushes {
		switch ba.brushes[b].content {
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

// axisAt extracts an axis component (0=X, 1=Y, 2=Z).
func axisAt(v vec3, i int) float64 {
	switch i {
	case 0:
		return v.X
	case 1:
		return v.Y
	default:
		return v.Z
	}
}

// divideBounds splits an AABB by plane p into front and back pieces (port
// of ericw DivideBounds). Axial planes cut exactly; non-axial planes make
// sloping cuts along each axis the normal participates in.
func divideBounds(in [2]vec3, p plane) (front, back [2]vec3) {
	front, back = in, in
	switch classifyPlane(p.Normal) {
	case planeX:
		front[0].X, back[1].X = p.Dist, p.Dist
		return
	case planeY:
		front[0].Y, back[1].Y = p.Dist, p.Dist
		return
	case planeZ:
		front[0].Z, back[1].Z = p.Dist, p.Dist
		return
	}
	const normalEpsilon = 1e-6
	n := [3]float64{p.Normal.X, p.Normal.Y, p.Normal.Z}
	for a := 0; a < 3; a++ {
		if math.Abs(n[a]) < normalEpsilon {
			continue
		}
		b, c := (a+1)%3, (a+2)%3
		extent := axisAt(in[1], a) - axisAt(in[0], a)
		if extent == 0 {
			continue
		}
		splitMins := axisAt(in[1], a)
		splitMaxs := axisAt(in[0], a)
		for i := 0; i < 2; i++ {
			for j := 0; j < 2; j++ {
				corner := [3]float64{in[0].X, in[0].Y, in[0].Z}
				corner[b] = axisAt(in[i], b)
				corner[c] = axisAt(in[j], c)
				corner[a] = axisAt(in[0], a)
				dist1 := corner[0]*n[0] + corner[1]*n[1] + corner[2]*n[2] - p.Dist
				corner[a] = axisAt(in[1], a)
				dist2 := corner[0]*n[0] + corner[1]*n[1] + corner[2]*n[2] - p.Dist
				mid := axisAt(in[0], a) + extent*(dist1/(dist1-dist2))
				splitMins = math.Max(math.Min(mid, splitMins), axisAt(in[0], a))
				splitMaxs = math.Min(math.Max(mid, splitMaxs), axisAt(in[1], a))
			}
		}
		if n[a] > 0 {
			setAxis(&front[0], a, splitMins)
			setAxis(&back[1], a, splitMaxs)
		} else {
			setAxis(&back[0], a, splitMins)
			setAxis(&front[1], a, splitMaxs)
		}
	}
	return
}

// boundsVolume returns the AABB volume.
func boundsVolume(b [2]vec3) float64 {
	return (b[1].X - b[0].X) * (b[1].Y - b[0].Y) * (b[1].Z - b[0].Z)
}

// splitPlaneMetric scores a candidate split plane against the node bounds:
// a good split has equal volumes front and back (ericw SplitPlaneMetric).
// Pure AABB math, no brush classification.
func splitPlaneMetric(p plane, bounds [2]vec3) float64 {
	front, back := divideBounds(bounds, p)
	return math.Abs(boundsVolume(front) - boundsVolume(back))
}

// regionHasPlane reports whether the region already bounds on the plane
// table entry pn (integer identity; region bound planes were registered
// from the same normalized candidates).
func regionHasPlane(region leafRegion, pn int) bool {
	for _, bp := range region.bs {
		if bp.pi == pn {
			return true
		}
	}
	return false
}

// chooseMidPlaneFromList picks the plane whose cut of the node bounds is
// most volume-balanced (ericw ChooseMidPlaneFromList): candidates are scored
// by pure AABB math, axial planes preferred. This is the FAST heuristic the
// AUTO policy uses for nodes above maxNodeSize, where scoring every brush
// against every candidate plane is superlinear waste.
func chooseMidPlaneFromList(ba *brushArena, brushes []brushRef, region leafRegion, bounds [2]vec3) (plane, bool) {
	found := false
	var bestAny, bestAxial plane
	bestAnyMetric := math.Inf(1)
	bestAxialMetric := math.Inf(1)
	seen := make(map[int]bool, 64) // dedup candidate planes (canonical entries)
	for _, b := range brushes {
		for i := range ba.sidesOf(b) {
			s := &ba.sidesOf(b)[i]
			if s.onnode {
				continue
			}
			// A side with a dead winding has nothing visible to split
			// along (ericw skips !side.w the same way); decompiled maps
			// carry thousands of these and they poison the candidate set.
			if s.w.count == 0 {
				continue
			}
			if seen[int(s.planenum)] {
				continue
			}
			seen[int(s.planenum)] = true
			p := normalizePlane(sidePlaneOf(s))
			if regionHasPlane(region, int(s.planenum)) {
				continue
			}
			if !planeSplitsBounds(bounds, p) {
				continue
			}
			m := splitPlaneMetric(p, bounds)
			if m < bestAnyMetric {
				bestAnyMetric = m
				bestAny = p
				found = true
			}
			if isAxial(p.Normal) && m < bestAxialMetric {
				bestAxialMetric = m
				bestAxial = p
			}
		}
	}
	if !found {
		return plane{}, false
	}
	if bestAxialMetric < math.Inf(1) {
		return bestAxial, true
	}
	return bestAny, true
}

// selectSplitPlane picks the best split plane from the brush side planes:
// the plane must split the region bounds (volume test) and the brush list;
// scoring follows ericw SelectSplitPlane: prefer fewer splits, balanced
// front/back, and axial planes. FAST takes the first valid plane; AUTO
// (world) switches to the budget midsplit heuristic for nodes above
// maxNodeSize, matching ericw's default maxnodesize behavior.
//
// Performance notes (jjj22_dfl profiled >24x slower than ericw without
// these): region-bound checks and candidate evaluation dedupe by exact
// plane bits — bit-identical planes score identically, so deduping repeats
// never changes the selection — and classifyBrush pretests the brush AABB
// (a box fully beyond the plane needs no vertex walk; no side can be
// coplanar either, so the FACING bit is impossible).
func selectSplitPlane(ba *brushArena, brushes []brushRef, policy splitPolicy, region leafRegion, bounds [2]vec3) (plane, bool) {
	if len(brushes) == 0 {
		return plane{}, false
	}
	// Region-bound candidates are skipped by integer identity against the
	// region's registered plane entries (both resolve to the same canonical
	// table entry, so the compare is orientation-correct); candidates that
	// tolerance-match but not entry-match fall through to the volume test,
	// which rejects region-bound planes anyway. A per-node map here cost
	// one allocation per node (OOM on large maps).
	hasPlane := func(pn int) bool {
		return regionHasPlane(region, pn)
	}

	// Two passes (structural, then detail), matching ericw's pass order:
	// when a structural candidate scores, detail-brush sides are never
	// scored — on detail-heavy decompiled maps that is most of the
	// candidate set.
	for pass := 0; pass < 2; pass++ {
		seen := make(map[orientedPlaneKey]struct{}, 64)
		found := false
		var bestPlane plane
		bestValue := -99999
		for _, b := range brushes {
			if ba.brushes[b].detail != (pass == 1) {
				continue
			}
			for i := range ba.sidesOf(b) {
				s := &ba.sidesOf(b)[i]
				if s.onnode {
					continue
				}
				if s.w.count == 0 {
					continue
				}
				sp := sidePlaneOf(s)
				if hasPlane(int(s.planenum)) {
					continue
				}
				// Normalize to the table-plane orientation (positive axial
				// normals): node children must align with the engine's
				// PointInLeaf (children[0] = front of the stored plane).
				p := normalizePlane(sp)
				key := orientedPlaneKeyOf(p)
				if _, dup := seen[key]; dup {
					continue
				}
				seen[key] = struct{}{}
				if !planeSplitsBounds(bounds, p) {
					continue
				}
				*ba.candsScored += int64(len(brushes))
				fronts, backs, splits, facing := 0, 0, 0, 0
				for _, b2 := range brushes {
					bits := classifyBrush(ba, b2, int(s.planenum), p)
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
				// ericw SelectSplitPlane metric: facing brushes (whose face
				// lies on the candidate plane) are strongly preferred;
				// without the facing bonus, boundary-sealing planes (wall
				// faces, trim edges) lose to balanced mid-map planes, small
				// brushes get swallowed into larger leaves, and
				// phantom-solid leaves create false leaks.
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
		if found {
			return bestPlane, true
		}
	}
	return plane{}, false
}

// orientedPlaneKey is a bit-exact hashable form of an oriented plane.
type orientedPlaneKey struct{ x, y, z, d uint64 }

func orientedPlaneKeyOf(p plane) orientedPlaneKey {
	return orientedPlaneKey{
		x: math.Float64bits(p.Normal.X),
		y: math.Float64bits(p.Normal.Y),
		z: math.Float64bits(p.Normal.Z),
		d: math.Float64bits(p.Dist),
	}
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
// straddle it. Split pieces are allocated from the arena; their lifetime
// ends when the calling build frame releases its checkpoint.
func splitBrushList(ba *brushArena, brushes []brushRef, pn int, p plane) ([]brushRef, []brushRef) {
	front := make([]brushRef, 0, len(brushes))
	back := make([]brushRef, 0, len(brushes))
	for _, b := range brushes {
		bits := classifyBrush(ba, b, pn, p)
		if bits&psideFront != 0 && bits&psideBack != 0 {
			*ba.straddle++
			f, bk := splitBrush(ba, b, pn, p)
			if f != -1 {
				front = append(front, f)
			}
			if bk != -1 {
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

// markSidesOnnode flags every side lying on plane p (either orientation) as
// already used as a node splitter (ericw marks sides in SeparateNodes).
// Without this, the midsplit heuristic can re-pick the same balanced plane
// across successive nodes whose brush lists never progress, exploding the
// tree (the e1m1/end OOM regression).
func markSidesOnnode(ba *brushArena, brushes []brushRef, p plane, pn int) {
	for _, b := range brushes {
		for i := range ba.sidesOf(b) {
			s := &ba.sidesOf(b)[i]
			// Integer identity: the plane table is orientation-canonical,
			// so sides on the same geometric plane share the entry. The
			// float compare here was ~9% of the mutator on large maps.
			if !s.onnode && s.planenum == int32(pn) {
				s.onnode = true
			}
		}
	}
}

// build runs the solidbsp recursion over brushes within bounds and returns
// the tree root.
func (t *treeUnit) build(bounds [2]vec3, region leafRegion, parent, side int, brushes []brushRef, policy splitPolicy, depth int) childRef {
	cp := t.ba.mark() // everything the subtree allocates dies at release

	t.hist[histBucket(len(brushes))]++
	if t.trace != nil && len(t.nodes) >= t.nextTrace {
		t.nextTrace += 20000
		t.trace("progress: nodes %d leafs %d brushes %d depth %d region.planes %d straddles %d cands %d",
			len(t.nodes), len(t.leafs), len(brushes), depth, len(region.bs), t.straddles, *t.ba.candsScored)
	}

	// AUTO budget: above maxNodeSize use the volume-mid split (no
	// per-brush classification in the candidate scan). Only accept a mid
	// split that actually separates the brush list: a cut nothing
	// straddles would recurse on an unchanged list — non-axial cuts don't
	// even shrink the child bounds — which is the superlinear blowup.
	if policy == splitAuto &&
		(t.nodeAboveMaxNodeSize(bounds) ||
			(t.midsplitFraction > 0 && len(brushes) > t.totalBrushesFraction())) {
		if mp, mok := chooseMidPlaneFromList(t.ba, brushes, region, bounds); mok {
			// Register the mid plane before attempting the split: a
			// rejected split keeps the entry (append-only table — the
			// planeKeys memo would go stale if entries were popped) and
			// unreferenced lump planes are legal.
			mpn := t.register(mp)
			mf, mb := splitBrushList(t.ba, brushes, mpn, mp)
			if len(mf) > 0 && len(mb) > 0 {
				markSidesOnnode(t.ba, brushes, mp, mpn)
				ch := t.splitNode(bounds, region, parent, side, mp, mpn, mf, mb, policy, depth)
				t.ba.release(cp)
				return ch
			}
			t.ba.release(cp)
		}
	}

	p, ok := selectSplitPlane(t.ba, brushes, policy, region, bounds)
	if !ok {
		idx := len(t.leafs)
		t.leafs = append(t.leafs, outLeaf{
			content: contentsOf(t.ba, brushes),
			region:  region,
			parent:  parent,
			side:    side,
		})
		t.ba.release(cp)
		return childRef{isLeaf: true, idx: idx}
	}
	pn := t.register(p)
	markSidesOnnode(t.ba, brushes, p, pn)
	front, back := splitBrushList(t.ba, brushes, pn, p)
	ch := t.splitNode(bounds, region, parent, side, p, pn, front, back, policy, depth)
	t.ba.release(cp)
	return ch
}

// totalBrushesFraction returns the brush count above which a node
// midsplits (the ericw midsplitbrushfraction gate).
func (t *treeUnit) totalBrushesFraction() int {
	return int(float64(t.totalBrushes) * t.midsplitFraction)
}

// nodeAboveMaxNodeSize reports whether any bounds dimension exceeds the
// midsplit budget (ericw maxnodesize).
func (t *treeUnit) nodeAboveMaxNodeSize(bounds [2]vec3) bool {
	s := t.maxNodeSize - splitEpsilon
	return bounds[1].X-bounds[0].X > s ||
		bounds[1].Y-bounds[0].Y > s ||
		bounds[1].Z-bounds[0].Z > s
}

// parallelMinBrushes is the smallest subtree list worth a goroutine: below
// it, recursion stays inline (spawn + adopt overhead dominates).
const parallelMinBrushes = 1 << 30 // TEMP-DISABLED for bisect

// splitNode creates the node record and recurses into the split children.
// Large subtrees build in PARALLEL: each child becomes its own treeUnit
// (own arena working set + local plane table) running in a worker-bounded
// goroutine, then merges back in left-then-right order — the serialised
// output stays byte-identical to a sequential build.
func (t *treeUnit) splitNode(bounds [2]vec3, region leafRegion, parent, side int, p plane, pn int, front, back []brushRef, policy splitPolicy, depth int) childRef {
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

	if t.shared != nil &&
		len(front) >= parallelMinBrushes && len(back) >= parallelMinBrushes {
		select {
		case unitBudget(t.shared) <- struct{}{}:
			done := make(chan childRef, 2)
			mergeCh := make(chan *treeUnit, 2)
			go func() {
				cu := &treeUnit{}
				cu.init(t.shared, t, t.maxNodeSize, nil)
				cu.totalBrushes = t.totalBrushes
				cu.midsplitFraction = t.midsplitFraction
				list := cu.adoptList(t.ba, front)
				ch := cu.build(fb, region.addFront(pn, p), idx, 0, list, policy, depth+1)
				done <- ch
				mergeCh <- cu
			}()
			go func() {
				cu := &treeUnit{}
				cu.init(t.shared, t, t.maxNodeSize, nil)
				cu.totalBrushes = t.totalBrushes
				cu.midsplitFraction = t.midsplitFraction
				list := cu.adoptList(t.ba, back)
				ch := cu.build(bb, region.addBack(pn, p), idx, 1, list, policy, depth+1)
				done <- ch
				mergeCh <- cu
			}()
			ch0, ch1 := <-done, <-done
			cu0, cu1 := <-mergeCh, <-mergeCh
			<-unitBudget(t.shared)
			// Deterministic merge order: left first, then right.
			mch0 := t.mergeFrom(cu0, ch0)
			mch1 := t.mergeFrom(cu1, ch1)
			t.nodes[idx].children = [2]childRef{mch0, mch1}
			return childRef{isLeaf: false, idx: idx}
		default:
			// Worker budget exhausted: fall through to inline recursion.
		}
	}

	ch0 := t.build(fb, region.addFront(pn, p), idx, 0, front, policy, depth+1)
	ch1 := t.build(bb, region.addBack(pn, p), idx, 1, back, policy, depth+1)
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
func (t *treeUnit) finalize(rootBounds [2]vec3, solidBrushes []solidBrushDef) {
	// Scratch arena: facet windings are transient (bounds + solidity only);
	// per-leaf mark/release keeps the scratch bounded instead of piling
	// every leaf's facets into the monotonic arena.
	scratch := newWindingArena()
	for i := range t.leafs {
		m := scratch.mark()
		fs := t.leafs[i].region.facets(scratch, rootBounds)
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
		scratch.release(m)
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
