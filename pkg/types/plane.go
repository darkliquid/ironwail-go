// =============================================================================
// 3D Infinite Plane (Plane)
// =============================================================================
package types

import "math"

// Plane type constants matching Quake's plane classification
const (
	PlaneX    int32 = 0 // Axial X plane (normal is +X or -X)
	PlaneY    int32 = 1 // Axial Y plane (normal is +Y or -Y)
	PlaneZ    int32 = 2 // Axial Z plane (normal is +Z or -Z)
	PlaneAnyX int32 = 3 // Non-axial plane snapped nearest to X
	PlaneAnyY int32 = 4 // Non-axial plane snapped nearest to Y
	PlaneAnyZ int32 = 5 // Non-axial plane snapped nearest to Z
)

// Plane represents an infinite 2D plane embedded in 3D space defined by the
// equation: Normal · P - Dist = 0.
type Plane struct {
	Normal Vec3
	Dist   float32
	Type   int32 // PlaneX .. PlaneAnyZ
}

// ClassifyPlaneType determines whether a normal is axial (X, Y, Z) or non-axial.
func ClassifyPlaneType(n Vec3) int32 {
	if n.X == 1.0 || n.X == -1.0 {
		return PlaneX
	}
	if n.Y == 1.0 || n.Y == -1.0 {
		return PlaneY
	}
	if n.Z == 1.0 || n.Z == -1.0 {
		return PlaneZ
	}

	ax := math.Abs(float64(n.X))
	ay := math.Abs(float64(n.Y))
	az := math.Abs(float64(n.Z))

	if ax >= ay && ax >= az {
		return PlaneAnyX
	}
	if ay >= ax && ay >= az {
		return PlaneAnyY
	}
	return PlaneAnyZ
}

// NewPlane constructs a normalized Plane and classifies its orientation type.
func NewPlane(normal Vec3, dist float32) Plane {
	norm := normal.Normalize()
	return Plane{
		Normal: norm,
		Dist:   dist,
		Type:   ClassifyPlaneType(norm),
	}
}

// Plane32FromPoints constructs a Plane passing through three counter-clockwise vertices.
func Plane32FromPoints(a, b, c Vec3) Plane {
	ab := b.Sub(a)
	ac := c.Sub(a)
	norm := ab.Cross(ac).Normalize()
	dist := norm.Dot(a)
	return NewPlane(norm, dist)
}

// PlaneFromPoints32 is an alias for Plane32FromPoints.
func PlaneFromPoints32(a, b, c Vec3) Plane {
	return Plane32FromPoints(a, b, c)
}

// DistanceToPoint returns the signed Euclidean distance from the plane to point p.
// Positive distance indicates p is in front (in direction of normal); negative is behind.
func (pl Plane) DistanceToPoint(p Vec3) float32 {
	return pl.Normal.Dot(p) - pl.Dist
}

// PointOnSide classifies the point relative to the plane given a thickness epsilon:
//
//	+1: Front (positive half-space)
//	-1: Back (negative half-space)
//	 0: On plane (within ±epsilon)
func (pl Plane) PointOnSide(p Vec3, epsilon float32) int {
	d := pl.DistanceToPoint(p)
	if d > epsilon {
		return 1
	}
	if d < -epsilon {
		return -1
	}
	return 0
}

// Project returns the orthogonal projection of point p onto the plane surface.
func (pl Plane) Project(p Vec3) Vec3 {
	d := pl.DistanceToPoint(p)
	return p.Sub(pl.Normal.Scale(d))
}

// Reflect calculates the reflected velocity vector when bouncing off the plane
// with an optional elasticity / overbounce coefficient (1.0 = perfect elastic bounce).
func (pl Plane) Reflect(v Vec3, overbounce float32) Vec3 {
	proj := v.Dot(pl.Normal)
	return v.Sub(pl.Normal.Scale(proj * (1.0 + overbounce)))
}

