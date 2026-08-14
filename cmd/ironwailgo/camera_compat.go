package main

import "github.com/darkliquid/ironwail-go/pkg/types"

const runtimeMaxPredictedXYOffset = 4.0

func runtimePlayerOrigin() (types.Vec3, bool) {
	telemetry := runtimeOriginSelectTelemetry{
		XYOffsetThreshold:        runtimeMaxPredictedXYOffset,
		PredictionErrorThreshold: runtimeMaxPredictedXYOffset,
	}
	state := &globalViewCalc
	if g == nil || g.Client == nil {
		runtimeResetOriginSelectLatch(state)
		telemetry.RejectReason = runtimeOriginRejectMissingAuth
		runtimeDebugViewRecordOriginSelect(telemetry)
		return types.Vec3{}, false
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
		return types.Vec3{}, false
	}
	telemetry.RejectReason = runtimeOriginRejectMissingAuth
	runtimeDebugViewRecordOriginSelect(telemetry)
	return types.Vec3{}, false
}

func runtimeLatchOriginSelect(state *viewCalcState, authoritativeOrigin types.Vec3) {
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

type runtimePredictedXYDecision struct {
	Origin            types.Vec3
	OK                bool
	RejectReason      runtimeOriginRejectReason
	XYDelta           [2]float32
	PredictionErrorXY [2]float32
}

func runtimeEvaluatePredictedFirstPersonXYOrigin(authoritativeOrigin types.Vec3) runtimePredictedXYDecision {
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
		predictedOrigin.X - authoritativeOrigin.X,
		predictedOrigin.Y - authoritativeOrigin.Y,
	}
	decision.PredictionErrorXY = [2]float32{
		g.Client.PredictionError.X,
		g.Client.PredictionError.Y,
	}

	if runtimeLocalViewTeleportActive() {
		decision.RejectReason = runtimeOriginRejectTeleportGate
		return decision
	}
	if (predictedOrigin == types.Vec3{}) {
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

func runtimeAuthoritativePlayerOrigin() (types.Vec3, bool) {
	if g == nil || g.Client == nil {
		return types.Vec3{}, false
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

	if (g.Client.LastServerOrigin != types.Vec3{}) {
		return g.Client.LastServerOrigin, true
	}

	return types.Vec3{}, false
}

func runtimeLocalViewTeleportActive() bool {
	return g != nil && g.Client != nil && g.Client.LocalViewTeleport
}
