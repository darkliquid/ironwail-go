// Package qc implements the QuakeC Virtual Machine.
//
// QuakeC is the scripting language used by Quake for game logic.
// This package provides a complete bytecode interpreter that executes
// compiled .dat progs files containing game behavior.
//
// # Architecture Overview
//
// The QuakeC VM operates on several data structures:
//
//   - Globals: Shared variables accessible by all functions, stored in a flat array
//     of float32 values. Globals are accessed by offset (OFS_* constants).
//   - Entities (Edicts): Game objects with typed fields. Entity data is stored
//     in a contiguous byte array, with field offsets determined by the progs file.
//   - Functions: Both QuakeC-defined (bytecode) and built-in (Go implementations).
//   - String Table: All string literals and dynamically allocated strings.
//   - Stack: Call stack for function invocations and local variables.
//
// # Execution Model
//
// Functions are called with up to 8 parameters, passed through reserved global
// offsets (OFSParm0 through OFSParm7). Return values use OFSReturn.
//
// The instruction pointer (XStatement) advances through bytecode statements.
// Each statement is a 4-word tuple: {opcode, A, B, C} where operands are
// typically global offsets.
//
// # Built-in Functions
//
// Engine integration happens through built-in functions - Go functions that
// the VM can call. Builtins are registered by number and provide access to
// engine services like printing, entity manipulation, physics, etc.
//
// # Entity Fields
//
// Entity fields are defined in the progs file and accessed by offset.
// The EntVars struct maps the standard Quake entity fields for convenient
// access from Go code.
//
// # Thread Safety
//
// VM instances are NOT thread-safe. Each server/client should have its own
// VM instance. Use ExecuteProgram for synchronous execution.
package qc

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/compatrand"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// ProgHeaderCRC is the expected CRC checksum for the original Quake progs.dat.
// Custom mods may have different CRCs; this is used for validation.
const ProgHeaderCRC = 5927

// GlobalVars maps the standard QuakeC global variables.
// These are laid out at specific offsets in the globals array and provide
// the primary interface between the engine and QuakeC code.
//
// The structure mirrors the pr_global_struct_t from the original Quake source.
// Fields are in the exact order expected by the progs.dat format.
//
// Key concepts:
//   - self, other, world: Entity references (edict numbers as float32)
//   - time, frametime: Simulation timing in seconds
//   - mapname: String index pointing to the current map name
//   - trace_*: Results from the last trace line/ray cast operation
//   - *function fields: String indices of QuakeC function entry points
type GlobalVars struct {
	// Pad ensures the struct starts at the correct offset.
	// GlobalVars are loaded at offset ReservedOFS (28) in the globals array.
	Pad [28]int32

	// Self is the entity executing the current QuakeC function.
	// Most QuakeC code operates on 'self' implicitly.
	Self int32

	// Other is the secondary entity in collisions and interactions.
	// Set during touch events, attacks, etc.
	Other int32

	// World is entity 0, the worldspawn entity.
	// Represents the static level geometry.
	World int32

	// Time is the current game time in seconds.
	// Increments each frame by frametime.
	Time float32

	// FrameTime is the duration of the current frame in seconds.
	// Used for movement, physics, and animation timing.
	FrameTime float32

	// ForceRetouch forces entities to re-check touch triggers.
	// Non-zero causes all entities to re-evaluate trigger contacts.
	ForceRetouch float32

	// MapName is the string index of the current map name.
	// E.g., "e1m1", "start".
	MapName int32

	// Deathmatch is the deathmatch mode flag.
	// 0 = single player, 1-2 = various deathmatch rules.
	Deathmatch float32

	// Coop is the cooperative mode flag.
	// Non-zero when playing cooperatively.
	Coop float32

	// Teamplay controls team-based damage rules.
	// Non-zero enables team damage protection.
	Teamplay float32

	// ServerFlags stores persistent server state.
	// Used for episode progression, etc.
	ServerFlags float32

	// TotalSecrets is the count of secrets in the level.
	// Used for end-level statistics.
	TotalSecrets float32

	// TotalMonsters is the count of monsters in the level.
	TotalMonsters float32

	// FoundSecrets is the count of secrets discovered.
	FoundSecrets float32

	// KilledMonsters is the count of monsters killed.
	KilledMonsters float32

	// Parm stores level transition parameters.
	// Used to pass state between levels (health, items, etc.).
	Parm [16]float32

	// VForward, VUp, VRight are the view direction vectors.
	// Set by the engine based on player view angles.
	VForward [3]float32
	VUp      [3]float32
	VRight   [3]float32

	// TraceAllSolid indicates the trace was entirely in solid.
	TraceAllSolid float32

	// TraceStartSolid indicates the trace started in solid.
	TraceStartSolid float32

	// TraceFraction is how far along the ray the trace went.
	// 1.0 = didn't hit anything, 0.0 = hit immediately.
	TraceFraction float32

	// TraceEndPos is the world position where the trace ended.
	TraceEndPos [3]float32

	// TracePlaneNormal is the surface normal at the impact point.
	TracePlaneNormal [3]float32

	// TracePlaneDist is the plane distance at the impact point.
	TracePlaneDist float32

	// TraceEnt is the entity hit by the trace, if any.
	TraceEnt int32

	// TraceInOpen is non-zero if the trace passed through empty space.
	TraceInOpen float32

	// TraceInWater is non-zero if the trace passed through water.
	TraceInWater float32

	// MsgEntity is the entity to receive network messages.
	MsgEntity int32

	// Function entry points (string indices to function table):

	// Main is the initial entry point, called on level load.
	Main int32

	// StartFrame is called at the beginning of each server frame.
	StartFrame int32

	// PlayerPreThink is called before player physics.
	PlayerPreThink int32

	// PlayerPostThink is called after player physics.
	PlayerPostThink int32

	// ClientKill is called when a player uses the 'kill' command.
	ClientKill int32

	// ClientConnect is called when a player joins.
	ClientConnect int32

	// PutClientInServer is called to spawn a player into the level.
	PutClientInServer int32

	// ClientDisconnect is called when a player leaves.
	ClientDisconnect int32

	// SetNewParms is called to initialize level transition parms.
	SetNewParms int32

	// SetChangeParms is called to save parms for level transition.
	SetChangeParms int32
}

