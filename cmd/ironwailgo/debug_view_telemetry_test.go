package main

import (
	"strings"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func TestRuntimeDebugViewLevelCachesPerFrame(t *testing.T) {
	originalCVar := debugViewTelemetryCVar
	originalState := runtimeDebugView
	t.Cleanup(func() {
		debugViewTelemetryCVar = originalCVar
		runtimeDebugView = originalState
	})

	debugViewTelemetryCVar = cvar.Register("cl_debug_view_test_cache", "2", 0, "")
	runtimeDebugView = debugViewTelemetryState{}

	runtimeDebugViewBeginFrame()
	if !runtimeDebugViewEnabled(2) {
		t.Fatal("expected debug level 2 to be enabled after frame start")
	}

	debugViewTelemetryCVar.Int = 0
	if !runtimeDebugViewEnabled(2) {
		t.Fatal("expected cached debug level to remain enabled within the same frame")
	}

	runtimeDebugViewBeginFrame()
	if runtimeDebugViewEnabled(1) {
		t.Fatal("expected debug level cache to refresh on the next frame")
	}
}

func TestRuntimeDebugViewLogfCoalescesDuplicatePayloadsAcrossFrames(t *testing.T) {
	originalCVar := debugViewTelemetryCVar
	originalState := runtimeDebugView
	originalEmit := debugViewTelemetryEmit
	originalClient := g.Client
	t.Cleanup(func() {
		debugViewTelemetryCVar = originalCVar
		runtimeDebugView = originalState
		debugViewTelemetryEmit = originalEmit
		g.Client = originalClient
	})

	debugViewTelemetryCVar = cvar.Register("cl_debug_view_test_coalesce", "2", 0, "")
	runtimeDebugView = debugViewTelemetryState{}
	g.Client = cl.NewClient()
	g.Client.Time = 1

	var lines []string
	debugViewTelemetryEmit = func(line string) {
		lines = append(lines, line)
	}

	runtimeDebugViewBeginFrame()
	runtimeDebugViewLogf("origin_select", "source=%s reject=%s", "authoritative_only", "teleport_gate")
	runtimeDebugViewLogf("origin_select", "source=%s reject=%s", "authoritative_only", "teleport_gate")
	runtimeDebugViewLogf("origin_select", "source=%s reject=%s", "authoritative_only", "teleport_gate")
	g.Client.Time = 2
	runtimeDebugViewBeginFrame()

	if len(lines) != 2 {
		t.Fatalf("logged %d lines, want 2 (%v)", len(lines), lines)
	}
	if !strings.Contains(lines[0], "kind=origin_select") || !strings.Contains(lines[0], "source=authoritative_only reject=teleport_gate") {
		t.Fatalf("first line = %q", lines[0])
	}
	if !strings.Contains(lines[1], "kind=origin_select") || !strings.Contains(lines[1], "repeated x2") {
		t.Fatalf("repeat line = %q", lines[1])
	}
}

func TestRuntimePlayerOriginTelemetryUsesAuthoritativeOriginWhenPredictionAccepted(t *testing.T) {
	originalClient := g.Client
	originalDebugView := runtimeDebugView
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		runtimeDebugView = originalDebugView
		globalViewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.MTime = [2]float64{1, 0.9}
	g.Client.Time = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}, MsgTime: 1}
	g.Client.PredictedOrigin = [3]float32{102, 198, 280}
	markCurrentPredictionFresh(g.Client)

	origin, ok := runtimePlayerOrigin()
	if !ok {
		t.Fatal("runtimePlayerOrigin() reported no origin")
	}
	if want := [3]float32{100, 200, 300}; origin != want {
		t.Fatalf("runtimePlayerOrigin() = %v, want %v", origin, want)
	}
	if runtimeDebugView.originSelect.Source != runtimeOriginSourceAuthoritativePredictedXY {
		t.Fatalf("origin source = %s, want %s", runtimeDebugView.originSelect.Source, runtimeOriginSourceAuthoritativePredictedXY)
	}
	if runtimeDebugView.originSelect.RejectReason != runtimeOriginRejectNone {
		t.Fatalf("origin reject reason = %s, want %s", runtimeDebugView.originSelect.RejectReason, runtimeOriginRejectNone)
	}
	if runtimeDebugView.originSelect.FinalBaseOrigin != origin {
		t.Fatalf("origin telemetry final base = %v, want %v", runtimeDebugView.originSelect.FinalBaseOrigin, origin)
	}
	if runtimeDebugView.originSelect.XYDelta != [2]float32{2, -2} {
		t.Fatalf("origin telemetry XY delta = %v, want [2 -2]", runtimeDebugView.originSelect.XYDelta)
	}
}

