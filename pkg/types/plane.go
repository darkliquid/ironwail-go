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

// PlaneFromPoints constructs a Plane passing through three counter-clockwise vertices.
func PlaneFromPoints(a, b, c Vec3) Plane {
	ab := b.Sub(a)
	ac := c.Sub(a)
	norm := ab.Cross(ac).Normalize()
	dist := norm.Dot(a)
	return NewPlane(norm, dist)
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
