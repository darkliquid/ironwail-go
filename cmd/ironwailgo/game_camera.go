package main

import (
	"math"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/renderer"
	"github.com/ironwail/ironwail-go/internal/server"
	qtypes "github.com/ironwail/ironwail-go/pkg/types"
)

func runtimeViewState() (origin, angles [3]float32) {
	origin = [3]float32{0, 0, 128}
	angles = [3]float32{0, 0, 0}
	foundPlayerStart := false

	if g.Server != nil {
		for _, ent := range g.Server.Edicts {
			if ent == nil || ent.Free || ent.Vars == nil || ent.Vars.ClassName == 0 {
				continue
			}
			className := g.Server.GetString(ent.Vars.ClassName)
			if className != "info_player_start" && className != "info_player_deathmatch" {
				continue
			}
			origin = ent.Vars.Origin
			origin[2] += 22
			angles = ent.Vars.Angles
			foundPlayerStart = true
			break
		}
	}

	if !foundPlayerStart && g.Renderer != nil {
		if minBounds, maxBounds, ok := g.Renderer.GetWorldBounds(); ok {
			centerX := (minBounds[0] + maxBounds[0]) * 0.5
			centerY := (minBounds[1] + maxBounds[1]) * 0.5
			centerZ := (minBounds[2] + maxBounds[2]) * 0.5

			extentX := maxBounds[0] - minBounds[0]
			extentY := maxBounds[1] - minBounds[1]
			extentZ := maxBounds[2] - minBounds[2]

			radius := extentX
			if extentY > radius {
				radius = extentY
			}
			if extentZ > radius {
				radius = extentZ
			}
			if radius < 256 {
				radius = 256
			}

			origin = [3]float32{centerX, centerY + radius, centerZ + radius*0.5}
			angles = [3]float32{0, 0, 0}
		}
	}

	if g.Client != nil {
		if clientOrigin, ok := runtimePlayerOrigin(); ok {
			// Apply ViewHeight + bob to camera Z.
			// Mirrors C Ironwail V_CalcRefdef: r_refdef.vieworg[2] += cl.viewheight + bob.
			clientOrigin[2] += g.Client.ViewHeight
			bob := viewCalcBob(g.Client.Time, g.Client.Velocity)
			clientOrigin[2] += bob

			viewAngles := runtimeInterpolatedViewAngles()
			return clientOrigin, viewAngles
		}
	}

	return origin, angles
}

// runtimeWeaponBaseOrigin returns the weapon model base origin: entity origin + viewheight.
// This does NOT include bob — bob is applied separately by viewApplyBobToOrigin.
// Mirrors C Ironwail V_CalcRefdef: VectorCopy(ent->origin, view->origin); view->origin[2] += cl.viewheight;
func runtimeWeaponBaseOrigin() [3]float32 {
	if g.Client != nil {
		if clientOrigin, ok := runtimePlayerOrigin(); ok {
			clientOrigin[2] += g.Client.ViewHeight
			return clientOrigin
		}
	}
	// Fallback: use the camera origin from runtimeViewState (has bob, but no weapon
	// bob will be applied since velocity is zero in this case).
	origin, _ := runtimeViewState()
	return origin
}

func runtimePlayerOrigin() ([3]float32, bool) {
	if g.Client == nil {
		return [3]float32{}, false
	}

	if authoritativeOrigin, ok := runtimeAuthoritativePlayerOrigin(); ok {
		if predictedOffset, ok := runtimePredictedXYOffset(authoritativeOrigin); ok {
			authoritativeOrigin[0] += predictedOffset[0]
			authoritativeOrigin[1] += predictedOffset[1]
		}
		return authoritativeOrigin, true
	}

	clientOrigin := g.Client.PredictedOrigin
	if clientOrigin[0] != 0 || clientOrigin[1] != 0 || clientOrigin[2] != 0 {
		return clientOrigin, true
	}

	return [3]float32{}, false
}

func runtimeAuthoritativePlayerOrigin() ([3]float32, bool) {
	if g.Client == nil {
		return [3]float32{}, false
	}

	if g.Client.ViewEntity != 0 {
		if state, ok := g.Client.Entities[g.Client.ViewEntity]; ok {
			return state.Origin, true
		}
	}

	if g.Client.ViewEntity == 0 {
		if state, ok := g.Client.Entities[0]; ok {
			return state.Origin, true
		}
	}

	return [3]float32{}, false
}

