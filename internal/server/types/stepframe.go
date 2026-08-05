// stepframe.go defines the interfaces the server frame physics loop needs so
// the loop can move into the physics subpackage without importing package
// server. All surfaces are implemented by *server.Server.
package types

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
	srvdebug "github.com/darkliquid/ironwail-go/internal/server/debug"
)

// CVarReader abstracts the cvar lookups the frame loop performs.
// Implemented by *cvar.CVarSystem; defined here so types stays thin.
type CVarReader interface {
	// BoolValue returns the cvar's bool value.
	BoolValue(name string) bool
	// Get returns the cvar, or nil if unregistered.
	Get(name string) CvarHandle
}

// CvarHandle is the minimal cvar handle the loop reads.
type CvarHandle interface {
	Bool() bool
}

// TelemetrySink abstracts the debug telemetry hooks the frame loop emits.
// Implemented by *server.DebugTelemetry.
type TelemetrySink interface {
	EventsEnabled() bool
	BeginFrame(serverTime, frameTime float32)
	EndFrame()
	LogEventf(kind srvdebug.DebugEventKind, vm *qc.VM, entNum int, ent *Edict, format string, args ...any) bool
}

// FrameDriver bundles the remaining server surfaces the frame physics loop
// needs beyond the injected entity store, collision world, and client
// thinker: cvar reads, telemetry, time, static bounds, and dev stats.
type FrameDriver interface {
	CVarReader
	TelemetrySink
	ClientThinker
	// GetTime returns the current server time.
	GetTime() float32
	// GetFrameTime returns the frame delta.
	GetFrameTime() float32
	// MaxClients bound for the freeze-non-clients cap.
	MaxClients() int
	// RecordDevStatsEdicts records the active edict count into dev stats.
	RecordDevStatsEdicts(active int)
	// GetVM returns the QuakeC VM (may be nil).
	GetVM() *qc.VM
	// SyncQCVMGlobals publishes core server globals to the VM.
	SyncQCVMGlobals()
	// SetQCTimeGlobal writes the server time global.
	SetQCTimeGlobal(time float32)
	// ExecuteQCFunction runs a QC function by index.
	ExecuteQCFunction(funcIdx int) error
}
