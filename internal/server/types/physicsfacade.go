// physicsfacade.go defines the interface the per-entity physics leaf
// algorithms (FlyMove, PushMove, PhysicsWalk, ...) need from the server
// facade beyond entity/collision engines. It is implemented by
// *server.Server and injected into the physics subpackage so the leafs can
// be unit-tested in isolation.
package types

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// QCCallback is the set of QuakeC VM dispatch operations the leaf algorithms
// perform (think/touch/blocked function invocation with self/other globals).
type QCCallback interface {
	// RunThink executes the entity's think function if its nextthink time has
	// been reached. Returns false if the entity was freed by its think.
	RunThink(ent *Edict) bool
	// Impact runs touch functions for two collided entities.
	Impact(e1, e2 *Edict)
	// NumForEdict returns the edict index for an edict pointer.
	NumForEdict(ent *Edict) int
	// NumEdicts returns the active edict count.
	GetNumEdicts() int
	// EdictNum returns the edict at index (or nil).
	EdictNum(num int) *Edict
	// ExecuteQCFunction runs a QC function by index.
	ExecuteQCFunction(funcIdx int) error
	// SyncSpawnedEdictsFromQCVM re-links edicts spawned by QC.
	SyncSpawnedEdictsFromQCVM(startEntNum int)
	// SetQCTimeGlobal writes the server time global.
	SetQCTimeGlobal(time float32)
	// GetVM returns the QuakeC VM (may be nil).
	GetVM() *qc.VM
}

// PhysicsFacade bundles the server surfaces the physics leafs need: QC
// callback dispatch, telemetry, cvar reads, sound, and scratch buffer access.
type PhysicsFacade interface {
	QCCallback
	ClientThinker
	TelemetrySink
	CVarReader
	PhysicsConfig
	FrameTiming
	// UserFacing surfaces used by the leafs.
	StartSound(ent *Edict, channel int, sample string, volume int, attenuation float32)
	// FloatValue returns a cvar's float value.
	FloatValue(name string) float64
	// MaxClients returns the configured client slot count.
	MaxClients() int
	// SuppressTouchQC reports whether QC touch callbacks are suppressed.
	SuppressTouchQC() bool
	// DebugTriggerTouch logs a trigger touch debug event.
	DebugTriggerTouch(source string, touch, other *Edict)
	// PushMoveScratch returns the origin-restore scratch buffers (retained
	// across calls to avoid per-frame allocation).
	PushMoveScratch() (moved *[]*Edict, from *[][3]float32)
	// GetFieldGravity returns the QC "gravity" field offset (-1 if absent).
	GetFieldGravity() int
	// CaptureExecutionContext snapshots the QC VM execution context (self/other
	// globals, call depth) so an in-flight QC call can be restored after a
	// nested dispatch. Implemented by the server root.
	CaptureExecutionContext() any
	// RestoreExecutionContext restores a context captured by
	// CaptureExecutionContext.
	RestoreExecutionContext(ctx any)
}
