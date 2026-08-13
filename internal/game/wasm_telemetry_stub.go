//go:build !(js && wasm)

package game

// WasmTelemetryEntry represents a recorded milestone in the WASM telemetry ring buffer.
type WasmTelemetryEntry struct {
	TimeMs  int64  `json:"timeMs"`
	Phase   string `json:"phase"`
	Message string `json:"message"`
}

// RecordWasmTelemetry is a no-op on non-WASM targets.
func (g *Game) RecordWasmTelemetry(phase string, msg string) {}

// GetWasmTelemetryLog returns nil on non-WASM targets.
func (g *Game) GetWasmTelemetryLog() []WasmTelemetryEntry {
	return nil
}

// WasmGoroutineSnapshot returns empty telemetry on non-WASM targets.
func WasmGoroutineSnapshot() map[string]any {
	return map[string]any{
		"count": 0,
		"stack": "",
	}
}
