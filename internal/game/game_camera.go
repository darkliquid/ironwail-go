package game

import (
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/internal/server"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
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

func (g *Game) runtimeSmoothedLocalPlayerBaseOrigin() ([3]float32, bool) {
	origin, ok := g.runtimePlayerOrigin()
	if !ok || g.Client == nil {
		return origin, ok
	}

	state := &g.viewCalc
	entityZ := origin[2]
	frameTime := g.Client.Time
	onGround := g.Client.OnGround
	hardReset := g.runtimeLocalViewTeleportActive()
	if state.stairFrameValid &&
		state.stairFrameTime == frameTime &&
		state.stairFrameEntityZ == entityZ &&
		state.stairFrameOnGround == onGround &&
		state.stairFrameHardReset == hardReset {
		origin[2] = state.stairFrameSmoothedZ
		return origin, true
	}

	origin[2] += g.viewStairSmoothOffset(state, entityZ, onGround, g.runtimeViewDeltaTime(), hardReset)
	state.stairFrameValid = true
	state.stairFrameTime = frameTime
	state.stairFrameEntityZ = entityZ
	state.stairFrameOnGround = onGround
	state.stairFrameHardReset = hardReset
	state.stairFrameSmoothedZ = origin[2]
	return origin, true
}

func (g *Game) runtimeFirstPersonBobOffset() float32 {
	if g.Client == nil {
		return 0
	}
	return g.viewCalcBob(g.Client.Time, g.runtimeInterpolatedVelocity())
}

func (g *Game) runtimeViewState() (origin, angles [3]float32) {
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
		if clientOrigin, ok := g.runtimeSmoothedLocalPlayerBaseOrigin(); ok {
			// Keep the first-person camera anchored to the smoothed eye origin.
			clientOrigin[2] += g.Client.ViewHeight

			var viewAngles [3]float32
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
				clientOrigin[2] += g.runtimeFirstPersonBobOffset()
				viewAngles = g.runtimeInterpolatedViewAngles()
			}
			return clientOrigin, viewAngles
		}
	}

	return origin, angles
}

// runtimeWeaponBaseOrigin returns the weapon model base origin: entity origin + viewheight.
// Mirrors C Ironwail V_CalcRefdef: VectorCopy(ent->origin, view->origin); view->origin[2] += cl.viewheight;
func (g *Game) runtimeWeaponBaseOrigin() [3]float32 {
	if g.Client != nil {
		if clientOrigin, ok := g.runtimeSmoothedLocalPlayerBaseOrigin(); ok {
			clientOrigin[2] += g.Client.ViewHeight
			return clientOrigin
		}
	}
	// Fallback: use the camera origin from runtimeViewState.
	origin, _ := g.runtimeViewState()
	return origin
}

func (g *Game) runtimePlayerOrigin() ([3]float32, bool) {
	telemetry := runtimeOriginSelectTelemetry{
		XYOffsetThreshold:        RuntimeMaxPredictedXYOffset,
		PredictionErrorThreshold: RuntimeMaxPredictedXYOffset,
	}
	state := &g.viewCalc
	if g.Client == nil {
		g.runtimeResetOriginSelectLatch(state)
		telemetry.RejectReason = runtimeOriginRejectMissingAuth
		g.runtimeDebugViewRecordOriginSelect(telemetry)
		return [3]float32{}, false
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
		return [3]float32{}, false
	}
	telemetry.RejectReason = runtimeOriginRejectMissingAuth
	g.runtimeDebugViewRecordOriginSelect(telemetry)
	return [3]float32{}, false
}

