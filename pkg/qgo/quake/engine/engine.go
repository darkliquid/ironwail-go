// Package engine provides bindings to the Quake engine's built-in functions and shared globals.
//
// These functions are mapped to the native C implementations within the Quake engine
// via the //qgo:builtin N directive. The compiler emits function records with negative
// first_statement indices that the engine uses to dispatch calls.
package engine

import "github.com/ironwail/ironwail-go/pkg/qgo/quake"

// Engine globals that are shared between the engine and the QCVM.
// These variables are located at fixed offsets in the global address space.
var (
	// Self is the entity currently executing QuakeC code.
	Self quake.Entity

	// Other is the secondary entity involved in an interaction.
	Other quake.Entity

	// World is the entity representing the map geometry and global state (entity 0).
	World quake.Entity

	// Time is the current game time in seconds.
	Time float32

	// FrameTime is the duration of the current frame in seconds.
	FrameTime float32

	// NextEnt returns the next entity in the linked list of entities.
	NextEnt quake.Entity

	// MapName is the name of the current map.
	MapName string

	// Direction globals set by MakeVectors
	VForward quake.Vec3
	VUp      quake.Vec3
	VRight   quake.Vec3

	// Trace result globals set by TraceLine/TraceBox
	TraceFraction    float32
	TraceEndPos      quake.Vec3
	TracePlaneNormal quake.Vec3
	TracePlaneDist   float32
	TraceEnt         quake.Entity
	TraceAllSolid    float32
	TraceStartSolid  float32
	TraceInOpen      float32
	TraceInWater     float32
)

// --- Group 1-10: Vector/Math Operations ---

// MakeVectors computes forward, up, and right vectors from a rotation vector.
//
//qgo:builtin 1
//go:noinline
func MakeVectors(ang quake.Vec3) {}

// SetOrigin moves an entity to a specific position in the world.
//
//qgo:builtin 2
//go:noinline
func SetOrigin(e quake.Entity, org quake.Vec3) {}

// SetModel sets the visual model for an entity.
//
//qgo:builtin 3
//go:noinline
func SetModel(e quake.Entity, m string) {}

// SetSize sets the bounding box (mins and maxs) for an entity's physics.
//
//qgo:builtin 4
//go:noinline
func SetSize(e quake.Entity, min, max quake.Vec3) {}

// BreakStatement triggers a debugger break if one is attached.
//
//qgo:builtin 6
//go:noinline
func BreakStatement() {}

// Random returns a random float value between 0.0 and 1.0.
//
//qgo:builtin 7
//go:noinline
func Random() float32 { return 0 }

// Sound plays a sound effect from an entity.
//
//qgo:builtin 8
//go:noinline
func Sound(e quake.Entity, ch int, samp string, vol float32, atten float32) {}

// Normalize returns a vector with the same direction as the input but with a length of 1.
//
//qgo:builtin 9
//go:noinline
func Normalize(v quake.Vec3) quake.Vec3 { return v }

// Error prints a fatal error message and halts the server.
//
//qgo:builtin 10
//go:noinline
func Error(s string) {}

// --- Group 11-20: Math and Entity Management ---

// ObjError prints a fatal error message related to an entity.
//
//qgo:builtin 11
//go:noinline
func ObjError(s string) {}

// Vlen returns the length (magnitude) of a 3D vector.
//
//qgo:builtin 12
//go:noinline
func Vlen(v quake.Vec3) float32 { return 0 }

// Vectoyaw returns the yaw angle (in degrees) that a vector points towards.
//
//qgo:builtin 13
//go:noinline
func Vectoyaw(v quake.Vec3) float32 { return 0 }

// Spawn creates a new entity in the game world and returns its handle.
//
//qgo:builtin 14
//go:noinline
func Spawn() quake.Entity { return 0 }

// Remove deletes an entity from the game world.
//
//qgo:builtin 15
//go:noinline
func Remove(e quake.Entity) {}

// Traceline performs a ray-cast from v1 to v2.
//
//qgo:builtin 16
//go:noinline
func Traceline(v1, v2 quake.Vec3, nomonsters float32, e quake.Entity) {}

// CheckClient returns a client entity that is visible to the current entity.
//
//qgo:builtin 17
//go:noinline
func CheckClient() quake.Entity { return 0 }

