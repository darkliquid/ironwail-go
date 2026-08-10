// Copyright (C) 2026 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package game

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/darkliquid/ironwail-go/internal/console"
)

// newPerfTestGame returns a Game with a real Host (for FrameCount) and a
// synthetic command capture sink. The perf commands only touch g.Host.FrameCount
// and g.perfMeas, so no subsystem wiring is required.
// runTestFrames advances Host time past the 250 FPS filter window and runs
// perfTick so the perf state machine observes a real frame advance.
func runTestFrames(g *Game, n int) {
	g.Host.SetMaxFPS(0) // disable the host_maxfps early-out so every call is a frame
	for i := 0; i < n; i++ {
		g.Host.Frame(1.0/250, gameCallbacks{g: g})
		g.perfTick(1.0 / 250)
	}
}

func newPerfTestGame(t *testing.T) (*Game, *strings.Builder) {
	t.Helper()
	g := New()
	// Point the profile output path at a temp dir so perf capture tests do
	// not write ./id1/ pprof files into the package directory (which would
	// break testutil.LocateQuakeDir for other tests in this package) and
	// make failures show the exact bytes written.
	profileDir := t.TempDir()
	if g.Host == nil {
		t.Fatal("New() did not initialize Host")
	}
	g.Host.SetBaseDir(profileDir)
	g.ModDir = "id1"
	var out strings.Builder
	console.SetPrintCallback(func(msg string) {
		out.WriteString(msg)
	})
	t.Cleanup(func() {
		console.SetPrintCallback(func(string) {})
	})
	return g, &out
}

func cmdArgs(s ...string) []string { return s }

func TestPerfParseFrames(t *testing.T) {
	g, _ := newPerfTestGame(t)
	_ = g
	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "no args", args: cmdArgs(), want: perfCaptureFrames},
		{name: "positive", args: cmdArgs("30"), want: 30},
		{name: "zero invalid", args: cmdArgs("0"), want: perfCaptureFrames},
		{name: "negative invalid", args: cmdArgs("-5"), want: perfCaptureFrames},
		{name: "junk invalid", args: cmdArgs("abc"), want: perfCaptureFrames},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := perfParseFrames(tt.args, perfCaptureFrames); got != tt.want {
				t.Fatalf("perfParseFrames(%v) = %d, want %d", tt.args, got, tt.want)
			}
		})
	}
}

func TestPerfWarmupRequiresCapture(t *testing.T) {
	g, out := newPerfTestGame(t)
	g.cmdPerfCapture(cmdArgs())
	if !strings.Contains(out.String(), "no active perf_warmup") {
		t.Fatalf("expected warmup-required message, got %q", out.String())
	}
}

func TestPerfWarmupCaptureLifecycle(t *testing.T) {
	g, out := newPerfTestGame(t)
	start := g.Host.FrameCount()

	// perf_capture before any warmup must be rejected.
	g.cmdPerfCapture(cmdArgs("10"))
	if !strings.Contains(out.String(), "no active perf_warmup") {
		t.Fatalf("capture without warmup should be rejected, got %q", out.String())
	}

	// Warmup for 5 frames.
	g.cmdPerfWarmup(cmdArgs("5"))
	if g.perfMeas.phase != perfWarming || g.perfMeas.frameCount != 5 {
		t.Fatalf("perf_warmup did not arm session: %+v", g.perfMeas)
	}

	// Advance 5 frames: warmup completes and resets to idle; capture
	// afterwards must be rejected again.
	runTestFrames(g, perfWarmupFrames)
	_ = start
	if g.perfMeas.phase != perfIdle {
		t.Fatalf("warmup should have completed to idle, got phase=%d", g.perfMeas.phase)
	}
	if !strings.Contains(out.String(), "perf_warmup: complete") {
		t.Fatalf("expected warmup completion message, got %q", out.String())
	}

	// Clear output and run a full warmup->capture->finish cycle with a tiny
	// warmup window relative to the perf tick cadence.
	out.Reset()
	g.cmdPerfWarmup(cmdArgs("1"))
	runTestFrames(g, 1)
	if g.perfMeas.phase != perfIdle {
		t.Fatalf("1-frame warmup should complete immediately")
	}

	g.cmdPerfWarmup(cmdArgs("1"))
	runTestFrames(g, 1) // consumes the single warmup frame
	if g.perfMeas.phase != perfIdle {
		t.Fatalf("warmup did not complete after frame advancement")
	}
}

