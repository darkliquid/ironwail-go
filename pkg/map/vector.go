package mapfile

import (
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// Double-precision vector helpers copied from internal/qbsp/vector.go; the
// parser needs them and qbsp keeps its own copies for its internal math.

type vec3 = types.Vec3d

type plane = types.Plane64

func v3(x, y, z float64) vec3 { return types.Vec3d{X: x, Y: y, Z: z} }

func v3Dot(a, b vec3) float64 { return a.Dot(b) }

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

// planeFromPoints derives a plane from three non-collinear points using the
// exact ericw convention: normal = normalize(cross(p0-p1, p2-p1)) and
// dist = dot(p1, normal). The returned length can be zero for degenerate
// faces.
func planeFromPoints(p0, p1, p2 vec3) (plane, float64) {
	return types.PlaneFromPoints(p0, p1, p2)
}

// PlaneFromPoints is the exported form used by the decompiler.
func PlaneFromPoints(p0, p1, p2 Vec3) (Plane, float64) {
	return types.PlaneFromPoints(p0, p1, p2)
}

// planeEqual reports whether two planes are the same within the ericw
// DIST_EPSILON (0.0001).
func planeEqual(a, b plane) bool {
	return types.PlaneEqual(a, b, 0.0001)
}

// planeRoundNearInt rounds values within ZERO_EPSILON (0.0001) of an integer
// to that integer, mirroring the DarkPlaces-workaround rounding ericw
// applies to computed texture vectors.
func planeRoundNearInt(v float64) float64 {
	r := math.Round(v)
	if math.Abs(v-r) < 0.0001 {
		return r
	}
	return v
}