// Find locates an entity whose field matches a string.
//
//qgo:builtin 18
//go:noinline
func Find(e quake.Entity, field string, match string) quake.Entity { return 0 }

// PrecacheSound registers a sound file so it can be played.
//
//qgo:builtin 19
//go:noinline
func PrecacheSound(s string) string { return s }

// PrecacheModel registers a model file so it can be used by entities.
//
//qgo:builtin 20
//go:noinline
func PrecacheModel(s string) string { return s }

// --- Group 21-31: Utilities and Printing ---

// StuffCmd sends a command string to a client's console.
//
//qgo:builtin 21
//go:noinline
func StuffCmd(e quake.Entity, s string) {}

// FindRadius returns a chain of entities within a radius of a point.
//
//qgo:builtin 22
//go:noinline
func FindRadius(org quake.Vec3, radius float32) quake.Entity { return 0 }

// Bprint broadcasts a message to all connected clients and the console.
//
//qgo:builtin 23
//go:noinline
func Bprint(s string) {}

// SPrint prints a message intended for one client.
//
//qgo:builtin 24
//go:noinline
func SPrint(e quake.Entity, s string) {}

// Dprint prints a message to the engine console (only if developer is 1).
//
//qgo:builtin 25
//go:noinline
func Dprint(s string) {}

// Ftos converts a float value to a string.
//
//qgo:builtin 26
//go:noinline
func Ftos(f float32) string { return "" }

// Vtos converts a vector value to a string.
//
//qgo:builtin 27
//go:noinline
func Vtos(v quake.Vec3) string { return "" }

// EPrint prints an entity's information to the console.
//
//qgo:builtin 31
//go:noinline
func EPrint(e quake.Entity) {}

// --- Group 32-40: Physics and Math ---

// WalkMove moves an entity forward with collision detection.
//
//qgo:builtin 32
//go:noinline
func WalkMove(yaw float32, dist float32) float32 { return 0 }

// DropToFloor drops an entity down until it hits the floor.
//
//qgo:builtin 34
//go:noinline
func DropToFloor() float32 { return 0 }

// LightStyle sets the animation for a light style.
//
//qgo:builtin 35
//go:noinline
func LightStyle(style float32, value string) {}

// RInt returns the nearest integer value to f.
//
//qgo:builtin 36
//go:noinline
func RInt(f float32) float32 { return 0 }

// Floor returns the largest integer less than or equal to f.
//
//qgo:builtin 37
//go:noinline
func Floor(f float32) float32 { return 0 }

// Ceil returns the smallest integer greater than or equal to f.
//
//qgo:builtin 38
//go:noinline
func Ceil(f float32) float32 { return 0 }

// CheckBottom verifies that the entity is supported by the floor.
//
//qgo:builtin 40
//go:noinline
func CheckBottom(e quake.Entity) float32 { return 0 }

// --- Group 41-51: World and AI ---

// PointContents reports the contents of the world at a point.
//
//qgo:builtin 41
//go:noinline
func PointContents(p quake.Vec3) float32 { return 0 }

// FAbs returns the absolute value of f.
//
//qgo:builtin 43
//go:noinline
func FAbs(f float32) float32 { return 0 }

// Aim returns a vector pointing towards a target.
//
//qgo:builtin 44
//go:noinline
func Aim(e quake.Entity, speed float32) quake.Vec3 { return quake.Vec3{} }

// Cvar returns the current float value of a console variable.
//
//qgo:builtin 45
//go:noinline
func Cvar(s string) float32 { return 0 }

// LocalCmd adds text to the local command buffer.
//
//qgo:builtin 46
//go:noinline
func LocalCmd(s string) {}

// NextEnt returns the next entity in the linked list.
//
//qgo:builtin 47
//go:noinline
func NextEnt(e quake.Entity) quake.Entity { return 0 }

// Particle spawns a particle effect.
//
//qgo:builtin 48
//go:noinline
func Particle(org quake.Vec3, dir quake.Vec3, color float32, count float32) {}

// ChangeYaw smoothly rotates an entity toward its ideal yaw.
//
//qgo:builtin 49
//go:noinline
func ChangeYaw() {}

// VectoAngles converts a direction vector to Euler angles.
//
//qgo:builtin 51
//go:noinline
func VectoAngles(v quake.Vec3) quake.Vec3 { return quake.Vec3{} }

// --- Group 52-59: Networking/Writes ---

