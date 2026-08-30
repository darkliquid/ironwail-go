package game

import (
	"os"

	cameralib "github.com/darkliquid/ironwail-go/internal/game/camera"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/internal/server"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

func (g *Game) runtimeViewDeltaTime() float64 {
	if g.Host != nil {
		return g.Host.FrameTime()
	}
	if g.Client == nil {
		return 0
	}
	delta := g.Client.Time - g.Client.OldTime
	if delta < 0 {
		return 0
	}
	return delta
}

func (g *Game) runtimeSmoothedLocalPlayerBaseOrigin() (types.Vec3, bool) {
	origin, ok := g.runtimePlayerOrigin()
	if !ok || g.Client == nil {
		return origin, ok
	}

	state := &g.viewCalc
	entityZ := origin.Z
	frameTime := g.Client.Time
	onGround := g.Client.OnGround
	hardReset := g.runtimeLocalViewTeleportActive()
	if state.stairFrameValid &&
		state.stairFrameTime == frameTime &&
		state.stairFrameEntityZ == entityZ &&
		state.stairFrameOnGround == onGround &&
		state.stairFrameHardReset == hardReset {
		origin.Z = state.stairFrameSmoothedZ
		return origin, true
	}

	origin.Z += g.viewStairSmoothOffset(state, entityZ, onGround, g.runtimeViewDeltaTime(), hardReset)
	state.stairFrameValid = true
	state.stairFrameTime = frameTime
	state.stairFrameEntityZ = entityZ
	state.stairFrameOnGround = onGround
	state.stairFrameHardReset = hardReset
	state.stairFrameSmoothedZ = origin.Z
	return origin, true
}

func (g *Game) runtimeFirstPersonBobOffset() float32 {
	if g.Client == nil {
		return 0
	}
	return g.viewCalcBob(g.Client.Time, g.runtimeInterpolatedVelocity())
}

func (g *Game) runtimeViewState() (origin, angles types.Vec3) {
	if os.Getenv("PARITY_RUN") == "1" {
		pos, hasPos := parseParityAnglesEnv(os.Getenv("PARITY_POS"))
		ang, hasAng := parseParityAnglesEnv(os.Getenv("PARITY_ANGLES"))
		if hasPos && hasAng {
			return pos, ang
		}
	}

	origin = types.Vec3{X: 0, Y: 0, Z: 128}
	angles = types.Vec3{X: 0, Y: 0, Z: 0}
	foundPlayerStart := false

	if g.Server != nil {
		for _, ent := range g.Server.Edicts {
			if ent == nil || ent.Free || ent.ClassName(g.Server) == 0 {
				continue
			}
			className := g.Server.String(ent.ClassName(g.Server))
			if className != "info_player_start" && className != "info_player_deathmatch" {
				continue
			}
			origin = ent.Origin(g.Server)
			origin.Z += 22
			angles = ent.Angles(g.Server)
			foundPlayerStart = true
			break
		}
	}

	if !foundPlayerStart && g.Renderer != nil {
		if minBounds, maxBounds, ok := g.Renderer.WorldBounds(); ok {
			centerX := (minBounds.X + maxBounds.X) * 0.5
			centerY := (minBounds.Y + maxBounds.Y) * 0.5
			centerZ := (minBounds.Z + maxBounds.Z) * 0.5

			extentX := maxBounds.X - minBounds.X
			extentY := maxBounds.Y - minBounds.Y
			extentZ := maxBounds.Z - minBounds.Z

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

			origin = types.Vec3{X: centerX, Y: centerY + radius, Z: centerZ + radius*0.5}
			angles = types.Vec3{X: 0, Y: 0, Z: 0}
		}
	}

	if g.Client != nil {
		if clientOrigin, ok := g.runtimeSmoothedLocalPlayerBaseOrigin(); ok {
			// Keep the first-person camera anchored to the smoothed eye origin.
			viewHeight := g.Client.ViewHeight
			if viewHeight == 0 {
				viewHeight = 22
			}
			clientOrigin.Z += viewHeight

			var viewAngles types.Vec3
			if g.Client.Intermission != 0 {
				// During intermission the rendered camera must use the view entity's
				// server-authoritative angles, not cl.ViewAngles. Mirrors C Ironwail
				// V_CalcIntermissionRefdef: VectorCopy(ent->angles, r_refdef.viewangles).
				// The server positions the view entity at the info_intermission camera
				// spot and sets its angles; cl.ViewAngles is irrelevant and may have
				// been mutated by AdjustAngles (left/right keys), which would visually
				// rotate the camera in a way C never does.
				// No bob offset: C Ironwail's V_CalcIntermissionRefdef does not apply
				// V_CalcBob, so the intermission camera is rock-steady.
				if viewEnt, ok := g.Client.Entities[g.Client.ViewEntity]; ok {
					viewAngles = viewEnt.Angles
				} else {
					viewAngles = g.runtimeInterpolatedViewAngles()
				}
			} else {
				clientOrigin.Z += g.runtimeFirstPersonBobOffset()
				viewAngles = g.runtimeInterpolatedViewAngles()
			}
			return clientOrigin, viewAngles
		}
	}

	return origin, angles
}

// runtimeWeaponBaseOrigin returns the weapon model base origin: entity origin + viewheight.
// Mirrors C Ironwail V_CalcRefdef: VectorCopy(ent->origin, view->origin); view->origin.Z += cl.viewheight;
func (g *Game) runtimeWeaponBaseOrigin() types.Vec3 {
	if g.Client != nil {
		if clientOrigin, ok := g.runtimeSmoothedLocalPlayerBaseOrigin(); ok {
			clientOrigin.Z += g.Client.ViewHeight
			return clientOrigin
		}
	}
	// Fallback: use the camera origin from runtimeViewState.
	origin, _ := g.runtimeViewState()
	return origin
}

func (g *Game) runtimePlayerOrigin() (types.Vec3, bool) {
	telemetry := runtimeOriginSelectTelemetry{
		XYOffsetThreshold:        RuntimeMaxPredictedXYOffset,
		PredictionErrorThreshold: RuntimeMaxPredictedXYOffset,
	}
	state := &g.viewCalc
	if g.Client == nil {
		g.runtimeResetOriginSelectLatch(state)
		telemetry.RejectReason = runtimeOriginRejectMissingAuth
		g.runtimeDebugViewRecordOriginSelect(telemetry)
		return types.Vec3{}, false
	}
	telemetry.PredictedOrigin = g.Client.PredictedOrigin
	telemetry.PredictionValid = g.Client.HasFreshPredictionForCurrentEntity()

	if authoritativeOrigin, ok := g.runtimeAuthoritativePlayerOrigin(); ok {
		telemetry.AuthoritativeOrigin = authoritativeOrigin
		g.runtimeLatchOriginSelect(state, authoritativeOrigin)
		telemetry.Source = state.originSelectLatch.source
		telemetry.XYDelta = state.originSelectLatch.xyDelta
		telemetry.PredictionErrorXY = state.originSelectLatch.predictionErrorXY
		telemetry.RejectReason = state.originSelectLatch.rejectReason
		telemetry.FinalBaseOrigin = authoritativeOrigin
		g.runtimeDebugViewRecordOriginSelect(telemetry)
		return authoritativeOrigin, true
	}

	g.runtimeResetOriginSelectLatch(state)
	if !telemetry.PredictionValid {
		telemetry.RejectReason = runtimeOriginRejectInvalidPrediction
		g.runtimeDebugViewRecordOriginSelect(telemetry)
		return types.Vec3{}, false
	}
	telemetry.RejectReason = runtimeOriginRejectMissingAuth
	g.runtimeDebugViewRecordOriginSelect(telemetry)
	return types.Vec3{}, false
}

func (g *Game) runtimeLatchOriginSelect(state *viewCalcState, authoritativeOrigin types.Vec3) {
	if state == nil || g.Client == nil {
		return
	}
	serverUpdateTime := g.Client.MTime[0]
	if state.originSelectLatch.valid &&
		state.originSelectLatch.client == g.Client &&
		state.originSelectLatch.serverUpdateTime == serverUpdateTime &&
		(state.originSelectLatch.source != runtimeOriginSourceAuthoritativePredictedXY || g.Client.HasFreshPredictionForCurrentEntity()) &&
		!g.runtimeLocalViewTeleportActive() {
		return
	}

	decision := g.runtimeEvaluatePredictedFirstPersonXYOrigin(authoritativeOrigin)
	source := runtimeOriginSourceAuthoritativeOnly
	if decision.OK {
		source = runtimeOriginSourceAuthoritativePredictedXY
	}
	state.originSelectLatch = runtimeOriginSelectLatch{
		valid:             true,
		client:            g.Client,
		serverUpdateTime:  serverUpdateTime,
		source:            source,
		rejectReason:      decision.RejectReason,
		xyDelta:           decision.XYDelta,
		predictionErrorXY: decision.PredictionErrorXY,
	}
}

func (g *Game) runtimeResetOriginSelectLatch(state *viewCalcState) {
	if state == nil {
		return
	}
	state.originSelectLatch = runtimeOriginSelectLatch{}
}

type runtimePredictedXYDecision struct {
	Origin            types.Vec3
	OK                bool
	RejectReason      runtimeOriginRejectReason
	XYDelta           [2]float32
	PredictionErrorXY [2]float32
}

func (g *Game) runtimeEvaluatePredictedFirstPersonXYOrigin(authoritativeOrigin types.Vec3) runtimePredictedXYDecision {
	decision := runtimePredictedXYDecision{}
	if g.Client == nil {
		decision.RejectReason = runtimeOriginRejectMissingAuth
		return decision
	}
	if !g.Client.HasFreshPredictionForCurrentEntity() {
		decision.RejectReason = runtimeOriginRejectInvalidPrediction
		return decision
	}

	predictedOrigin := g.Client.PredictedOrigin
	decision.Origin = predictedOrigin
	decision.XYDelta = [2]float32{
		predictedOrigin.X - authoritativeOrigin.X,
		predictedOrigin.Y - authoritativeOrigin.Y,
	}
	decision.PredictionErrorXY = [2]float32{
		g.Client.PredictionError.X,
		g.Client.PredictionError.Y,
	}

	if g.runtimeLocalViewTeleportActive() {
		decision.RejectReason = runtimeOriginRejectTeleportGate
		return decision
	}
	if predictedOrigin == (types.Vec3{}) {
		decision.RejectReason = runtimeOriginRejectZeroPrediction
		return decision
	}
	if g.runtimeFloat32Abs(decision.XYDelta[0]) > RuntimeMaxPredictedXYOffset ||
		g.runtimeFloat32Abs(decision.XYDelta[1]) > RuntimeMaxPredictedXYOffset {
		decision.RejectReason = runtimeOriginRejectXYOffsetThreshold
		return decision
	}
	if g.runtimeFloat32Abs(decision.PredictionErrorXY[0]) > RuntimeMaxPredictedXYOffset ||
		g.runtimeFloat32Abs(decision.PredictionErrorXY[1]) > RuntimeMaxPredictedXYOffset {
		decision.RejectReason = runtimeOriginRejectPredictionErrorThreshold
		return decision
	}

	decision.OK = true
	return decision
}

func (g *Game) runtimeFloat32Abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (g *Game) runtimeAuthoritativePlayerOrigin() (types.Vec3, bool) {
	if g.Client == nil {
		return types.Vec3{}, false
	}

	if g.Client.ViewEntity != 0 {
		if state, ok := g.Client.Entities[g.Client.ViewEntity]; ok && state.Origin != (types.Vec3{}) {
			return state.Origin, true
		}
	}

	if g.Client.ViewEntity == 0 {
		if state, ok := g.Client.Entities[0]; ok && state.Origin != (types.Vec3{}) {
			return state.Origin, true
		}
	}

	if g.Client.LastServerOrigin != (types.Vec3{}) {
		return g.Client.LastServerOrigin, true
	}

	return types.Vec3{}, false
}

func (g *Game) runtimeInterpolatedVelocity() types.Vec3 {
	if g.Client == nil {
		return types.Vec3{}
	}

	current := g.Client.MVelocity[0]
	previous := g.Client.MVelocity[1]
	if current == (types.Vec3{}) && previous == (types.Vec3{}) {
		return g.Client.Velocity
	}

	frac := float32(g.Client.LerpPoint())
	if frac <= 0 {
		return previous
	}
	if frac >= 1 {
		return current
	}

	return previous.Add(current.Sub(previous).Scale(frac))
}

func (g *Game) runtimeLocalViewTeleportActive() bool {
	return g.Client != nil && g.Client.LocalViewTeleportActive()
}

func (g *Game) runtimeCameraState(origin, angles types.Vec3) renderer.CameraState {
	// Apply node-line bias to camera origin to prevent BSP z-fighting.
	// Mirrors C Ironwail: r_refdef.vieworg[i] += 1.0/32 (applied just before R_RenderView).
	cameraOrigin := g.viewNodeLineOffset(origin)

	// Apply V_BoundOffsets to clamp camera relative to entity origin.
	// Mirrors C Ironwail view.c:665-686.
	if g.Client != nil {
		if entityOrigin, ok := g.runtimeAuthoritativePlayerOrigin(); ok {
			cameraOrigin = g.viewBoundOffsets(cameraOrigin, entityOrigin)
		}
	}

	// C Ironwail V_RenderView: SCR_CalcFov decides r_refdef.basefov, the same
	// value the viewmodel+world both share. Passing the live fov cvar here
	// keeps the camera projection (and thus the near-plane depth range the
	// viewmodel draws into) consistent with the world projection and the C
	// reference instead of a hardcoded 96.
	camera := renderer.ConvertClientStateToCamera(cameraOrigin, angles, g.currentRuntimeFOV())
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
				punch := g.runtimeGunKickAngles()
				camera.Angles.X += punch.X
				camera.Angles.Y += punch.Y
				camera.Angles.Z += punch.Z

				// Apply damage kick (V_CalcViewRoll damage kick block).
				// Mirrors C Ironwail view.c:718-722.
				deltaTime := 0.0
				if g.Host != nil {
					deltaTime = g.Host.FrameTime()
				}
				cameraAngles := types.Vec3{X: camera.Angles.X, Y: camera.Angles.Y, Z: camera.Angles.Z}
				cameraAngles = g.viewApplyDamageKick(&g.viewCalc, cameraAngles, deltaTime)
				camera.Angles.X = cameraAngles.X
				camera.Angles.Y = cameraAngles.Y
				camera.Angles.Z = cameraAngles.Z

				// View roll from lateral movement (V_CalcViewRoll).
				roll := g.viewCalcRoll(angles, g.runtimeInterpolatedVelocity())
				camera.Angles.Z += roll

				// Idle sway on the camera (V_AddIdle).
				cameraAngles = types.Vec3{X: camera.Angles.X, Y: camera.Angles.Y, Z: camera.Angles.Z}
				cameraAngles = g.viewAddIdle(cameraAngles, g.Client.Time)
				camera.Angles.X = cameraAngles.X
				camera.Angles.Y = cameraAngles.Y
				camera.Angles.Z = cameraAngles.Z
			}
		}
		camera.Time = float32(g.Client.Time)
	}
	if g.Host.CVar.BoolValue("chase_active") {
		traceFn := g.runtimeChaseTraceFn()
		chaseOrigin, chaseAngles := g.chaseUpdate(
			origin,
			angles,
			float32(g.Host.CVar.FloatValue("chase_back")),
			float32(g.Host.CVar.FloatValue("chase_up")),
			float32(g.Host.CVar.FloatValue("chase_right")),
			traceFn,
		)
		camera.Origin.X = chaseOrigin.X
		camera.Origin.Y = chaseOrigin.Y
		camera.Origin.Z = chaseOrigin.Z
		camera.Angles.X = chaseAngles.X
		camera.Angles.Y = chaseAngles.Y
		camera.Angles.Z = chaseAngles.Z
	}
	// Apply r_waterwarp > 1 FOV oscillation when underwater.
	_, wwFOV, _ := g.runtimeWaterwarpState()
	camera.WaterwarpFOV = wwFOV
	return camera
}

