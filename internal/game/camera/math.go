// math.go implements the pure camera vector-math helpers extracted from the
// game root (game_camera*.go). These have no dependencies on the Game struct,
// so they are unit-testable in isolation.
package camera

import (
	"math"

	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// ChaseTraceFunc traces a segment and returns the clipped end point, used by
// chase-camera placement against world geometry.
type ChaseTraceFunc func(start, end [3]float32) [3]float32

// ChaseCrosshairTraceDistance is the maximum distance used to project the
// chase-camera crosshair along the view direction.
const ChaseCrosshairTraceDistance = float32(1 << 20)

// AngleVectors computes the forward/right/up orthonormal vectors from Euler
// angles (wrapping pkg/types.AngleVectors with [3]float32 conversions).
func AngleVectors(angles [3]float32) (forward, right, up [3]float32) {
	forwardVec, rightVec, upVec := qtypes.AngleVectors(qtypes.Vec3{
		X: angles[0],
		Y: angles[1],
		Z: angles[2],
	})
	return [3]float32{forwardVec.X, forwardVec.Y, forwardVec.Z},
		[3]float32{rightVec.X, rightVec.Y, rightVec.Z},
		[3]float32{upVec.X, upVec.Y, upVec.Z}
}

// VectorAngles converts a forward direction vector into (pitch, yaw, 0) Euler
// angles, matching the C Quake VectorAngles / anglemod convention.
func VectorAngles(forward [3]float32) [3]float32 {
	var yaw, pitch float32

	if forward[0] == 0 && forward[1] == 0 {
		yaw = 0
		if forward[2] > 0 {
			pitch = -90
		} else {
			pitch = 90
		}
	} else {
		yaw = float32(math.Atan2(float64(forward[1]), float64(forward[0])) * (180.0 / math.Pi))
		if yaw < 0 {
			yaw += 360
		}
		tmp := float32(math.Sqrt(float64(forward[0]*forward[0] + forward[1]*forward[1])))
		pitch = -float32(math.Atan2(float64(forward[2]), float64(tmp)) * (180.0 / math.Pi))
	}

	return [3]float32{pitch, yaw, 0}
}

// ApplyBobToOrigin offsets the view origin along the forward vector by the
// view bob amount (0.4 lateral, full vertical), matching C V_CalcView bob.
func ApplyBobToOrigin(origin, forward [3]float32, bob float32) [3]float32 {
	origin[0] += forward[0] * bob * 0.4
	origin[1] += forward[1] * bob * 0.4
	origin[2] += forward[2] * bob * 0.4
	origin[2] += bob
	return origin
}

// NodeLineOffset nudges the view origin by the BSP node-line bias to avoid
// coplanar trace aliasing (1/32 world unit), matching C.
func NodeLineOffset(origin [3]float32) [3]float32 {
	const bias = 1.0 / 32.0
	origin[0] += bias
	origin[1] += bias
	origin[2] += bias
	return origin
}

// BoundOffsets clamps the view origin near the player origin (14 units lateral,
// 22 below / 30 above), matching C V_CalcView's view bounding.
func BoundOffsets(vieworg, entityOrigin [3]float32) [3]float32 {
	if vieworg[0] < entityOrigin[0]-14 {
		vieworg[0] = entityOrigin[0] - 14
	}
	if vieworg[0] > entityOrigin[0]+14 {
		vieworg[0] = entityOrigin[0] + 14
	}
	if vieworg[1] < entityOrigin[1]-14 {
		vieworg[1] = entityOrigin[1] - 14
	}
	if vieworg[1] > entityOrigin[1]+14 {
		vieworg[1] = entityOrigin[1] + 14
	}
	if vieworg[2] < entityOrigin[2]-22 {
		vieworg[2] = entityOrigin[2] - 22
	}
	if vieworg[2] > entityOrigin[2]+30 {
		vieworg[2] = entityOrigin[2] + 30
	}
	return vieworg
}

// ChaseUpdate positions the chase camera behind/above/right of the target
// origin, tracing both the camera and its crosshair, then aiming the camera at
// the crosshair end point. Returns the camera origin and angles.
func ChaseUpdate(origin, angles [3]float32, chaseBack, chaseUp, chaseRight float32, traceFn ChaseTraceFunc) ([3]float32, [3]float32) {
	forward, right, _ := AngleVectors(angles)

	ideal := origin
	for i := range ideal {
		ideal[i] = origin[i] - forward[i]*chaseBack + right[i]*chaseRight
	}
	// Match Ironwail chase.c: chase_up is world-up offset, not camera-up.
	ideal[2] += chaseUp

	if traceFn != nil {
		ideal = traceFn(origin, ideal)
	}

	crosshair := [3]float32{
		origin[0] + forward[0]*ChaseCrosshairTraceDistance,
		origin[1] + forward[1]*ChaseCrosshairTraceDistance,
		origin[2] + forward[2]*ChaseCrosshairTraceDistance,
	}
	if traceFn != nil {
		crosshair = traceFn(origin, crosshair)
	}

	lookDir := [3]float32{
		crosshair[0] - ideal[0],
		crosshair[1] - ideal[1],
		crosshair[2] - ideal[2],
	}

	cameraAngles := VectorAngles(lookDir)
	if cameraAngles[0] == 90 || cameraAngles[0] == -90 {
		cameraAngles[1] = angles[1]
	}
	return ideal, cameraAngles
}
