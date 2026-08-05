// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package game

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/metrics"
	runtimepprof "runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/darkliquid/ironwail-go/internal/console"
)

func (g *Game) profileBaseDirAndModDir() (baseDir, modDir string) {
	baseDir = "."
	if g.Host != nil && strings.TrimSpace(g.Host.BaseDir()) != "" {
		baseDir = g.Host.BaseDir()
	}
	modDir = strings.TrimSpace(g.ModDir)
	if modDir == "" {
		modDir = "id1"
	}
	return baseDir, modDir
}

func (g *Game) resolveProfileOutputPath(filename, kind string, now time.Time) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = filepath.Join("profiles", fmt.Sprintf("ironwail_%s_%s.pprof", now.Format("20060102_150405"), kind))
	}
	if filepath.IsAbs(filename) {
		return filename
	}
	baseDir, modDir := g.profileBaseDirAndModDir()
	return filepath.Join(baseDir, modDir, filename)
}

func (g *Game) ensureProfileOutputPath(filename, kind string) (string, error) {
	outputPath := g.resolveProfileOutputPath(filename, kind, time.Now())
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", fmt.Errorf("create output directory: %w", err)
	}
	return outputPath, nil
}

func (g *Game) writeNamedRuntimeProfile(kind, filename string) error {
	outputPath, err := g.ensureProfileOutputPath(filename, kind)
	if err != nil {
		return err
	}
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create profile file: %w", err)
	}
	defer func() { _ = f.Close() }()

	runtime.GC()

	switch kind {
	case "heap":
		if err := runtimepprof.WriteHeapProfile(f); err != nil {
			return fmt.Errorf("write heap profile: %w", err)
		}
	case "allocs":
		profile := runtimepprof.Lookup("allocs")
		if profile == nil {
			return fmt.Errorf("allocs profile unavailable")
		}
		if err := profile.WriteTo(f, 0); err != nil {
			return fmt.Errorf("write allocs profile: %w", err)
		}
	default:
		return fmt.Errorf("unknown profile kind %q", kind)
	}

	console.Printf("%s profile saved to %s\n", kind, outputPath)
	return nil
}

func (g *Game) cmdProfileCPUStart(args []string) {
	if len(args) > 1 {
		console.Printf("usage: profile_cpu_start [filename]\n")
		return
	}
	filename := ""
	if len(args) == 1 {
		filename = args[0]
	}
	outputPath, err := g.ensureProfileOutputPath(filename, "cpu")
	if err != nil {
		console.Printf("profile_cpu_start: %v\n", err)
		return
	}

	cpu := &g.cpuProfile
	cpu.mu.Lock()
	defer cpu.mu.Unlock()
	if cpu.file != nil {
		console.Printf("profile_cpu_start: CPU profiling already active (%s)\n", cpu.path)
		return
	}

	f, err := os.Create(outputPath)
	if err != nil {
		console.Printf("profile_cpu_start: create profile file: %v\n", err)
		return
	}
	if err := runtimepprof.StartCPUProfile(f); err != nil {
		_ = f.Close()
		console.Printf("profile_cpu_start: start CPU profile: %v\n", err)
		return
	}

	cpu.file = f
	cpu.path = outputPath
	console.Printf("CPU profile started: %s\n", outputPath)
}

func (g *Game) cmdProfileCPUStop(_ []string) {
	path, active, err := g.stopCPUProfile()
	if !active {
		console.Printf("profile_cpu_stop: CPU profiling is not active\n")
		return
	}
	if err != nil {
		console.Printf("profile_cpu_stop: close profile file: %v\n", err)
		return
	}
	console.Printf("CPU profile saved to %s\n", path)
}

func (g *Game) cmdProfileDumpHeap(args []string) {
	if len(args) > 1 {
		console.Printf("usage: profile_dump_heap [filename]\n")
		return
	}
	filename := ""
	if len(args) == 1 {
		filename = args[0]
	}
	if err := g.writeNamedRuntimeProfile("heap", filename); err != nil {
		console.Printf("profile_dump_heap: %v\n", err)
	}
}

func (g *Game) cmdProfileDumpAllocs(args []string) {
	if len(args) > 1 {
		console.Printf("usage: profile_dump_allocs [filename]\n")
		return
	}
	filename := ""
	if len(args) == 1 {
		filename = args[0]
	}
	if err := g.writeNamedRuntimeProfile("allocs", filename); err != nil {
		console.Printf("profile_dump_allocs: %v\n", err)
	}
}

