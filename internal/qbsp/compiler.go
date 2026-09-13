package qbsp

import (
	"fmt"
	"math"
	"runtime"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// Options configure a compile run.
type Options struct {
	// BSP2 emits the extended 32-bit BSP2 format instead of BSP29.
	BSP2 bool
	// TwoPSB (with BSP2) emits the BSP2RMQ format: 32-bit indices with
	// 16-bit node/leaf bounds ("2psb" magic, large-map compromise).
	TwoPSB bool
	// Margin is retained for API compatibility; the solidbsp region is the
	// union of the world brush bounds (the classic qbsp entity bounds).
	Margin float64
	// Log receives progress diagnostics; may be nil.
	Log func(format string, a ...any)
	// OmitDetail drops func_detail* brush entities entirely.
	OmitDetail bool
	// MaxNodeSize is the AUTO-policy midsplit budget (ericw maxnodesize):
	// nodes larger than this in any dimension use the volume-mid split
	// instead of per-brush scoring. Default 1024 (ericw default) when 0.
	// Lower values trade tree quality for compile speed on huge maps.
	MaxNodeSize float64
	// MidsplitFraction midsplits nodes holding more than this fraction of
	// the model's brushes, regardless of bounds size (ericw
	// midsplitbrushfraction). Default 0.1 when 0; <=0 disables only via
	// explicit negative values.
	MidsplitFraction float64
}

func (o *Options) log(format string, a ...any) {
	if o.Log != nil {
		o.Log(format, a...)
	}
}

// CompileResult is the compiled BSP ready for serialisation.
type CompileResult struct {
	BSP2     bool
	Data     []byte // serialised lump image (header appended by writeBSP)
	Log      []string
	LeakPath []Point // leak point trail (nil when sealed)
	Leaked   bool
	// PortalFile is the PRT1 portal file for vis (nil when the tree is
	// degenerate or sealed with no portals).
	PortalFile *PortalFile
	// Models is the number of model records emitted (1 + brush entities).
	Models int
}

// Point is the exported alias for the compiler's double-precision 3D point,
// used in public API surfaces (leak trails).
type Point = vec3

// worldBrush is a compiler-side brush: its outward-facing planes refer into
// the global plane table, with the brush's content and per-face textures.
type worldBrush struct {
	orig    MapBrush
	planes  []int // plane table indices (one per face)
	outward []plane
	content int32
	bounds  [2]vec3
	sortKey int64
	detail  bool // from a func_detail* entity (split pass ordering)
}

// texinfoEntry is one final texinfo (deduplicated).
type texinfoEntry struct {
	vecs    [2][4]float64
	texture string
	flags   int32
}

// compiler holds per-run state.
type compiler struct {
	opts    Options
	planes  []plane
	texinfo []texinfoEntry
	// wa backs all winding allocations for the compile. Tree-path frames
	// checkpoint/rollback it in build(); everything else allocates
	// monotonically (winding lifetime is output-scale).
	wa *windingArena
	// ba owns the solidbsp working set (brush/side slabs referencing wa).
	ba *brushArena
	// maxWorkers bounds concurrent tree units (default GOMAXPROCS).
	maxWorkers int
	// midsplitFraction is the resolved brush-fraction midsplit gate.
	midsplitFraction float64
	// planeMu guards planes/planeKeys: tree units read concurrently
	// (RUnlock-ed lookups); sequential phases hold the write lock.
	planeMu sync.RWMutex
	// maxNodeSize is the resolved AUTO midsplit budget (Options.MaxNodeSize
	// with the ericw default 1024 applied).
	maxNodeSize float64
	// texByPlane maps a plane index to the texinfo entry used by the brush
	// that owns it (first brush wins).
	texByPlane map[int]int
	// planeKeys memoizes bit-exact plane lookups (see lookupPlaneIndex):
	// the tolerant linear scan is quadratic on large maps, so scan hits
	// record their resolved index here. Near-duplicates that miss the map
	// still resolve through the grid-bucketed scan, preserving
	// first-match semantics.
	planeKeys map[orientedPlaneKey]int
	planeGrid map[[4]int][]int32
	logs      []string
}

func (c *compiler) logf(format string, a ...any) {
	c.logs = append(c.logs, fmt.Sprintf(format, a...))
	c.opts.log(format, a...)
}

// modelOut is one emitted model record (world or brush-entity submodel).
type modelOut struct {
	mins, maxs vec3
	origin     vec3
	root       childRef // absolute node/leaf ref into the shared tables
	visLeafs   int32
	firstFace  int
	numFaces   int
	clipRoot1  int32 // headnode[1] (hull-1 clip root)
	clipRoot2  int32 // headnode[2] (hull-2 clip root)
}

// Compile runs the qbsp pipeline over a parsed map and returns a writable
// result: per-model solidbsp trees (world + brush entities), chops,
// leaf-content resolution, faces/edges/vertexes, leak detection, and
// per-model clipnode hulls, then serialises the lumps.
func Compile(m *Map, opts Options) (*CompileResult, error) {
	if len(m.Entities) == 0 {
		return nil, fmt.Errorf("qbsp: no entities")
	}
	c := &compiler{
		opts:       opts,
		texByPlane: map[int]int{},
		planeKeys:  map[orientedPlaneKey]int{},
		planeGrid:  map[[4]int][]int32{},
		wa:         newWindingArena(),
	}
	c.ba = newBrushArena(c.wa)
	c.maxNodeSize = opts.MaxNodeSize
	if c.maxNodeSize == 0 {
		c.maxNodeSize = 1024 // ericw-tools maxnodesize default
	}
	c.midsplitFraction = opts.MidsplitFraction
	if c.midsplitFraction == 0 {
		c.midsplitFraction = 0.1
	}
	c.maxWorkers = runtime.GOMAXPROCS(0)
	if c.maxWorkers > 8 {
		c.maxWorkers = 8
	}
	c.logf("--- qbsp %d entities, building planes ---", len(m.Entities))

	groups, err := c.collectAllBrushes(m, opts.OmitDetail)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 || len(groups[0].brushes) == 0 {
		return nil, fmt.Errorf("qbsp: no world geometry")
	}

	var models []modelOut
	var allFaces []outFace
	var allNodes []outNode
	var allLeafs []outLeaf
	var allClips []outClipNode

	var pf *PortalFile
	var leakPath []vec3
	leaked := false

	for gi, g := range groups {
		world := g.isWorld
		bounds := worldBoundsOf(&g)
		unit := &treeUnit{}
		unit.init(c, nil, c.maxNodeSize, c.opts.Log)
		unit.totalBrushes = len(g.brushes)
		unit.midsplitFraction = c.midsplitFraction
		list := unit.bspBrushList(&g)
		list = chopBrushes(unit.ba, list)
		policy := splitAuto // world: ericw AUTO (midsplit budget above maxNodeSize)
		if !world {
			policy = splitFast
		}
		root := unit.build(bounds, rootRegion(bounds), -1, -1, list, policy, 0)
		unit.mergeUp()
		var solidBrushes []solidBrushDef
		for _, wb := range g.brushes {
			if wb.content == bsp.ContentsSolid {
				solidBrushes = append(solidBrushes, solidBrushDef{
					bounds: wb.bounds,
					planes: wb.OutwardPlanes(),
				})
			}
		}
		unit.finalize(bounds, solidBrushes)

		if world {
			leakPath, leaked = c.floodLeakCheck(bounds, root, unit.nodes, unit.leafs, m)
		}

		// Renumber leaves non-solid-first (per model) and offset into the
		// shared node/leaf tables. Paths are computed on the local tree
		// before offsetting (parent links stay model-local afterwards).
		nodes, leafs, remap := renumberLeaves(unit.nodes, unit.leafs)
		paths := modelPaths(unit.nodes, unit.leafs, remap)
		nodeBase, leafBase := len(allNodes), len(allLeafs)
		for i := range nodes {
			for ch := 0; ch < 2; ch++ {
				ref := &nodes[i].children[ch]
				if ref.isLeaf {
					// renumberLeaves already remapped this leaf ref.
					ref.idx += leafBase
				} else {
					ref.idx += nodeBase
				}
			}
		}
		if root.isLeaf {
			root.idx = remap[root.idx] + leafBase
		} else {
			root.idx += nodeBase
		}

		// Single-leaf trees get a dummy node so headnode points into the
		// node lump (the engine assumes model headnodes are nodes).
		if root.isLeaf {
			dmy := outNode{
				planenum: 0,
				splitN:   v3(1, 0, 0),
				splitD:   bounds[1].X,
				bounds:   bounds,
				parent:   -1,
				side:     -1,
				children: [2]childRef{{isLeaf: true, idx: root.idx}, {isLeaf: true, idx: root.idx}},
			}
			nodes = append(nodes, dmy)
			root = childRef{isLeaf: false, idx: nodeBase + len(nodes) - 1}
		}

		var faces []outFace
		var attach [][]int
		if world {
			faces, attach, pf = c.buildWorldSurfaces(bounds, nodes, leafs, paths)
		} else {
			faces, attach = c.buildModelSurfaces(bounds, root, nodes, nodeBase, leafBase, leafs, paths)
		}
		for i := range leafs {
			leafs[i].marksurface = attach[i]
		}

		// Clip hulls (per model, shared lump): hull 1 (player box) and
		// hull 2 (large box). The world's hull-1 tree lands at clipnode 0,
		// which the engine's world collision uses directly; submodels get
		// both trees with roots in headnode[1]/[2].
		clipRoot1, clipRoot2 := unit.buildClipHulls(world, list, bounds, &allClips)

		mo := modelOut{
			mins:      bounds[0],
			maxs:      bounds[1],
			origin:    g.origin,
			root:      root,
			visLeafs:  visLeafs(leafs),
			firstFace: len(allFaces),
			numFaces:  len(faces),
			clipRoot1: clipRoot1,
			clipRoot2: clipRoot2,
		}
		if !world {
			// Q1 shrunken submodel bounds (the engine compensates).
			mo.mins.X += 1
			mo.mins.Y += 1
			mo.mins.Z += 1
			mo.maxs.X -= 1
			mo.maxs.Y -= 1
			mo.maxs.Z -= 1
		}
		models = append(models, mo)
		allFaces = append(allFaces, faces...)
		allNodes = append(allNodes, nodes...)
		allLeafs = append(allLeafs, leafs...)

		c.logf("model %d: %s, nodes %d, leafs %d, faces %d, clipnodes %d",
			gi, map[bool]string{true: "world", false: "submodel"}[world],
			len(nodes), len(leafs), len(faces), len(allClips))
	}

	// T-junction fixing (crack elimination between coplanar faces), then
	// global face plane-ordering + node spans.
	if len(allFaces) > 1 {
		fixTJunctions(allFaces)
	}
	orderFacesByPlane(&allFaces, allLeafs)
	setNodeFaceSpans(allNodes, allFaces)

	res, err := c.assemble(m, models, allFaces, allNodes, allLeafs, allClips, leakPath, leaked)
	if err != nil {
		return nil, err
	}
	res.PortalFile = pf
	res.Models = len(models)
	if leaked {
		c.logf("LEAK: map leaks to the void (%d points in trail)", len(leakPath))
	}
	// Append the BRUSHLIST BSPX lump (tools/verification; the engine
	// tolerates its absence and ignores appended data).
	if bx, err := AppendBSPX(res.Data, groups); err == nil {
		res.Data = bx
	}
	return res, nil
}

// frozenLen is the plane-table length while tree units run: the table is
// append-only and only grows during sequential merges, so unit builds see
// it read-only.
func (c *compiler) frozenLen() int { return len(c.planes) }

// buildClipHulls compiles and appends the per-model clip trees (hull 1 =
// player box, hull 2 = large box) into the shared clipnode lump, returning
// their roots (clipnode indices).
func (u *treeUnit) buildClipHulls(world bool, list []brushRef, bounds [2]vec3, allClips *[]outClipNode) (int32, int32) {
	hulls := list
	if !world {
		var solid []brushRef
		for _, b := range list {
			if u.ba.brushes[b].content == bsp.ContentsSolid {
				solid = append(solid, b)
			}
		}
		hulls = solid
	}
	// SEQUENTIAL for now: parallel hull units double peak memory (two full
	// unit working sets) and the corpus harness already runs maps in
	// parallel — re-land with the worker budget integrated (each hull
	// acquiring a token) before enabling.
	appendTree := func(ext [2]vec3) int32 {
		base := int32(len(*allClips))
		clip := u.buildHullClipNodes(hulls, bounds, ext)
		for i := range clip {
			clip[i].children[0] = offsetClipChild(clip[i].children[0], base)
			clip[i].children[1] = offsetClipChild(clip[i].children[1], base)
		}
		*allClips = append(*allClips, clip...)
		return base
	}
	// The world's hull-1 tree must stay at clipnode 0 (the engine's world
	// collision traces from FirstClipNode=0).
	r1 := appendTree(hull1Extents)
	_ = r1
	return r1, appendTree(hull2Extents)
}

// offsetClipChild rebases a clipnode child (>=0 node index) by base;
// negative children are contents and are untouched.
func offsetClipChild(ch int32, base int32) int32 {
	if ch >= 0 {
		return ch + base
	}
	return ch
}

// lookupPlaneIndex finds a tolerance-equal table plane for p, or -1.
// Bit-identical queries hit the c.planeKeys memo; misses run the exact
// first-match scan (bucketed variants leaked: coverage edge cases change
// which duplicates merge, which changes sealing). The scan is ~10% of the
// mutator on large maps — acceptable for exact semantics.
// Write-locked: call sites in sequential phases only.
func (c *compiler) lookupPlaneIndex(p plane) int {
	c.planeMu.Lock()
	defer c.planeMu.Unlock()
	return c.lookupPlaneIndexLocked(p)
}

func (c *compiler) lookupPlaneIndexLocked(p plane) int {
	key := orientedPlaneKeyOf(p)
	if i, ok := c.planeKeys[key]; ok {
		return i
	}
	best := -1
	for _, i := range c.probePlaneBuckets(p) {
		if planeEqualNear(p, c.planes[i]) && (best == -1 || i < best) {
			best = i
		}
	}
	if best >= 0 {
		c.planeKeys[key] = best
	}
	return best
}

// lookupPlaneIndexRO is the read-only variant for parallel tree units: it
// never writes the memo (units keep their own local dedup maps).
func (c *compiler) lookupPlaneIndexRO(p plane) int {
	c.planeMu.RLock()
	defer c.planeMu.RUnlock()
	key := orientedPlaneKeyOf(p)
	if i, ok := c.planeKeys[key]; ok {
		return i
	}
	best := -1
	for _, i := range c.probePlaneBuckets(p) {
		if planeEqualNear(p, c.planes[i]) && (best == -1 || i < best) {
			best = i
		}
	}
	return best
}

// plane-grid quantization: planeEqualNear admits same-orientation normals
// within ~0.0141 per component (dot >= 1-1e-4) and distances within 0.01,
// so 1/64 cells with +-1 neighbor probes cover every within-tolerance
// plane once the key is canonicalized to the positive orientation — the
// match is orientation-agnostic, and without canonicalization (n,d) and
// (-n,-d) land in opposite cells so opposite-facing wall duplicates never
// merge (the leak on e3m5/e3m6/e2m5/e2m7).
const (
	planeGridQuant  = 64.0
	planeGridOrigin = 1 << 20
)

// canonicalPlaneKey negates (n, d) when the dominant normal component is
// negative, so both orientations of one geometric plane share a grid key.
func canonicalPlaneKey(n vec3, d float64) (vec3, float64) {
	cn := classifyPlane(n)
	dom := 0.0
	switch cn {
	case planeX:
		dom = n.X
	case planeY:
		dom = n.Y
	case planeZ:
		dom = n.Z
	default:
		dom = n.X
		if absF(n.Y) > absF(dom) {
			dom = n.Y
		}
		if absF(n.Z) > absF(dom) {
			dom = n.Z
		}
	}
	if dom < 0 {
		return n.Neg(), -d
	}
	return n, d
}

func absF(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func planeGridKey(n vec3, d float64) [4]int {
	cn, cd := canonicalPlaneKey(n, d)
	return [4]int{
		int(cn.X*planeGridQuant) + planeGridOrigin,
		int(cn.Y*planeGridQuant) + planeGridOrigin,
		int(cn.Z*planeGridQuant) + planeGridOrigin,
		int(cd*planeGridQuant) + planeGridOrigin,
	}
}

// indexPlane registers a table entry in its home grid cell.
func (c *compiler) indexPlane(i int, p plane) {
	k := planeGridKey(p.Normal, p.Dist)
	c.planeGrid[k] = append(c.planeGrid[k], int32(i))
}

// probePlaneBuckets returns candidate table indices within tolerance
// reach of p (the +-1 neighbor cells of the canonical key).
func (c *compiler) probePlaneBuckets(p plane) []int {
	k := planeGridKey(p.Normal, p.Dist)
	seen := map[int32]bool{}
	var out []int
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			for dz := -1; dz <= 1; dz++ {
				for dd := -1; dd <= 1; dd++ {
					gk := [4]int{k[0] + dx, k[1] + dy, k[2] + dz, k[3] + dd}
					for _, i := range c.planeGrid[gk] {
						if !seen[i] {
							seen[i] = true
							out = append(out, int(i))
						}
					}
				}
			}
		}
	}
	return out
}

