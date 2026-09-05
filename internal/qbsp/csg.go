package qbsp

import (
)

// halfspace binds a plane-table index to an orientation: front=true means
// the region satisfies dot(n, x) >= d; false means dot(n, x) <= d.
type halfspace struct {
	plane int
	front bool
}

// region is a convex cell of the plane arrangement: the intersection of a
// set of oriented halfspaces. All facets derive from the halfspaces.
type region struct {
	hs []halfspace
}

// cell is a region with a resolved BSP leaf content.
type cell struct {
	region
	content int32
}

func (r *region) hasPlane(pi int) bool {
	for _, h := range r.hs {
		if h.plane == pi {
			return true
		}
	}
	return false
}

// oriented returns the plane of hs with its orientation sign applied
// (front = +normal equation dot(n,x) >= d).
func oriented(h halfspace, planes []plane) plane {
	p := planes[h.plane]
	if h.front {
		return p
	}
	return plane{Normal: v3(-p.Normal[0], -p.Normal[1], -p.Normal[2]), Dist: -p.Dist}
}

// facets returns the facet polygons of the region clipped to the given
// bounding box. A facet lies on one bounding halfspace and is the
// intersection of the box with all other halfspaces.
func (r *region) facets(planes []plane, bounds [2]vec3) []winding {
	out := make([]winding, 0, len(r.hs))
	for _, h := range r.hs {
		w := windingFromBoxPlane(planes[h.plane], bounds[0], bounds[1])
		if w == nil {
			continue
		}
		// Clip by every halfspace (including h itself, which keeps the
		// facet since its points are on the plane).
		for _, other := range r.hs {
			op := oriented(other, planes)
			clipped, ok := clipWinding(w, op)
			if !ok {
				w = nil
				break
			}
			w = clipped
		}
		if w == nil {
			continue
		}
		w = windingRemoveColinear(w)
		if len(w) >= 3 {
			out = append(out, w)
		}
	}
	return out
}

// empty reports whether the region's convex hull has no volume, by
// checking whether every facet is degenerate. A non-empty convex region
// always has at least one non-degenerate facet.
func (r *region) empty(planes []plane, bounds [2]vec3) bool {
	for _, w := range r.facets(planes, bounds) {
		if len(w) >= 3 {
			return false
		}
	}
	return true
}

// contains tests a point against every halfspace (with a small tolerance).
func (r *region) contains(p vec3, planes []plane, tol float64) bool {
	for _, h := range r.hs {
		op := oriented(h, planes)
		d := v3Dot(op.Normal, p) - op.Dist
		if d < -tol {
			return false
		}
	}
	return true
}

// signature returns a canonical per-plane orientation key for the region.
// Every region of a full arrangement contains every plane exactly once, so
// the key is a bitstring over the sorted plane list.
func (r *region) signature(numPlanes int) string {
	bits := make([]byte, (numPlanes+7)/8)
	for _, h := range r.hs {
		if h.front {
			bits[h.plane/8] |= 1 << uint(h.plane%8)
		}
	}
	return string(bits)
}

// mirrorNbr returns the halfspace set of the region across the facet on
// plane pi (flipping the orientation), or nil if this region does not
// bound on pi.
func mirrorNbr(r *region, pi int) []halfspace {
	out := make([]halfspace, 0, len(r.hs))
	found := false
	for _, h := range r.hs {
		if h.plane == pi {
			out = append(out, halfspace{plane: pi, front: !h.front})
			found = true
		} else {
			out = append(out, h)
		}
	}
	if !found {
		return nil
	}
	return out
}

// arrangement is the full plane arrangement of a set of planes within a
// bounding box: every non-empty intersection of oriented halfspaces.
type arrangement struct {
	planes []plane
	bounds [2]vec3
	cells  []*cell
	// bySig maps a signature string to the cell index.
	bySig map[string]int
}

