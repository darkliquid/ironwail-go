package qbsp

import (
	"math"
	"sort"
)

// fixTJunctions eliminates T-junction cracks between coplanar faces: when a
// face's edge is split by another face's vertex (on the same plane), the
// long face gains that vertex so the shared edge is split identically on
// both sides (edgeTables then welds them into the same edge). Vertices are
// inserted only for faces lying on the same geometric plane — cracks only
// occur between coplanar neighbours (the classic tjunc interaction rule).
func fixTJunctions(faces []outFace) {
	// Group faces by geometric plane (cracks only occur between coplanar
	// faces sharing boundary lines).
	byPlane := map[int][]int{}
	for fi := range faces {
		byPlane[faces[fi].planenum] = append(byPlane[faces[fi].planenum], fi)
	}
	for _, members := range byPlane {
		if len(members) < 2 {
			continue
		}
		tjuncGroup(faces, members)
	}
}

// tjuncGroup fixes T-junctions within one plane's face group.
func tjuncGroup(faces []outFace, members []int) {
	// Vertex spatial hash (4-unit cells) restricted to the group.
	type key struct{ x, y, z int32 }
	cell := 4.0
	keyOf := func(p vec3) key {
		return key{
			int32(math.Floor(p[0] / cell)),
			int32(math.Floor(p[1] / cell)),
			int32(math.Floor(p[2] / cell)),
		}
	}
	verts := map[key][]vec3{}
	for _, fi := range members {
		for _, p := range faces[fi].poly {
			verts[keyOf(p)] = append(verts[keyOf(p)], p)
		}
	}

	// Inserted vertices per face, deduped by position to avoid duplicate
	// insertion when two neighbours break the same edge.
	inserted := make([][]vec3, len(faces))

	for _, fi := range members {
		poly := faces[fi].poly
		if len(poly) < 3 {
			continue
		}
		n := len(poly)
		for k := 0; k < n; k++ {
			a := poly[k]
			b := poly[(k+1)%n]
			// AABB of the edge expanded by the tolerance.
			mins := vec3{math.Min(a[0], b[0]) - tjEps, math.Min(a[1], b[1]) - tjEps, math.Min(a[2], b[2]) - tjEps}
			maxs := vec3{math.Max(a[0], b[0]) + tjEps, math.Max(a[1], b[1]) + tjEps, math.Max(a[2], b[2]) + tjEps}
			k0, k1 := keyOf(mins), keyOf(maxs)
			cand := map[[3]float32]bool{}
			for cx := k0.x; cx <= k1.x; cx++ {
				for cy := k0.y; cy <= k1.y; cy++ {
					for cz := k0.z; cz <= k1.z; cz++ {
						for _, v := range verts[key{cx, cy, cz}] {
							cand[[3]float32{float32(v[0]), float32(v[1]), float32(v[2])}] = true
						}
					}
				}
			}
			var ins []vec3
			for ck := range cand {
				v := vec3{float64(ck[0]), float64(ck[1]), float64(ck[2])}
				if pointOnSegment(v, a, b) {
					ins = append(ins, v)
				}
			}
			if len(ins) == 0 {
				continue
			}
			// Sort along the edge (by projection onto b-a).
			ab := v3Sub(b, a)
			sort.Slice(ins, func(i, j int) bool {
				return v3Dot(v3Sub(ins[i], a), ab) < v3Dot(v3Sub(ins[j], a), ab)
			})
			inserted[fi] = append(inserted[fi], ins...)
		}
		// Rebuild the polygon with the insertions.
		var out winding
		for k := 0; k < len(poly); k++ {
			a := poly[k]
			b := poly[(k+1)%len(poly)]
			out = append(out, a)
			var between []vec3
			for _, v := range inserted[fi] {
				if pointOnSegment(v, a, b) {
					between = append(between, v)
				}
			}
			ab := v3Sub(b, a)
			sort.Slice(between, func(i, j int) bool {
				return v3Dot(v3Sub(between[i], a), ab) < v3Dot(v3Sub(between[j], a), ab)
			})
			out = append(out, between...)
		}
		if len(out) >= 3 {
			faces[fi].poly = windingDedupe(out)
		}
	}
}

// tjEps is the on-edge tolerance for T-junction insertion.
const tjEps = 0.01

// pointOnSegment reports whether v lies on the closed segment [a,b] within
// tjEps (used to find vertices that split an edge).
func pointOnSegment(v, a, b vec3) bool {
	ab := v3Sub(b, a)
	av := v3Sub(v, a)
	len2 := v3Dot(ab, ab)
	if len2 < 1e-9 {
		return false
	}
	// Distance from the line: |cross(ab, av)| / |ab|.
	cr := v3Cross(ab, av)
	if v3Dot(cr, cr)/len2 > tjEps*tjEps {
		return false
	}
	// Projection in [0, len2]: strictly interior of the edge.
	proj := v3Dot(av, ab)
	if proj < tjEps || proj > len2-tjEps {
		return false
	}
	return true
}

// windingDedupe removes adjacent exact-duplicate points only (T-junction
// vertices are collinear by design and must be preserved).
func windingDedupe(w winding) winding {
	if len(w) < 3 {
		return w
	}
	out := make(winding, 0, len(w))
	for _, p := range w {
		if len(out) > 0 && v3Dist(out[len(out)-1], p) < 1e-4 {
			continue
		}
		out = append(out, p)
	}
	// Close the loop.
	if len(out) > 2 && v3Dist(out[0], out[len(out)-1]) < 1e-4 {
		out = out[:len(out)-1]
	}
	return out
}

// v3Dist returns the Euclidean distance between two points.
func v3Dist(a, b vec3) float64 {
	d := v3Sub(a, b)
	return math.Sqrt(v3Dot(d, d))
}