// BuiltinFunc is the signature for built-in functions.
// Built-ins are Go functions that QuakeC can call for engine services.
// The VM pointer provides access to globals, entities, and execution state.
//
// Example built-ins include:
//   - print: Output text to console
//   - spawn: Create a new entity
//   - remove: Delete an entity
//   - traceline: Ray cast for collision detection
//   - sound: Play a sound effect
type BuiltinFunc func(vm *VM)

// GetVM returns the VM instance to satisfy the VMProvider interface.
func (vm *VM) GetVM() *VM {
	return vm
}

type VM struct {
	// Progs is the header from the loaded .dat file.
	// Contains metadata about sections and sizes.
	Progs *DProgs

	// Functions is the function table.
	// Index by function number to get DFunction.
	Functions []DFunction

	// Statements is the bytecode instruction array.
	// Executed sequentially with branching via goto/if.
	Statements []DStatement

	// Globals is the global variable storage.
	// Accessed by offset; all values are float32.
	Globals []float32

	// FieldDefs are entity field definitions.
	// Maps field names to offsets in entity data.
	FieldDefs []DDef

	// GlobalDefs are global variable definitions.
	// Maps global names to offsets in Globals.
	GlobalDefs []DDef

	// Strings is the string table from the progs.
	// Null-terminated strings indexed by offset.
	Strings []byte

	// StringTable stores dynamically allocated strings.
	// Negative indices point here; positive to Strings.
	StringTable map[int32]string

	// EdictSize is the size of each entity in bytes.
	EdictSize int

	// Edicts is the entity storage array.
	// Each entity occupies EdictSize bytes.
	Edicts []byte

	// NumEdicts is the current entity count.
	// Entity 0 is always the worldspawn.
	NumEdicts int

	// MaxEdicts is the maximum number of entities.
	MaxEdicts int

	// EntityFields is the number of float fields per entity.
	EntityFields int

	// Builtins is the registered built-in function array.
	Builtins []BuiltinFunc

	// NumBuiltins is the count of registered built-ins.
	NumBuiltins int

	// ArgC is the argument count for the current call.
	ArgC int

	// BuiltinError aborts the current VM execution when set by a builtin.
	// callFunction consumes and returns it as a normal ExecuteProgram error.
	BuiltinError error

	// Trace enables execution tracing for debugging.
	Trace bool

	// TraceFunc is called for each statement when Trace is true.
	TraceFunc func(vm *VM, stmtIdx int, st *DStatement, op Opcode)

	// TraceCallFunc is called on QuakeC function entry/exit and builtin calls
	// when non-nil. Unlike TraceFunc, this is call-oriented and avoids
	// per-statement noise.
	TraceCallFunc func(vm *VM, event TraceCallEvent)

	// BreakHook, when non-nil, is called before each statement executes. If
	// it returns true, the statement loop aborts with ErrBreak WITHOUT
	// unwinding the stack, so a debugger can inspect live state and resume
	// from the exact statement via ExecuteFrom. Zero-overhead when nil
	// (plan 25 Phase C statement debugger).
	BreakHook func(vm *VM, stmtIdx int) bool

	// resumeRequested is set by ExecuteFrom so the interpreter entry skips
	// the XStatement reset and continues where ErrBreak stopped.
	resumeRequested bool

	// XFunction is the currently executing function.
	XFunction *DFunction

	// XFunctionIndex is the index of the currently executing function.
	XFunctionIndex int32

	// XStatement is the current instruction pointer.
	XStatement int

	// RunawayLoopLimit overrides the default per-ExecuteProgram statement budget
	// when > 0. A value <= 0 keeps the Quake-compatible default guard.
	RunawayLoopLimit int

	// CRC is the checksum of the loaded progs.
	CRC uint16

	// Stack is the call stack for function returns.
	Stack []PRStack

	// Depth is the current call stack depth.
	Depth int

	// LocalStack stores local variable values.
	LocalStack []int32

	// LocalUsed is the current local stack usage.
	LocalUsed int

	// Time is the current game time.
	Time float64

	// ReservedEdicts is the count of reserved entities.
	ReservedEdicts int

	// GlobalVars is a typed view of the globals.
	// Points into Globals at the correct offset.
	GlobalVars *GlobalVars

	// IsServerActive reports whether server simulation is fully active.
	// OP_ADDRESS world-entity write checks are gated by this callback to
	// match C behavior (error only when sv.state == ss_active).
	IsServerActive func() bool

	// Cvars provides access to the engine's cvar system (used by the cvar
	// and cvar_set builtins, and various compatibility-fix gates). May be
	// nil in tests; cvar lookups degrade gracefully in that case.
	Cvars *cvar.CVarSystem

	// ServerHooks provides per-VM engine-side implementations of the
	// QuakeC builtins. Injected via SetServerHooks / RegisterServerHooks.
	// Zero value is a valid no-op set (all fields nil).
	ServerHooks ServerBuiltinHooks
	compatRNG   *compatrand.RNG
}

