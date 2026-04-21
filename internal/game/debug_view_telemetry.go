package game

import (
	"fmt"
	"os"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

const debugViewTelemetryCVarName = "cl_debug_view"

var debugViewTelemetryCVar *cvar.CVar
var debugViewTelemetryEmit = func(line string) {
	fmt.Fprintln(os.Stderr, line)
}

type debugViewTelemetryState struct {
	frame               uint64
	currentLevel        int
	levelLoaded         bool
	lastEntityOrigin    [3]float32
	lastViewOrigin      [3]float32
	lastViewModelOrigin [3]float32
	haveEntityOrigin    bool
	haveViewOrigin      bool
	haveViewModelOrigin bool
	viewModelFrame      uint64
	originSelect        runtimeOriginSelectTelemetry
	entityCollection    map[string]string
	coalesceKey         string
	coalesceKind        string
	coalesceCount       int
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

func (g *Game) registerDebugViewTelemetryCVar() {
	debugViewTelemetryCVar = cvar.Register(debugViewTelemetryCVarName, "0", 0, "Client view debug telemetry (0=off, 1=view, 2=relink+view+lerp+prediction+origin_select, 3=include viewmodel)")
}

func (g *Game) runtimeDebugViewLevel() int {
	if !runtimeDebugView.levelLoaded {
		g.runtimeDebugViewReloadLevel()
	}
	return runtimeDebugView.currentLevel
}

func (g *Game) runtimeDebugViewEnabled(level int) bool {
	return g.runtimeDebugViewLevel() >= level
}

func (g *Game) runtimeDebugViewBeginFrame() {
	g.runtimeDebugViewFlushCoalescedRepeats()
	g.runtimeDebugViewReloadLevel()
	if !g.runtimeDebugViewEnabled(1) {
		return
	}
	runtimeDebugView.frame++
	runtimeDebugView.viewModelFrame = 0
}

func (g *Game) runtimeDebugViewReloadLevel() {
	runtimeDebugView.currentLevel = 0
	if debugViewTelemetryCVar != nil {
		runtimeDebugView.currentLevel = debugViewTelemetryCVar.Int
	}
	runtimeDebugView.levelLoaded = true
}

func (g *Game) runtimeDebugViewLogf(kind, format string, args ...any) {
	if !g.runtimeDebugViewEnabled(1) {
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
	g.runtimeDebugViewFlushCoalescedRepeats()
	runtimeDebugView.coalesceKey = key
	runtimeDebugView.coalesceKind = kind
	runtimeDebugView.coalesceCount = 0
	debugViewTelemetryEmit(fmt.Sprintf("[cldbg frame=%d time=%.3f kind=%s] %s",
		runtimeDebugView.frame, clientTime, kind, payload))
}

func (g *Game) runtimeDebugViewFlushCoalescedRepeats() {
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

func (g *Game) runtimeDebugViewLogEntityCollection(collector string, entNum int, state inet.EntityState, modelName, status string) {
	if !g.runtimeDebugViewEnabled(2) {
		return
	}
	if runtimeDebugView.entityCollection == nil {
		runtimeDebugView.entityCollection = make(map[string]string)
	}
	key := fmt.Sprintf("%s:%d", collector, entNum)
	summary := fmt.Sprintf("%s|%s|%d", status, modelName, state.ModelIndex)
	if runtimeDebugView.entityCollection[key] == summary {
		return
	}
	runtimeDebugView.entityCollection[key] = summary
	g.runtimeDebugViewLogf(
		"entity",
		"collector=%s ent=%d status=%s model=%q modelindex=%d msgtime=%.3f curtime=%.3f origin=%s",
		collector,
		entNum,
		status,
		modelName,
		state.ModelIndex,
		state.MsgTime,
		g.Client.MTime[0],
		g.debugVec3(state.Origin),
	)
}

func (g *Game) runtimeDebugViewLogRelinkPhase(phase string) {
	if !g.runtimeDebugViewEnabled(2) || g.Client == nil {
		return
	}
	entNum := g.Client.ViewEntity
	state, ok := g.Client.Entities[entNum]
	if !ok {
		g.runtimeDebugViewLogf("relink", "phase=%s ent=%d missing frac=%.3f onground=%t", phase, entNum, g.Client.LerpPoint(), g.Client.OnGround)
		return
	}

	entityDelta := [3]float32{}
	if runtimeDebugView.haveEntityOrigin {
		entityDelta[0] = state.Origin[0] - runtimeDebugView.lastEntityOrigin[0]
		entityDelta[1] = state.Origin[1] - runtimeDebugView.lastEntityOrigin[1]
		entityDelta[2] = state.Origin[2] - runtimeDebugView.lastEntityOrigin[2]
	}
	runtimeDebugView.lastEntityOrigin = state.Origin
	runtimeDebugView.haveEntityOrigin = true

	cmd := g.Client.PendingCmd
	interpVelocity := g.runtimeInterpolatedVelocity()
	g.runtimeDebugViewLogf(
		"relink",
		"phase=%s ent=%d frac=%.3f force=%t tele=%t lerp=0x%x onground=%t msg_prev=%s msg_curr=%s origin=%s d_origin=%s predicted=%s vel=%s ivel=%s cmd=(%.1f %.1f %.1f)",
		phase,
		entNum,
		g.Client.LerpPoint(),
		state.ForceLink,
		g.Client.LocalViewTeleport,
		state.LerpFlags,
		g.Client.OnGround,
		g.debugVec3(state.MsgOrigins[1]),
		g.debugVec3(state.MsgOrigins[0]),
		g.debugVec3(state.Origin),
		g.debugVec3(entityDelta),
		g.debugVec3(g.Client.PredictedOrigin),
		g.debugVec3(g.Client.Velocity),
		g.debugVec3(interpVelocity),
		cmd.Forward,
		cmd.Side,
		cmd.Up,
	)
}

func (g *Game) runtimeDebugViewLogState(viewOrigin, viewAngles [3]float32) {
	if !g.runtimeDebugViewEnabled(1) || g.Client == nil {
		return
	}

	viewDelta := [3]float32{}
	if runtimeDebugView.haveViewOrigin {
		viewDelta[0] = viewOrigin[0] - runtimeDebugView.lastViewOrigin[0]
		viewDelta[1] = viewOrigin[1] - runtimeDebugView.lastViewOrigin[1]
		viewDelta[2] = viewOrigin[2] - runtimeDebugView.lastViewOrigin[2]
	}
	runtimeDebugView.lastViewOrigin = viewOrigin
	runtimeDebugView.haveViewOrigin = true

	authoritativeOrigin, _ := g.runtimeAuthoritativePlayerOrigin()
	bob := g.viewCalcBob(g.Client.Time, g.runtimeInterpolatedVelocity())
	g.runtimeDebugViewLogf(
		"view",
		"auth=%s view=%s d_view=%s angles=%s bob=%.3f viewheight=%.1f onground=%t punch=%s",
		g.debugVec3(authoritativeOrigin),
		g.debugVec3(viewOrigin),
		g.debugVec3(viewDelta),
		g.debugVec3(viewAngles),
		bob,
		g.Client.ViewHeight,
		g.Client.OnGround,
		g.debugVec3(g.Client.PunchAngle),
	)
}

func (g *Game) runtimeDebugViewRecordOriginSelect(telemetry runtimeOriginSelectTelemetry) {
	runtimeDebugView.originSelect = telemetry
}

func (g *Game) runtimeDebugViewLogLerp() {
	if !g.runtimeDebugViewEnabled(2) || g.Client == nil {
		return
	}
	telemetry := g.Client.LerpTelemetrySnapshot()
	rawFrac := "n/a"
	if telemetry.HasRawFrac {
		rawFrac = fmt.Sprintf("%.3f", telemetry.RawFrac)
	}
	g.runtimeDebugViewLogf(
		"lerp",
		"reason=%s time=%.3f->%.3f old=%.3f mtime=(%.3f %.3f)->(%.3f %.3f) f=%.3f->%.3f raw=%s frac=%.3f gap=%t snap=%t",
		telemetry.Reason.String(),
		telemetry.TimeBefore,
		telemetry.TimeAfter,
		telemetry.OldTime,
		telemetry.MTime0Before,
		telemetry.MTime1Before,
		telemetry.MTime0After,
		telemetry.MTime1After,
		telemetry.FrameDeltaBefore,
		telemetry.FrameDeltaAfter,
		rawFrac,
		telemetry.Frac,
		telemetry.GapClamped,
		telemetry.TimeSnapped,
	)
}

func (g *Game) runtimeDebugViewLogPrediction() {
	if !g.runtimeDebugViewEnabled(2) || g.Client == nil {
		return
	}
	telemetry := g.Client.PredictionReplayTelemetrySnapshot()
	oldest := "-"
	newest := "-"
	if telemetry.HasReplayedCmds {
		oldest = g.debugUserCmd(telemetry.OldestReplayedCmd)
		newest = g.debugUserCmd(telemetry.NewestReplayedCmd)
	}
	g.runtimeDebugViewLogf(
		"prediction",
		"time=%.3f ent=%d found=%t valid=%t base_changed=%t server_origin=%s server_vel=%s prev_pred=%s rebased=%s rebased_vel=%s out=%s out_vel=%s cmds=%d->%d replayed=%d fallback=%t pending=%s oldest=%s newest=%s",
		telemetry.FrameTime,
		telemetry.EntityNum,
		telemetry.EntityFound,
		telemetry.Valid,
		telemetry.ServerBaseChanged,
		g.debugVec3(telemetry.ServerBaseOrigin),
		g.debugVec3(telemetry.ServerBaseVelocity),
		g.debugVec3(telemetry.PreviousPredictedOrigin),
		g.debugVec3(telemetry.RebasedPredictedOrigin),
		g.debugVec3(telemetry.RebasedPredictedVelocity),
		g.debugVec3(telemetry.OutputPredictedOrigin),
		g.debugVec3(telemetry.OutputPredictedVelocity),
		telemetry.CommandCountBeforeAck,
		telemetry.CommandCountAfterAck,
		telemetry.ReplayedCommandCount,
		telemetry.UsedPendingCmdFallback,
		g.debugUserCmd(telemetry.PendingCmd),
		oldest,
		newest,
	)
}

func (g *Game) runtimeDebugViewLogOriginSelect() {
	if !g.runtimeDebugViewEnabled(2) {
		return
	}
	telemetry := runtimeDebugView.originSelect
	g.runtimeDebugViewLogf(
		"origin_select",
		"source=%s reject=%s pred_valid=%t auth=%s predicted=%s final=%s d_xy=(%.3f %.3f) pred_err=(%.3f %.3f) xy_thresh=%.3f err_thresh=%.3f",
		telemetry.Source.String(),
		telemetry.RejectReason.String(),
		telemetry.PredictionValid,
		g.debugVec3(telemetry.AuthoritativeOrigin),
		g.debugVec3(telemetry.PredictedOrigin),
		g.debugVec3(telemetry.FinalBaseOrigin),
		telemetry.XYDelta[0],
		telemetry.XYDelta[1],
		telemetry.PredictionErrorXY[0],
		telemetry.PredictionErrorXY[1],
		telemetry.XYOffsetThreshold,
		telemetry.PredictionErrorThreshold,
	)
}

func (g *Game) runtimeDebugViewLogViewModel(entity *renderer.AliasModelEntity) {
	if !g.runtimeDebugViewEnabled(3) || entity == nil || runtimeDebugView.viewModelFrame == runtimeDebugView.frame {
		return
	}
	viewModelDelta := [3]float32{}
	if runtimeDebugView.haveViewModelOrigin {
		viewModelDelta[0] = entity.Origin[0] - runtimeDebugView.lastViewModelOrigin[0]
		viewModelDelta[1] = entity.Origin[1] - runtimeDebugView.lastViewModelOrigin[1]
		viewModelDelta[2] = entity.Origin[2] - runtimeDebugView.lastViewModelOrigin[2]
	}
	runtimeDebugView.lastViewModelOrigin = entity.Origin
	runtimeDebugView.haveViewModelOrigin = true
	runtimeDebugView.viewModelFrame = runtimeDebugView.frame

	g.runtimeDebugViewLogf(
		"viewmodel",
		"origin=%s d_origin=%s angles=%s alpha=%.3f frame=%d",
		g.debugVec3(entity.Origin),
		g.debugVec3(viewModelDelta),
		g.debugVec3(entity.Angles),
		entity.Alpha,
		entity.Frame,
	)
}

func (g *Game) debugVec3(v [3]float32) string {
	return fmt.Sprintf("(%.3f %.3f %.3f)", v[0], v[1], v[2])
}

func (g *Game) debugUserCmd(cmd cl.UserCmd) string {
	return fmt.Sprintf("(%.1f %.1f %.1f msec=%d)", cmd.Forward, cmd.Side, cmd.Up, cmd.Msec)
}