// planeIndexFor finds or creates the plane-table entry for a face.
func (c *compiler) planeIndexFor(face MapFace) (int, bool) {
	p := normalizePlane(face.Plane())
	p.Dist = snapPlaneDist(p.Normal, p.Dist)
	if i := c.lookupPlaneIndex(p); i >= 0 {
		return i, true
	}
	c.planes = append(c.planes, p)
	c.indexPlane(len(c.planes)-1, p)
	c.planeKeys[orientedPlaneKeyOf(p)] = len(c.planes) - 1
	return len(c.planes) - 1, true
}

// texinfoIndex dedupes a texinfo entry.
func (c *compiler) texinfoIndex(face MapFace) int {
	vecs := face.Vecs
	flags := int32(0)
	if contentsKindForTexture(face.TexName) == bsp.ContentsSky {
		flags |= bsp.TexSpecial
	}
	for i, ti := range c.texinfo {
		if ti.texture == face.TexName && ti.vecs == vecs && ti.flags == flags {
			return i
		}
	}
	c.texinfo = append(c.texinfo, texinfoEntry{vecs: vecs, texture: face.TexName, flags: flags})
	return len(c.texinfo) - 1
}

// brushBounds computes the AABB of a brush from its face planes.
func brushBounds(brush MapBrush) (vec3, vec3, error) {
	verts := brushVerts(brush)
	if len(verts) == 0 {
		return vec3{}, vec3{}, fmt.Errorf("brush has no volume")
	}
	mins, maxs := verts[0], verts[0]
	for _, v := range verts[1:] {
		if v.X < mins.X {
			mins.X = v.X
		}
		if v.X > maxs.X {
			maxs.X = v.X
		}
		if v.Y < mins.Y {
			mins.Y = v.Y
		}
		if v.Y > maxs.Y {
			maxs.Y = v.Y
		}
		if v.Z < mins.Z {
			mins.Z = v.Z
		}
		if v.Z > maxs.Z {
			maxs.Z = v.Z
		}
	}
	return mins, maxs, nil
}