// buildArrangement splits the bounded box by every plane, producing one
// cell per non-empty sign combination. Order of plane processing is a
// heuristic for cell count; no plane ordering changes the result. The
// caller must append the six box planes as the LAST entries of the plane
// table (boxPlanes of the bounds), which the root cell references.
func buildArrangement(planes []plane, bounds [2]vec3) *arrangement {
	a := &arrangement{planes: planes, bounds: bounds}
	np := len(planes)
	root := &cell{}
	a.cells = []*cell{root}
	// Root region = the box interior: back of the +maxs planes (x<=maxs)
	// and front of the +mins planes (x>=mins), per the normalized boxPlanes.
	for i := 0; i < 6; i++ {
		root.hs = append(root.hs, halfspace{plane: np - 6 + i, front: i%2 == 1})
	}

	for pi := 0; pi < np-6; pi++ {
		var next []*cell
		for _, c := range a.cells {
			if c.hasPlane(pi) {
				next = append(next, c)
				continue
			}
			front := &cell{region: region{hs: append(append([]halfspace{}, c.hs...), halfspace{plane: pi, front: true})}}
			back := &cell{region: region{hs: append(append([]halfspace{}, c.hs...), halfspace{plane: pi, front: false})}}
			if !front.empty(planes, bounds) {
				next = append(next, front)
			}
			if !back.empty(planes, bounds) {
				next = append(next, back)
			}
		}
		a.cells = next
	}

	a.bySig = make(map[string]int, len(a.cells))
	for i, c := range a.cells {
		a.bySig[c.signature(np)] = i
	}
	return a
}

// neighborPlane returns the cell index adjacent to cell ci across its facet
// on plane pi, or -1. In a full arrangement the neighbor is the region with
// pi's orientation flipped (and is unique).
func (a *arrangement) neighborPlane(ci, pi int) int {
	if ci < 0 || ci >= len(a.cells) {
		return -1
	}
	m := mirrorNbr(&a.cells[ci].region, pi)
	if m == nil {
		return -1
	}
	r := &region{hs: m}
	idx, ok := a.bySig[r.signature(len(a.planes))]
	if !ok {
		return -1
	}
	return idx
}

// cellAt returns the cell containing the point, or -1.
func (a *arrangement) cellAt(p vec3) int {
	for i, c := range a.cells {
		if c.contains(p, a.planes, 0.001) {
			return i
		}
	}
	return -1
}

// cellBounds computes the AABB of a cell from its facets.
func (a *arrangement) cellBounds(ci int) (vec3, vec3) {
	var mins, maxs vec3
	first := true
	for _, w := range a.cells[ci].facets(a.planes, a.bounds) {
		m, x := windingBounds(w)
		if first {
			mins, maxs = m, x
			first = false
			continue
		}
		for i := 0; i < 3; i++ {
			if m[i] < mins[i] {
				mins[i] = m[i]
			}
			if x[i] > maxs[i] {
				maxs[i] = x[i]
			}
		}
	}
	if first {
		return a.bounds[0], a.bounds[1]
	}
	return mins, maxs
}

// cellCenter is the centroid of the facet vertex soup (good enough as an
// interior probe for convex cells).
func (a *arrangement) cellCenter(ci int) vec3 {
	// The average of the facet vertices lies strictly inside a convex cell,
	// unlike the AABB midpoint which can fall outside thin cells near brush
	// boundaries (misclassifying wall cells as empty). This matters for
	// content assignment and leak detection.
	var sum vec3
	count := 0
	for _, w := range a.cells[ci].facets(a.planes, a.bounds) {
		for _, p := range w {
			sum[0] += p[0]
			sum[1] += p[1]
			sum[2] += p[2]
			count++
		}
	}
	if count == 0 {
		mins, maxs := a.cellBounds(ci)
		return vec3{(mins[0] + maxs[0]) / 2, (mins[1] + maxs[1]) / 2, (mins[2] + maxs[2]) / 2}
	}
	return vec3{sum[0] / float64(count), sum[1] / float64(count), sum[2] / float64(count)}
}
