package engine

import "github.com/ironwail/ironwail-go/pkg/qgo/quake"

// These functions are mapped to the native implementations within the engine
// via the //qgo:builtin N directive. The compiler emits function records with negative
// first_statement indices that the engine uses to dispatch calls.

// --- Group 1-10: Fundamental Math and Physics ---

// MakeVectors computes forward, up, and right vectors from a rotation vector.
// The results are stored in the global variables VForward, VUp, and VRight.
//
// Implementation: internal/qc/builtins_math.go:makevectors
//
//qgo:builtin 1
//go:noinline
func MakeVectors(ang quake.Vec3) {}

// SetOrigin moves an entity to a specific position in the world.
// This is an absolute movement that bypasses physics and collision.
//
// Implementation: internal/qc/builtins_entity.go:setorigin
//
//qgo:builtin 2
//go:noinline
func SetOrigin(e *quake.Entity, org quake.Vec3) {}

// SetModel sets the visual model for an entity using a model path (e.g. "progs/player.mdl").
// This also updates the entity's modelindex.
//
// Implementation: internal/qc/builtins_entity.go:setmodel
//
//qgo:builtin 3
//go:noinline
func SetModel(e *quake.Entity, m string) {}

// SetSize sets the bounding box (mins and maxs) for an entity's physics.
// The bounding box is relative to the entity's origin.
//
// Implementation: internal/qc/builtins_entity.go:setsize
//
//qgo:builtin 4
//go:noinline
func SetSize(e *quake.Entity, min, max quake.Vec3) {}

// BreakStatement triggers a debugger break if a debugger is attached to the VM.
//
// Implementation: internal/qc/builtins.go:breakBuiltin
//
//qgo:builtin 6
//go:noinline
func BreakStatement() {}

// Random returns a random float value in the range [0, 1).
// The endpoints depend on the 'sv_gameplayfix_random' console variable.
//
// Implementation: internal/qc/builtins_math.go:random
//
//qgo:builtin 7
//go:noinline
func Random() float32 { return 0 }

// Sound plays a sound effect from the specified entity.
//
// Implementation: internal/qc/builtins_world.go:sound
//
//qgo:builtin 8
//go:noinline
func Sound(e *quake.Entity, ch int, samp string, vol float32, atten float32) {}

// Normalize returns a vector with the same direction as the input but with a length of 1.
//
// Implementation: internal/qc/builtins_math.go:normalize
//
//qgo:builtin 9
//go:noinline
func Normalize(v quake.Vec3) quake.Vec3 { return v }

// Error prints a fatal error message to the console and halts the server.
//
// Implementation: internal/qc/builtins.go:errorBuiltin
//
//qgo:builtin 10
//go:noinline
func Error(s string) {}

// --- Group 11-20: Math and Entity Management ---

// ObjError prints a fatal error message related to a specific entity and halts the server.
//
// Implementation: internal/qc/builtins.go:objerrorBuiltin
//
//qgo:builtin 11
//go:noinline
func ObjError(s string) {}

// Vlen returns the length (magnitude) of a 3D vector.
//
// Implementation: internal/qc/builtins_math.go:vlen
//
//qgo:builtin 12
//go:noinline
func Vlen(v quake.Vec3) float32 { return 0 }

// Vectoyaw returns the yaw angle (0-360) that a direction vector points towards.
//
// Implementation: internal/qc/builtins_math.go:vectoyaw
//
//qgo:builtin 13
//go:noinline
func Vectoyaw(v quake.Vec3) float32 { return 0 }

// Spawn creates a new, empty entity in the game world and returns its handle.
//
// Implementation: internal/qc/builtins_entity.go:spawn
//
//qgo:builtin 14
//go:noinline
func Spawn() *quake.Entity { return nil }

// Remove deletes an entity from the game world and deallocates its slot.
//
// Implementation: internal/qc/builtins_entity.go:remove
//
//qgo:builtin 15
//go:noinline
func Remove(e *quake.Entity) {}

// Traceline performs a ray-cast from v1 to v2 and stores the results in trace globals.
//
// Implementation: internal/qc/builtins_world.go:traceline
//
//qgo:builtin 16
//go:noinline
func Traceline(v1, v2 quake.Vec3, nomonsters float32, e *quake.Entity) {}

// CheckClient returns a client entity that is currently visible to the executing entity.
//
// Implementation: internal/qc/builtins_world.go:checkclient
//
//qgo:builtin 17
//go:noinline
func CheckClient() *quake.Entity { return nil }