func (vm *VM) SetBuiltinError(err error) {
	if vm == nil || err == nil {
		return
	}
	vm.BuiltinError = err
}

func (vm *VM) consumeBuiltinError() error {
	if vm == nil {
		return nil
	}
	err := vm.BuiltinError
	vm.BuiltinError = nil
	return err
}

// TraceCallEvent describes a QuakeC function-oriented trace event.
type TraceCallEvent struct {
	Phase         string
	Depth         int
	FunctionIndex int32
}

// NewVM creates a new uninitialized QuakeC VM.
// The returned VM has stacks allocated but no progs loaded.
// Call LoadProgs to initialize with a .dat file.
func NewVM() *VM {
	return &VM{
		Builtins:       make([]BuiltinFunc, MaxBuiltins),
		Stack:          make([]PRStack, MaxStackDepth),
		LocalStack:     make([]int32, LocalStackSize),
		StringTable:    make(map[int32]string),
		XFunctionIndex: -1,
		compatRNG:      compatrand.New(),
	}
}

func (vm *VM) SetCompatRNG(rng *compatrand.RNG) {
	if rng == nil {
		vm.compatRNG = compatrand.New()
		return
	}
	vm.compatRNG = rng
}

func (vm *VM) statementBudgetLimit() int {
	if vm != nil && vm.RunawayLoopLimit > 0 {
		return vm.RunawayLoopLimit
	}
	return runawayLoopLimit
}

// GFloat returns a float global value by offset.
// QuakeC stores all values as float32; integers are bit-cast.
func (vm *VM) GFloat(o int) float32 {
	return vm.Globals[o]
}