func runtimePredictedXYOffset(authoritativeOrigin [3]float32) ([2]float32, bool) {
	if g.Client == nil || g.Client.State != cl.StateActive {
		return [2]float32{}, false
	}

	cmd := g.Client.PendingCmd
	if cmd.Forward == 0 && cmd.Side == 0 {
		return [2]float32{}, false
	}

	clientOrigin := g.Client.PredictedOrigin
	if clientOrigin[0] == 0 && clientOrigin[1] == 0 && clientOrigin[2] == 0 {
		return [2]float32{}, false
	}

	if runtimeLocalViewTeleportActive() || predictionErrorXYMagnitude(g.Client.PredictionError) > runtimeMaxPredictedXYOffset {
		return [2]float32{}, false
	}

	offset := [2]float32{
		clientOrigin[0] - authoritativeOrigin[0],
		clientOrigin[1] - authoritativeOrigin[1],
	}
	offsetMagnitude := predictionErrorXYMagnitude([3]float32{offset[0], offset[1], 0})
	if offsetMagnitude == 0 {
		return [2]float32{}, false
	}

	if offsetMagnitude > runtimeMaxPredictedXYOffset {
		scale := float32(runtimeMaxPredictedXYOffset / offsetMagnitude)
		offset[0] *= scale
		offset[1] *= scale
	}

	return offset, true
}

func predictionErrorXYMagnitude(v [3]float32) float64 {
	return math.Hypot(float64(v[0]), float64(v[1]))
}

func runtimeLocalViewTeleportActive() bool {
	return g.Client != nil && g.Client.LocalViewTeleportActive()
}

func runtimeCameraState(origin, angles [3]float32) renderer.CameraState {
	// Apply node-line bias to camera origin to prevent BSP z-fighting.
	// Mirrors C Ironwail: r_refdef.vieworg[i] += 1.0/32 (applied just before R_RenderView).
	cameraOrigin := viewNodeLineOffset(origin)

	// Apply V_BoundOffsets to clamp camera relative to entity origin.
	// Mirrors C Ironwail view.c:665-686.
	if g.Client != nil {
		if entityOrigin, ok := runtimeAuthoritativePlayerOrigin(); ok {
			cameraOrigin = viewBoundOffsets(cameraOrigin, entityOrigin)

			// Apply stair step smoothing.
			// Mirrors C Ironwail V_CalcRefdef stair smoothing (view.c:871-888).
			deltaTime := 0.0
			if g.Host != nil {
				deltaTime = g.Host.FrameTime()
			}
			stairOffset := viewStairSmoothOffset(&globalViewCalc, entityOrigin[2], g.Client.OnGround, deltaTime, runtimeLocalViewTeleportActive())
			cameraOrigin[2] += stairOffset
		}
	}

	camera := renderer.ConvertClientStateToCamera(cameraOrigin, angles, 96.0)
	if g.Client != nil {
		if g.Client.Intermission == 0 {
			deadPlayer := false
			// Check for dead view angle (health <= 0 → roll = 80).
			// Mirrors C Ironwail view.c:728-731.
			health := g.Client.Health()
			if health <= 0 {
				camera.Angles.Z = 80
				// Dead players don't get other view effects.
				deadPlayer = true
			}

			if !deadPlayer {
				punch := runtimeGunKickAngles()
				camera.Angles.X += punch[0]
				camera.Angles.Y += punch[1]
				camera.Angles.Z += punch[2]

				// Apply damage kick (V_CalcViewRoll damage kick block).
				// Mirrors C Ironwail view.c:718-722.
				deltaTime := 0.0
				if g.Host != nil {
					deltaTime = g.Host.FrameTime()
				}
				cameraAngles := [3]float32{camera.Angles.X, camera.Angles.Y, camera.Angles.Z}
				cameraAngles = viewApplyDamageKick(&globalViewCalc, cameraAngles, deltaTime)
				camera.Angles.X = cameraAngles[0]
				camera.Angles.Y = cameraAngles[1]
				camera.Angles.Z = cameraAngles[2]

				// View roll from lateral movement (V_CalcViewRoll).
				roll := viewCalcRoll(angles, g.Client.Velocity)
				camera.Angles.Z += roll

				// Idle sway on the camera (V_AddIdle).
				cameraAngles = [3]float32{camera.Angles.X, camera.Angles.Y, camera.Angles.Z}
				cameraAngles = viewAddIdle(cameraAngles, g.Client.Time)
				camera.Angles.X = cameraAngles[0]
				camera.Angles.Y = cameraAngles[1]
				camera.Angles.Z = cameraAngles[2]
			}
		}
		camera.Time = float32(g.Client.Time)
	}
	if cvar.BoolValue("chase_active") {
		traceFn := runtimeChaseTraceFn()
		chaseOrigin, chaseAngles := chaseUpdate(
			origin,
			angles,
			float32(cvar.FloatValue("chase_back")),
			float32(cvar.FloatValue("chase_up")),
			float32(cvar.FloatValue("chase_right")),
			traceFn,
		)
		camera.Origin.X = chaseOrigin[0]
		camera.Origin.Y = chaseOrigin[1]
		camera.Origin.Z = chaseOrigin[2]
		camera.Angles.X = chaseAngles[0]
		camera.Angles.Y = chaseAngles[1]
		camera.Angles.Z = chaseAngles[2]
	}
	// Apply r_waterwarp > 1 FOV oscillation when underwater.
	_, wwFOV, _ := runtimeWaterwarpState()
	camera.WaterwarpFOV = wwFOV
	return camera
}

