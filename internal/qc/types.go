// Package qc provides a QuakeC virtual machine implementation.
//
// QuakeC is the scripting language used by Quake for game logic. This package
// implements the bytecode interpreter that executes compiled .qc files (.progs
// dat files).
//
// The VM operates on:
//   - Global variables (shared state across all functions)
//   - Entity fields (per-entity state)
//   - A stack for function calls and local variables
//   - A string table for all string literals
//
// Execution model:
//   - Functions are called with up to 8 parameters
//   - Parameters and return values use reserved global offsets
//   - Built-in functions (implemented in Go) provide engine integration
package qc

// ProgVersion is the expected version number for QuakeC progs files.
const ProgVersion = 6

// VM constants define operational limits.
const (
	// MaxParms is the maximum number of function parameters.
	MaxParms = 8
	// MaxStackDepth is the maximum call stack depth.
	MaxStackDepth = 1024
	// LocalStackSize is the size of the local variable stack.
	LocalStackSize = 16384
	// MaxBuiltins is the maximum number of builtin functions.
	MaxBuiltins = 1280

	// DefSaveGlobal is a flag indicating a global should be saved.
	DefSaveGlobal = 1 << 15
)

// EType represents the type of a QuakeC value.
type EType int

// QuakeC value types.
const (
	EvBad        EType = -1 // Invalid type
	EvVoid       EType = 0  // No value
	EvString     EType = 1  // String value
	EvFloat      EType = 2  // Floating-point value
	EvVector     EType = 3  // 3D vector (3 floats)
	EvEntity     EType = 4  // Entity reference
	EvField      EType = 5  // Entity field offset
	EvFunction   EType = 6  // Function reference
	EvPointer    EType = 7  // Raw pointer
	EvExtInteger EType = 8  // Extended integer type
)

// Global variable offsets in the QuakeC VM.
// These offsets are used to access the globals array.
//
// The first 28 slots are reserved for system use, then the
// GlobalVars structure begins with self, other, world, time, etc.
const (
	OFSNull   = 0  // Null offset
	OFSReturn = 1  // Return value offset (3 slots for vector)
	OFSParm0  = 4  // Parameter 0 offset (3 slots for vector)
	OFSParm1  = 7  // Parameter 1 offset (3 slots for vector)
	OFSParm2  = 10 // Parameter 2 offset (3 slots for vector)
	OFSParm3  = 13 // Parameter 3 offset (3 slots for vector)
	OFSParm4  = 16 // Parameter 4 offset (3 slots for vector)
	OFSParm5  = 19 // Parameter 5 offset (3 slots for vector)
	OFSParm6  = 22 // Parameter 6 offset (3 slots for vector)
	OFSParm7  = 25 // Parameter 7 offset (3 slots for vector)
	// View direction vectors (set by makevectors builtin).
	// These offsets match the standard Quake globalvars layout.
	OFSGlobalVForward = 59 // v_forward global offset (vec3)
	OFSGlobalVUp      = 62 // v_up global offset (vec3)
	OFSGlobalVRight   = 65 // v_right global offset (vec3)

	// Trace result globals populated by `traceline`.
	OFSTraceAllSolid    = 68 // trace_allsolid
	OFSTraceStartSolid  = 69 // trace_startsolid
	OFSTraceFraction    = 70 // trace_fraction
	OFSTraceEndPos      = 71 // trace_endpos (vec3)
	OFSTracePlaneNormal = 74 // trace_plane_normal (vec3)
	OFSTracePlaneDist   = 77 // trace_plane_dist
	OFSTraceEnt         = 78 // trace_ent
	OFSTraceInOpen      = 79 // trace_inopen
	OFSTraceInWater     = 80 // trace_inwater
	OFSMsgEntity        = 81 // msg_entity

	ReservedOFS = 28 // First available global offset (also where GlobalVars begins)

	// GlobalVars field offsets (relative to ReservedOFS=28)
	// These match globalvars_t from progdefs.q1
	OFSSelf           = 28 // self entity
	OFSOther          = 29 // other entity
	OFSWorld          = 30 // world entity
	OFSTime           = 31 // game time
	OFSFrameTime      = 32 // frame delta time
	OFSForceRetouch   = 33 // force entity retouch
	OFSMapName        = 34 // current map name (string index)
	OFSDeathmatch     = 35 // deathmatch mode
	OFSCoop           = 36 // cooperative mode
	OFSTeamplay       = 37 // teamplay rules
	OFSServerFlags    = 38 // server state flags
	OFSTotalSecrets   = 39 // total secrets in level
	OFSTotalMonsters  = 40 // total monsters in level
	OFSFoundSecrets   = 41 // secrets found
	OFSKilledMonsters = 42 // monsters killed
	OFSParmStart      = 43 // parm1..parm16 begin here
)

