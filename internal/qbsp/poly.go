package qbsp

import (
	"math"
)

// winding is an ordered polygon (convex, planar), used for facets, faces,
// and portal math. Vertices are float64.
type winding []vec3

// onPlaneEpsilon is the tolerance for coplanarity when clipping.
const onPlaneEpsilon = 0.001

// planeSide returns +1 when v is on the front of p, -1 behind, 0 on it.
func planeSide(p plane, v vec3) int {
	d := v3Dot(p.Normal, v) - p.Dist
	if d > onPlaneEpsilon {
		return 1
	}
	if d < -onPlaneEpsilon {
		return -1
	}
	return 0
}

// clipWinding returns the subset of w on the front side of p (d >= 0),
// plus whether the result is non-empty. This is the classic Sutherland-
// Hodgman polygon clip against a plane.
func clipWinding(w winding, p plane) (winding, bool) {
	if len(w) == 0 {
		return nil, false
	}
	side := make([]int, len(w))
	fronts := 0
	backs := 0
	for i, v := range w {
		switch planeSide(p, v) {
		case 1:
			side[i] = 1
			fronts++
		case -1:
			side[i] = -1
			backs++
		default:
			side[i] = 0
		}
	}
	if backs == 0 {
		// Nothing strictly behind the plane: keep as-is (points ON the
		// plane must survive — planar seed polygons have fronts==0).
		out := make(winding, len(w))
		copy(out, w)
		return out, true
	}
	if fronts == 0 {
		return nil, false
	}

	out := make(winding, 0, len(w)+4)
	n := len(w)
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		if side[i] >= 0 {
			out = append(out, w[i])
		}
		if side[i]*side[j] < 0 {
			// Edge crosses the plane; intersect.
			t := (p.Dist - p.Normal.Dot(w[i])) / p.Normal.Dot(w[j].Sub(w[i]))
			inter := w[i].Lerp(w[j], t)
			out = append(out, inter)
		}
	}
	return out, len(out) >= 3
}

// windingFromBoxPlane computes the polygon of plane p intersecting the
// axis-aligned box [mins,maxs]. The winding is generated from the box face
// whose outward normal is closest to the plane normal, then clipped by the
// other five box planes, producing up to 7 points.
// windingFromBoxPlane computes the polygon of plane p intersecting the
// axis-aligned box [mins,maxs]. The seed quad lies ON the plane at the
// plane's intercept along its dominant axis, spanning the other two axes;
// clipping by all six box faces trims it to the box. This is the classic
// Quake polylib approach and yields the exact intersection polygon.
func windingFromBoxPlane(p plane, mins, maxs vec3) winding {
	bestAxis := 0
	bestDot := math.Abs(p.Normal.X)
	if math.Abs(p.Normal.Y) > bestDot {
		bestDot = math.Abs(p.Normal.Y)
		bestAxis = 1
	}
	if math.Abs(p.Normal.Z) > bestDot {
		bestDot = math.Abs(p.Normal.Z)
		bestAxis = 2
	}
	if bestDot < 1e-9 {
		return nil
	}
	pn := getAxis(p.Normal, bestAxis)
	coord := p.Dist / pn
	if coord < getAxis(mins, bestAxis) {
		coord = getAxis(mins, bestAxis)
	}
	if coord > getAxis(maxs, bestAxis) {
		coord = getAxis(maxs, bestAxis)
	}

	u := (bestAxis + 1) % 3
	v := (bestAxis + 2) % 3

	// Four corners on the plane-spanning quad; ordered so the area vector
	// points along the plane normal's dominant component.
	var seed winding
	corners := [4][2]int{
		{0, 0}, {0, 1}, {1, 1}, {1, 0},
	}
	minsU, maxsU := getAxis(mins, u), getAxis(maxs, u)
	minsV, maxsV := getAxis(mins, v), getAxis(maxs, v)
	for _, c := range corners {
		var pt vec3
		setAxis(&pt, bestAxis, coord)
		setAxis(&pt, u, minsU+float64(c[0])*(maxsU-minsU))
		if pn > 0 {
			setAxis(&pt, v, minsV+float64(c[1])*(maxsV-minsV))
		} else {
			setAxis(&pt, v, minsV+float64(1-c[1])*(maxsV-minsV))
		}
		seed = append(seed, pt)
	}

	// Trim to the box. boxPlanes returns normalized planes where the box
	// interior is the BACK of the +maxs planes (x<=maxs) and the FRONT of
	// the +mins planes (x>=mins); clipWinding keeps front, so the +maxs
	// planes must be negated.
	box := boxPlanes(mins, maxs)
	for i, bp := range box {
		cp := bp
		if i%2 == 0 {
			cp = plane{Normal: bp.Normal.Neg(), Dist: -bp.Dist}
		}
		clipped, ok := clipWinding(seed, cp)
		if !ok {
			return nil
		}
		seed = clipped
	}
	return seed
}