//qgo:builtin 52
//go:noinline
func WriteByte(dest float32, b float32) {}

//qgo:builtin 53
//go:noinline
func WriteChar(dest float32, b float32) {}

//qgo:builtin 54
//go:noinline
func WriteShort(dest float32, b float32) {}

//qgo:builtin 55
//go:noinline
func WriteLong(dest float32, b float32) {}

//qgo:builtin 56
//go:noinline
func WriteCoord(dest float32, b float32) {}

//qgo:builtin 57
//go:noinline
func WriteAngle(dest float32, b float32) {}

//qgo:builtin 58
//go:noinline
func WriteString(dest float32, s string) {}

//qgo:builtin 59
//go:noinline
func WriteEntity(dest float32, e quake.Entity) {}

// --- Group 60-80: Math, AI and Level ---

//qgo:builtin 60
//go:noinline
func Sin(f float32) float32 { return 0 }

//qgo:builtin 61
//go:noinline
func Cos(f float32) float32 { return 0 }

//qgo:builtin 62
//go:noinline
func Sqrt(f float32) float32 { return 0 }

//qgo:builtin 65
//go:noinline
func Etos(e quake.Entity) string { return "" }

//qgo:builtin 67
//go:noinline
func MoveToGoal(dist float32) {}

//qgo:builtin 68
//go:noinline
func PrecacheFile(s string) string { return s }

//qgo:builtin 69
//go:noinline
func MakeStatic(e quake.Entity) {}

//qgo:builtin 70
//go:noinline
func Changelevel(s string) {}

//qgo:builtin 72
//go:noinline
func CvarSet(s string, v string) {}

//qgo:builtin 73
//go:noinline
func Centerprint(s string) {}

//qgo:builtin 74
//go:noinline
func Ambientsound(pos quake.Vec3, samp string, vol float32, atten float32) {}

//qgo:builtin 78
//go:noinline
func SetSpawnParms(e quake.Entity) {}

//qgo:builtin 79
//go:noinline
func FinaleFinished() {}

//qgo:builtin 80
//go:noinline
func LocalSound(e quake.Entity, s string) {}

// --- Group 81+: Extended Functions ---

//qgo:builtin 81
//go:noinline
func Stof(s string) float32 { return 0 }

//qgo:builtin 94
//go:noinline
func Min(a, b float32) float32 { return 0 }

//qgo:builtin 95
//go:noinline
func Max(a, b float32) float32 { return 0 }

//qgo:builtin 96
//go:noinline
func Bound(min, val, max float32) float32 { return 0 }

//qgo:builtin 97
//go:noinline
func Pow(base, exp float32) float32 { return 0 }

//qgo:builtin 114
//go:noinline
func Strlen(s string) float32 { return 0 }

//qgo:builtin 115
//go:noinline
func Strcat(s1, s2 string) string { return "" }

//qgo:builtin 116
//go:noinline
func Substring(s string, start, length float32) string { return "" }

//qgo:builtin 117
//go:noinline
func Stov(s string) quake.Vec3 { return quake.Vec3{} }

//qgo:builtin 118
//go:noinline
func Strzone(s string) string { return s }

//qgo:builtin 119
//go:noinline
func Strunzone(s string) {}

//qgo:builtin 222
//go:noinline
func Str2chr(s string, index float32) float32 { return 0 }

//qgo:builtin 223
//go:noinline
func Chr2str(f float32) string { return "" }

//qgo:builtin 245
//go:noinline
func Mod(a, b float32) float32 { return 0 }

// --- Trigonometry Extended ---

//qgo:builtin 471
//go:noinline
func Asin(f float32) float32 { return 0 }

//qgo:builtin 472
//go:noinline
func Acos(f float32) float32 { return 0 }

//qgo:builtin 473
//go:noinline
func Atan(f float32) float32 { return 0 }

//qgo:builtin 474
//go:noinline
func Atan2(y, x float32) float32 { return 0 }

//qgo:builtin 475
//go:noinline
func Tan(f float32) float32 { return 0 }

// --- Helpers ---

// CRandom returns a random float value between -1.0 and 1.0.
func CRandom() float32 { return Random()*2 - 1 }

//qgo:builtin 99
//go:noinline
func CheckExtension(s string) float32 { return 0 }

//qgo:builtin 100
//go:noinline
func CheckPlayerEXFlags(e quake.Entity) float32 { return 0 }
