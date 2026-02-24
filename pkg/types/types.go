package types

import "math"

// Vec3 represents a 3D vector with X, Y, Z components.
type Vec3 struct {
	X float32
	Y float32
	Z float32
}

// Vec3Add adds two vectors component-wise.
func Vec3Add(a, b Vec3) Vec3 {
	return Vec3{
		X: a.X + b.X,
		Y: a.Y + b.Y,
		Z: a.Z + b.Z,
	}
}

// Vec3Sub subtracts two vectors component-wise.
func Vec3Sub(a, b Vec3) Vec3 {
	return Vec3{
		X: a.X - b.X,
		Y: a.Y - b.Y,
		Z: a.Z - b.Z,
	}
}

// Vec3Scale scales a vector by a scalar.
func Vec3Scale(v Vec3, s float32) Vec3 {
	return Vec3{
		X: v.X * s,
		Y: v.Y * s,
		Z: v.Z * s,
	}
}

// Vec3Dot returns the dot product of two vectors.
func Vec3Dot(a, b Vec3) float32 {
	return a.X*b.X + a.Y*b.Y + a.Z*b.Z
}

// Vec3Cross returns the cross product of two vectors.
func Vec3Cross(a, b Vec3) Vec3 {
	return Vec3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

// Vec3Len returns the length (magnitude) of a vector.
func Vec3Len(v Vec3) float32 {
	return float32(math.Sqrt(float64(v.X*v.X + v.Y*v.Y + v.Z*v.Z)))
}

// Vec3Normalize normalizes a vector to unit length.
func Vec3Normalize(v Vec3) Vec3 {
	length := Vec3Len(v)
	if length > 0 {
		return Vec3Scale(v, 1.0/length)
	}
	return v
}
