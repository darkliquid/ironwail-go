package main

import (
	"math"

	"github.com/ironwail/ironwail-go/internal/cvar"
	qtypes "github.com/ironwail/ironwail-go/pkg/types"
)

// viewCalcState holds persistent (frame-to-frame) state for view calculations.
// It mirrors the static locals in C Ironwail's CalcGunAngle and V_CalcViewRoll.
type viewCalcState struct {
	oldGunYaw   float32
	oldGunPitch float32
	// Damage kick state (V_ParseDamage / V_CalcViewRoll)
	dmgTime  float32
	dmgRoll  float32
	dmgPitch float32
	// Stair smoothing state (V_CalcRefdef oldz)
	oldZ     float32
	oldZInit bool
}

// globalViewCalc is the singleton view calc state used during gameplay.
var globalViewCalc viewCalcState

// viewCalcBob returns the view bob offset for the current frame, matching
// C Ironwail V_CalcBob.  The result is in world units and is clamped to
// [-7, 4].
//
// Parameters:
//   - clientTime: cl.time (seconds)
//   - velocity:   XY components of the player's velocity
func viewCalcBob(clientTime float64, velocity [3]float32) float32 {
	bobcycleCv := cvar.Get("cl_bobcycle")
	if bobcycleCv == nil {
		return 0
	}
	bobcycle := float32(bobcycleCv.Float)
	if bobcycle == 0 {
		return 0
	}

	bobupCv := cvar.Get("cl_bobup")
	bobCv := cvar.Get("cl_bob")
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
	speed := math.Sqrt(float64(velocity[0]*velocity[0] + velocity[1]*velocity[1]))
	bob := float32(speed) * float32(bobCv.Float)
	bob = bob*0.3 + bob*0.7*float32(math.Sin(float64(radians)))

	if bob > 4 {
		bob = 4
	} else if bob < -7 {
		bob = -7
	}
	return bob
}