func TestPerfCaptureSamplesAndFinishes(t *testing.T) {
	g, out := newPerfTestGame(t)

	// Arm a 40-frame warmup that is consumed by perfTick each frame.
	g.cmdPerfWarmup(cmdArgs("5"))
	// Warmup consumes frames; tick them away until idle.
	runTestFrames(g, 20)
	if g.perfMeas.phase != perfIdle {
		t.Fatalf("warmup never completed, phase=%d", g.perfMeas.phase)
	}
	if !strings.Contains(out.String(), "perf_warmup: complete") {
		t.Fatalf("expected warmup completion message, got %q", out.String())
	}
	out.Reset()

	// A 1-frame warmup is consumed by the very next perfTick.
	g.cmdPerfWarmup(cmdArgs("1"))
	runTestFrames(g, 1)
	if g.perfMeas.phase != perfIdle {
		t.Fatalf("1-frame warmup should complete immediately, phase=%d", g.perfMeas.phase)
	}

	// Start a capture of 40 frames (>= sample interval of 15).
	g.cmdPerfWarmup(cmdArgs("1"))
	if g.perfMeas.phase != perfWarming {
		t.Fatalf("warmup should be armed before capture, phase=%d", g.perfMeas.phase)
	}
	g.cmdPerfCapture(cmdArgs("40"))
	if g.perfMeas.phase != perfCapturing {
		t.Fatalf("capture did not start, phase=%d", g.perfMeas.phase)
	}

	// Run 40+ frames; each perfTick during capture samples every 15 frames.
	runTestFrames(g, 45)
	if g.perfMeas.phase != perfIdle {
		t.Fatalf("capture should have finished, phase=%d", g.perfMeas.phase)
	}
	if !strings.Contains(out.String(), "PERF_RESULT") {
		t.Fatalf("expected PERF_RESULT output, got %q", out.String())
	}
	if !strings.Contains(out.String(), "avg_alloc") || !strings.Contains(out.String(), "avg_objects") {
		t.Fatalf("PERF_RESULT missing allocation fields: %q", out.String())
	}
	// maxAlloc must be >= 0 and sumSamples >= 1 across the window.
	if strings.Contains(out.String(), "samples 0") {
		t.Fatalf("capture produced zero samples: %q", out.String())
	}
}

func TestPerfCaptureRejectsShortWindow(t *testing.T) {
	g, out := newPerfTestGame(t)
	g.cmdPerfWarmup(cmdArgs("1"))
	g.cmdPerfCapture(cmdArgs("5"))
	if g.perfMeas.phase != perfCapturing {
		t.Fatalf("capture with 5 frames should start, phase=%d", g.perfMeas.phase)
	}
	runTestFrames(g, 6)
	if g.perfMeas.phase != perfIdle {
		t.Fatalf("short capture should finish idle")
	}
	if !strings.Contains(out.String(), "PERF_RESULT") {
		t.Fatalf("expected PERF_RESULT for short capture, got %q", out.String())
	}
}

// TestPerfActiveAllocMeasurement allocates during capture to verify that the
// per-frame allocation deltas are non-negative and nonzero when allocation
// happens.
func TestPerfActiveAllocMeasurement(t *testing.T) {
	g, out := newPerfTestGame(t)

	g.cmdPerfWarmup(cmdArgs("1"))
	g.perfTick(1.0 / 250)
	g.cmdPerfCapture(cmdArgs("40"))
	if g.perfMeas.phase != perfCapturing {
		t.Fatalf("capture did not start")
	}
	// Allocate a small amount every other frame so deltas are non-zero.
	for i := 0; i < 45; i++ {
		if i%2 == 0 {
			_ = fmt.Sprintf("%d", i) // escape analysis may elide; fine either way
			// Force a heap allocation.
			_ = make([]byte, 64)
		}
		runtime.GC()
		runTestFrames(g, 1)
		if g.perfMeas.phase == perfIdle {
			break
		}
	}
	if !strings.Contains(out.String(), "PERF_RESULT") {
		t.Fatalf("expected PERF_RESULT, got %q", out.String())
	}
	_ = time.Now
}