func TestRuntimePlayerOriginTelemetryRejectsTeleportPrediction(t *testing.T) {
	originalClient := g.Client
	originalDebugView := runtimeDebugView
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		runtimeDebugView = originalDebugView
		globalViewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.MTime = [2]float64{1, 0.9}
	g.Client.Time = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{512, 256, 128}, MsgTime: 1}
	g.Client.PredictedOrigin = [3]float32{540, 280, 128}
	markCurrentPredictionFresh(g.Client)
	g.Client.LocalViewTeleport = true

	origin, ok := runtimePlayerOrigin()
	if !ok {
		t.Fatal("runtimePlayerOrigin() reported no origin")
	}
	if want := [3]float32{512, 256, 128}; origin != want {
		t.Fatalf("runtimePlayerOrigin() = %v, want %v", origin, want)
	}
	if runtimeDebugView.originSelect.Source != runtimeOriginSourceAuthoritativeOnly {
		t.Fatalf("origin source = %s, want %s", runtimeDebugView.originSelect.Source, runtimeOriginSourceAuthoritativeOnly)
	}
	if runtimeDebugView.originSelect.RejectReason != runtimeOriginRejectTeleportGate {
		t.Fatalf("origin reject reason = %s, want %s", runtimeDebugView.originSelect.RejectReason, runtimeOriginRejectTeleportGate)
	}
}

func TestRuntimePlayerOriginTelemetryRejectsMissingAuthoritativeOriginEvenWithFreshPrediction(t *testing.T) {
	originalClient := g.Client
	originalDebugView := runtimeDebugView
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		runtimeDebugView = originalDebugView
		globalViewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.PredictedOrigin = [3]float32{12, 34, 56}
	markCurrentPredictionFresh(g.Client)

	origin, ok := runtimePlayerOrigin()
	if ok {
		t.Fatalf("runtimePlayerOrigin() = %v, want no origin without authoritative entity", origin)
	}
	if runtimeDebugView.originSelect.Source != runtimeOriginSourceNone {
		t.Fatalf("origin source = %s, want %s", runtimeDebugView.originSelect.Source, runtimeOriginSourceNone)
	}
	if runtimeDebugView.originSelect.RejectReason != runtimeOriginRejectMissingAuth {
		t.Fatalf("origin reject reason = %s, want %s", runtimeDebugView.originSelect.RejectReason, runtimeOriginRejectMissingAuth)
	}
}

func TestRuntimePlayerOriginTelemetryRejectsStalePredictionWithoutAuthoritativeOrigin(t *testing.T) {
	originalClient := g.Client
	originalDebugView := runtimeDebugView
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		runtimeDebugView = originalDebugView
		globalViewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.PredictedOrigin = [3]float32{12, 34, 56}

	origin, ok := runtimePlayerOrigin()
	if ok {
		t.Fatalf("runtimePlayerOrigin() = %v, want no origin when prediction is stale", origin)
	}
	if runtimeDebugView.originSelect.Source != runtimeOriginSourceNone {
		t.Fatalf("origin source = %s, want %s", runtimeDebugView.originSelect.Source, runtimeOriginSourceNone)
	}
	if runtimeDebugView.originSelect.RejectReason != runtimeOriginRejectInvalidPrediction {
		t.Fatalf("origin reject reason = %s, want %s", runtimeDebugView.originSelect.RejectReason, runtimeOriginRejectInvalidPrediction)
	}
}

