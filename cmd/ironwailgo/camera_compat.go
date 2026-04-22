package main

import cl "github.com/darkliquid/ironwail-go/internal/client"

const runtimeMaxPredictedXYOffset = 4.0

func runtimePlayerOrigin() ([3]float32, bool) {
	telemetry := runtimeOriginSelectTelemetry{
		XYOffsetThreshold:        runtimeMaxPredictedXYOffset,
		PredictionErrorThreshold: runtimeMaxPredictedXYOffset,
	}
	state := &globalViewCalc
	if g == nil || g.Client == nil {
		runtimeResetOriginSelectLatch(state)
		telemetry.RejectReason = runtimeOriginRejectMissingAuth
		runtimeDebugViewRecordOriginSelect(telemetry)
		return [3]float32{}, false
	}
	telemetry.PredictedOrigin = g.Client.PredictedOrigin
	telemetry.PredictionValid = g.Client.HasFreshPredictionForCurrentEntity()

	if authoritativeOrigin, ok := runtimeAuthoritativePlayerOrigin(); ok {
		telemetry.AuthoritativeOrigin = authoritativeOrigin
		runtimeLatchOriginSelect(state, authoritativeOrigin)
		telemetry.Source = state.originSelectLatch.source
		telemetry.XYDelta = state.originSelectLatch.xyDelta
		telemetry.PredictionErrorXY = state.originSelectLatch.predictionErrorXY
		telemetry.RejectReason = state.originSelectLatch.rejectReason
		telemetry.FinalBaseOrigin = authoritativeOrigin
		runtimeDebugViewRecordOriginSelect(telemetry)
		return authoritativeOrigin, true
	}

	runtimeResetOriginSelectLatch(state)
	if !telemetry.PredictionValid {
		telemetry.RejectReason = runtimeOriginRejectInvalidPrediction
		runtimeDebugViewRecordOriginSelect(telemetry)
		return [3]float32{}, false
	}
	telemetry.RejectReason = runtimeOriginRejectMissingAuth
	runtimeDebugViewRecordOriginSelect(telemetry)
	return [3]float32{}, false
}

func runtimeLatchOriginSelect(state *viewCalcState, authoritativeOrigin [3]float32) {
	if state == nil || g == nil || g.Client == nil {
		return
	}
	serverUpdateTime := g.Client.MTime[0]
	if state.originSelectLatch.valid &&
		state.originSelectLatch.client == g.Client &&
		state.originSelectLatch.serverUpdateTime == serverUpdateTime &&
		(state.originSelectLatch.source != runtimeOriginSourceAuthoritativePredictedXY || g.Client.HasFreshPredictionForCurrentEntity()) &&
		!runtimeLocalViewTeleportActive() {
		return
	}

	decision := runtimeEvaluatePredictedFirstPersonXYOrigin(authoritativeOrigin)
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

func runtimeResetOriginSelectLatch(state *viewCalcState) {
	if state == nil {
		return
	}
	state.originSelectLatch = runtimeOriginSelectLatch{}
}

func runtimePredictedFirstPersonXYOrigin(authoritativeOrigin [3]float32) ([3]float32, bool) {
	decision := runtimeEvaluatePredictedFirstPersonXYOrigin(authoritativeOrigin)
	return decision.Origin, decision.OK
}

type runtimePredictedXYDecision struct {
	Origin            [3]float32
	OK                bool
	RejectReason      runtimeOriginRejectReason
	XYDelta           [2]float32
	PredictionErrorXY [2]float32
}

func runtimeEvaluatePredictedFirstPersonXYOrigin(authoritativeOrigin [3]float32) runtimePredictedXYDecision {
	decision := runtimePredictedXYDecision{}
	if g == nil || g.Client == nil {
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

	if runtimeLocalViewTeleportActive() {
		decision.RejectReason = runtimeOriginRejectTeleportGate
		return decision
	}
	if predictedOrigin == [3]float32{} {
		decision.RejectReason = runtimeOriginRejectZeroPrediction
		return decision
	}
	if runtimeFloat32Abs(decision.XYDelta[0]) > runtimeMaxPredictedXYOffset ||
		runtimeFloat32Abs(decision.XYDelta[1]) > runtimeMaxPredictedXYOffset {
		decision.RejectReason = runtimeOriginRejectXYOffsetThreshold
		return decision
	}
	if runtimeFloat32Abs(decision.PredictionErrorXY[0]) > runtimeMaxPredictedXYOffset ||
		runtimeFloat32Abs(decision.PredictionErrorXY[1]) > runtimeMaxPredictedXYOffset {
		decision.RejectReason = runtimeOriginRejectPredictionErrorThreshold
		return decision
	}

	decision.OK = true
	return decision
}

func runtimeFloat32Abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func runtimeAuthoritativePlayerOrigin() ([3]float32, bool) {
	if g == nil || g.Client == nil {
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

func runtimeInterpolatedVelocity() [3]float32 {
	if g == nil || g.Client == nil {
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

func runtimeLocalViewTeleportActive() bool {
	return g != nil && g.Client != nil && g.Client.LocalViewTeleport
}

func runtimeClient() *cl.Client {
	if g == nil {
		return nil
	}
	return g.Client
}
