// math.go implements the pure camera vector-math helpers extracted from the
// game root (game_camera*.go). These have no dependencies on the Game struct,
// so they are unit-testable in isolation.
package camera

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// CVarReader abstracts the cvar lookups the view-calc helpers perform.
// Implemented by *cvar.CVarSystem; defined here so camera stays decoupled.
type CVarReader interface {
	// Get returns the cvar, or nil if unregistered.
	Get(name string) *cvar.CVar
}

// CalcBob returns the view bob offset for the current frame, matching
// C Ironwail V_CalcBob.  The result is in world units and is clamped to
// [-7, 4].
//
// Parameters:
//   - cv:       cvar reader (cl_bobcycle, cl_bobup, cl_bob)
//   - clientTime: cl.time (seconds)
//   - velocity:   XY components of the player's velocity
func CalcBob(cv CVarReader, clientTime float64, velocity qtypes.Vec3) float32 {
	bobcycleCv := cv.Get("cl_bobcycle")
	if bobcycleCv == nil {
		return 0
	}
	bobcycle := float32(bobcycleCv.Float)
	if bobcycle == 0 {
		return 0
	}

	bobupCv := cv.Get("cl_bobup")
	bobCv := cv.Get("cl_bob")
	if bobupCv == nil || bobCv == nil {
		return 0
	}

	// Compute where we are inside the current bob cycle [0, 1).
	cycle := float32(clientTime) - float32(int(clientTime/float64(bobcycle)))*bobcycle
	cycle /= bobcycle

	bobup := float32(bobupCv.Float)
	var radians float32
	if cycle < bobup {
		radians = math.Pi * cycle / bobup
	} else {
		radians = math.Pi + math.Pi*(cycle-bobup)/(1.0-bobup)
	}

	// Horizontal speed scaled by cl_bob.
	speed := math.Sqrt(float64(velocity.X*velocity.X + velocity.Y*velocity.Y))
	bob := float32(speed) * float32(bobCv.Float)
	bob = bob*0.3 + bob*0.7*float32(math.Sin(float64(radians)))

	if bob > 4 {
		bob = 4
	} else if bob < -7 {
		bob = -7
	}
	return bob
}

// CalcRoll returns the camera roll angle (in degrees) caused by lateral
// strafing velocity, matching C Ironwail V_CalcRoll / CalcRoll.
//
// Parameters:
//   - cv:       cvar reader (cl_rollangle, cl_rollspeed)
//   - angles:   player/camera Euler angles (pitch, yaw, roll)
//   - velocity: player velocity
func CalcRoll(cv CVarReader, angles, velocity qtypes.Vec3) float32 {
	rollAngleCv := cv.Get("cl_rollangle")
	rollSpeedCv := cv.Get("cl_rollspeed")
	if rollAngleCv == nil || rollSpeedCv == nil {
		return 0
	}

	_, right, _ := AngleVectors(angles)
	side := velocity.Dot(right)

	sign := float32(1)
	if side < 0 {
		sign = -1
		side = -side
	}

	rollAngle := float32(rollAngleCv.Float)
	rollSpeed := float32(rollSpeedCv.Float)

	if rollSpeed == 0 {
		return 0
	}
	if side < rollSpeed {
		side = side * rollAngle / rollSpeed
	} else {
		side = rollAngle
	}
	return side * sign
}

// AddIdle adds idle sway to camera angles, matching C Ironwail V_AddIdle.
func AddIdle(cv CVarReader, angles qtypes.Vec3, clientTime float64) qtypes.Vec3 {
	cvarV := cv.Get("v_idlescale")
	if cvarV == nil {
		return angles
	}
	idleScale := float32(cvarV.Float)
	if idleScale == 0 {
		return angles
	}

	irollCycle := cv.Get("v_iroll_cycle")
	irollLevel := cv.Get("v_iroll_level")
	ipitchCycle := cv.Get("v_ipitch_cycle")
	ipitchLevel := cv.Get("v_ipitch_level")
	iyawCycle := cv.Get("v_iyaw_cycle")
	iyawLevel := cv.Get("v_iyaw_level")
	if irollCycle == nil || irollLevel == nil || ipitchCycle == nil ||
		ipitchLevel == nil || iyawCycle == nil || iyawLevel == nil {
		return angles
	}

	t := clientTime
	angles.Z += idleScale *
		float32(math.Sin(t*irollCycle.Float)) *
		float32(irollLevel.Float)
	angles.X += idleScale *
		float32(math.Sin(t*ipitchCycle.Float)) *
		float32(ipitchLevel.Float)
	angles.Y += idleScale *
		float32(math.Sin(t*iyawCycle.Float)) *
		float32(iyawLevel.Float)
	return angles
}