// GInt returns an integer global value by offset.
// Uses bit-cast (not value conversion) to match C's eval_t union semantics:
// the raw float32 bits are reinterpreted as int32.
func (vm *VM) GInt(o int) int32 {
	return int32(math.Float32bits(vm.Globals[o]))
}

// GVector returns a 3-component vector global by offset.
// Vectors occupy 3 consecutive global slots.
func (vm *VM) GVector(o int) types.Vec3 {
	return types.Vec3{X: vm.Globals[o], Y: vm.Globals[o+1], Z: vm.Globals[o+2]}
}

// GString returns a string global value by offset.
// The offset contains a string table index.
func (vm *VM) GString(o int) string {
	idx := vm.GInt(o)
	return vm.String(idx)
}

// GFunction returns a function reference by offset.
// The value is a function table index.
func (vm *VM) GFunction(o int) int32 {
	return vm.GInt(o)
}

// GEntity returns an entity reference by offset.
// The value is an edict number (0 = world).
func (vm *VM) GEntity(o int) int32 {
	return vm.GInt(o)
}

// SetGFloat sets a float global value by offset.
func (vm *VM) SetGFloat(o int, v float32) {
	vm.Globals[o] = v
}

// SetGInt sets an integer global value by offset.
// Uses bit-cast (not value conversion) to match C's eval_t union semantics:
// the raw int32 bits are reinterpreted as float32.
func (vm *VM) SetGInt(o int, v int32) {
	vm.Globals[o] = math.Float32frombits(uint32(v))
}

// SetGVector sets a 3-component vector global by offset.
// Vectors occupy 3 consecutive global slots.
func (vm *VM) SetGVector(o int, v types.Vec3) {
	vm.Globals[o] = v.X
	vm.Globals[o+1] = v.Y
	vm.Globals[o+2] = v.Z
}

// SetGString sets a string global value by offset.
// The string is allocated in the dynamic string table.
func (vm *VM) SetGString(o int, s string) {
	idx := vm.AllocString(s)
	vm.SetGInt(o, idx)
}

// String retrieves a string by its table index.
// Positive indices look up in the static Strings array.
// Negative indices look up in the dynamic StringTable.
// Returns empty string for invalid indices.
func (vm *VM) String(idx int32) string {
	if idx < 0 {
		if s, ok := vm.StringTable[idx]; ok {
			return s
		}
		return ""
	}
	if int(idx) >= len(vm.Strings) {
		return ""
	}
	end := idx
	for end < int32(len(vm.Strings)) && vm.Strings[end] != 0 {
		end++
	}
	return string(vm.Strings[idx:end])
}

// AllocString allocates a new string in the dynamic table.
// Returns a negative index that can be used with String.
// The same string can be allocated multiple times; each gets
// a distinct index.
func (vm *VM) AllocString(s string) int32 {
	idx := int32(-len(vm.StringTable) - 1)
	vm.StringTable[idx] = s
	return idx
}

// SetEngineString returns a progs string index for an engine-provided string.
// If the string exists in static progs string storage, returns its positive
// offset. Otherwise returns a managed negative knownstring slot.
func (vm *VM) SetEngineString(s string) int32 {
	if s == "" {
		return 0
	}
	if idx, ok := vm.findStaticStringOffset(s); ok {
		return idx
	}
	for idx, existing := range vm.StringTable {
		if existing == s {
			return idx
		}
	}
	return vm.AllocString(s)
}

func (vm *VM) findStaticStringOffset(s string) (int32, bool) {
	if len(vm.Strings) == 0 {
		return 0, false
	}
	start := 0
	for start < len(vm.Strings) {
		end := start
		for end < len(vm.Strings) && vm.Strings[end] != 0 {
			end++
		}
		if string(vm.Strings[start:end]) == s {
			return int32(start), true
		}
		if end >= len(vm.Strings) {
			break
		}
		start = end + 1
	}

	// Some mods may contain non-canonical trailing bytes; allow a conservative
	// fallback search for exact null-terminated matches.
	needle := []byte(s)
	for i := 0; i+len(needle) < len(vm.Strings); i++ {
		if vm.Strings[i+len(needle)] != 0 {
			continue
		}
		if strings.HasPrefix(string(vm.Strings[i:]), s+"\x00") {
			return int32(i), true
		}
	}
	return 0, false
}