// boxPlanes returns six planes bounding the box with POSITIVE axial
// normals (the engine's fast-path convention). The box interior is the
// BACK side of the +maxs planes and the FRONT side of the +mins planes:
//
//	x <= maxs  <=>  back  of (1,0,0)@maxs
//	x >= mins  <=>  front of (1,0,0)@mins
//	(and likewise for y, z)
func boxPlanes(mins, maxs vec3) [6]plane {
	return [6]plane{
		{Normal: v3(1, 0, 0), Dist: maxs.X},
		{Normal: v3(1, 0, 0), Dist: mins.X},
		{Normal: v3(0, 1, 0), Dist: maxs.Y},
		{Normal: v3(0, 1, 0), Dist: mins.Y},
		{Normal: v3(0, 0, 1), Dist: maxs.Z},
		{Normal: v3(0, 0, 1), Dist: mins.Z},
	}
}

// windingBounds returns the AABB of a winding.
func windingBounds(w winding) (vec3, vec3) {
	if len(w) == 0 {
		return vec3{}, vec3{}
	}
	mins, maxs := w[0], w[0]
	for _, p := range w[1:] {
		if p.X < mins.X {
			mins.X = p.X
		}
		if p.X > maxs.X {
			maxs.X = p.X
		}
		if p.Y < mins.Y {
			mins.Y = p.Y
		}
		if p.Y > maxs.Y {
			maxs.Y = p.Y
		}
		if p.Z < mins.Z {
			mins.Z = p.Z
		}
		if p.Z > maxs.Z {
			maxs.Z = p.Z
		}
	}
	return mins, maxs
}

// windingRemoveColinear drops collinear points, matching qbsp's
// RemoveColinearPoints which keeps the endpoints of straight runs.
func windingRemoveColinear(w winding) winding {
	if len(w) < 3 {
		return w
	}
	out := make(winding, 0, len(w))
	n := len(w)
	for i := 0; i < n; i++ {
		prev := w[(i-1+n)%n]
		cur := w[i]
		next := w[(i+1)%n]
		v1 := prev.Sub(cur)
		v2 := next.Sub(cur)
		prodLen := v1.Len() * v2.Len()
		if prodLen > 0 && math.Abs(v1.Dot(v2)) >= (1-1e-6)*prodLen {
			continue // collinear
		}
		out = append(out, cur)
	}
	return out
}

// windingOrientTo reverses the winding if its area vector opposes n, so a
// viewer looking along n sees the polygon counter-clockwise.
func windingOrientTo(w winding, n vec3) winding {
	if len(w) < 3 {
		return w
	}
	sum := vec3{}
	for i := 0; i < len(w); i++ {
		a := w[i]
		b := w[(i+1)%len(w)]
		cr := a.Cross(b)
		sum = sum.Add(cr)
	}
	if sum.Dot(n) < 0 {
		// reverse
		out := make(winding, len(w))
		for i := range out {
			out[i] = w[len(w)-1-i]
		}
		return out
	}
	return w
}