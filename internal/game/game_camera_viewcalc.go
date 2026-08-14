package game

import (
	"math"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	cameralib "github.com/darkliquid/ironwail-go/internal/game/camera"
	"github.com/darkliquid/ironwail-go/pkg/types"
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
	// Per-frame runtime stair smoothing cache so multiple consumers
	// (camera/viewmodel/audio) share the same smoothed local-player Z.
	stairFrameValid     bool
	stairFrameTime      float64
	stairFrameEntityZ   float32
	stairFrameOnGround  bool
	stairFrameHardReset bool
	stairFrameSmoothedZ float32
	originSelectLatch   runtimeOriginSelectLatch
}

type runtimeOriginSelectLatch struct {
	valid             bool
	client            *cl.Client
	serverUpdateTime  float64
	source            runtimeOriginSource
	rejectReason      runtimeOriginRejectReason
	xyDelta           [2]float32
	predictionErrorXY [2]float32
}

// viewCalcBob returns the view bob offset for the current frame, matching
// C Ironwail V_CalcBob.  The result is in world units and is clamped to
// [-7, 4].
//
// Parameters:
//   - clientTime: cl.time (seconds)
//   - velocity:   XY components of the player's velocity
func (g *Game) viewCalcBob(clientTime float64, velocity types.Vec3) float32 {
	return cameralib.CalcBob(g.Host.CVar, clientTime, velocity)
}

// viewCalcRoll returns the camera roll angle (in degrees) caused by lateral
// strafing velocity, matching C Ironwail V_CalcRoll / CalcRoll.
//
// Parameters:
//   - angles:   player/camera Euler angles (pitch, yaw, roll)
//   - velocity: player velocity
func (g *Game) viewCalcRoll(angles, velocity types.Vec3) float32 {
	return cameralib.CalcRoll(g.Host.CVar, angles, velocity)
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
func (g *Game) viewCalcGunAngle(state *viewCalcState, viewAngles types.Vec3, clientTime, frameTime float64) types.Vec3 {
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
	var out types.Vec3
	out.Y = viewAngles.Y + yaw
	out.X = -(viewAngles.X + pitch)
	out.Z = viewAngles.Z

	// Idle sway on the weapon model.
	idleScaleCv := g.Host.CVar.Get("v_idlescale")
	if idleScaleCv != nil && idleScaleCv.Float != 0 {
		idleScale := float32(idleScaleCv.Float)
		irollCycle := g.Host.CVar.Get("v_iroll_cycle")
		irollLevel := g.Host.CVar.Get("v_iroll_level")
		ipitchCycle := g.Host.CVar.Get("v_ipitch_cycle")
		ipitchLevel := g.Host.CVar.Get("v_ipitch_level")
		iyawCycle := g.Host.CVar.Get("v_iyaw_cycle")
		iyawLevel := g.Host.CVar.Get("v_iyaw_level")
		if irollCycle != nil && irollLevel != nil &&
			ipitchCycle != nil && ipitchLevel != nil &&
			iyawCycle != nil && iyawLevel != nil {
			t := float64(clientTime)
			out.Z -= idleScale *
				float32(math.Sin(t*irollCycle.Float)) *
				float32(irollLevel.Float)
			out.X -= idleScale *
				float32(math.Sin(t*ipitchCycle.Float)) *
				float32(ipitchLevel.Float)
			out.Y -= idleScale *
				float32(math.Sin(t*iyawCycle.Float)) *
				float32(iyawLevel.Float)
		}
	}

	return out
}

// viewAddIdle adds idle sway to camera angles, matching C Ironwail V_AddIdle.
func (g *Game) viewAddIdle(angles types.Vec3, clientTime float64) types.Vec3 {
	return cameralib.AddIdle(g.Host.CVar, angles, clientTime)
}

// viewApplyBobToOrigin applies the view-bob offset to a weapon/view origin,
// matching the V_CalcRefdef origin update in C Ironwail:
//
//	view->origin[i] += forward[i]*bob*0.4
//	view->origin[2] += bob
func (g *Game) viewApplyBobToOrigin(origin types.Vec3, forward types.Vec3, bob float32) types.Vec3 {
	return cameralib.ApplyBobToOrigin(origin, forward, bob)
}

// viewNodeLineOffset applies the small node-line bias added to vieworg in C
// Ironwail to prevent z-fighting on the first BSP node:
//
//	r_refdef.vieworg[0] += 1.0/32
//	r_refdef.vieworg[1] += 1.0/32
//	r_refdef.vieworg[2] += 1.0/32
func (g *Game) viewNodeLineOffset(origin types.Vec3) types.Vec3 {
	return cameralib.NodeLineOffset(origin)
}

// viewApplyViewmodelQuakeFudge applies the r_viewmodel_quake origin fudge
// that nudges the weapon origin based on scr_viewsize, matching C Ironwail.
func (g *Game) viewApplyViewmodelQuakeFudge(origin types.Vec3, scrViewSize float64) types.Vec3 {
	return cameralib.ApplyViewmodelQuakeFudge(g.Host.CVar, origin, scrViewSize)
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
func (g *Game) viewApplyDamageKick(state *viewCalcState, angles types.Vec3, deltaTime float64) types.Vec3 {
	if state.dmgTime > 0 {
		kickTimeCv := g.Host.CVar.Get("v_kicktime")
		if kickTimeCv == nil || kickTimeCv.Float == 0 {
			state.dmgTime = 0
			return angles
		}
		kickTime := float32(kickTimeCv.Float)
		angles.Z += state.dmgTime / kickTime * state.dmgRoll  // ROLL
		angles.X += state.dmgTime / kickTime * state.dmgPitch // PITCH
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
func (g *Game) viewBoundOffsets(vieworg, entityOrigin types.Vec3) types.Vec3 {
	return cameralib.BoundOffsets(vieworg, entityOrigin)
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
func (g *Game) viewStairSmoothOffset(state *viewCalcState, entityZ float32, onGround bool, deltaTime float64, hardReset bool) float32 {
	if hardReset {
		state.oldZ = entityZ
		state.oldZInit = true
		return 0
	}

	// Initialize oldZ on first call.
	if !state.oldZInit {
		state.oldZ = entityZ
		state.oldZInit = true
		return 0
	}

	rise := entityZ - state.oldZ
	if rise > 0 && onGround {
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