// EdictNum converts an entity number to an edict index.
// In this implementation, they are the same value.
func (vm *VM) EdictNum(n int) int {
	return n
}

// NumForEdict converts an edict index to an entity number.
// In this implementation, they are the same value.
func (vm *VM) NumForEdict(e int) int {
	return e
}

// SetGlobal sets a global variable by name using reflection.
// Supports float32, int, int32, and *Edict types.
// Does nothing if the global name is not found.
//
// Example:
//
//	vm.SetGlobal("time", float32(10.5))
//	vm.SetGlobal("self", edict)
func (vm *VM) SetGlobal(name string, value any) {
	def, ok := vm.findGlobalDef(name)
	if !ok {
		return
	}
	ofs := int(def.Ofs)
	typ := EType(def.Type & 0x7fff)
	switch v := value.(type) {
	case float32:
		vm.SetGFloat(ofs, v)
	case int:
		if typ == EvFloat {
			vm.SetGFloat(ofs, float32(v))
		} else {
			vm.SetGInt(ofs, int32(v))
		}
	case int32:
		if typ == EvFloat {
			vm.SetGFloat(ofs, float32(v))
		} else {
			vm.SetGInt(ofs, v)
		}
	case *Edict:
		vm.SetGInt(ofs, int32(v.Num))
	}
}

// SetGlobalInt sets a global integer variable by name.
// Convenience wrapper around SetGlobal.
func (vm *VM) SetGlobalInt(name string, value int) {
	vm.SetGlobal(name, value)
}

// SetGlobalFloat sets a float global by name without boxing the value into
// an interface (avoids convT32/convT64 per call in per-frame paths).
func (vm *VM) SetGlobalFloat(name string, value float32) {
	def, ok := vm.findGlobalDef(name)
	if !ok {
		return
	}
	ofs := int(def.Ofs)
	if EType(def.Type&0x7fff) == EvFloat {
		vm.SetGFloat(ofs, value)
	} else {
		vm.SetGInt(ofs, int32(value))
	}
}

// SetGlobalInt32 sets an int32 global by name without boxing the value into
// an interface. "self"/"other" entity-number globals are written every frame
// through this path.
func (vm *VM) SetGlobalInt32(name string, value int32) {
	def, ok := vm.findGlobalDef(name)
	if !ok {
		return
	}
	ofs := int(def.Ofs)
	if EType(def.Type&0x7fff) == EvFloat {
		vm.SetGFloat(ofs, float32(value))
	} else {
		vm.SetGInt(ofs, value)
	}
}

// GlobalInt retrieves an integer global by name.
// Returns 0 if the global is not found.
func (vm *VM) GlobalInt(name string) int {
	ofs := vm.FindGlobal(name)
	if ofs < 0 {
		return 0
	}
	return int(vm.GInt(ofs))
}

// GlobalFloat retrieves a float global by name.
// Returns 0 if the global is not found.
func (vm *VM) GlobalFloat(name string) float32 {
	ofs := vm.FindGlobal(name)
	if ofs < 0 {
		return 0
	}
	return vm.GFloat(ofs)
}

// ExecuteFunction executes a QuakeC function by number.
// This is a convenience wrapper around ExecuteProgram.
// Returns any error from bytecode execution.
func (vm *VM) ExecuteFunction(fnum int) error {
	return vm.ExecuteProgram(fnum)
}

// ErrBreak is returned by the statement loop when the BreakHook fires
// (plan 25 Phase C). The stack is left intact; resume from the current
// statement with ExecuteFrom.
var ErrBreak = errors.New("qc: breakpoint")

// ExecuteFrom resumes statement execution from the CURRENT XStatement within
// the given function index, using the existing (non-unwound) stack left by an
// ErrBreak. The caller (a debugger) knows which function it broke into, so it
// passes that index; the stack/Depth are authoritative for continuation.
func (vm *VM) ExecuteFrom(fidx int) error {
	if vm.Depth <= 0 {
		return fmt.Errorf("qc: ExecuteFrom with empty stack")
	}
	if fidx < 0 || fidx >= len(vm.Functions) || vm.Functions[fidx].FirstStatement < 0 {
		return fmt.Errorf("qc: ExecuteFrom: not a bytecode function: %d", fidx)
	}
	vm.resumeRequested = true
	defer func() { vm.resumeRequested = false }()
	return vm.ExecuteProgram(fidx)
}