// brushVerts returns the vertices of the convex brush volume: every
// triple-plane intersection point that lies inside all face planes.
func brushVerts(brush MapBrush) []vec3 {
	planes := make([]plane, 0, len(brush.Faces))
	for _, face := range brush.Faces {
		p := face.Plane()
		p.Dist = snapPlaneDist(p.Normal, p.Dist)
		planes = append(planes, p)
	}
	var out []vec3
	n := len(planes)
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				v, ok := planeTriplePoint(planes[i], planes[j], planes[k])
				if !ok || !inBrush(v, planes) {
					continue
				}
				dup := false
				for _, e := range out {
					if math.Abs(e.X-v.X) < 0.01 && math.Abs(e.Y-v.Y) < 0.01 && math.Abs(e.Z-v.Z) < 0.01 {
						dup = true
						break
					}
				}
				if !dup {
					out = append(out, v)
				}
			}
		}
	}
	return out
}

// planeTriplePoint solves the 3x3 system n_i . x = d_i for three planes.
func planeTriplePoint(a, b, c plane) (vec3, bool) {
	m := [3][3]float64{
		{a.Normal.X, a.Normal.Y, a.Normal.Z},
		{b.Normal.X, b.Normal.Y, b.Normal.Z},
		{c.Normal.X, c.Normal.Y, c.Normal.Z},
	}
	det := m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) -
		m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
		m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
	if math.Abs(det) < 1e-9 {
		return vec3{}, false
	}
	d := [3]float64{a.Dist, b.Dist, c.Dist}
	// Cramer's rule.
	solve := func(col int) float64 {
		var t [3][3]float64
		copy(t[:], m[:])
		for r := 0; r < 3; r++ {
			t[r][col] = d[r]
		}
		return (t[0][0]*(t[1][1]*t[2][2]-t[1][2]*t[2][1]) -
			t[0][1]*(t[1][0]*t[2][2]-t[1][2]*t[2][0]) +
			t[0][2]*(t[1][0]*t[2][1]-t[1][1]*t[2][0])) / det
	}
	return vec3{X: solve(0), Y: solve(1), Z: solve(2)}, true
}

func inBrush(p vec3, planes []plane) bool {
	for _, pl := range planes {
		if pl.Normal.Dot(p)-pl.Dist > 0.01 {
			return false
		}
	}
	return true
}
