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
	"math"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/compatrand"
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

// EntVars maps the standard QuakeC entity field structure.
// Each entity (edict) in Quake has this structure stored in its
// private data area. The fields are accessed by QuakeC code and
// the engine to control entity behavior.
//
// Field types:
//   - float32: Numbers (health, ammo, flags, etc.)
//   - [3]float32: Vectors (origin, velocity, angles)
//   - int32: String indices (classname, model, target) or entity references
//
// The layout matches the entvars_t from the original Quake source.
// Fields are stored at fixed offsets determined by the progs.dat file.
type EntVars struct {
	// ModelIndex is the index into the model precache list.
	// 0 = no model, 1+ = precached models.
	ModelIndex float32

	// AbsMin, AbsMax are the absolute bounding box corners.
	// Computed by the engine after origin changes.
	AbsMin [3]float32
	AbsMax [3]float32

	// LTime is the local time for this entity.
	// Used for animations and timed events.
	LTime float32

	// MoveType controls how the entity moves.
	// 0=none, 1=angleclip, 2=angleclamp, 3=fly, 4=toss, 5=bounce, 6=push, 7=noclip
	MoveType float32

	// Solid controls collision detection.
	// 0=not solid, 1=trigger, 2=bbox, 3=slidebox, 4=bsp
	Solid float32

	// Origin is the entity's position in world coordinates.
	Origin [3]float32

	// OldOrigin is the previous frame's position.
	// Used for interpolation and movement prediction.
	OldOrigin [3]float32

	// Velocity is the movement speed in units per second.
	Velocity [3]float32

	// Angles are the entity's orientation (pitch, yaw, roll).
	Angles [3]float32

	// AVelocity is the angular velocity (rotation speed).
	AVelocity [3]float32

	// PunchAngle is the view kick from damage/explosions.
	PunchAngle [3]float32

	// ClassName is the string index of the entity's type.
	// E.g., "info_player_start", "monster_army".
	ClassName int32

	// Model is the string index of the model name.
	Model int32

	// Frame is the current animation frame number.
	Frame float32

	// Skin is the skin number for multi-skin models.
	Skin float32

	// Effects controls visual effects (glow, particles, etc.).
	Effects float32

	// Mins, Maxs define the collision bounding box.
	// Relative to origin.
	Mins [3]float32
	Maxs [3]float32

	// Size is computed as Maxs - Mins.
	Size [3]float32

	// Touch, Use, Think, Blocked are function indices.
	// Called by the engine at appropriate times.
	Touch   int32
	Use     int32
	Think   int32
	Blocked int32

	// NextThink is the game time when Think should be called.
	// -1 = never think.
	NextThink float32

	// GroundEntity is the entity this one is standing on.
	// 0 = in air.
	GroundEntity int32

	// Health is the entity's hit points.
	Health float32

	// Frags is the player's kill count.
	Frags float32

	// Weapon is the current weapon number.
	Weapon float32

	// WeaponModel is the string index of the weapon model.
	WeaponModel int32

	// WeaponFrame is the weapon animation frame.
	WeaponFrame float32

	// CurrentAmmo, Ammo* track ammunition.
	CurrentAmmo float32
	AmmoShells  float32 // Shotgun ammo
	AmmoNails   float32 // Nailgun ammo
	AmmoRockets float32 // Rocket launcher ammo
	AmmoCells   float32 // Lightning gun ammo

	// Items is a bitmask of held items.
	Items float32

	// TakeDamage controls damage susceptibility.
	// 0=immune, 1=normal, 2=armor absorbs damage.
	TakeDamage float32

	// Chain links entities in lists.
	Chain int32

	// DeadFlag tracks death state.
	// 0=alive, 1=dying, 2=dead.
	DeadFlag float32

	// ViewOfs is the eye position offset from origin.
	ViewOfs [3]float32

	// Button0, Button1, Button2 are input buttons.
	// 0=not pressed, 1=pressed.
	Button0 float32 // Attack
	Button1 float32 // (unused)
	Button2 float32 // Jump

	// Impulse is the weapon switch impulse.
	Impulse float32

	// FixAngle forces client view angles.
	FixAngle float32

	// VAngle is the client's view angles.
	VAngle [3]float32

	// IdealPitch is for automatic pitch adjustment.
	IdealPitch float32

	// NetName is the string index of player's name.
	NetName int32

	// Enemy is the entity's current target.
	Enemy int32

	// Flags stores entity state flags.
	// FL_FLY, FL_SWIM, FL_GODMODE, etc.
	Flags float32

	// Colormap is the player color setting.
	Colormap float32

	// Team is the player's team number.
	Team float32

	// MaxHealth is the maximum health.
	MaxHealth float32

	// TeleportTime prevents teleporter recursion.
	TeleportTime float32

	// ArmorType is the armor class (0, 1=green, 2=yellow).
	ArmorType float32

	// ArmorValue is the current armor points.
	ArmorValue float32

	// WaterLevel indicates how submerged the entity is.
	// 0=dry, 1=feet, 2=waist, 3=underwater.
	WaterLevel float32

	// WaterType is the contents type at the entity's position.
	WaterType float32

	// IdealYaw is the desired yaw angle for AI.
	IdealYaw float32

	// YawSpeed is the AI turn rate.
	YawSpeed float32

	// AimEnt is the entity being aimed at.
	AimEnt int32

	// GoalEntity is the AI's navigation target.
	GoalEntity int32

	// SpawnFlags are the entity's spawn parameters.
	SpawnFlags float32

	// Target is the string index of what to trigger.
	Target int32

	// TargetName is the string index of this entity's trigger name.
	TargetName int32

	// DmgTake, DmgSave are damage accumulators.
	DmgTake float32
	DmgSave float32

	// DmgInflictor is the entity causing damage.
	DmgInflictor int32

	// Owner is the entity that created this one.
	Owner int32

	// MoveDir is the movement direction.
	MoveDir [3]float32

	// Message is a string to display.
	Message int32

	// Sounds is the sound preset number.
	Sounds float32

	// Noise, Noise1-3 are sound string indices.
	Noise  int32
	Noise1 int32
	Noise2 int32
	Noise3 int32
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

// VM represents the complete state of a QuakeC Virtual Machine.
// A single VM instance is used to execute one progs.dat file.
// The server and client each have their own VM instances.
//
// VMs must be created with NewVM and initialized with LoadProgs
// before execution.
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

	compatRNG *compatrand.RNG
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
func (vm *VM) GVector(o int) [3]float32 {
	return [3]float32{vm.Globals[o], vm.Globals[o+1], vm.Globals[o+2]}
}

// GString returns a string global value by offset.
// The offset contains a string table index.
func (vm *VM) GString(o int) string {
	idx := vm.GInt(o)
	return vm.GetString(idx)
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
func (vm *VM) SetGVector(o int, v [3]float32) {
	vm.Globals[o] = v[0]
	vm.Globals[o+1] = v[1]
	vm.Globals[o+2] = v[2]
}

// SetGString sets a string global value by offset.
// The string is allocated in the dynamic string table.
func (vm *VM) SetGString(o int, s string) {
	idx := vm.AllocString(s)
	vm.SetGInt(o, idx)
}

// GetString retrieves a string by its table index.
// Positive indices look up in the static Strings array.
// Negative indices look up in the dynamic StringTable.
// Returns empty string for invalid indices.
func (vm *VM) GetString(idx int32) string {
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
// Returns a negative index that can be used with GetString.
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
func (vm *VM) SetGlobal(name string, value interface{}) {
	ofs := vm.FindGlobal(name)
	if ofs < 0 {
		return
	}
	switch v := value.(type) {
	case float32:
		vm.SetGFloat(ofs, v)
	case int:
		vm.SetGInt(ofs, int32(v))
	case int32:
		vm.SetGInt(ofs, v)
	case *Edict:
		vm.SetGInt(ofs, int32(v.Num))
	}
}

// SetGlobalInt sets a global integer variable by name.
// Convenience wrapper around SetGlobal.
func (vm *VM) SetGlobalInt(name string, value int) {
	vm.SetGlobal(name, value)
}

// GetGlobalInt retrieves an integer global by name.
// Returns 0 if the global is not found.
func (vm *VM) GetGlobalInt(name string) int {
	ofs := vm.FindGlobal(name)
	if ofs < 0 {
		return 0
	}
	return int(vm.GInt(ofs))
}

// GetGlobalFloat retrieves a float global by name.
// Returns 0 if the global is not found.
func (vm *VM) GetGlobalFloat(name string) float32 {
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

// Edict represents a server entity for QuakeC integration.
// Num is the entity number (0 = worldspawn).
// Vars provides typed access to the entity's fields.
//
// Edicts are the fundamental game objects in Quake:
// players, monsters, items, triggers, etc.
type Edict struct {
	// Num is the entity number in the edict array.
	Num int

	// Vars is a typed view of the entity's field data.
	// May be nil if not yet accessed.
	Vars *EntVars
}

// Entity Field Access Methods
//
// These methods provide access to entity field data stored in the
// Edicts byte array. Fields are accessed by their offset from the
// start of the entity's private data area.
//
// Entity layout in Edicts array:
//
//   [0..27]: edict prefix/header data
//   [28..]: EntVars fields (EntityFields * 4 bytes)
//
// Field offsets are determined by the progs.dat file and can be
// looked up using FindField.

// EdictData returns a slice of the entity's private data area.
// This is the raw byte storage for EntVars fields.
func (vm *VM) EdictData(edictNum int) []byte {
	if edictNum < 0 || edictNum >= vm.NumEdicts {
		return nil
	}
	offset := edictNum * vm.EdictSize
	if offset+vm.EdictSize > len(vm.Edicts) {
		return nil
	}
	// Skip the edict_t header prefix used before entvars data.
	return vm.Edicts[offset+28 : offset+vm.EdictSize]
}

// EFloat returns a float entity field value.
// The offset is in float-units (multiply by 4 for byte offset).
func (vm *VM) EFloat(edictNum int, fieldOfs int) float32 {
	data := vm.EdictData(edictNum)
	if data == nil || fieldOfs*4+4 > len(data) {
		return 0
	}
	// Read as little-endian float32
	bits := uint32(data[fieldOfs*4]) |
		uint32(data[fieldOfs*4+1])<<8 |
		uint32(data[fieldOfs*4+2])<<16 |
		uint32(data[fieldOfs*4+3])<<24
	return math.Float32frombits(bits)
}

// EInt returns an integer entity field value.
// The offset is in float-units (multiply by 4 for byte offset).
func (vm *VM) EInt(edictNum int, fieldOfs int) int32 {
	// Bit-cast: EFloat reads raw IEEE 754 bits, reinterpret as int32.
	return int32(math.Float32bits(vm.EFloat(edictNum, fieldOfs)))
}

// EVector returns a 3-component vector entity field.
// The offset points to the first component.
func (vm *VM) EVector(edictNum int, fieldOfs int) [3]float32 {
	return [3]float32{
		vm.EFloat(edictNum, fieldOfs),
		vm.EFloat(edictNum, fieldOfs+1),
		vm.EFloat(edictNum, fieldOfs+2),
	}
}

// EString returns a string entity field value.
// Returns the string table index, not the actual string.
func (vm *VM) EString(edictNum int, fieldOfs int) int32 {
	return vm.EInt(edictNum, fieldOfs)
}

// EFunction returns a function reference entity field.
func (vm *VM) EFunction(edictNum int, fieldOfs int) int32 {
	return vm.EInt(edictNum, fieldOfs)
}

// EEntity returns an entity reference entity field.
func (vm *VM) EEntity(edictNum int, fieldOfs int) int32 {
	return vm.EInt(edictNum, fieldOfs)
}

// SetEFloat sets a float entity field value.
func (vm *VM) SetEFloat(edictNum int, fieldOfs int, v float32) {
	data := vm.EdictData(edictNum)
	if data == nil || fieldOfs*4+4 > len(data) {
		return
	}
	bits := math.Float32bits(v)
	data[fieldOfs*4] = byte(bits)
	data[fieldOfs*4+1] = byte(bits >> 8)
	data[fieldOfs*4+2] = byte(bits >> 16)
	data[fieldOfs*4+3] = byte(bits >> 24)
}

// SetEInt sets an integer entity field value.
func (vm *VM) SetEInt(edictNum int, fieldOfs int, v int32) {
	// Bit-cast: store int32 bits as float32 for SetEFloat.
	vm.SetEFloat(edictNum, fieldOfs, math.Float32frombits(uint32(v)))
}

// SetEVector sets a 3-component vector entity field.
func (vm *VM) SetEVector(edictNum int, fieldOfs int, v [3]float32) {
	vm.SetEFloat(edictNum, fieldOfs, v[0])
	vm.SetEFloat(edictNum, fieldOfs+1, v[1])
	vm.SetEFloat(edictNum, fieldOfs+2, v[2])
}

// SetEString sets a string entity field by table index.
func (vm *VM) SetEString(edictNum int, fieldOfs int, stringIdx int32) {
	vm.SetEInt(edictNum, fieldOfs, stringIdx)
}

// SetEFunction sets a function reference entity field.
func (vm *VM) SetEFunction(edictNum int, fieldOfs int, funcNum int32) {
	vm.SetEInt(edictNum, fieldOfs, funcNum)
}

// SetEEntity sets an entity reference entity field.
func (vm *VM) SetEEntity(edictNum int, fieldOfs int, entityNum int32) {
	vm.SetEInt(edictNum, fieldOfs, entityNum)
}

// GetEdict returns the edict for the given entity number.
// Returns nil if the entity number is invalid.
func (vm *VM) GetEdict(num int) *Edict {
	if num < 0 || num >= vm.NumEdicts {
		return nil
	}
	return &Edict{Num: num}
}

// EdictFieldOffset returns the byte offset for an entity field.
// The offset is relative to the start of the entity's private data.
func (vm *VM) EdictFieldOffset(fieldOfs int) int {
	return 28 + fieldOfs*4
}
