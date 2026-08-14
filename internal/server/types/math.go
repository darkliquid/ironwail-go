// math.go defines vector math functions and physics epsilon constants for server types.
package types

import (
	"math"

	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

const (
	MoveEpsilon             = float32(0.01)
	StopEpsilon             = float32(0.1)
	OneEpsilon              = float32(0.001)
	ViewHeight              = float32(22)
	DefaultSoundVolume      = 255
	DefaultSoundAttenuation = 1.0
)

func VecAdd(a, b qtypes.Vec3) qtypes.Vec3 {
	return a.Add(b)
}

func VecSub(a, b qtypes.Vec3) qtypes.Vec3 {
	return a.Sub(b)
}

func VecScale(v qtypes.Vec3, s float32) qtypes.Vec3 {
	return v.Scale(s)
}

func VecDot(a, b qtypes.Vec3) float32 {
	return a.Dot(b)
}

func VecLen(v qtypes.Vec3) float32 {
	return v.Len()
}

func VecNormalize(v *qtypes.Vec3) float32 {
	length := v.Len()
	if length != 0 {
		*v = v.Normalize()
	}
	return length
}

func VecCopy(src qtypes.Vec3, dst *qtypes.Vec3) {
	*dst = src
}

func VecCross(a, b qtypes.Vec3) qtypes.Vec3 {
	return a.Cross(b)
}

func AngleMod(a float32) float32 {
	return qtypes.AngleMod(a)
}

// AngleVectors computes forward/right/up orthonormal vectors from Euler angles.
func AngleVectors(angles qtypes.Vec3, forward, right, up *qtypes.Vec3) {
	f, r, u := qtypes.AngleVectors(angles)
	if forward != nil {
		*forward = f
	}
	if right != nil {
		*right = r
	}
	if up != nil {
		*up = u
	}
}

// WalkMoveNeedsUnstick reports whether a forward move was obstructed by a low
// step (only X/Y drift while Z stayed put), meaning the attempted stair-step
// should be retried from the original position. Mirrors C Ironwail
// SV_WalkMove's unstick heuristic.
func WalkMoveNeedsUnstick(oldOrg, newOrg qtypes.Vec3) bool {
	return math.Abs(float64(oldOrg.X-newOrg.X)) < float64(DistEpsilon) &&
		math.Abs(float64(oldOrg.Y-newOrg.Y)) < float64(DistEpsilon)
}
