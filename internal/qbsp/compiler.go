package qbsp

import (
	"fmt"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// Options configure a compile run.
type Options struct {
	// BSP2 emits the extended 32-bit BSP2 format instead of BSP29.
	BSP2 bool
	// Margin is retained for API compatibility; the solidbsp region is the
	// union of the world brush bounds (the classic qbsp entity bounds).
	Margin float64
	// Log receives progress diagnostics; may be nil.
	Log func(format string, a ...any)
	// OmitDetail drops func_detail* brush entities entirely.
	OmitDetail bool
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
	content int32
	bounds  [2]vec3
	sortKey int64
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
	// texByPlane maps a plane index to the texinfo entry used by the brush
	// that owns it (first brush wins).
	texByPlane map[int]int
	logs       []string
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
	clipRoot   int32 // headnode[1] (clip hull root)
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
		list := c.bspBrushList(&g)
		list = chopBrushes(list)
		policy := splitPrecise
		if !world {
			policy = splitFast
		}
		tb := &treeBuild{register: c.addPlaneIndex}
		root := tb.build(bounds, rootRegion(bounds), -1, -1, list, policy)
		tb.finalize(bounds)

		// Renumber leaves non-solid-first (per model) and offset into the
		// shared node/leaf tables. Paths are computed on the local tree
		// before offsetting (parent links stay model-local afterwards).
		nodes, leafs, remap := renumberLeaves(tb.nodes, tb.leafs)
		paths := modelPaths(tb.nodes, tb.leafs, remap)
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
				splitD:   bounds[1][0],
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
			faces, attach, pf, leakPath, leaked = c.buildWorldSurfaces(bounds, root, nodes, leafs, paths, m)
		} else {
			faces, attach = c.buildModelSurfaces(bounds, root, nodes, leafs, paths)
		}
		for i := range leafs {
			leafs[i].marksurface = attach[i]
		}

		// Clip hulls (per model, shared lump).
		clipBase := int32(len(allClips))
		hulls := list
		if !world {
			var solid []*bspBrush
			for _, b := range list {
				if b.content == bsp.ContentsSolid {
					solid = append(solid, b)
				}
			}
			hulls = solid
		}
		expanded := c.expandSolidBrushes(hulls, bounds)
		clip := c.buildHullClipNodes(expanded, bounds)
		for i := range clip {
			clip[i].children[0] = offsetClipChild(clip[i].children[0], clipBase)
			clip[i].children[1] = offsetClipChild(clip[i].children[1], clipBase)
		}
		allClips = append(allClips, clip...)

		mo := modelOut{
			mins:      bounds[0],
			maxs:      bounds[1],
			origin:    g.origin,
			root:      root,
			visLeafs:  visLeafs(leafs),
			firstFace: len(allFaces),
			numFaces:  len(faces),
			clipRoot:  clipBase,
		}
		if !world {
			// Q1 shrunken submodel bounds (the engine compensates).
			for i := 0; i < 3; i++ {
				mo.mins[i] += 1
				mo.maxs[i] -= 1
			}
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
	return res, nil
}

// offsetClipChild rebases a clipnode child (>=0 node index) by base;
// negative children are contents and are untouched.
func offsetClipChild(ch int32, base int32) int32 {
	if ch >= 0 {
		return ch + base
	}
	return ch
}

// planeIndexFor finds or creates the plane-table entry for a face.
func (c *compiler) planeIndexFor(face MapFace) (int, bool) {
	p := normalizePlane(face.Plane())
	p.Dist = snapPlaneDist(p.Dist)
	for i, existing := range c.planes {
		if planeEqualNear(p, existing) {
			return i, true
		}
	}
	c.planes = append(c.planes, p)
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
		for i := 0; i < 3; i++ {
			if v[i] < mins[i] {
				mins[i] = v[i]
			}
			if v[i] > maxs[i] {
				maxs[i] = v[i]
			}
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
		p.Dist = snapPlaneDist(p.Dist)
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
					if math.Abs(e[0]-v[0]) < 0.01 && math.Abs(e[1]-v[1]) < 0.01 && math.Abs(e[2]-v[2]) < 0.01 {
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
		{a.Normal[0], a.Normal[1], a.Normal[2]},
		{b.Normal[0], b.Normal[1], b.Normal[2]},
		{c.Normal[0], c.Normal[1], c.Normal[2]},
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
	return vec3{solve(0), solve(1), solve(2)}, true
}

func inBrush(p vec3, planes []plane) bool {
	for _, pl := range planes {
		if v3Dot(pl.Normal, p)-pl.Dist > 0.01 {
			return false
		}
	}
	return true
}