const (
	// cmdPerfFramesArgIndex is the sole optional positional argument.
	cmdPerfFramesArgIndex = 0
	// cmdPerfDefaultFrames is used when no frame count argument is given.
	cmdPerfDefaultFrames = perfCaptureFrames
)

// perfParseFrames converts the optional [frames] positional argument into a
// positive frame count, or returns defaultFrames when absent or unparsable.
func perfParseFrames(args []string, defaultFrames int) int {
	if len(args) <= cmdPerfFramesArgIndex {
		return defaultFrames
	}
	if v, err := strconv.Atoi(args[cmdPerfFramesArgIndex]); err == nil && v > 0 {
		return v
	}
	return defaultFrames
}

func (g *Game) cmdPerfWarmup(args []string) {
	frames := perfParseFrames(args, cmdPerfDefaultFrames)
	g.perfMeas.phase = perfWarming
	g.perfMeas.frameCount = frames
	g.perfMeas.startFrame = g.Host.FrameCount()
	console.Printf("perf_warmup: measuring %d frames; issue perf_capture to begin steady-state sampling\n", frames)
}

func (g *Game) cmdPerfCapture(args []string) {
	frames := perfParseFrames(args, cmdPerfDefaultFrames)
	if frames <= 0 {
		console.Printf("perf_capture: invalid frame count\n")
		return
	}
	if g.perfMeas.phase != perfWarming {
		console.Printf("perf_capture: no active perf_warmup session; start one first\n")
		return
	}
	g.perfMeas.phase = perfCapturing
	g.perfMeas.frameCount = frames
	g.perfMeas.startFrame = g.Host.FrameCount()
	runtime.GC()
	g.perfMeas.startAllocBytes, g.perfMeas.startAllocObjects = readAllocMetrics()
	// Dump the start heap profile: `go tool pprof -diff_base` between the
	// two window endpoints attributes the per-frame churn (plan 20.1).
	if path, err := g.ensureProfileOutputPath("perf_start_heap.pprof", "heap"); err == nil {
		g.perfMeas.startProfile = path
		if err := g.dumpHeapProfileTo(path); err != nil {
			console.Printf("perf_capture: start heap profile failed: %v\n", err)
			g.perfMeas.startProfile = ""
		}
	}
	g.perfMeas.startTime = time.Now()
	g.perfMeas.totalAlloc = 0
	g.perfMeas.totalObjects = 0
	g.perfMeas.sumSamples = 0
	g.perfMeas.lastSampleFrame = 0
	g.perfMeas.maxAllocPerFrame = 0
	g.perfMeas.maxObjectsPerFrame = 0
	console.Printf("perf_capture: sampling %d steady-state frames\n", frames)
}

// readAllocMetrics returns the cumulative allocation counters from
// runtime/metrics. Unlike runtime.ReadMemStats — a full stop-the-world
// snapshot — these counters are read without pausing the world, so capture
// sampling can be dense without distorting the measurement.
func readAllocMetrics() (bytes, objects uint64) {
	samples := make([]metrics.Sample, 2)
	samples[0].Name = "/gc/heap/allocs:bytes"
	samples[1].Name = "/gc/heap/allocs:objects"
	metrics.Read(samples)
	if samples[0].Value.Kind() == metrics.KindUint64 {
		bytes = samples[0].Value.Uint64()
	}
	if samples[1].Value.Kind() == metrics.KindUint64 {
		objects = samples[1].Value.Uint64()
	}
	return bytes, objects
}

// dumpHeapProfileTo writes a heap pprof to the given path (used for the
// capture-window attribution profiles).
func (g *Game) dumpHeapProfileTo(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := runtimepprof.WriteHeapProfile(f); err != nil {
		return fmt.Errorf("write heap profile %s: %w", path, err)
	}
	return nil
}

func (g *Game) cmdPerfReset(_ []string) {
	if g.perfMeas.phase == perfIdle {
		console.Printf("perf_reset: no active measurement session\n")
		return
	}
	g.perfMeas.phase = perfIdle
	g.perfMeas.frameCount = 0
	g.perfMeas.startFrame = 0
	console.Printf("perf_reset: session cleared\n")
}