// Find locates an entity whose string field (e.g. "classname") matches the specified value.
// The search starts from the entity after 'e'.
//
// Implementation: internal/qc/builtins_entity.go:find
//
//qgo:builtin 18
//go:noinline
func Find(e *quake.Entity, field string, match string) *quake.Entity { return nil }

// PrecacheSound registers a sound file path so it can be loaded and played.
//
// Implementation: internal/qc/builtins_entity.go:precacheSound
//
//qgo:builtin 19
//go:noinline
func PrecacheSound(s string) string { return s }

// PrecacheModel registers a model file path so it can be loaded and used.
//
// Implementation: internal/qc/builtins_entity.go:precacheModel
//
//qgo:builtin 20
//go:noinline
func PrecacheModel(s string) string { return s }

// --- Group 21-30: Communication and Strings ---

// StuffCmd sends a command string to a client's console to be executed.
//
// Implementation: internal/qc/builtins_entity.go:stuffcmd
//
//qgo:builtin 21
//go:noinline
func StuffCmd(e *quake.Entity, s string) {}

// FindRadius returns a linked chain of entities within a specific radius of a point.
//
// Implementation: internal/qc/builtins_entity.go:findradius
//
//qgo:builtin 22
//go:noinline
func FindRadius(org quake.Vec3, radius float32) *quake.Entity { return nil }

// Bprint broadcasts a message to all connected clients and the engine console.
//
// Implementation: internal/qc/builtins.go:bprint
//
//qgo:builtin 23
//go:noinline
func Bprint(s string) {}

// SPrint sends a message to a specific client's console.
//
// Implementation: internal/qc/builtins.go:sprint
//
//qgo:builtin 24
//go:noinline
func SPrint(e *quake.Entity, s string) {}

// Print prints a message to the engine console.
//
// Implementation: internal/qc/builtins.go:print
//
//qgo:builtin 24
//go:noinline
func Print(s string) {}

// Dprint prints a message to the engine console only if the 'developer' cvar is 1.
//
// Implementation: internal/qc/builtins.go:dprint
//
//qgo:builtin 25
//go:noinline
func Dprint(s string) {}

// Ftos converts a float value to its string representation.
//
// Implementation: internal/qc/builtins.go:ftosBuiltin
//
//qgo:builtin 26
//go:noinline
func Ftos(f float32) string { return "" }

// Vtos converts a vector value to its string representation (e.g. "'1 2 3'").
//
// Implementation: internal/qc/builtins.go:vtosBuiltin
//
//qgo:builtin 27
//go:noinline
func Vtos(v quake.Vec3) string { return "" }

// --- Group 31-40: Physics and Rounding ---

// EPrint prints all fields of an entity to the engine console for debugging.
//
// Implementation: internal/qc/builtins.go:eprint
//
//qgo:builtin 31
//go:noinline
func EPrint(e *quake.Entity) {}

// WalkMove moves an entity forward in its current direction, checking for collisions.
//
// Implementation: internal/qc/builtins_world.go:walkmove
//
//qgo:builtin 32
//go:noinline
func WalkMove(yaw float32, dist float32) float32 { return 0 }

// DropToFloor moves an entity vertically down until it hits a solid floor.
//
// Implementation: internal/qc/builtins_world.go:droptofloor
//
//qgo:builtin 34
//go:noinline
func DropToFloor() float32 { return 0 }

// LightStyle sets the illumination pattern for a specific light style index (0-63).
//
// Implementation: internal/qc/builtins_world.go:lightstyle
//
//qgo:builtin 35
//go:noinline
func LightStyle(style float32, value string) {}

// RInt returns the nearest integer value to the input float.
//
// Implementation: internal/qc/builtins_math.go:rintBuiltin
//
//qgo:builtin 36
//go:noinline
func RInt(f float32) float32 { return 0 }

// Floor returns the largest integer value less than or equal to the input float.
//
// Implementation: internal/qc/builtins_math.go:floorBuiltin
//
//qgo:builtin 37
//go:noinline
func Floor(f float32) float32 { return 0 }

// Ceil returns the smallest integer value greater than or equal to the input float.
//
// Implementation: internal/qc/builtins_math.go:ceilBuiltin
//
//qgo:builtin 38
//go:noinline
func Ceil(f float32) float32 { return 0 }

