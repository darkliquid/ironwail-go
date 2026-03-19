package main

import (
	"fmt"

	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

const debugViewTelemetryCVarName = "cl_debug_view"

var debugViewTelemetryCVar *cvar.CVar

type debugViewTelemetryState struct {
	frame               uint64
	lastEntityOrigin    [3]float32
	lastViewOrigin      [3]float32
	lastViewModelOrigin [3]float32
	haveEntityOrigin    bool
	haveViewOrigin      bool
	haveViewModelOrigin bool
	viewModelFrame      uint64
}

var runtimeDebugView debugViewTelemetryState

func registerDebugViewTelemetryCVar() {
	debugViewTelemetryCVar = cvar.Register(debugViewTelemetryCVarName, "0", 0, "Client view debug telemetry (0=off, 1=view, 2=relink+view, 3=include viewmodel)")
}

func runtimeDebugViewLevel() int {
	if debugViewTelemetryCVar == nil {
		return 0
	}
	return debugViewTelemetryCVar.Int
}

func runtimeDebugViewEnabled(level int) bool {
	return runtimeDebugViewLevel() >= level
}

func runtimeDebugViewBeginFrame() {
	if !runtimeDebugViewEnabled(1) {
		return
	}
	runtimeDebugView.frame++
	runtimeDebugView.viewModelFrame = 0
}

func runtimeDebugViewLogf(kind, format string, args ...any) {
	if !runtimeDebugViewEnabled(1) {
		return
	}
	clientTime := 0.0
	if g.Client != nil {
		clientTime = g.Client.Time
	}
	console.Printf("[cldbg frame=%d time=%.3f kind=%s] %s\n",
		runtimeDebugView.frame, clientTime, kind, fmt.Sprintf(format, args...))
}

func runtimeDebugViewLogRelinkPhase(phase string) {
	if !runtimeDebugViewEnabled(2) || g.Client == nil {
		return
	}
	entNum := g.Client.ViewEntity
	state, ok := g.Client.Entities[entNum]
	if !ok {
		runtimeDebugViewLogf("relink", "phase=%s ent=%d missing frac=%.3f onground=%t", phase, entNum, g.Client.LerpPoint(), g.Client.OnGround)
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
	interpVelocity := runtimeInterpolatedVelocity()
	runtimeDebugViewLogf(
		"relink",
		"phase=%s ent=%d frac=%.3f force=%t tele=%t lerp=0x%x onground=%t msg_prev=%s msg_curr=%s origin=%s d_origin=%s predicted=%s vel=%s ivel=%s cmd=(%.1f %.1f %.1f)",
		phase,
		entNum,
		g.Client.LerpPoint(),
		state.ForceLink,
		g.Client.LocalViewTeleport,
		state.LerpFlags,
		g.Client.OnGround,
		debugVec3(state.MsgOrigins[1]),
		debugVec3(state.MsgOrigins[0]),
		debugVec3(state.Origin),
		debugVec3(entityDelta),
		debugVec3(g.Client.PredictedOrigin),
		debugVec3(g.Client.Velocity),
		debugVec3(interpVelocity),
		cmd.Forward,
		cmd.Side,
		cmd.Up,
	)
}

func runtimeDebugViewLogState(viewOrigin, viewAngles [3]float32) {
	if !runtimeDebugViewEnabled(1) || g.Client == nil {
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

	authoritativeOrigin, _ := runtimeAuthoritativePlayerOrigin()
	bob := viewCalcBob(g.Client.Time, runtimeInterpolatedVelocity())
	runtimeDebugViewLogf(
		"view",
		"auth=%s view=%s d_view=%s angles=%s bob=%.3f viewheight=%.1f onground=%t punch=%s",
		debugVec3(authoritativeOrigin),
		debugVec3(viewOrigin),
		debugVec3(viewDelta),
		debugVec3(viewAngles),
		bob,
		g.Client.ViewHeight,
		g.Client.OnGround,
		debugVec3(g.Client.PunchAngle),
	)
}

func runtimeDebugViewLogViewModel(entity *renderer.AliasModelEntity) {
	if !runtimeDebugViewEnabled(3) || entity == nil || runtimeDebugView.viewModelFrame == runtimeDebugView.frame {
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

	runtimeDebugViewLogf(
		"viewmodel",
		"origin=%s d_origin=%s angles=%s alpha=%.3f frame=%d",
		debugVec3(entity.Origin),
		debugVec3(viewModelDelta),
		debugVec3(entity.Angles),
		entity.Alpha,
		entity.Frame,
	)
}

func debugVec3(v [3]float32) string {
	return fmt.Sprintf("(%.3f %.3f %.3f)", v[0], v[1], v[2])
}
