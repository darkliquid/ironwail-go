package qbsp

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// outFace is a compiler face; vertex/edge tables are built separately.
type outFace struct {
	planenum  int
	side      int8
	texinfo   int
	styles    [4]byte
	lightOfs  int32
	poly      winding
	firstEdge int32 // set at write time
	numEdges  int16
}

// denseCell returns the side of a boundary facet whose content is more
// "material" (solid > liquids > empty), which owns the face orientation.
func denseCell(c1, c2 int32) int {
	if c1 == bsp.ContentsSolid {
		return 0
	}
	if c2 == bsp.ContentsSolid {
		return 1
	}
	if c1 < c2 {
		return 0
	}
	return 1
}

// makeFaces generates the world faces from the arrangement: for every pair
// of cells with different contents sharing a facet, a face whose normal
// points from the denser side into the lighter side. Returns faces (in a
// stable cell order) and a per-cell attachment map.
func (c *compiler) makeFaces(arr *arrangement) ([]outFace, map[int][]int, error) {
	var faces []outFace
	attach := map[int][]int{}

	for ci := range arr.cells {
		cell := arr.cells[ci]
		for _, h := range cell.hs {
			pi := h.plane
			nj := arr.neighborPlane(ci, pi)
			if nj < 0 {
				continue // box boundary: no face
			}
			nbr := arr.cells[nj]
			if nbr.content == cell.content {
				continue
			}
			dense := denseCell(cell.content, nbr.content)
			denseCellIdx, lightCellIdx := ci, nj
			if dense == 1 {
				denseCellIdx, lightCellIdx = nj, ci
			}
			// Each boundary pair resolves to exactly one face, generated
			// from the denser side (skipping the lighter side's iteration).
			if ci != denseCellIdx {
				continue
			}

			// Face normal: outward from the dense cell through its facet.
			outward := !h.front
			if denseCellIdx == nj {
				// flip the orientation of the neighbor's halfspace
				outward = mirrorOutward(nbr, pi)
			}
			pl := c.planes[pi]
			var n vec3
			if outward {
				n = pl.Normal
			} else {
				n = v3(-pl.Normal[0], -pl.Normal[1], -pl.Normal[2])
			}

			// Facet polygon of the dense cell on plane pi, oriented to n.
			poly := c.facetOf(arr, denseCellIdx, pi)
			if poly == nil {
				continue
			}
			poly = windingOrientTo(poly, n)

			side := int8(0)
			if v3Dot(pl.Normal, n) < 0 {
				side = 1
			}

			// Texinfo from the brush on the dense side owning this plane.
			ti := c.texinfoForPlane(arr, denseCellIdx, pi)

			gi := len(faces)
			faces = append(faces, outFace{
				planenum: pi,
				side:     side,
				texinfo:  ti,
				poly:     poly,
			})
			attach[lightCellIdx] = append(attach[lightCellIdx], gi)
		}
	}
	return faces, attach, nil
}

// mirrorOutward determines whether the face normal through the neighbor's
// facet points along the plane's positive normal.
func mirrorOutward(nbr *cell, pi int) bool {
	for _, h := range nbr.hs {
		if h.plane == pi {
			return h.front
		}
	}
	return false
}

// facetOf returns the facet polygon of cell ci on plane pi (nil when the
// cell does not bound on pi or the facet is degenerate).
func (c *compiler) facetOf(arr *arrangement, ci, pi int) winding {
	cell := arr.cells[ci]
	found := false
	for _, h := range cell.hs {
		if h.plane == pi {
			found = true
			break
		}
	}
	if !found {
		return nil
	}
	for _, w := range cell.facets(arr.planes, arr.bounds) {
		// The facet on plane pi: all its points satisfy |dot(n,x)-d| ~ 0.
		pl := c.planes[pi]
		matches := 0
		for _, p := range w {
			if planeSide(pl, p) == 0 {
				matches++
			}
		}
		if matches == len(w) && len(w) >= 3 {
			return append(winding{}, w...)
		}
	}
	return nil
}

// texinfoForPlane picks the texinfo for a plane from the brush that
// contains the given cell among those owning the plane.
func (c *compiler) texinfoForPlane(arr *arrangement, ci, pi int) int {
	center := arr.cellCenter(ci)
	// Prefer a brush on the dense side; fall back to the first registered.
	for _, b := range c.brushes {
		for _, bpi := range b.planes {
			if bpi == pi && insideBrush(center, b) {
				if ti, ok := c.texByPlane[pi]; ok {
					return ti
				}
			}
		}
	}
	if ti, ok := c.texByPlane[pi]; ok {
		return ti
	}
	return 0
}

// edgeTables builds the global vertex/edge/surfedge tables for the faces.
// Faces are processed in order so their firstedge indices land contiguously
// in the surfedge lump. Vertices are deduplicated at float32 precision,
// edges are canonicalised by index order, and surfedges encode reversed use
// as -(edge+1) per the Quake convention.
func edgeTables(faces []outFace) (vertexes []vec3, edges [][2]int32, surfedges []int32) {
	vIdx := map[[3]float32]int32{}
	edgeIdx := map[[2]int32]int32{}

	vIndex := func(v vec3) int32 {
		key := [3]float32{float32(v[0]), float32(v[1]), float32(v[2])}
		if i, ok := vIdx[key]; ok {
			return i
		}
		i := int32(len(vertexes))
		vIdx[key] = i
		vertexes = append(vertexes, vec3{float64(key[0]), float64(key[1]), float64(key[2])})
		return i
	}
	eIndex := func(a, b int32) int32 {
		lo, hi := a, b
		if hi < lo {
			lo, hi = hi, lo
		}
		key := [2]int32{lo, hi}
		if i, ok := edgeIdx[key]; ok {
			return i
		}
		i := int32(len(edges))
		edgeIdx[key] = i
		edges = append(edges, key)
		return i
	}

	for fi := range faces {
		poly := faces[fi].poly
		n := len(poly)
		start := int32(len(surfedges))
		faces[fi].firstEdge = start
		faces[fi].numEdges = int16(n)
		for k := 0; k < n; k++ {
			va := vIndex(poly[k])
			vb := vIndex(poly[(k+1)%n])
			e := eIndex(va, vb)
			if va < vb {
				surfedges = append(surfedges, e)
			} else {
				surfedges = append(surfedges, -e-1)
			}
		}
	}
	return vertexes, edges, surfedges
}

// buildTree is the compiler-level wrapper over treeBuilder.
func (c *compiler) buildTree(arr *arrangement, faceByCell map[int][]int) (root childRef, nodes []outNode, leafs []outLeaf, cellLeaf map[int]int) {
	tb := &treeBuilder{faces: faceByCell, cellLeaf: map[int]int{}}
	cells := make([]int, len(arr.cells))
	for i := range arr.cells {
		cells[i] = i
	}
	root = tb.build(arr, cells)
	return root, tb.nodes, tb.leafs, tb.cellLeaf
}