// CheckBottom returns true if the entity's bounding box is supported by a floor.
//
// Implementation: internal/qc/builtins_world.go:checkbottom
//
//qgo:builtin 40
//go:noinline
func CheckBottom(e *quake.Entity) float32 { return 0 }

// --- Group 41-50: World Queries and AI ---

// PointContents returns the contents of the map at a specific point (e.g. empty, water, lava).
//
// Implementation: internal/qc/builtins_world.go:pointcontents
//
//qgo:builtin 41
//go:noinline
func PointContents(p quake.Vec3) float32 { return 0 }

// FAbs returns the absolute value of the input float.
//
// Implementation: internal/qc/builtins_math.go:fabsBuiltin
//
//qgo:builtin 43
//go:noinline
func FAbs(f float32) float32 { return 0 }

// Aim returns a direction vector pointing towards a target for a missile with the given speed.
//
// Implementation: internal/qc/builtins_world.go:aimBuiltin
//
//qgo:builtin 44
//go:noinline
func Aim(e *quake.Entity, speed float32) quake.Vec3 { return quake.Vec3{} }

// Cvar returns the current float value of a console variable.
//
// Implementation: internal/qc/builtins.go:cvarBuiltin
//
//qgo:builtin 45
//go:noinline
func Cvar(s string) float32 { return 0 }

// LocalCmd appends text to the engine's local command buffer for execution.
//
// Implementation: internal/qc/builtins.go:localcmd
//
//qgo:builtin 46
//go:noinline
func LocalCmd(s string) {}

// NextEnt returns the next entity in the world's entity list after the given entity.
//
// Implementation: internal/qc/builtins_entity.go:nextent
//
//qgo:builtin 47
//go:noinline
func GetNextEnt(e *quake.Entity) *quake.Entity { return nil }

// Particle spawns a group of particles at the specified origin.
//
// Implementation: internal/qc/builtins_world.go:particle
//
//qgo:builtin 48
//go:noinline
func Particle(org quake.Vec3, dir quake.Vec3, color float32, count float32) {}

// ChangeYaw smoothly rotates the entity towards its 'ideal_yaw' at 'yaw_speed'.
//
// Implementation: internal/qc/builtins_world.go:changeyaw
//
//qgo:builtin 49
//go:noinline
func ChangeYaw() {}

// --- Group 51-60: Vector Conversion and Networking ---

// VectoAngles converts a direction vector into its equivalent Euler angles.
//
// Implementation: internal/qc/builtins_math.go:vectoangles
//
//qgo:builtin 51
//go:noinline
func VectoAngles(v quake.Vec3) quake.Vec3 { return quake.Vec3{} }

// WriteByte adds a byte value to a network message.
//
// Implementation: internal/qc/builtins.go:writeByteBuiltin
//
//qgo:builtin 52
//go:noinline
func WriteByte(dest float32, b float32) {}

// WriteChar adds a character value to a network message.
//
// Implementation: internal/qc/builtins.go:writeCharBuiltin
//
//qgo:builtin 53
//go:noinline
func WriteChar(dest float32, b float32) {}

// WriteShort adds a 16-bit short value to a network message.
//
// Implementation: internal/qc/builtins.go:writeShortBuiltin
//
//qgo:builtin 54
//go:noinline
func WriteShort(dest float32, b float32) {}

// WriteLong adds a 32-bit long value to a network message.
//
// Implementation: internal/qc/builtins.go:writeLongBuiltin
//
//qgo:builtin 55
//go:noinline
func WriteLong(dest float32, b float32) {}

// WriteCoord adds a coordinate value to a network message.
//
// Implementation: internal/qc/builtins.go:writeCoordBuiltin
//
//qgo:builtin 56
//go:noinline
func WriteCoord(dest float32, b float32) {}

// WriteAngle adds an angle value to a network message.
//
// Implementation: internal/qc/builtins.go:writeAngleBuiltin
//
//qgo:builtin 57
//go:noinline
func WriteAngle(dest float32, b float32) {}

// WriteString adds a string to a network message.
//
// Implementation: internal/qc/builtins.go:writeStringBuiltin
//
//qgo:builtin 58
//go:noinline
func WriteString(dest float32, s string) {}

// WriteEntity adds an entity handle to a network message.
//
// Implementation: internal/qc/builtins.go:writeEntityBuiltin
//
//qgo:builtin 59
//go:noinline
func WriteEntity(dest float32, e *quake.Entity) {}

