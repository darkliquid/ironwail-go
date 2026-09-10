package qbsp

import (
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// vec3 is a double-precision 3D vector used throughout the compiler.
// float64 matches the ericw-tools qvec3d/parser precision so plane and
// texture-vector calculations survive float32 conversion at BSP write time
// without collapsing distinct geometry.
type vec3 = types.Vec3d

func v3(x, y, z float64) vec3 { return types.Vec3d{X: x, Y: y, Z: z} }

func v3Sub(a, b vec3) vec3 { return a.Sub(b) }

func v3Dot(a, b vec3) float64 { return a.Dot(b) }

func v3Cross(a, b vec3) vec3 { return a.Cross(b) }

func v3Length(v vec3) float64 { return v.Len() }

// v3Normalize normalises v and returns its original length. A zero-length
// vector returns itself with length 0, mirroring ericw's qv::normalize
// used for the brush-plane degenerate check.
func v3Normalize(v vec3) (vec3, float64) {
	length := v.Len()
	if length < 1e-30 {
		return v, 0
	}
	return v.Scale(1.0 / length), length
}

// getAxis returns the x (0), y (1), or z (2) component of v.
func getAxis(v vec3, i int) float64 {
	switch i {
	case 0:
		return v.X
	case 1:
		return v.Y
	default:
		return v.Z
	}
}

// setAxis sets the x (0), y (1), or z (2) component of v.
func setAxis(v *vec3, i int, val float64) {
	switch i {
	case 0:
		v.X = val
	case 1:
		v.Y = val
	default:
		v.Z = val
	}
}

// plane is a Quake plane: all points x with dot(normal, x) == dist. The
// dist is stored sign-normalised exactly like the BSP plane format.
type plane = types.Plane64

// planeFromPoints derives a plane from three non-collinear points using the
// exact ericw convention: normal = normalize(cross(p0-p1, p2-p1)) and
// dist = dot(p1, normal). The returned length can be zero for degenerate
// faces.
func planeFromPoints(p0, p1, p2 vec3) (plane, float64) {
	return types.PlaneFromPoints(p0, p1, p2)
}
