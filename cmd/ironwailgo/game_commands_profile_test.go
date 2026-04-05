package main

import (
	"os"
	"path/filepath"
	runtimepprof "runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/darkliquid/ironwail-go/internal/host"
)

func TestResolveProfileOutputPathDefaultsToProfilesDir(t *testing.T) {
	originalHost := g.Host
	originalModDir := g.ModDir
	t.Cleanup(func() {
		g.Host = originalHost
		g.ModDir = originalModDir
	})

	baseDir := t.TempDir()
	g.ModDir = "qbj2"
	h := host.NewHost()
	if err := h.Init(&host.InitParams{BaseDir: baseDir, UserDir: t.TempDir()}, &host.Subsystems{}); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host = h

	got := resolveProfileOutputPath("", "cpu", mustParseProfileTime(t, "2026-04-05T19:00:00Z"))
	want := filepath.Join(baseDir, "qbj2", "profiles", "ironwail_20260405_190000_cpu.pprof")
	if got != want {
		t.Fatalf("resolveProfileOutputPath() = %q, want %q", got, want)
	}
}

func TestProfileCPUStartStopWritesFile(t *testing.T) {
	cpuProfileState.mu.Lock()
	if cpuProfileState.file != nil {
		runtimepprof.StopCPUProfile()
		_ = cpuProfileState.file.Close()
		cpuProfileState.file = nil
		cpuProfileState.path = ""
	}
	cpuProfileState.mu.Unlock()

	path := filepath.Join(t.TempDir(), "cpu.pprof")
	cmdProfileCPUStart([]string{path})
	for i := 0; i < 10000; i++ {
		_ = strings.Repeat("cpu", 4)
	}
	cmdProfileCPUStop(nil)

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(cpu profile): %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("cpu profile file is empty")
	}
}

func TestProfileDumpHeapAndAllocsWriteFiles(t *testing.T) {
	heapPath := filepath.Join(t.TempDir(), "heap.pprof")
	allocsPath := filepath.Join(t.TempDir(), "allocs.pprof")

	cmdProfileDumpHeap([]string{heapPath})
	cmdProfileDumpAllocs([]string{allocsPath})

	for _, path := range []string{heapPath, allocsPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat(%s): %v", path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", path)
		}
	}
}

func mustParseProfileTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("time.Parse(%q): %v", value, err)
	}
	return parsed
}
