// Package light bakes lightmaps for compiled BSPs (bead ironwail-go-t63,
// M4): it computes per-face lightmap sample grids from the texinfo vectors,
// accumulates direct point-light contributions with shadow traces against
// the BSP, and writes the Lighting lump plus an optional QLIT v1 colored
// sidecar that the engine's ApplyLitFile accepts.
package light

import (
	"math"
)

// Extents are the face's S/T texture-coordinate bounds, which size the
// lightmap grid (classic Quake: 16 units per luxel).
type Extents struct {
	Mins, Maxs [2]float64
	W, H       int
}

// CalcExtents computes the lightmap extents of a face polygon from its
// texinfo vectors, mirroring qbsp's CalcSurfaceExtents: for each vertex,
// s = dot(v, vecs[0]) + vecs[0][3] and t = dot(v, vecs[1]) + vecs[1][3];
// the lightmap size is (max-min)/16 + 1 per axis.
func CalcExtents(poly [][3]float64, vecs [2][4]float64) Extents {
	var e Extents
	first := true
	for _, v := range poly {
		s := v[0]*vecs[0][0] + v[1]*vecs[0][1] + v[2]*vecs[0][2] + vecs[0][3]
		t := v[0]*vecs[1][0] + v[1]*vecs[1][1] + v[2]*vecs[1][2] + vecs[1][3]
		if first {
			e.Mins = [2]float64{s, t}
			e.Maxs = [2]float64{s, t}
			first = false
			continue
		}
		if s < e.Mins[0] {
			e.Mins[0] = s
		}
		if s > e.Maxs[0] {
			e.Maxs[0] = s
		}
		if t < e.Mins[1] {
			e.Mins[1] = t
		}
		if t > e.Maxs[1] {
			e.Maxs[1] = t
		}
	}
	e.W = int(math.Floor((e.Maxs[0]-e.Mins[0])/16)) + 1
	e.H = int(math.Floor((e.Maxs[1]-e.Mins[1])/16)) + 1
	return e
}