// ApplyViewmodelQuakeFudge applies the r_viewmodel_quake origin fudge
// that nudges the weapon origin based on scr_viewsize, matching C Ironwail.
func ApplyViewmodelQuakeFudge(cv CVarReader, origin qtypes.Vec3, scrViewSize float64) qtypes.Vec3 {
	cvarV := cv.Get("r_viewmodel_quake")
	if cvarV == nil || cvarV.Int == 0 {
		return origin
	}
	switch int(scrViewSize) {
	case 110:
		origin.Z += 1
	case 100:
		origin.Z += 2
	case 90:
		origin.Z += 1
	case 80:
		origin.Z += 0.5
	}
	return origin
}

// ChaseTraceFunc traces a segment and returns the clipped end point, used by
// chase-camera placement against world geometry.
type ChaseTraceFunc func(start, end qtypes.Vec3) qtypes.Vec3

// ChaseCrosshairTraceDistance is the maximum distance used to project the
// chase-camera crosshair along the view direction.
const ChaseCrosshairTraceDistance = float32(1 << 20)

// AngleVectors computes the forward/right/up orthonormal vectors from Euler
// angles (wrapping pkg/types.AngleVectors).
func AngleVectors(angles qtypes.Vec3) (forward, right, up qtypes.Vec3) {
	return qtypes.AngleVectors(angles)
}

// VectorAngles converts a forward direction vector into (pitch, yaw, 0) Euler
// angles, matching the C Quake VectorAngles / anglemod convention.
func VectorAngles(forward qtypes.Vec3) qtypes.Vec3 {
	var yaw, pitch float32

	if forward.X == 0 && forward.Y == 0 {
		yaw = 0
		if forward.Z > 0 {
			pitch = -90
		} else {
			pitch = 90
		}
	} else {
		yaw = float32(math.Atan2(float64(forward.Y), float64(forward.X)) * (180.0 / math.Pi))
		if yaw < 0 {
			yaw += 360
		}
		tmp := float32(math.Sqrt(float64(forward.X*forward.X + forward.Y*forward.Y)))
		pitch = -float32(math.Atan2(float64(forward.Z), float64(tmp)) * (180.0 / math.Pi))
	}

	return qtypes.Vec3{X: pitch, Y: yaw, Z: 0}
}

// ApplyBobToOrigin offsets the view origin along the forward vector by the
// view bob amount (0.4 lateral, full vertical), matching C V_CalcView bob.
func ApplyBobToOrigin(origin, forward qtypes.Vec3, bob float32) qtypes.Vec3 {
	origin.X += forward.X * bob * 0.4
	origin.Y += forward.Y * bob * 0.4
	origin.Z += forward.Z * bob * 0.4
	origin.Z += bob
	return origin
}

// NodeLineOffset nudges the view origin by the BSP node-line bias to avoid
// coplanar trace aliasing (1/32 world unit), matching C.
func NodeLineOffset(origin qtypes.Vec3) qtypes.Vec3 {
	const bias = 1.0 / 32.0
	origin.X += bias
	origin.Y += bias
	origin.Z += bias
	return origin
}

// BoundOffsets clamps the view origin near the player origin (14 units lateral,
// 22 below / 30 above), matching C V_CalcView's view bounding.
func BoundOffsets(vieworg, entityOrigin qtypes.Vec3) qtypes.Vec3 {
	if vieworg.X < entityOrigin.X-14 {
		vieworg.X = entityOrigin.X - 14
	}
	if vieworg.X > entityOrigin.X+14 {
		vieworg.X = entityOrigin.X + 14
	}
	if vieworg.Y < entityOrigin.Y-14 {
		vieworg.Y = entityOrigin.Y - 14
	}
	if vieworg.Y > entityOrigin.Y+14 {
		vieworg.Y = entityOrigin.Y + 14
	}
	if vieworg.Z < entityOrigin.Z-22 {
		vieworg.Z = entityOrigin.Z - 22
	}
	if vieworg.Z > entityOrigin.Z+30 {
		vieworg.Z = entityOrigin.Z + 30
	}
	return vieworg
}

// ChaseUpdate positions the chase camera behind/above/right of the target
// origin, tracing both the camera and its crosshair, then aiming the camera at
// the crosshair end point. Returns the camera origin and angles.
func ChaseUpdate(origin, angles qtypes.Vec3, chaseBack, chaseUp, chaseRight float32, traceFn ChaseTraceFunc) (qtypes.Vec3, qtypes.Vec3) {
	forward, right, _ := AngleVectors(angles)

	ideal := origin.Sub(forward.Scale(chaseBack)).Add(right.Scale(chaseRight))
	// Match Ironwail chase.c: chase_up is world-up offset, not camera-up.
	ideal.Z += chaseUp

	if traceFn != nil {
		ideal = traceFn(origin, ideal)
	}

	crosshair := origin.Add(forward.Scale(ChaseCrosshairTraceDistance))
	if traceFn != nil {
		crosshair = traceFn(origin, crosshair)
	}

	lookDir := crosshair.Sub(ideal)

	cameraAngles := VectorAngles(lookDir)
	if cameraAngles.X == 90 || cameraAngles.X == -90 {
		cameraAngles.Y = angles.Y
	}
	return ideal, cameraAngles
}