// Sin returns the sine of an angle given in degrees.
//
// Implementation: internal/qc/builtins_math.go:sinBuiltin
//
//qgo:builtin 60
//go:noinline
func Sin(f float32) float32 { return 0 }

// --- Group 61-70: Math and Level Transitions ---

// Cos returns the cosine of an angle given in degrees.
//
// Implementation: internal/qc/builtins_math.go:cosBuiltin
//
//qgo:builtin 61
//go:noinline
func Cos(f float32) float32 { return 0 }

// Sqrt returns the square root of the input float.
//
// Implementation: internal/qc/builtins_math.go:sqrtBuiltin
//
//qgo:builtin 62
//go:noinline
func Sqrt(f float32) float32 { return 0 }

// Etos converts an entity handle to its string representation (e.g. "entity 1").
//
// Implementation: internal/qc/builtins_string.go:etosBuiltin
//
//qgo:builtin 65
//go:noinline
func Etos(e *quake.Entity) string { return "" }

// MoveToGoal moves the entity its 'dist' towards its 'goalentity'.
//
// Implementation: internal/qc/builtins_world.go:movetogoal
//
//qgo:builtin 67
//go:noinline
func MoveToGoal(dist float32) {}

// PrecacheFile registers a generic file path with the engine.
//
// Implementation: internal/qc/builtins.go:precacheFile
//
//qgo:builtin 68
//go:noinline
func PrecacheFile(s string) string { return s }

// MakeStatic converts an entity into a permanent, non-interactive part of the world.
//
// Implementation: internal/qc/builtins_entity.go:makestatic
//
//qgo:builtin 69
//go:noinline
func MakeStatic(e *quake.Entity) {}

// Changelevel triggers a level transition to the specified map name.
//
// Implementation: internal/qc/builtins.go:changelevel
//
//qgo:builtin 70
//go:noinline
func Changelevel(s string) {}

// --- Group 71-80: Configuration and Sound ---

// CvarSet sets the value of a console variable.
//
// Implementation: internal/qc/builtins.go:cvarSetBuiltin
//
//qgo:builtin 72
//go:noinline
func CvarSet(s string, v string) {}

// Centerprint prints a centered message on a specific client's screen.
//
// Implementation: internal/qc/builtins_world.go:centerprint
//
//qgo:builtin 73
//go:noinline
func Centerprint(s string) {}

// Ambientsound plays a looping sound at a specific position.
//
// Implementation: internal/qc/builtins_world.go:ambientsound
//
//qgo:builtin 74
//go:noinline
func Ambientsound(pos quake.Vec3, samp string, vol float32, atten float32) {}

// PrecacheModel2 registers a model file path so it can be loaded and used.
//
// Implementation: internal/qc/builtins_entity.go:precacheModel
//
//qgo:builtin 75
//go:noinline
func PrecacheModel2(s string) string { return s }

// PrecacheSound2 registers a sound file path so it can be loaded and played.
//
// Implementation: internal/qc/builtins_entity.go:precacheSound
//
//qgo:builtin 76
//go:noinline
func PrecacheSound2(s string) string { return s }

// SetSpawnParms prepares the spawn parameters for a client entity.
//
// Implementation: internal/qc/builtins_entity.go:setspawnparms
//
//qgo:builtin 78
//go:noinline
func SetSpawnParms(e *quake.Entity) {}

// FinaleFinished signals the engine that a finale sequence has completed.
//
// Implementation: internal/qc/builtins.go:finaleFinished
//
//qgo:builtin 79
//go:noinline
func FinaleFinished() float32 { return 0 }

// LocalSound plays a sound that is only heard by a specific client.
//
// Implementation: internal/qc/builtins_world.go:localsound
//
//qgo:builtin 80
//go:noinline
func LocalSound(e *quake.Entity, s string) {}

// --- Group 81-100: Math Extensions ---

// Stof converts a string representation of a float to a float32.
//
// Implementation: internal/qc/builtins_math.go:stofBuiltin
//
//qgo:builtin 81
//go:noinline
func Stof(s string) float32 { return 0 }

// Min returns the smaller of two float values.
//
// Implementation: internal/qc/builtins_math.go:minBuiltin
//
//qgo:builtin 94
//go:noinline
func Min(a, b float32) float32 { return 0 }

// Max returns the larger of two float values.
//
// Implementation: internal/qc/builtins_math.go:maxBuiltin
//
//qgo:builtin 95
//go:noinline
func Max(a, b float32) float32 { return 0 }

