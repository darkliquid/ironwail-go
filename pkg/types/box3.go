// =============================================================================
// Axis-Aligned Bounding Box (Box3 / AABB)
// =============================================================================
package types

import "math"

// Box3 represents a 3-dimensional axis-aligned bounding box (AABB) defined
// by its minimum and maximum coordinates.
type Box3 struct {
	Min Vec3
	Max Vec3
}

// NewBox3 creates a Box3 from minimum and maximum bounds.
func NewBox3(min, max Vec3) Box3 {
	return Box3{
		Min: min,
		Max: max,
	}
}

// Box3FromCenterSize constructs a Box3 given a center position and full dimensions.
func Box3FromCenterSize(center, size Vec3) Box3 {
	half := size.Scale(0.5)
	return Box3{
		Min: center.Sub(half),
		Max: center.Add(half),
	}
}

// Center returns the midpoint of the bounding box.
func (b Box3) Center() Vec3 {
	return Vec3{
		X: (b.Min.X + b.Max.X) * 0.5,
		Y: (b.Min.Y + b.Max.Y) * 0.5,
		Z: (b.Min.Z + b.Max.Z) * 0.5,
	}
}

// Size returns the total dimensions (width, depth, height) of the box.
func (b Box3) Size() Vec3 {
	return Vec3{
		X: b.Max.X - b.Min.X,
		Y: b.Max.Y - b.Min.Y,
		Z: b.Max.Z - b.Min.Z,
	}
}

// Extents returns the half-size of the box from its center.
func (b Box3) Extents() Vec3 {
	return b.Size().Scale(0.5)
}

// IsEmpty returns true if any component of Min exceeds Max.
func (b Box3) IsEmpty() bool {
	return b.Min.X > b.Max.X || b.Min.Y > b.Max.Y || b.Min.Z > b.Max.Z
}

// ContainsPoint returns true if the given point is inside or on the boundary of the box.
func (b Box3) ContainsPoint(p Vec3) bool {
	return p.X >= b.Min.X && p.X <= b.Max.X &&
		p.Y >= b.Min.Y && p.Y <= b.Max.Y &&
		p.Z >= b.Min.Z && p.Z <= b.Max.Z
}

// ContainsBox returns true if other is completely enclosed within b.
func (b Box3) ContainsBox(other Box3) bool {
	return other.Min.X >= b.Min.X && other.Max.X <= b.Max.X &&
		other.Min.Y >= b.Min.Y && other.Max.Y <= b.Max.Y &&
		other.Min.Z >= b.Min.Z && other.Max.Z <= b.Max.Z
}

// Intersects returns true if b and other overlap in 3D space.
func (b Box3) Intersects(other Box3) bool {
	if b.Max.X < other.Min.X || b.Min.X > other.Max.X {
		return false
	}
	if b.Max.Y < other.Min.Y || b.Min.Y > other.Max.Y {
		return false
	}
	if b.Max.Z < other.Min.Z || b.Min.Z > other.Max.Z {
		return false
	}
	return true
}

// Expand returns a new Box3 expanded uniformly in all directions by delta.
func (b Box3) Expand(delta float32) Box3 {
	return Box3{
		Min: Vec3{X: b.Min.X - delta, Y: b.Min.Y - delta, Z: b.Min.Z - delta},
		Max: Vec3{X: b.Max.X + delta, Y: b.Max.Y + delta, Z: b.Max.Z + delta},
	}
}

// Union returns the smallest Box3 that encloses both b and other.
func (b Box3) Union(other Box3) Box3 {
	return Box3{
		Min: Vec3{
			X: float32(math.Min(float64(b.Min.X), float64(other.Min.X))),
			Y: float32(math.Min(float64(b.Min.Y), float64(other.Min.Y))),
			Z: float32(math.Min(float64(b.Min.Z), float64(other.Min.Z))),
		},
		Max: Vec3{
			X: float32(math.Max(float64(b.Max.X), float64(other.Max.X))),
			Y: float32(math.Max(float64(b.Max.Y), float64(other.Max.Y))),
			Z: float32(math.Max(float64(b.Max.Z), float64(other.Max.Z))),
		},
	}
}

// UnionPoint expands the box to include the point p.
func (b Box3) UnionPoint(p Vec3) Box3 {
	return Box3{
		Min: Vec3{
			X: float32(math.Min(float64(b.Min.X), float64(p.X))),
			Y: float32(math.Min(float64(b.Min.Y), float64(p.Y))),
			Z: float32(math.Min(float64(b.Min.Z), float64(p.Z))),
		},
		Max: Vec3{
			X: float32(math.Max(float64(b.Max.X), float64(p.X))),
			Y: float32(math.Max(float64(b.Max.Y), float64(p.Y))),
			Z: float32(math.Max(float64(b.Max.Z), float64(p.Z))),
		},
	}
}

// Translate returns the box shifted by offset.
func (b Box3) Translate(offset Vec3) Box3 {
	return Box3{
		Min: b.Min.Add(offset),
		Max: b.Max.Add(offset),
	}
}

// ClosestPoint returns the point inside or on the box boundary closest to p.
func (b Box3) ClosestPoint(p Vec3) Vec3 {
	return Vec3{
		X: Clamp(p.X, b.Min.X, b.Max.X),
		Y: Clamp(p.Y, b.Min.Y, b.Max.Y),
		Z: Clamp(p.Z, b.Min.Z, b.Max.Z),
	}
}

// DistanceToPoint returns the Euclidean distance from p to the box surface (0 if inside).
func (b Box3) DistanceToPoint(p Vec3) float32 {
	closest := b.ClosestPoint(p)
	return p.Distance(closest)
}
