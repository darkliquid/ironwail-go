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