// Bound clamps a value between a minimum and maximum float.
//
// Implementation: internal/qc/builtins_math.go:boundBuiltin
//
//qgo:builtin 96
//go:noinline
func Bound(min, val, max float32) float32 { return 0 }

// Pow raises a base to the specified exponent.
//
// Implementation: internal/qc/builtins_math.go:powBuiltin
//
//qgo:builtin 97
//go:noinline
func Pow(base, exp float32) float32 { return 0 }

// CheckExtension returns true if the engine supports the specified extension string.
//
// Implementation: internal/qc/builtins.go:noopBuiltin (placeholder)
//
//qgo:builtin 99
//go:noinline
func CheckExtension(s string) float32 { return 0 }

// CheckPlayerEXFlags returns extended flags for a player entity.
//
// Implementation: internal/qc/builtins.go:noopBuiltin (placeholder)
//
//qgo:builtin 100
//go:noinline
func CheckPlayerEXFlags(e *quake.Entity) float32 { return 0 }

// --- Group 101+: String and Trig Extensions ---

// Strlen returns the number of characters in a string.
//
// Implementation: internal/qc/builtins_string.go:strlenBuiltin
//
//qgo:builtin 114
//go:noinline
func Strlen(s string) float32 { return 0 }

// Strcat concatenates two strings and returns the result.
//
// Implementation: internal/qc/builtins_string.go:strcatBuiltin
//
//qgo:builtin 115
//go:noinline
func Strcat(s1, s2 string) string { return "" }

// Substring returns a portion of a string starting at 'start' with 'length'.
//
// Implementation: internal/qc/builtins_string.go:substringBuiltin
//
//qgo:builtin 116
//go:noinline
func Substring(s string, start, length float32) string { return "" }

// Stov converts a string like "'1 2 3'" into a Vec3.
//
// Implementation: internal/qc/builtins_string.go:stovBuiltin
//
//qgo:builtin 117
//go:noinline
func Stov(s string) quake.Vec3 { return quake.Vec3{} }

// Strzone allocates a permanent copy of a string. No-op in Go (GC handles it).
//
// Implementation: internal/qc/builtins_string.go:strzoneBuiltin
//
//qgo:builtin 118
//go:noinline
func Strzone(s string) string { return s }

// Strunzone frees a zoned string. No-op in Go (GC handles it).
//
// Implementation: internal/qc/builtins_string.go:strunzoneBuiltin
//
//qgo:builtin 119
//go:noinline
func Strunzone(s string) {}

// Str2chr returns the character code at the specified index in a string.
//
// Implementation: internal/qc/builtins_string.go:str2chrBuiltin
//
//qgo:builtin 222
//go:noinline
func Str2chr(s string, index float32) float32 { return 0 }

// Chr2str converts a character code to a single-character string.
//
// Implementation: internal/qc/builtins_string.go:chr2strBuiltin
//
//qgo:builtin 223
//go:noinline
func Chr2str(f float32) string { return "" }

// Mod returns the remainder of a division (a % b).
//
// Implementation: internal/qc/builtins_math.go:modBuiltin
//
//qgo:builtin 245
//go:noinline
func Mod(a, b float32) float32 { return 0 }

// --- Trigonometry Extended ---

// Asin returns the arcsine of the input float in degrees.
//
// Implementation: internal/qc/builtins_math.go:asinBuiltin
//
//qgo:builtin 471
//go:noinline
func Asin(f float32) float32 { return 0 }

// Acos returns the arccosine of the input float in degrees.
//
// Implementation: internal/qc/builtins_math.go:acosBuiltin
//
//qgo:builtin 472
//go:noinline
func Acos(f float32) float32 { return 0 }

// Atan returns the arctangent of the input float in degrees.
//
// Implementation: internal/qc/builtins_math.go:atanBuiltin
//
//qgo:builtin 473
//go:noinline
func Atan(f float32) float32 { return 0 }

// Atan2 returns the arctangent of y/x in degrees.
//
// Implementation: internal/qc/builtins_math.go:atan2Builtin
//
//qgo:builtin 474
//go:noinline
func Atan2(y, x float32) float32 { return 0 }

// Tan returns the tangent of an angle given in degrees.
//
// Implementation: internal/qc/builtins_math.go:tanBuiltin
//
//qgo:builtin 475
//go:noinline
func Tan(f float32) float32 { return 0 }