// Entity field offsets for entvars_t.

// Entity field offsets for entvars_t.
// These match entvars_t from progdefs.q1 and are used for
// direct entity field access in the EFloat/EInt/EVector methods.
const (
	// Entity fields - offsets in float-units
	EntFieldModelIndex   = 0   // modelindex
	EntFieldAbsMin       = 1   // absmin (vec3)
	EntFieldAbsMax       = 4   // absmax (vec3)
	EntFieldLTime        = 7   // ltime
	EntFieldMoveType     = 8   // movetype
	EntFieldSolid        = 9   // solid
	EntFieldOrigin       = 10  // origin (vec3)
	EntFieldOldOrigin    = 13  // oldorigin (vec3)
	EntFieldVelocity     = 16  // velocity (vec3)
	EntFieldAngles       = 19  // angles (vec3)
	EntFieldAVelocity    = 22  // avelocity (vec3)
	EntFieldPunchAngle   = 25  // punchangle (vec3)
	EntFieldClassName    = 28  // classname (string index)
	EntFieldModel        = 29  // model (string index)
	EntFieldFrame        = 30  // frame
	EntFieldSkin         = 31  // skin
	EntFieldEffects      = 32  // effects
	EntFieldMins         = 33  // mins (vec3)
	EntFieldMaxs         = 36  // maxs (vec3)
	EntFieldSize         = 39  // size (vec3)
	EntFieldTouch        = 42  // touch (function index)
	EntFieldUse          = 43  // use (function index)
	EntFieldThink        = 44  // think (function index)
	EntFieldBlocked      = 45  // blocked (function index)
	EntFieldNextThink    = 46  // nextthink
	EntFieldGroundEnt    = 47  // groundentity
	EntFieldHealth       = 48  // health
	EntFieldFrags        = 49  // frags
	EntFieldWeapon       = 50  // weapon
	EntFieldWeaponModel  = 51  // weaponmodel (string index)
	EntFieldWeaponFrame  = 52  // weaponframe
	EntFieldCurrentAmmo  = 53  // currentammo
	EntFieldAmmoShells   = 54  // ammo_shells
	EntFieldAmmoNails    = 55  // ammo_nails
	EntFieldAmmoRockets  = 56  // ammo_rockets
	EntFieldAmmoCells    = 57  // ammo_cells
	EntFieldItems        = 58  // items
	EntFieldTakeDamage   = 59  // takedamage
	EntFieldChain        = 60  // chain
	EntFieldDeadFlag     = 61  // deadflag
	EntFieldViewOfs      = 62  // view_ofs (vec3)
	EntFieldButton0      = 65  // button0
	EntFieldButton1      = 66  // button1
	EntFieldButton2      = 67  // button2
	EntFieldImpulse      = 68  // impulse
	EntFieldFixAngle     = 69  // fixangle
	EntFieldVAngle       = 70  // v_angle (vec3)
	EntFieldIdealPitch   = 73  // idealpitch
	EntFieldNetName      = 74  // netname (string index)
	EntFieldEnemy        = 75  // enemy
	EntFieldFlags        = 76  // flags
	EntFieldColormap     = 77  // colormap
	EntFieldTeam         = 78  // team
	EntFieldMaxHealth    = 79  // max_health
	EntFieldTeleportTime = 80  // teleport_time
	EntFieldArmorType    = 81  // armortype
	EntFieldArmorValue   = 82  // armorvalue
	EntFieldWaterLevel   = 83  // waterlevel
	EntFieldWaterType    = 84  // watertype
	EntFieldIdealYaw     = 85  // ideal_yaw
	EntFieldYawSpeed     = 86  // yaw_speed
	EntFieldAimEnt       = 87  // aiment
	EntFieldGoalEntity   = 88  // goalentity
	EntFieldSpawnFlags   = 89  // spawnflags
	EntFieldTarget       = 90  // target (string index)
	EntFieldTargetName   = 91  // targetname (string index)
	EntFieldDmgTake      = 92  // dmg_take
	EntFieldDmgSave      = 93  // dmg_save
	EntFieldDmgInflictor = 94  // dmg_inflictor
	EntFieldOwner        = 95  // owner
	EntFieldMoveDir      = 96  // movedir (vec3)
	EntFieldMessage      = 99  // message (string index)
	EntFieldSounds       = 100 // sounds
	EntFieldNoise        = 101 // noise (string index)
	EntFieldNoise1       = 102 // noise1 (string index)
	EntFieldNoise2       = 103 // noise2 (string index)
	EntFieldNoise3       = 104 // noise3 (string index)
)

// Opcode represents a QuakeC bytecode instruction.
type Opcode uint16