func TestRuntimePlayerOriginTelemetryKeepsLatchedUnsafeChoiceForServerInterval(t *testing.T) {
	originalClient := g.Client
	originalDebugView := runtimeDebugView
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		runtimeDebugView = originalDebugView
		globalViewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.MTime = [2]float64{1, 0.9}
	g.Client.Time = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}, MsgTime: 1}
	g.Client.PredictedOrigin = [3]float32{105, 200, 280}
	markCurrentPredictionFresh(g.Client)

	origin, ok := runtimePlayerOrigin()
	if !ok {
		t.Fatal("runtimePlayerOrigin() reported no origin")
	}
	if want := [3]float32{100, 200, 300}; origin != want {
		t.Fatalf("first runtimePlayerOrigin() = %v, want %v", origin, want)
	}
	if runtimeDebugView.originSelect.Source != runtimeOriginSourceAuthoritativeOnly {
		t.Fatalf("first origin source = %s, want %s", runtimeDebugView.originSelect.Source, runtimeOriginSourceAuthoritativeOnly)
	}
	if runtimeDebugView.originSelect.RejectReason != runtimeOriginRejectXYOffsetThreshold {
		t.Fatalf("first origin reject reason = %s, want %s", runtimeDebugView.originSelect.RejectReason, runtimeOriginRejectXYOffsetThreshold)
	}

	g.Client.PredictedOrigin = [3]float32{102, 198, 280}

	origin, ok = runtimePlayerOrigin()
	if !ok {
		t.Fatal("runtimePlayerOrigin() reported no origin on second frame")
	}
	if want := [3]float32{100, 200, 300}; origin != want {
		t.Fatalf("second runtimePlayerOrigin() = %v, want latched authoritative origin %v", origin, want)
	}
	if runtimeDebugView.originSelect.Source != runtimeOriginSourceAuthoritativeOnly {
		t.Fatalf("second origin source = %s, want %s", runtimeDebugView.originSelect.Source, runtimeOriginSourceAuthoritativeOnly)
	}
	if runtimeDebugView.originSelect.RejectReason != runtimeOriginRejectXYOffsetThreshold {
		t.Fatalf("second origin reject reason = %s, want %s", runtimeDebugView.originSelect.RejectReason, runtimeOriginRejectXYOffsetThreshold)
	}
	if runtimeDebugView.originSelect.FinalBaseOrigin != origin {
		t.Fatalf("second origin telemetry final base = %v, want %v", runtimeDebugView.originSelect.FinalBaseOrigin, origin)
	}
}

func TestRuntimePlayerOriginTelemetryReevaluatesChoiceOnNewServerInterval(t *testing.T) {
	originalClient := g.Client
	originalDebugView := runtimeDebugView
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		runtimeDebugView = originalDebugView
		globalViewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.MTime = [2]float64{1, 0.9}
	g.Client.Time = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}, MsgTime: 1}
	g.Client.PredictedOrigin = [3]float32{105, 200, 280}
	markCurrentPredictionFresh(g.Client)

	if _, ok := runtimePlayerOrigin(); !ok {
		t.Fatal("runtimePlayerOrigin() reported no origin")
	}

	g.Client.MTime = [2]float64{1.1, 1}
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}, MsgTime: 1.1}
	g.Client.PredictedOrigin = [3]float32{102, 198, 280}

	origin, ok := runtimePlayerOrigin()
	if !ok {
		t.Fatal("runtimePlayerOrigin() reported no origin after new interval")
	}
	if want := [3]float32{100, 200, 300}; origin != want {
		t.Fatalf("runtimePlayerOrigin() after new interval = %v, want %v", origin, want)
	}
	if runtimeDebugView.originSelect.Source != runtimeOriginSourceAuthoritativePredictedXY {
		t.Fatalf("origin source after new interval = %s, want %s", runtimeDebugView.originSelect.Source, runtimeOriginSourceAuthoritativePredictedXY)
	}
	if runtimeDebugView.originSelect.RejectReason != runtimeOriginRejectNone {
		t.Fatalf("origin reject reason after new interval = %s, want %s", runtimeDebugView.originSelect.RejectReason, runtimeOriginRejectNone)
	}
}

