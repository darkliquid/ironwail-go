// math.go defines vector math functions and physics epsilon constants for server types.
package types

import "math"

const (
	MoveEpsilon             = float32(0.01)
	StopEpsilon             = float32(0.1)
	OneEpsilon              = float32(0.001)
	ViewHeight              = float32(22)
	DefaultSoundVolume      = 255
	DefaultSoundAttenuation = 1.0
)

func VecAdd(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

func VecSub(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

func VecScale(v [3]float32, s float32) [3]float32 {
	return [3]float32{v[0] * s, v[1] * s, v[2] * s}
}

func VecDot(a, b [3]float32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

func VecLen(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

func VecNormalize(v *[3]float32) float32 {
	length := VecLen(*v)
	if length != 0 {
		ilength := 1.0 / length
		v[0] *= ilength
		v[1] *= ilength
		v[2] *= ilength
	}
	return length
}

func VecCopy(src [3]float32, dst *[3]float32) {
	*dst = src
}

func VecCross(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func AngleMod(a float32) float32 {
	return float32(float64(int(a*(65536.0/360.0))&65535) * (360.0 / 65536.0))
}

// AngleVectors computes forward/right/up orthonormal vectors from Euler angles.
func AngleVectors(angles [3]float32, forward, right, up *[3]float32) {
	angle := float64(angles[0]) * (math.Pi * 2 / 360)
	sp := float32(math.Sin(angle))
	cp := float32(math.Cos(angle))

	angle = float64(angles[1]) * (math.Pi * 2 / 360)
	sy := float32(math.Sin(angle))
	cy := float32(math.Cos(angle))

	angle = float64(angles[2]) * (math.Pi * 2 / 360)
	sr := float32(math.Sin(angle))
	cr := float32(math.Cos(angle))

	forward[0] = cp * cy
	forward[1] = cp * sy
	forward[2] = -sp

	right[0] = -sr*sp*cy + -cr*-sy
	right[1] = -sr*sp*sy + -cr*cy
	right[2] = -sr * cp

	up[0] = cr*sp*cy + -sr*-sy
	up[1] = cr*sp*sy + -sr*cy
	up[2] = cr * cp
}

// WalkMoveNeedsUnstick reports whether a forward move was obstructed by a low
// step (only X/Y drift while Z stayed put), meaning the attempted stair-step
// should be retried from the original position. Mirrors C Ironwail
// SV_WalkMove's unstick heuristic.
func WalkMoveNeedsUnstick(oldOrg, newOrg [3]float32) bool {
	return math.Abs(float64(oldOrg[0]-newOrg[0])) < float64(DistEpsilon) &&
		math.Abs(float64(oldOrg[1]-newOrg[1])) < float64(DistEpsilon)
}