// QuakeC bytecode opcodes.
const (
	OPDone  Opcode = iota // Stop execution
	OPMulF                // Float multiplication
	OPMulV                // Vector dot product
	OPMulFV               // Float * Vector
	OPMulVF               // Vector * Float
	OPDivF                // Float division
	OPAddF                // Float addition
	OPAddV                // Vector addition
	OPSubF                // Float subtraction
	OPSubV                // Vector subtraction

	OPEqF   // Float equality
	OPEqV   // Vector equality
	OPEqS   // String equality
	OPEqE   // Entity equality
	OPEqFNC // Function equality

	OPNeF   // Float inequality
	OPNeV   // Vector inequality
	OPNeS   // String inequality
	OPNeE   // Entity inequality
	OPNeFNC // Function inequality

	OPLE // Float less-or-equal
	OPGE // Float greater-or-equal
	OPLT // Float less-than
	OPGT // Float greater-than

	OPLoadF   // Load float from pointer
	OPLoadV   // Load vector from pointer
	OPLoadS   // Load string from pointer
	OPLoadEnt // Load entity from pointer
	OPLoadFld // Load field from pointer
	OPLoadFNC // Load function from pointer

	OPAddress // Get address of field

	OPStoreF   // Store float to global
	OPStoreV   // Store vector to global
	OPStoreS   // Store string to global
	OPStoreEnt // Store entity to global
	OPStoreFld // Store field to global
	OPStoreFNC // Store function to global

	OPStorePF   // Store float to pointer
	OPStorePV   // Store vector to pointer
	OPStorePS   // Store string to pointer
	OPStorePEnt // Store entity to pointer
	OPStorePFld // Store field to pointer
	OPStorePFNC // Store function to pointer

	OPReturn // Return from function
	OPNotF   // Logical not (float)
	OPNotV   // Logical not (vector)
	OPNotS   // Logical not (string)
	OPNotEnt // Logical not (entity)
	OPNotFNC // Logical not (function)
	OPIF     // Conditional jump if true
	OPIFNot  // Conditional jump if false
	OPCall0  // Call function (0 args)
	OPCall1  // Call function (1 arg)
	OPCall2  // Call function (2 args)
	OPCall3  // Call function (3 args)
	OPCall4  // Call function (4 args)
	OPCall5  // Call function (5 args)
	OPCall6  // Call function (6 args)
	OPCall7  // Call function (7 args)
	OPCall8  // Call function (8 args)
	OPState  // Set entity state
	OPGoto   // Unconditional jump
	OPAnd    // Logical and
	OPOr     // Logical or

	OPBitAnd // Bitwise and
	OPBitOr  // Bitwise or
)

// DStatement represents a single bytecode instruction.
// A, B, C are operands whose meaning depends on the opcode.
type DStatement struct {
	Op uint16 // Opcode
	A  uint16 // First operand
	B  uint16 // Second operand
	C  uint16 // Third operand (usually destination)
}

// DDef represents a global or field definition in the progs file.
type DDef struct {
	Type uint16 // Value type (EType)
	Ofs  uint16 // Offset in globals or entity fields
	Name int32  // String table index of name
}

// DFunction represents a QuakeC function definition.
type DFunction struct {
	FirstStatement int32          // Index of first statement (negative for builtins)
	ParmStart      int32          // Offset of first parameter
	Locals         int32          // Number of local variables
	Profile        int32          // Profiling counter
	Name           int32          // String table index of function name
	File           int32          // String table index of source file
	NumParms       int32          // Number of parameters
	ParmSize       [MaxParms]byte // Size of each parameter
}

// DProgs is the header structure for .dat progs files.
type DProgs struct {
	Version       int32 // Progs format version (must be 6)
	CRC           int32 // CRC checksum
	Statements    int32 // File offset to statements
	NumStatements int32 // Number of statements
	GlobalDefs    int32 // File offset to global definitions
	NumGlobalDefs int32 // Number of global definitions
	FieldDefs     int32 // File offset to field definitions
	NumFieldDefs  int32 // Number of field definitions
	Functions     int32 // File offset to functions
	NumFunctions  int32 // Number of functions
	Strings       int32 // File offset to string table
	NumStrings    int32 // Size of string table in bytes
	Globals       int32 // File offset to global variables
	NumGlobals    int32 // Number of global variables
	EntityFields  int32 // Number of entity fields
}

// Eval is a union type for evaluating QuakeC values.
type Eval struct {
	String   int32      // String value (index into string table)
	Float    float32    // Floating-point value
	Vector   [3]float32 // Vector value
	Function int32      // Function index
	Int      int32      // Integer value
	Entity   int32      // Entity index
}

// PRStack represents a single frame in the call stack.
type PRStack struct {
	S         int        // Saved statement pointer
	Func      *DFunction // Saved function pointer
	FuncIndex int32      // Saved function index
	LocalBase int        // Saved local stack base
}
