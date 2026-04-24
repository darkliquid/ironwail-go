package main

import (
	"fmt"
	"os"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

var debugViewTelemetryCVar *cvar.CVar
var debugViewTelemetryEmit = func(line string) {
	fmt.Fprintln(os.Stderr, line)
}

type debugViewTelemetryState struct {
	frame         uint64
	currentLevel  int
	levelLoaded   bool
	viewModelFrame uint64
	originSelect  runtimeOriginSelectTelemetry
	coalesceKey   string
	coalesceKind  string
	coalesceCount int
}

var runtimeDebugView debugViewTelemetryState

type runtimeOriginSource uint8

const (
	runtimeOriginSourceNone runtimeOriginSource = iota
	runtimeOriginSourceAuthoritativeOnly
	runtimeOriginSourceAuthoritativePredictedXY
	runtimeOriginSourcePredictedFallback
)

func (s runtimeOriginSource) String() string {
	switch s {
	case runtimeOriginSourceAuthoritativeOnly:
		return "authoritative_only"
	case runtimeOriginSourceAuthoritativePredictedXY:
		return "authoritative_plus_predicted_xy"
	case runtimeOriginSourcePredictedFallback:
		return "predicted_fallback"
	default:
		return "none"
	}
}

type runtimeOriginRejectReason uint8

const (
	runtimeOriginRejectNone runtimeOriginRejectReason = iota
	runtimeOriginRejectMissingAuth
	runtimeOriginRejectInvalidPrediction
	runtimeOriginRejectTeleportGate
	runtimeOriginRejectZeroPrediction
	runtimeOriginRejectXYOffsetThreshold
	runtimeOriginRejectPredictionErrorThreshold
)

func (r runtimeOriginRejectReason) String() string {
	switch r {
	case runtimeOriginRejectMissingAuth:
		return "missing_auth"
	case runtimeOriginRejectInvalidPrediction:
		return "invalid_prediction"
	case runtimeOriginRejectTeleportGate:
		return "teleport_gate"
	case runtimeOriginRejectZeroPrediction:
		return "zero_prediction"
	case runtimeOriginRejectXYOffsetThreshold:
		return "xy_offset_threshold"
	case runtimeOriginRejectPredictionErrorThreshold:
		return "prediction_error_threshold"
	default:
		return "none"
	}
}

type runtimeOriginSelectTelemetry struct {
	Source                   runtimeOriginSource
	RejectReason             runtimeOriginRejectReason
	AuthoritativeOrigin      [3]float32
	PredictedOrigin          [3]float32
	PredictionValid          bool
	FinalBaseOrigin          [3]float32
	XYDelta                  [2]float32
	PredictionErrorXY        [2]float32
	XYOffsetThreshold        float32
	PredictionErrorThreshold float32
}

func runtimeDebugViewLevel() int {
	if !runtimeDebugView.levelLoaded {
		runtimeDebugViewReloadLevel()
	}
	return runtimeDebugView.currentLevel
}

func runtimeDebugViewEnabled(level int) bool {
	return runtimeDebugViewLevel() >= level
}

func runtimeDebugViewBeginFrame() {
	runtimeDebugViewFlushCoalescedRepeats()
	runtimeDebugViewReloadLevel()
	if !runtimeDebugViewEnabled(1) {
		return
	}
	runtimeDebugView.frame++
	runtimeDebugView.viewModelFrame = 0
}

func runtimeDebugViewReloadLevel() {
	runtimeDebugView.currentLevel = 0
	if debugViewTelemetryCVar != nil {
		runtimeDebugView.currentLevel = debugViewTelemetryCVar.Int
	}
	runtimeDebugView.levelLoaded = true
}

func runtimeDebugViewLogf(kind, format string, args ...any) {
	if !runtimeDebugViewEnabled(1) {
		return
	}
	clientTime := 0.0
	if g.Client != nil {
		clientTime = g.Client.Time
	}
	payload := fmt.Sprintf(format, args...)
	key := kind + "|" + payload
	if key == runtimeDebugView.coalesceKey {
		runtimeDebugView.coalesceCount++
		return
	}
	runtimeDebugViewFlushCoalescedRepeats()
	runtimeDebugView.coalesceKey = key
	runtimeDebugView.coalesceKind = kind
	runtimeDebugView.coalesceCount = 0
	debugViewTelemetryEmit(fmt.Sprintf("[cldbg frame=%d time=%.3f kind=%s] %s",
		runtimeDebugView.frame, clientTime, kind, payload))
}

func runtimeDebugViewFlushCoalescedRepeats() {
	if runtimeDebugView.coalesceCount > 0 {
		clientTime := 0.0
		if g.Client != nil {
			clientTime = g.Client.Time
		}
		debugViewTelemetryEmit(fmt.Sprintf("[cldbg frame=%d time=%.3f kind=%s] repeated x%d",
			runtimeDebugView.frame, clientTime, runtimeDebugView.coalesceKind, runtimeDebugView.coalesceCount))
	}
	runtimeDebugView.coalesceKey = ""
	runtimeDebugView.coalesceKind = ""
	runtimeDebugView.coalesceCount = 0
}

func runtimeDebugViewRecordOriginSelect(telemetry runtimeOriginSelectTelemetry) {
	runtimeDebugView.originSelect = telemetry
}