func TestRuntimePlayerOriginTelemetryTeleportRelatchesUntilNextInterval(t *testing.T) {
	originalClient := g.Client
	originalDebugView := runtimeDebugView
	originalViewCalc := globalViewCalc
	t.Cleanup(func() {
		g.Client = originalClient
		runtimeDebugView = originalDebugView
		globalViewCalc = originalViewCalc
	})

	g.Client = cl.NewClient()
	g.Client.State = cl.StateActive
	g.Client.ViewEntity = 1
	g.Client.MTime = [2]float64{1, 0.9}
	g.Client.Time = 1
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{100, 200, 300}, MsgTime: 1}
	g.Client.PredictedOrigin = [3]float32{102, 198, 280}
	markCurrentPredictionFresh(g.Client)

	origin, ok := runtimePlayerOrigin()
	if !ok {
		t.Fatal("runtimePlayerOrigin() reported no origin")
	}
	if want := [3]float32{100, 200, 300}; origin != want {
		t.Fatalf("runtimePlayerOrigin() before teleport = %v, want %v", origin, want)
	}

	g.Client.LocalViewTeleport = true
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{512, 256, 128}, MsgTime: 1}
	g.Client.PredictedOrigin = [3]float32{514, 258, 128}
	markCurrentPredictionFresh(g.Client)

	origin, ok = runtimePlayerOrigin()
	if !ok {
		t.Fatal("runtimePlayerOrigin() reported no origin on teleport")
	}
	if want := [3]float32{512, 256, 128}; origin != want {
		t.Fatalf("runtimePlayerOrigin() on teleport = %v, want %v", origin, want)
	}
	if runtimeDebugView.originSelect.Source != runtimeOriginSourceAuthoritativeOnly {
		t.Fatalf("teleport origin source = %s, want %s", runtimeDebugView.originSelect.Source, runtimeOriginSourceAuthoritativeOnly)
	}
	if runtimeDebugView.originSelect.RejectReason != runtimeOriginRejectTeleportGate {
		t.Fatalf("teleport origin reject reason = %s, want %s", runtimeDebugView.originSelect.RejectReason, runtimeOriginRejectTeleportGate)
	}

	g.Client.LocalViewTeleport = false
	g.Client.PredictedOrigin = [3]float32{513, 257, 128}

	origin, ok = runtimePlayerOrigin()
	if !ok {
		t.Fatal("runtimePlayerOrigin() reported no origin after teleport cleared")
	}
	if want := [3]float32{512, 256, 128}; origin != want {
		t.Fatalf("runtimePlayerOrigin() after teleport cleared = %v, want latched authoritative origin %v", origin, want)
	}
	if runtimeDebugView.originSelect.Source != runtimeOriginSourceAuthoritativeOnly {
		t.Fatalf("post-teleport origin source = %s, want %s", runtimeDebugView.originSelect.Source, runtimeOriginSourceAuthoritativeOnly)
	}
	if runtimeDebugView.originSelect.RejectReason != runtimeOriginRejectTeleportGate {
		t.Fatalf("post-teleport origin reject reason = %s, want %s", runtimeDebugView.originSelect.RejectReason, runtimeOriginRejectTeleportGate)
	}

	g.Client.MTime = [2]float64{1.1, 1}
	g.Client.Entities[1] = inet.EntityState{Origin: [3]float32{512, 256, 128}, MsgTime: 1.1}
	g.Client.PredictedOrigin = [3]float32{514, 258, 128}

	origin, ok = runtimePlayerOrigin()
	if !ok {
		t.Fatal("runtimePlayerOrigin() reported no origin after next interval")
	}
	if want := [3]float32{512, 256, 128}; origin != want {
		t.Fatalf("runtimePlayerOrigin() after next interval = %v, want %v", origin, want)
	}
	if runtimeDebugView.originSelect.Source != runtimeOriginSourceAuthoritativePredictedXY {
		t.Fatalf("next-interval origin source = %s, want %s", runtimeDebugView.originSelect.Source, runtimeOriginSourceAuthoritativePredictedXY)
	}
	if runtimeDebugView.originSelect.RejectReason != runtimeOriginRejectNone {
		t.Fatalf("next-interval origin reject reason = %s, want %s", runtimeDebugView.originSelect.RejectReason, runtimeOriginRejectNone)
	}
}