func (g *Game) runtimeChaseTraceFn() chaseTraceFunc {
	if g.Server == nil {
		return nil
	}

	var passEnt *server.Edict
	if g.Client != nil && g.Client.ViewEntity > 0 {
		passEnt = g.Server.EdictNum(g.Client.ViewEntity)
	}

	return func(start, end types.Vec3) types.Vec3 {
		trace := g.Server.SV_Move(start, types.Vec3{}, types.Vec3{}, end, server.MoveType(server.MoveNoMonsters), passEnt)
		return trace.EndPos
	}
}

func (g *Game) runtimeInterpolatedViewAngles() types.Vec3 {
	if g.Client == nil {
		return types.Vec3{}
	}
	return g.Client.ViewAngles
}

func (g *Game) runtimeGunKickAngles() types.Vec3 {
	if g.Client == nil {
		return types.Vec3{}
	}
	mode := 2
	if cv := g.Host.CVar.Get("v_gunkick"); cv != nil {
		mode = cv.Int
	}
	switch mode {
	case 0:
		return types.Vec3{}
	case 1:
		return g.Client.PunchAngle
	default:
		return g.runtimeInterpolatedPunchAngles()
	}
}

func (g *Game) runtimeInterpolatedPunchAngles() types.Vec3 {
	if g.Client == nil {
		return types.Vec3{}
	}
	prev, curr := g.Client.PunchAngles[1], g.Client.PunchAngles[0]
	if prev == (types.Vec3{}) && curr == (types.Vec3{}) {
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

	return prev.Add(curr.Sub(prev).Scale(alpha))
}

func (g *Game) runtimeAngleVectors(angles types.Vec3) (forward, right, up types.Vec3) {
	return cameralib.AngleVectors(angles)
}

func (g *Game) UpdateZoom(dt float64) {
	if g == nil {
		return
	}
	if g.CameraSys != nil {
		g.CameraSys.Zoom = g.Zoom
		g.CameraSys.ZoomDir = g.ZoomDir
		g.CameraSys.UpdateZoom(dt)
		g.Zoom = g.CameraSys.Zoom
		g.ZoomDir = g.CameraSys.ZoomDir
		return
	}
	if g.ZoomDir == 0 {
		return
	}
	g.Zoom += float32(dt) * g.ZoomDir * 5.0
	if g.Zoom < 1.0 {
		g.Zoom = 1.0
		g.ZoomDir = 0
	} else if g.Zoom > 4.0 {
		g.Zoom = 4.0
		g.ZoomDir = 0
	}
}
