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
	// Margin is the void ring around the map used for leak detection and
	// the root bounding box (units).
	Margin float64
	// Log receives progress diagnostics; may be nil.
	Log func(format string, a ...any)
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
	brushes    []worldBrush
	wbounds    [2]vec3
	logs       []string
}

func (c *compiler) logf(format string, a ...any) {
	c.logs = append(c.logs, fmt.Sprintf(format, a...))
	c.opts.log(format, a...)
}

// Compile runs the qbsp pipeline over a parsed map and returns a writable
// result. The pipeline is: collect world brushes and planes, build the
// plane arrangement (CSG), resolve cell contents, detect leaks, generate
// faces/edges/vertexes, build clipnode hulls, then serialise lumps.
func Compile(m *Map, opts Options) (*CompileResult, error) {
	if opts.Margin == 0 {
		opts.Margin = 64
	}
	c := &compiler{
		opts:       opts,
		texByPlane: map[int]int{},
	}
	c.logf("--- qbsp %d entities, building planes ---", len(m.Entities))

	world, err := c.collectBrushes(m)
	if err != nil {
		return nil, err
	}

	// 1. Planes: brush faces + six box planes.
	c.addBoxPlanes()

	// 2. Arrangement (CSG).
	arr, err := c.buildWorldArrangement()
	if err != nil {
		return nil, err
	}

	// 3. Cell contents.
	c.assignContents(arr, world)

	// 4. Leak detection.
	leakPath, leaked := c.detectLeak(m, arr)

	// 5. Faces + edges + texinfo.
	faces, faceByCell, err := c.makeFaces(arr)
	if err != nil {
		return nil, err
	}

	// 6. BSP node/leaf tree for the world (non-solid leaves first, so the
	// engine's PVS row math lines up with leaf indices).
	root, nodes, leafs, cellLeaf := c.buildTree(arr, faceByCell)
	nodes, leafs, leafRenumber := renumberLeaves(nodes, leafs)

	// 6b. Portal file (.prt) for vis: shared facets between non-solid
	// leaves, using the renumbered leaf indices.
	portalData := c.gatherPortals(arr, cellLeaf, leafRenumber)

	// 7. Clipnode hulls (one expanded tree shared by hulls 1 and 2).
	clipNodes := c.buildHullClipNodes(world)

	res, err := c.assemble(m, faces, nodes, leafs, root, clipNodes, leakPath, leaked)
	if err != nil {
		return nil, err
	}
	res.PortalFile = portalData
	if leaked {
		c.logf("LEAK: map leaks to the void (%d points in trail)", len(leakPath))
	}
	return res, nil
}

// collectBrushes enumerates world + brush-entity brushes and registers
// their planes/texinfo entries.
func (c *compiler) collectBrushes(m *Map) ([]worldBrush, error) {
	faces := []MapFace{}
	planeIdx := []int{}
	planeOwner := []int{} // brush index per plane
	var bounds [2]vec3
	haveBounds := false

	addBrush := func(brush MapBrush, content int32) (worldBrush, error) {
		wb := worldBrush{orig: brush, content: content}
		for _, face := range brush.Faces {
			pi, ok := c.planeIndexFor(face)
			if !ok {
				// degenerate; skip
				continue
			}
			wb.planes = append(wb.planes, pi)
			if _, exists := c.texByPlane[pi]; !exists {
				// texture from this brush's face
				ti := c.texinfoIndex(face)
				c.texByPlane[pi] = ti
			}
			_ = faces
			_ = planeIdx
			_ = planeOwner
		}
		// bounds of the brush (from its faces' planes)
		wm, wx, err := brushBounds(brush)
		if err != nil {
			return wb, err
		}
		wb.bounds = [2]vec3{wm, wx}
		if !haveBounds {
			bounds = wb.bounds
			haveBounds = true
		} else {
			for i := 0; i < 3; i++ {
				if wm[i] < bounds[0][i] {
					bounds[0][i] = wm[i]
				}
				if wx[i] > bounds[1][i] {
					bounds[1][i] = wx[i]
				}
			}
		}
		return wb, nil
	}

	// World entity brushes.
	world := m.Entities[0]
	for _, brush := range world.Brushes {
		content, draw := contentsForBrush(brush.Faces)
		if !draw {
			continue
		}
		wb, err := addBrush(brush, content)
		if err != nil {
			return nil, err
		}
		c.brushes = append(c.brushes, wb)
	}

	// Expand world bounds by the margin.
	bounds[0][0] -= c.opts.Margin
	bounds[0][1] -= c.opts.Margin
	bounds[0][2] -= c.opts.Margin
	bounds[1][0] += c.opts.Margin
	bounds[1][1] += c.opts.Margin
	bounds[1][2] += c.opts.Margin
	c.wbounds = bounds

	return c.brushes, nil
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

// addBoxPlanes appends the root bounding box planes (always last in the
// table, which buildArrangement requires).
func (c *compiler) addBoxPlanes() {
	bp := boxPlanes(c.wbounds[0], c.wbounds[1])
	for _, p := range bp {
		c.planes = append(c.planes, p)
	}
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


// buildWorldArrangement constructs the CSG arrangement over the world
// brushes' planes within the bounds box.
func (c *compiler) buildWorldArrangement() (*arrangement, error) {
	// planes table already contains brush planes + 6 box planes.
	if len(c.planes) < 6 {
		return nil, fmt.Errorf("no world geometry")
	}
	return buildArrangement(c.planes, c.wbounds), nil
}

// assignContents resolves each cell's contents: the content of the last
// world brush containing the cell centre (later brushes override, matching
// Quake's water-in-pit semantics), else empty.
func (c *compiler) assignContents(arr *arrangement, brushes []worldBrush) {
	for ci := range arr.cells {
		center := arr.cellCenter(ci)
		content := int32(bsp.ContentsEmpty)
		for _, b := range brushes {
			if insideBrush(center, b) {
				content = b.content
			}
		}
		arr.cells[ci].content = content
	}
}

// insideBrush tests a point against the brush's outward planes
// (interior = dot(n,x) <= d + tol).
func insideBrush(p vec3, b worldBrush) bool {
	// b.planes refers to c.planes — but assignContents doesn't have c. Use
	// the original faces instead; this mirrors the caller's data.
	for _, face := range b.orig.Faces {
		pl := face.Plane()
		if v3Dot(pl.Normal, p)-pl.Dist > 0.01 {
			return false
		}
	}
	return true
}