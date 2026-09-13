package qbsp

import "sync"

// A treeUnit is a self-contained solidbsp tree build: its own brushArena
// working set (slabs + winding arena), own node/leaf accumulators, and own
// local plane table layered over its parent's. Units make tree builds
// composable and parallel: front/back subtrees and clip-hull trees run
// concurrently without locks, and each unit's output merges into its
// parent in deterministic order (left first, then right), so the
// serialised BSP is byte-identical to a sequential build.
//
// Plane-index spaces: indices below localStart refer to the ancestor
// chain (shared table or ancestor locals) and pass through merges
// unchanged; indices at/above it index localPlanes and are interned into
// the parent's space during merge.

// treeUnit owns one solidbsp tree build.
type treeUnit struct {
	nodes []outNode
	leafs []outLeaf
	wa    *windingArena
	ba    *brushArena
	// register returns the plane index for a geometric plane; units use
	// unitRegister (set in init).
	register func(p plane) int
	// maxNodeSize is the AUTO midsplit budget for this build
	// (Options.MaxNodeSize, default 1024).
	maxNodeSize float64
	// totalBrushes is the root list size; midsplitFraction gates the
	// midsplit on the node's share of the model's brushes (ericw
	// midsplitbrushfraction).
	totalBrushes     int
	midsplitFraction float64
	// trace, when non-nil, receives node-count progress diagnostics (debug).
	trace     func(format string, a ...any)
	nextTrace int

	localPlanes []plane
	localKeys   map[orientedPlaneKey]int
	// hist buckets nodes by input-list size; straddles counts brushes
	// split (CSG volume diagnostics).
	hist        [6]int64
	straddles   int64
	candsScored int64
	shared      *compiler
	parentUnit  *treeUnit
	// localStart is this unit's local-index origin captured at init: the
	// ancestor-space length (shared table plus every ancestor's locals) at
	// the moment the unit spawns. Indices below it are ancestor-space and
	// pass through merges; indices at/above it index localPlanes.
	localStart int
}

// init prepares a unit over the compiler's (frozen) shared table.
func (u *treeUnit) init(shared *compiler, parent *treeUnit, maxNodeSize float64, trace func(string, ...any)) {
	u.shared = shared
	u.parentUnit = parent
	u.localKeys = map[orientedPlaneKey]int{}
	u.localStart = shared.frozenLen()
	for p := parent; p != nil; p = p.parentUnit {
		u.localStart += len(p.localPlanes)
	}
	u.wa = newWindingArena()
	u.ba = newBrushArena(u.wa)
	u.ba.straddle = &u.straddles
	u.ba.candsScored = &u.candsScored
	u.maxNodeSize = maxNodeSize
	u.trace = trace
	u.register = u.unitRegister
}

// unitRegister resolves a plane to an index in THIS unit's space: ancestor
// locals first (nearest ancestor last), then the shared table (read-only
// while units run, so its memo map is concurrency-safe), then this unit's
// locals. New planes append locally; merges intern them upward in
// deterministic order.
func (u *treeUnit) unitRegister(p plane) int {
	p = normalizePlane(p)
	p.Dist = snapPlaneDist(p.Normal, p.Dist)
	key := orientedPlaneKeyOf(p)
	for a := u.parentUnit; a != nil; a = a.parentUnit {
		if i, ok := a.localKeys[key]; ok {
			return i
		}
	}
	// The shared table is frozen while units run: read-only lookup (the
	// shared memo is only written by sequential phases).
	if i := u.shared.lookupPlaneIndexRO(p); i >= 0 {
		return i
	}
	if i, ok := u.localKeys[key]; ok {
		return i
	}
	idx := u.localStart + len(u.localPlanes)
	u.localPlanes = append(u.localPlanes, p)
	u.localKeys[key] = idx
	return idx
}

// mergePlanenum interns a unit-local plane index into the PARENT's index
// space (the parent's locals, or the shared table for root merges).
func (u *treeUnit) mergePlanenum(local int) int {
	if local < u.localStart {
		return local
	}
	p := u.localPlanes[local-u.localStart]
	if u.parentUnit != nil {
		key := orientedPlaneKeyOf(p)
		if i, ok := u.parentUnit.localKeys[key]; ok {
			return i
		}
		idx := u.parentUnit.localStart + len(u.parentUnit.localPlanes)
		u.parentUnit.localPlanes = append(u.parentUnit.localPlanes, p)
		u.parentUnit.localKeys[key] = idx
		return idx
	}
	return u.shared.addPlaneIndex(p)
}