// viewCalcRoll returns the camera roll angle (in degrees) caused by lateral
// strafing velocity, matching C Ironwail V_CalcRoll / CalcRoll.
//
// Parameters:
//   - angles:   player/camera Euler angles (pitch, yaw, roll)
//   - velocity: player velocity
func viewCalcRoll(angles, velocity [3]float32) float32 {
	rollAngleCv := cvar.Get("cl_rollangle")
	rollSpeedCv := cvar.Get("cl_rollspeed")
	if rollAngleCv == nil || rollSpeedCv == nil {
		return 0
	}

	_, right, _ := qtypes.AngleVectors(qtypes.Vec3{X: angles[0], Y: angles[1], Z: angles[2]})
	side := velocity[0]*right.X + velocity[1]*right.Y + velocity[2]*right.Z

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

// viewCalcGunAngle updates the weapon-model Euler angles to smoothly follow
// the camera with rate-limited drift, then adds idle sway.  It mirrors
// C Ironwail CalcGunAngle.
//
// Parameters:
//   - state:      persistent state (oldyaw / oldpitch); modified in-place
//   - viewAngles: current camera angles (pitch, yaw, roll)
//   - clientTime: cl.time
//   - frameTime:  host_frametime
//
// Returns the weapon-model angles to use for this frame.
func viewCalcGunAngle(state *viewCalcState, viewAngles [3]float32, clientTime, frameTime float64) [3]float32 {
	const (
		pitchIdx = 0
		yawIdx   = 1
		rollIdx  = 2
	)

	// C code: yaw = angledelta(yaw - r_refdef.viewangles[YAW]) * 0.4
	// Since yaw was just set to viewangles[YAW], the delta is always 0, so
	// gun yaw/pitch corrections are entirely driven by the rate-limit below.
	yaw := float32(0)
	pitch := float32(0)

	// Rate-limit toward 0 (mirrors the move = host_frametime*20 clamp in C).
	move := float32(frameTime) * 20
	if yaw > state.oldGunYaw {
		if state.oldGunYaw+move < yaw {
			yaw = state.oldGunYaw + move
		}
	} else {
		if state.oldGunYaw-move > yaw {
			yaw = state.oldGunYaw - move
		}
	}
	if pitch > state.oldGunPitch {
		if state.oldGunPitch+move < pitch {
			pitch = state.oldGunPitch + move
		}
	} else {
		if state.oldGunPitch-move > pitch {
			pitch = state.oldGunPitch - move
		}
	}
	state.oldGunYaw = yaw
	state.oldGunPitch = pitch

	// Base weapon angles track the view.
	var out [3]float32
	out[yawIdx] = viewAngles[yawIdx] + yaw
	out[pitchIdx] = -(viewAngles[pitchIdx] + pitch)
	out[rollIdx] = viewAngles[rollIdx]

	// Idle sway on the weapon model.
	idleScaleCv := cvar.Get("v_idlescale")
	if idleScaleCv != nil && idleScaleCv.Float != 0 {
		idleScale := float32(idleScaleCv.Float)
		irollCycle := cvar.Get("v_iroll_cycle")
		irollLevel := cvar.Get("v_iroll_level")
		ipitchCycle := cvar.Get("v_ipitch_cycle")
		ipitchLevel := cvar.Get("v_ipitch_level")
		iyawCycle := cvar.Get("v_iyaw_cycle")
		iyawLevel := cvar.Get("v_iyaw_level")
		if irollCycle != nil && irollLevel != nil &&
			ipitchCycle != nil && ipitchLevel != nil &&
			iyawCycle != nil && iyawLevel != nil {
			t := float64(clientTime)
			out[rollIdx] -= idleScale *
				float32(math.Sin(t*irollCycle.Float)) *
				float32(irollLevel.Float)
			out[pitchIdx] -= idleScale *
				float32(math.Sin(t*ipitchCycle.Float)) *
				float32(ipitchLevel.Float)
			out[yawIdx] -= idleScale *
				float32(math.Sin(t*iyawCycle.Float)) *
				float32(iyawLevel.Float)
		}
	}

	return out
}

// viewAddIdle adds idle sway to camera angles, matching C Ironwail V_AddIdle.
func viewAddIdle(angles [3]float32, clientTime float64) [3]float32 {
	cv := cvar.Get("v_idlescale")
	if cv == nil {
		return angles
	}
	idleScale := float32(cv.Float)
	if idleScale == 0 {
		return angles
	}

	irollCycle := cvar.Get("v_iroll_cycle")
	irollLevel := cvar.Get("v_iroll_level")
	ipitchCycle := cvar.Get("v_ipitch_cycle")
	ipitchLevel := cvar.Get("v_ipitch_level")
	iyawCycle := cvar.Get("v_iyaw_cycle")
	iyawLevel := cvar.Get("v_iyaw_level")
	if irollCycle == nil || irollLevel == nil || ipitchCycle == nil ||
		ipitchLevel == nil || iyawCycle == nil || iyawLevel == nil {
		return angles
	}

	t := clientTime
	const (
		rollIdx  = 2
		pitchIdx = 0
		yawIdx   = 1
	)
	angles[rollIdx] += idleScale *
		float32(math.Sin(t*irollCycle.Float)) *
		float32(irollLevel.Float)
	angles[pitchIdx] += idleScale *
		float32(math.Sin(t*ipitchCycle.Float)) *
		float32(ipitchLevel.Float)
	angles[yawIdx] += idleScale *
		float32(math.Sin(t*iyawCycle.Float)) *
		float32(iyawLevel.Float)
	return angles
}

// viewApplyBobToOrigin applies the view-bob offset to a weapon/view origin,
// matching the V_CalcRefdef origin update in C Ironwail:
//
//	view->origin[i] += forward[i]*bob*0.4
//	view->origin[2] += bob
func viewApplyBobToOrigin(origin [3]float32, forward [3]float32, bob float32) [3]float32 {
	origin[0] += forward[0] * bob * 0.4
	origin[1] += forward[1] * bob * 0.4
	origin[2] += forward[2] * bob * 0.4
	origin[2] += bob
	return origin
}

// viewNodeLineOffset applies the small node-line bias added to vieworg in C
// Ironwail to prevent z-fighting on the first BSP node:
//
//	r_refdef.vieworg[0] += 1.0/32
//	r_refdef.vieworg[1] += 1.0/32
//	r_refdef.vieworg[2] += 1.0/32
func viewNodeLineOffset(origin [3]float32) [3]float32 {
	const bias = 1.0 / 32.0
	origin[0] += bias
	origin[1] += bias
	origin[2] += bias
	return origin
}

// viewApplyViewmodelQuakeFudge applies the r_viewmodel_quake origin fudge
// that nudges the weapon origin based on scr_viewsize, matching C Ironwail.
func viewApplyViewmodelQuakeFudge(origin [3]float32, scrViewSize float64) [3]float32 {
	cv := cvar.Get("r_viewmodel_quake")
	if cv == nil || cv.Int == 0 {
		return origin
	}
	switch int(scrViewSize) {
	case 110:
		origin[2] += 1
	case 100:
		origin[2] += 2
	case 90:
		origin[2] += 1
	case 80:
		origin[2] += 0.5
	}
	return origin
}

// viewSetDamageKick initializes damage kick state from a damage event.
// Mirrors C Ironwail V_ParseDamage damage kick calculation (view.c:329-345).
//
// Parameters:
//   - state:        persistent view state (modified in-place)
//   - count:        damage amount (blood*0.5 + armor*0.5, min 10)
//   - from:         normalized direction vector from damage source to player
//   - entityAngles: player entity angles (for computing right/forward vectors)
func viewSetDamageKick(state *viewCalcState, count float32, from, entityAngles [3]float32) {
	kickRollCv := cvar.Get("v_kickroll")
	kickPitchCv := cvar.Get("v_kickpitch")
	kickTimeCv := cvar.Get("v_kicktime")
	if kickRollCv == nil || kickPitchCv == nil || kickTimeCv == nil {
		return
	}

	// Compute right and forward vectors from entity angles.
	_, right, _ := qtypes.AngleVectors(qtypes.Vec3{X: entityAngles[0], Y: entityAngles[1], Z: entityAngles[2]})
	forward, _, _ := qtypes.AngleVectors(qtypes.Vec3{X: entityAngles[0], Y: entityAngles[1], Z: entityAngles[2]})

	// Roll kick: lateral component of damage direction.
	sideRoll := from[0]*right.X + from[1]*right.Y + from[2]*right.Z
	state.dmgRoll = count * sideRoll * float32(kickRollCv.Float)

	// Pitch kick: forward/back component of damage direction.
	sidePitch := from[0]*forward.X + from[1]*forward.Y + from[2]*forward.Z
	state.dmgPitch = count * sidePitch * float32(kickPitchCv.Float)

	state.dmgTime = float32(kickTimeCv.Float)
}

// viewApplyDamageKick applies damage-induced camera roll/pitch and decays the
// damage kick timer.  Mirrors C Ironwail V_CalcViewRoll damage kick block
// (view.c:718-722).
//
// Parameters:
//   - state:     persistent view state (dmgTime/dmgRoll/dmgPitch); modified in-place
//   - angles:    current camera angles [pitch, yaw, roll]
//   - deltaTime: time elapsed since last frame (host_frametime or cl.time - cl.oldtime)
//
// Returns the updated camera angles.
func viewApplyDamageKick(state *viewCalcState, angles [3]float32, deltaTime float64) [3]float32 {
	if state.dmgTime > 0 {
		kickTimeCv := cvar.Get("v_kicktime")
		if kickTimeCv == nil || kickTimeCv.Float == 0 {
			state.dmgTime = 0
			return angles
		}
		kickTime := float32(kickTimeCv.Float)
		angles[2] += state.dmgTime / kickTime * state.dmgRoll  // ROLL
		angles[0] += state.dmgTime / kickTime * state.dmgPitch // PITCH
		state.dmgTime -= float32(math.Abs(deltaTime))
		if state.dmgTime < 0 {
			state.dmgTime = 0
		}
	}
	return angles
}

// viewBoundOffsets clamps the camera origin to within ±14 units in XY and
// -22/+30 units in Z relative to the entity origin.  Mirrors C Ironwail
// V_BoundOffsets (view.c:665-686).
func viewBoundOffsets(vieworg, entityOrigin [3]float32) [3]float32 {
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

// viewStairSmooth computes and applies stair step smoothing offset.
// Mirrors C Ironwail V_CalcRefdef stair smoothing block (view.c:871-888).
//
// Parameters:
//   - state:      persistent view state (oldZ); modified in-place
//   - entityZ:    player entity Z coordinate
//   - onGround:   whether player is on ground
//   - deltaTime:  time elapsed since last frame
//
// Returns the smoothing offset to add to both camera and weapon Z coordinates.
func viewStairSmoothOffset(state *viewCalcState, entityZ float32, onGround bool, deltaTime float64) float32 {
	// Initialize oldZ on first call.
	if !state.oldZInit {
		state.oldZ = entityZ
		state.oldZInit = true
		return 0
	}

	// Only smooth when on ground and moving upward (stairs).
	// !noclip_anglehack is assumed (we don't have this hack in Go port).
	if onGround && entityZ-state.oldZ > 0 {
		steptime := float32(deltaTime)
		if steptime < 0 {
			steptime = 0
		}

		// Smooth oldZ toward entityZ at 80 units/sec.
		state.oldZ += steptime * 80
		if state.oldZ > entityZ {
			state.oldZ = entityZ
		}
		// Clamp smoothing to max 12 units below current position.
		if entityZ-state.oldZ > 12 {
			state.oldZ = entityZ - 12
		}

		// Return the offset: oldZ - entityZ.
		return state.oldZ - entityZ
	}

	state.oldZ = entityZ
	return 0
}
