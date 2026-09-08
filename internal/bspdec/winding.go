package bspdec

import (
	"math"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// onEpsilon matches ericw's decompile-side epsilon; generous because BSP
// float32 plane dists drift relative to the float64 brush math.
const onEpsilon = 0.1

// baseWindingExtent is the half-size of the initial plane quad. Quake maps
// fit in +/-32768; double it for headroom (ericw uses a similar overshoot).
const baseWindingExtent = 65536

// Winding is a convex polygon. Windings are immutable: Clip returns new
// storage (or the receiver when nothing was clipped).
type Winding struct {
	Points []mapfile.Vec3
}

// vc constructs a mapfile vector (alias of types.Vec3d).
func vc(x, y, z float64) mapfile.Vec3 { return mapfile.Vec3{X: x, Y: y, Z: z} }

func v3Add(a, b mapfile.Vec3) mapfile.Vec3 { return a.Add(b) }
func v3Sub(a, b mapfile.Vec3) mapfile.Vec3 { return a.Sub(b) }
func v3Dot(a, b mapfile.Vec3) float64      { return a.Dot(b) }
func v3Scale(a mapfile.Vec3, s float64) mapfile.Vec3 {
	return mapfile.Vec3{X: a.X * s, Y: a.Y * s, Z: a.Z * s}
}
func v3Cross(a, b mapfile.Vec3) mapfile.Vec3 {
	return mapfile.Vec3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}
func v3Len(a mapfile.Vec3) float64 { return a.Len() }
func v3Normalize(a mapfile.Vec3) mapfile.Vec3 {
	l := v3Len(a)
	if l == 0 {
		return mapfile.Vec3{}
	}
	return v3Scale(a, 1/l)
}

func negatePlane(p mapfile.Plane) mapfile.Plane {
	return mapfile.Plane{Normal: v3Scale(p.Normal, -1), Dist: -p.Dist}
}

// BaseWinding returns a huge quad on p.
//
// Where in C: BaseWindingForPlane in ericw-tools common/polylib.cc (id
// winding.cc).
func BaseWinding(p mapfile.Plane) *Winding {
	// dominant axis of the normal chooses the "up" reference vector
	ax, ay, az := math.Abs(p.Normal.X), math.Abs(p.Normal.Y), math.Abs(p.Normal.Z)
	x := 0
	if ay >= ax && ay >= az {
		x = 1
	}
	if az >= ax && az >= ay {
		x = 2
	}
	up := mapfile.Vec3{}
	if x == 0 || x == 1 {
		up.Z = 1
	} else {
		up.X = 1
	}
	// project up onto the plane and normalize
	v := v3Dot(up, p.Normal)
	up = v3Sub(up, v3Scale(p.Normal, v))
	up = v3Normalize(up)
	org := v3Scale(p.Normal, p.Dist)
	right := v3Cross(up, p.Normal)
	up = v3Scale(up, baseWindingExtent)
	right = v3Scale(right, baseWindingExtent)
	return &Winding{Points: []mapfile.Vec3{
		v3Add(v3Sub(org, right), up),
		v3Add(v3Add(org, right), up),
		v3Sub(v3Add(org, right), up),
		v3Sub(v3Sub(org, right), up),
	}}
}

const (
	sideFront = iota
	sideBack
	sideOn
)

// Clip returns the part of w in front of p (dot(n,x) >= dist), or nil when
// nothing survives. On-plane points are kept.
//
// Where in C: ClipWindingEpsilon in ericw-tools common/polylib.cc.
func (w *Winding) Clip(p mapfile.Plane) *Winding {
	n := len(w.Points)
	dists := make([]float64, n+1)
	sides := make([]int, n+1)
	counts := [3]int{}
	for i, pt := range w.Points {
		d := v3Dot(pt, p.Normal) - p.Dist
		dists[i] = d
		switch {
		case d > onEpsilon:
			sides[i] = sideFront
			counts[sideFront]++
		case d < -onEpsilon:
			sides[i] = sideBack
			counts[sideBack]++
		default:
			sides[i] = sideOn
			counts[sideOn]++
		}
	}
	sides[n] = sides[0]
	dists[n] = dists[0]
	if counts[sideFront] == 0 {
		return nil
	}
	if counts[sideBack] == 0 {
		return w
	}
	out := &Winding{Points: make([]mapfile.Vec3, 0, n+4)}
	for i, pt := range w.Points {
		if sides[i] == sideOn {
			out.Points = append(out.Points, pt)
			continue
		}
		if sides[i] == sideFront {
			out.Points = append(out.Points, pt)
		}
		if sides[i+1] == sideOn || sides[i+1] == sides[i] {
			continue
		}
		// edge crosses the plane: interpolate the split point
		j := (i + 1) % n
		d := dists[i] / (dists[i] - dists[j])
		mid := v3Add(pt, v3Scale(v3Sub(w.Points[j], pt), d))
		out.Points = append(out.Points, mid)
	}
	if len(out.Points) < 3 {
		return nil
	}
	return out
}

// Area returns the polygon area (always non-negative).
//
// Where in C: WindingArea in id winding.cc.
func (w *Winding) Area() float64 {
	var total mapfile.Vec3
	for i := 2; i < len(w.Points); i++ {
		total = v3Add(total, v3Cross(v3Sub(w.Points[i-1], w.Points[0]), v3Sub(w.Points[i], w.Points[0])))
	}
	return v3Len(total) / 2
}

// canonicalizeWinding orients w so that re-deriving the plane from its
// points with the ericw convention (normal = normalize(cross(p0-p1, p2-p1)))
// yields the side's outward normal. Winding orientation is not relied on
// during clipping, but the writer's emitted points must re-derive outward
// planes, or recompiling the .map flips faces inside-out.
func canonicalizeWinding(w *Winding, p mapfile.Plane) {
	if len(w.Points) < 3 {
		return
	}
	n, length := mapfile.PlaneFromPoints(w.Points[0], w.Points[1], w.Points[2])
	if length < 0.01 {
		return
	}
	if v3Dot(n.Normal, p.Normal) < 0 {
		for i, j := 0, len(w.Points)-1; i < j; i, j = i+1, j-1 {
			w.Points[i], w.Points[j] = w.Points[j], w.Points[i]
		}
	}
}

// Centroid returns the average of the winding's points.
func (w *Winding) Centroid() mapfile.Vec3 {
	var c mapfile.Vec3
	for _, p := range w.Points {
		c = v3Add(c, p)
	}
	if len(w.Points) > 0 {
		c = v3Scale(c, 1/float64(len(w.Points)))
	}
	return c
}