func runtimeChaseTraceFn() chaseTraceFunc {
	if g.Server == nil {
		return nil
	}

	var passEnt *server.Edict
	if g.Client != nil && g.Client.ViewEntity > 0 {
		passEnt = g.Server.EdictNum(g.Client.ViewEntity)
	}

	return func(start, end [3]float32) [3]float32 {
		trace := g.Server.SV_Move(start, [3]float32{}, [3]float32{}, end, server.MoveType(server.MoveNoMonsters), passEnt)
		return trace.EndPos
	}
}

func runtimeInterpolatedViewAngles() [3]float32 {
	if g.Client == nil {
		return [3]float32{}
	}
	if g.Client.FixAngle || g.Client.DemoPlayback {
		return g.Client.ViewAngles
	}
	prev, curr := g.Client.MViewAngles[1], g.Client.MViewAngles[0]
	if prev == [3]float32{} && curr == [3]float32{} {
		return g.Client.ViewAngles
	}
	frac := float32(g.Client.LerpPoint())
	if frac < 0 {
		frac = 0
	} else if frac > 1 {
		frac = 1
	}
	var out [3]float32
	for i := range out {
		out[i] = angleLerp(prev[i], curr[i], frac)
	}
	return out
}

func runtimeGunKickAngles() [3]float32 {
	if g.Client == nil {
		return [3]float32{}
	}
	mode := 2
	if cv := cvar.Get("v_gunkick"); cv != nil {
		mode = cv.Int
	}
	switch mode {
	case 0:
		return [3]float32{}
	case 1:
		return g.Client.PunchAngle
	default:
		return runtimeInterpolatedPunchAngles()
	}
}

func angleLerp(prev, curr, frac float32) float32 {
	delta := curr - prev
	for delta > 180 {
		delta -= 360
	}
	for delta < -180 {
		delta += 360
	}
	return prev + delta*frac
}

func runtimeInterpolatedPunchAngles() [3]float32 {
	if g.Client == nil {
		return [3]float32{}
	}
	prev, curr := g.Client.PunchAngles[1], g.Client.PunchAngles[0]
	if prev == [3]float32{} && curr == [3]float32{} {
		return g.Client.PunchAngle
	}
	alpha := float32(1.0)
	if g.Client.PunchTime > 0 {
		alpha = float32((g.Client.Time - g.Client.PunchTime) / 0.1)
		if alpha < 0 {
			alpha = 0
		} else if alpha > 1 {
			alpha = 1
		}
	}
	var out [3]float32
	for i := range out {
		out[i] = prev[i] + (curr[i]-prev[i])*alpha
	}
	return out
}

func runtimeAngleVectors(angles [3]float32) (forward, right, up [3]float32) {
	forwardVec, rightVec, upVec := qtypes.AngleVectors(qtypes.Vec3{
		X: angles[0],
		Y: angles[1],
		Z: angles[2],
	})
	return [3]float32{forwardVec.X, forwardVec.Y, forwardVec.Z},
		[3]float32{rightVec.X, rightVec.Y, rightVec.Z},
		[3]float32{upVec.X, upVec.Y, upVec.Z}
}