func (g *Game) runtimeLatchOriginSelect(state *viewCalcState, authoritativeOrigin [3]float32) {
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

func (g *Game) runtimePredictedFirstPersonXYOrigin(authoritativeOrigin [3]float32) ([3]float32, bool) {
	decision := g.runtimeEvaluatePredictedFirstPersonXYOrigin(authoritativeOrigin)
	return decision.Origin, decision.OK
}

type runtimePredictedXYDecision struct {
	Origin            [3]float32
	OK                bool
	RejectReason      runtimeOriginRejectReason
	XYDelta           [2]float32
	PredictionErrorXY [2]float32
}

func (g *Game) runtimeEvaluatePredictedFirstPersonXYOrigin(authoritativeOrigin [3]float32) runtimePredictedXYDecision {
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
		predictedOrigin[0] - authoritativeOrigin[0],
		predictedOrigin[1] - authoritativeOrigin[1],
	}
	decision.PredictionErrorXY = [2]float32{
		g.Client.PredictionError[0],
		g.Client.PredictionError[1],
	}

	if g.runtimeLocalViewTeleportActive() {
		decision.RejectReason = runtimeOriginRejectTeleportGate
		return decision
	}
	if predictedOrigin == [3]float32{} {
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

func (g *Game) runtimeAuthoritativePlayerOrigin() ([3]float32, bool) {
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

	if g.Client.LastServerOrigin != [3]float32{} {
		return g.Client.LastServerOrigin, true
	}

	return [3]float32{}, false
}

func (g *Game) runtimeInterpolatedVelocity() [3]float32 {
	if g.Client == nil {
		return [3]float32{}
	}

	current := g.Client.MVelocity[0]
	previous := g.Client.MVelocity[1]
	if current == [3]float32{} && previous == [3]float32{} {
		return g.Client.Velocity
	}

	frac := float32(g.Client.LerpPoint())
	if frac <= 0 {
		return previous
	}
	if frac >= 1 {
		return current
	}

	return [3]float32{
		previous[0] + frac*(current[0]-previous[0]),
		previous[1] + frac*(current[1]-previous[1]),
		previous[2] + frac*(current[2]-previous[2]),
	}
}

func (g *Game) runtimeLocalViewTeleportActive() bool {
	return g.Client != nil && g.Client.LocalViewTeleportActive()
}

func (g *Game) runtimeCameraState(origin, angles [3]float32) renderer.CameraState {
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
				punch := g.runtimeGunKickAngles()
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
				cameraAngles = g.viewApplyDamageKick(&g.viewCalc, cameraAngles, deltaTime)
				camera.Angles.X = cameraAngles[0]
				camera.Angles.Y = cameraAngles[1]
				camera.Angles.Z = cameraAngles[2]

				// View roll from lateral movement (V_CalcViewRoll).
				roll := g.viewCalcRoll(angles, g.runtimeInterpolatedVelocity())
				camera.Angles.Z += roll

				// Idle sway on the camera (V_AddIdle).
				cameraAngles = [3]float32{camera.Angles.X, camera.Angles.Y, camera.Angles.Z}
				cameraAngles = g.viewAddIdle(cameraAngles, g.Client.Time)
				camera.Angles.X = cameraAngles[0]
				camera.Angles.Y = cameraAngles[1]
				camera.Angles.Z = cameraAngles[2]
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
		camera.Origin.X = chaseOrigin[0]
		camera.Origin.Y = chaseOrigin[1]
		camera.Origin.Z = chaseOrigin[2]
		camera.Angles.X = chaseAngles[0]
		camera.Angles.Y = chaseAngles[1]
		camera.Angles.Z = chaseAngles[2]
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

	return func(start, end [3]float32) [3]float32 {
		trace := g.Server.SV_Move(start, [3]float32{}, [3]float32{}, end, server.MoveType(server.MoveNoMonsters), passEnt)
		return trace.EndPos
	}
}

func (g *Game) runtimeInterpolatedViewAngles() [3]float32 {
	if g.Client == nil {
		return [3]float32{}
	}
	return g.Client.ViewAngles
}

func (g *Game) runtimeGunKickAngles() [3]float32 {
	if g.Client == nil {
		return [3]float32{}
	}
	mode := 2
	if cv := g.Host.CVar.Get("v_gunkick"); cv != nil {
		mode = cv.Int
	}
	switch mode {
	case 0:
		return [3]float32{}
	case 1:
		return g.Client.PunchAngle
	default:
		return g.runtimeInterpolatedPunchAngles()
	}
}

func (g *Game) angleLerp(prev, curr, frac float32) float32 {
	delta := curr - prev
	for delta > 180 {
		delta -= 360
	}
	for delta < -180 {
		delta += 360
	}
	return prev + delta*frac
}

func (g *Game) runtimeInterpolatedPunchAngles() [3]float32 {
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

func (g *Game) runtimeAngleVectors(angles [3]float32) (forward, right, up [3]float32) {
	forwardVec, rightVec, upVec := qtypes.AngleVectors(qtypes.Vec3{
		X: angles[0],
		Y: angles[1],
		Z: angles[2],
	})
	return [3]float32{forwardVec.X, forwardVec.Y, forwardVec.Z},
		[3]float32{rightVec.X, rightVec.Y, rightVec.Z},
		[3]float32{upVec.X, upVec.Y, upVec.Z}
}
