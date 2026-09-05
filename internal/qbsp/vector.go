package qbsp

import "math"

// vec3 is a double-precision 3D vector used throughout the compiler.
// float64 matches the ericw-tools qvec3d/parser precision so plane and
// texture-vector calculations survive float32 conversion at BSP write time
// without collapsing distinct geometry.
type vec3 [3]float64

func v3(x, y, z float64) vec3 { return vec3{x, y, z} }

func v3Sub(a, b vec3) vec3 { return vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }

func v3Dot(a, b vec3) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

func v3Cross(a, b vec3) vec3 {
	return vec3{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func v3Length(v vec3) float64 { return math.Sqrt(v3Dot(v, v)) }

// v3Normalize normalises v and returns its original length. A zero-length
// vector returns itself with length 0, mirroring ericw's qv::normalize
// used for the brush-plane degenerate check.
func v3Normalize(v vec3) (vec3, float64) {
	length := v3Length(v)
	if length < 1e-30 {
		return v, 0
	}
	return vec3{v[0] / length, v[1] / length, v[2] / length}, length
}

// plane is a Quake plane: all points x with dot(normal, x) == dist. The
// dist is stored sign-normalised exactly like the BSP plane format.
type plane struct {
	Normal vec3
	Dist   float64
}

// planeFromPoints derives a plane from three non-collinear points using the
// exact ericw convention: normal = normalize(cross(p0-p1, p2-p1)) and
// dist = dot(p1, normal). The returned length can be zero for degenerate
// faces.
func planeFromPoints(p0, p1, p2 vec3) (plane, float64) {
	ab := v3Sub(p0, p1)
	cb := v3Sub(p2, p1)
	normal, length := v3Normalize(v3Cross(ab, cb))
	return plane{Normal: normal, Dist: v3Dot(p1, normal)}, length
}

// planeEqual reports whether two planes are the same within the ericw
// DIST_EPSILON (0.0001) used for duplicate-plane detection and brush
// pruning.
func planeEqual(a, b plane) bool {
	const eps = 0.0001
	for i := 0; i < 3; i++ {
		if math.Abs(a.Normal[i]-b.Normal[i]) > eps {
			return false
		}
	}
	return math.Abs(a.Dist-b.Dist) <= eps
}

// PlaneRoundNearInt rounds values within ZERO_EPSILON (0.0001) of an
// integer to that integer, mirroring the DarkPlaces-workaround rounding
// ericw applies to computed texture vectors.
func planeRoundNearInt(v float64) float64 {
	r := math.Round(v)
	if math.Abs(v-r) < 0.0001 {
		return r
	}
	return v
}