// perfTick advances the warmup/capture state machine once per engine frame.
// It is called from the game loop after Host.Frame so per-frame allocation
// counters reflect a complete frame. Callers pass the wall time spent in the
// frame (dt) only for reporting purposes.
func (g *Game) perfTick(dt float64) {
	m := &g.perfMeas
	switch m.phase {
	case perfWarming:
		if g.Host.FrameCount()-m.startFrame >= m.frameCount {
			// Discard the warmup window; perf_capture re-reads the metrics
			// baseline explicitly so stale deltas cannot leak in.
			finish := m.frameCount
			g.perfMeas = perfMeasureState{}
			console.Printf("perf_warmup: complete after %d frames\n", finish)
		}
	case perfCapturing:
		elapsed := g.Host.FrameCount() - m.startFrame
		if elapsed >= m.frameCount {
			g.finishPerfCapture()
			return
		}
		if m.sumSamples == 0 || elapsed%perfSampleInterval == 0 {
			allocsBytes, allocsObjects := readAllocMetrics()

			allocDelta := allocsBytes - m.startAllocBytes
			objDelta := allocsObjects - m.startAllocObjects
			// Normalize cumulative counters to a per-frame rate over the
			// frames elapsed since the last sample.
			framesElapsed := elapsed - m.lastSampleFrame
			if framesElapsed <= 0 {
				framesElapsed = 1
			}
			allocPer := allocDelta / uint64(framesElapsed)
			objectsPer := objDelta / uint64(framesElapsed)
			m.totalAlloc += allocPer
			m.totalObjects += objectsPer
			m.sumSamples++
			if allocPer > m.maxAllocPerFrame {
				m.maxAllocPerFrame = allocPer
			}
			if objectsPer > m.maxObjectsPerFrame {
				m.maxObjectsPerFrame = objectsPer
			}
			m.lastSampleFrame = elapsed
		}
	}
	_ = dt
}

// finishPerfCapture finalizes a completed steady-state capture, emits a
// machine-readable PERF_RESULT line for external harnesses, dumps the end
// heap profile for -diff_base attribution, and returns the session to idle.
func (g *Game) finishPerfCapture() {
	m := &g.perfMeas
	elapsed := time.Since(m.startTime)
	// Dump the end heap profile so the harness can diff start→end with
	// `go tool pprof -diff_base` and attribute the window's allocations.
	if path, err := g.ensureProfileOutputPath("perf_end_heap.pprof", "heap"); err == nil {
		m.endProfile = path
		if err := g.dumpHeapProfileTo(path); err != nil {
			console.Printf("perf_capture: end heap profile failed: %v\n", err)
			m.endProfile = ""
		}
	}
	console.Printf("PERF_RESULT frame_budget %.3f avg_alloc %.0f avg_objects %.0f max_alloc_frame %.0f max_objects_frame %.0f samples %d wall_seconds %.3f\n",
		elapsed.Seconds(), float64(m.totalAlloc)/float64(m.sumSamples),
		float64(m.totalObjects)/float64(m.sumSamples),
		float64(m.maxAllocPerFrame), float64(m.maxObjectsPerFrame),
		m.sumSamples, elapsed.Seconds())
	if m.startProfile != "" || m.endProfile != "" {
		console.Printf("PERF_DELTA start=%s end=%s (diff with: go tool pprof -diff_base=%s %s -sample_index=alloc_objects -top)\n",
			m.startProfile, m.endProfile, m.startProfile, m.endProfile)
	}
	g.perfMeas = perfMeasureState{}
}

func (g *Game) stopCPUProfile() (path string, active bool, err error) {
	return g.StopCPUProfile()
}

// StopCPUProfile stops an active CPU profile capture if one is in progress.
// It returns the output path, whether profiling was active, and any error
// from closing the profile file.
func (g *Game) StopCPUProfile() (path string, active bool, err error) {
	cpu := &g.cpuProfile
	cpu.mu.Lock()
	defer cpu.mu.Unlock()
	if cpu.file == nil {
		return "", false, nil
	}

	runtimepprof.StopCPUProfile()
	path = cpu.path
	err = cpu.file.Close()
	cpu.file = nil
	cpu.path = ""
	return path, true, err
}
