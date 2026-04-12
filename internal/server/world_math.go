package server

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/model"
)

func boxOnPlaneSide(mins, maxs [3]float32, plane *model.MPlane) int {
	// Fast axial cases
	if plane.Type < 3 {
		if plane.Dist <= mins[plane.Type] {
			return 1 // Front
		}
		if plane.Dist >= maxs[plane.Type] {
			return 2 // Back
		}
		return 3 // Crossing
	}

	// General case - compute corners based on plane normal signs
	var corners [2][3]float32
	for i := 0; i < 3; i++ {
		if plane.Normal[i] < 0 {
			corners[0][i] = mins[i]
			corners[1][i] = maxs[i]
		} else {
			corners[0][i] = maxs[i]
			corners[1][i] = mins[i]
		}
	}

	// Check front corner
	d1 := plane.Normal[0]*corners[0][0] + plane.Normal[1]*corners[0][1] + plane.Normal[2]*corners[0][2] - plane.Dist
	// Check back corner
	d2 := plane.Normal[0]*corners[1][0] + plane.Normal[1]*corners[1][1] + plane.Normal[2]*corners[1][2] - plane.Dist

	var sides int
	if d1 >= 0 {
		sides = 1
	}
	if d2 < 0 {
		sides |= 2
	}

	return sides
}

// Vec3Len returns the length of a 3D vector.
func Vec3Len(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

// Vec3Normalize normalizes a 3D vector in place.
func Vec3Normalize(v *[3]float32) {
	length := Vec3Len(*v)
	if length > 0 {
		v[0] /= length
		v[1] /= length
		v[2] /= length
	}
}
