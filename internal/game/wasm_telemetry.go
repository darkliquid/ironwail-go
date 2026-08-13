//go:build js && wasm

package game

import (
	"runtime"
	"sync"
	"time"
)

// WasmTelemetryEntry represents a recorded milestone in the WASM telemetry ring buffer.
type WasmTelemetryEntry struct {
	TimeMs  int64  `json:"timeMs"`
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

const maxWasmTelemetryEntries = 128

var (
	wasmTelemetryMu  sync.RWMutex
	wasmTelemetryLog []WasmTelemetryEntry
	wasmStartTime    = time.Now()
)

// RecordWasmTelemetry appends a telemetry milestone entry to the WASM ring buffer.
func (g *Game) RecordWasmTelemetry(phase string, msg string) {
	wasmTelemetryMu.Lock()
	defer wasmTelemetryMu.Unlock()

	entry := WasmTelemetryEntry{
		TimeMs:  time.Since(wasmStartTime).Milliseconds(),
		Phase:   phase,
		Message: msg,
	}

	if len(wasmTelemetryLog) >= maxWasmTelemetryEntries {
		wasmTelemetryLog = wasmTelemetryLog[1:]
	}
	wasmTelemetryLog = append(wasmTelemetryLog, entry)
}

// GetWasmTelemetryLog returns a copy of the recorded WASM telemetry milestones.
func (g *Game) GetWasmTelemetryLog() []WasmTelemetryEntry {
	wasmTelemetryMu.RLock()
	defer wasmTelemetryMu.RUnlock()

	out := make([]WasmTelemetryEntry, len(wasmTelemetryLog))
	copy(out, wasmTelemetryLog)
	return out
}

// WasmGoroutineSnapshot returns the current count and stack traces of active goroutines.
func WasmGoroutineSnapshot() map[string]any {
	buf := make([]byte, 64*1024)
	n := runtime.Stack(buf, true)
	return map[string]any{
		"count": runtime.NumGoroutine(),
		"stack": string(buf[:n]),
	}
}