// mergeUp remaps every unit-local plane index in this unit's nodes and
// leaf regions into the parent space, bottom-up through the unit chain.
// Root units intern into the shared (append-only) table.
func (u *treeUnit) mergeUp() {
	for i := range u.nodes {
		u.nodes[i].planenum = u.mergePlanenum(u.nodes[i].planenum)
	}
	for i := range u.leafs {
		bs := u.leafs[i].region.bs
		for bi := range bs {
			bs[bi].pi = u.mergePlanenum(bs[bi].pi)
		}
	}
	// Post-merge, every local index resolves through the shared table;
	// dropping the locals keeps later sibling units (hulls) from reading
	// stale mappings.
	u.localPlanes = nil
	u.localKeys = nil
	if u.trace != nil {
		u.trace("tree hist: empty=%d tiny=%d small=%d mid=%d large=%d huge=%d straddles=%d cands=%d",
			u.hist[0], u.hist[1], u.hist[2], u.hist[3], u.hist[4], u.hist[5], u.straddles, *u.ba.candsScored)
	}
}

// adopt copies a brush (sides + windings) from another unit's arena into
// this unit's, re-indexing everything, so subtree units own their whole
// working set while the parent's arena stays untouched.
func (u *treeUnit) adopt(src *brushArena, id brushRef) brushRef {
	srcSides := src.sidesOf(id)
	sides := make([]sideRec, len(srcSides))
	for i := range srcSides {
		sides[i] = srcSides[i]
		w := src.w.at(sides[i].w)
		view, ref := u.wa.reserve(len(w))
		view = view[:len(w)]
		copy(view, w)
		ref.count = int32(len(w))
		sides[i].w = ref
	}
	b := src.brushes[id]
	return u.ba.addBrush(sides, b.content, b.sortKey)
}

// adoptList adopts a whole brush list.
func (u *treeUnit) adoptList(src *brushArena, list []brushRef) []brushRef {
	out := make([]brushRef, len(list))
	for i, id := range list {
		out[i] = u.adopt(src, id)
	}
	return out
}

// mergeFrom folds a finished child unit into this unit: appends the
// child's nodes/leafs, remaps its plane indices into this unit's space,
// and rebases its internal node/leaf indices. Returns the child's root
// reference, rebased.
func (u *treeUnit) mergeFrom(child *treeUnit, root childRef) childRef {
	nodeBase := len(u.nodes)
	leafBase := len(u.leafs)

	for i := range child.nodes {
		nd := child.nodes[i]
		nd.planenum = child.mergePlanenum(nd.planenum)
		if nd.parent >= 0 {
			nd.parent += int(nodeBase)
		}
		for ci, ch := range nd.children {
			if ch.isLeaf {
				ch.idx += leafBase
			} else {
				ch.idx += nodeBase
			}
			nd.children[ci] = ch
		}
		u.nodes = append(u.nodes, nd)
	}
	for i := range child.leafs {
		lf := child.leafs[i]
		if lf.parent >= 0 {
			lf.parent += int(nodeBase)
		}
		for bi := range lf.region.bs {
			lf.region.bs[bi].pi = child.mergePlanenum(lf.region.bs[bi].pi)
		}
		u.leafs = append(u.leafs, lf)
	}

	if root.isLeaf {
		root.idx += leafBase
	} else {
		root.idx += nodeBase
	}
	return root
}

// unitBudget returns the compiler's worker semaphore bounding concurrent
// subtree units.
func unitBudget(c *compiler) chan struct{} {
	budget_mu.Lock()
	defer budget_mu.Unlock()
	if v, ok := unitBudgets[c]; ok {
		return v
	}
	v := make(chan struct{}, c.maxWorkers)
	unitBudgets[c] = v
	return v
}

var (
	budget_mu   sync.Mutex
	unitBudgets = map[*compiler]chan struct{}{}
)

// histBucket indexes the brush-count histogram (diagnostic).
func histBucket(n int) int {
	switch {
	case n == 0:
		return 0
	case n <= 2:
		return 1
	case n <= 16:
		return 2
	case n <= 128:
		return 3
	case n <= 1024:
		return 4
	default:
		return 5
	}
}