// Normalize ensures the plane normal is unit length.
func (pl Plane) Normalize() Plane {
	l := pl.Normal.Len()
	if l > 0 && l != 1.0 {
		inv := 1.0 / l
		return Plane{
			Normal: pl.Normal.Scale(inv),
			Dist:   pl.Dist * inv,
			Type:   pl.Type,
		}
	}
	return pl
}

// =============================================================================
// Double-Precision Plane (Plane64)
// =============================================================================

// Plane64 represents an infinite 2D plane in double precision (Normal · P - Dist = 0).
type Plane64 struct {
	Normal Vec3d
	Dist   float64
	Type   int32
}

// ClassifyPlaneType64 determines whether a 64-bit normal is axial or non-axial.
func ClassifyPlaneType64(n Vec3d) int32 {
	if n.X == 1.0 || n.X == -1.0 {
		return PlaneX
	}
	if n.Y == 1.0 || n.Y == -1.0 {
		return PlaneY
	}
	if n.Z == 1.0 || n.Z == -1.0 {
		return PlaneZ
	}

	ax := math.Abs(n.X)
	ay := math.Abs(n.Y)
	az := math.Abs(n.Z)

	if ax >= ay && ax >= az {
		return PlaneAnyX
	}
	if ay >= ax && ay >= az {
		return PlaneAnyY
	}
	return PlaneAnyZ
}

// NewPlane64 constructs a normalized Plane64 and classifies its orientation type.
func NewPlane64(normal Vec3d, dist float64) Plane64 {
	norm := normal.Normalize()
	return Plane64{
		Normal: norm,
		Dist:   dist,
		Type:   ClassifyPlaneType64(norm),
	}
}

// PlaneFromPoints constructs a Plane64 from three non-collinear points:
// normal = normalize(cross(p0-p1, p2-p1)) and dist = dot(p1, normal).
// It returns the resulting plane and the original (unnormalized cross-product) length.
// If the points are degenerate/collinear, length is 0 and an empty Plane64 is returned.
func PlaneFromPoints(p0, p1, p2 Vec3d) (Plane64, float64) {
	ab := p0.Sub(p1)
	cb := p2.Sub(p1)
	cr := ab.Cross(cb)
	length := cr.Len()
	if !(length >= 1e-30) || math.IsNaN(length) {
		return Plane64{}, 0
	}
	normal := cr.Scale(1.0 / length)
	return Plane64{
		Normal: normal,
		Dist:   p1.Dot(normal),
		Type:   ClassifyPlaneType64(normal),
	}, length
}

// Plane64FromPoints is an alias for PlaneFromPoints.
func Plane64FromPoints(p0, p1, p2 Vec3d) (Plane64, float64) {
	return PlaneFromPoints(p0, p1, p2)
}

// PlaneEqual reports whether two Plane64 values match within epsilon tolerance.
func PlaneEqual(a, b Plane64, eps float64) bool {
	if math.Abs(a.Normal.X-b.Normal.X) > eps ||
		math.Abs(a.Normal.Y-b.Normal.Y) > eps ||
		math.Abs(a.Normal.Z-b.Normal.Z) > eps {
		return false
	}
	return math.Abs(a.Dist-b.Dist) <= eps
}

// DistToPoint returns the signed Euclidean distance from the plane to point pt.
// Positive distance indicates pt is in front; negative indicates behind.
func (p Plane64) DistToPoint(pt Vec3d) float64 {
	return p.Normal.Dot(pt) - p.Dist
}

// DistanceToPoint returns the signed Euclidean distance from the plane to point pt.
// Alias for DistToPoint.
func (p Plane64) DistanceToPoint(pt Vec3d) float64 {
	return p.DistToPoint(pt)
}

// PointOnSide classifies the point relative to the plane given a thickness epsilon:
//
//	+1: Front (positive half-space)
//	-1: Back (negative half-space)
//	 0: On plane (within ±epsilon)
func (p Plane64) PointOnSide(pt Vec3d, epsilon float64) int {
	d := p.DistToPoint(pt)
	if d > epsilon {
		return 1
	}
	if d < -epsilon {
		return -1
	}
	return 0
}
