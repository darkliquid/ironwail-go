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
	// Self is the entity currently executing QuakeC code (e.g., the entity
	// whose .think function was called).
	Self quake.Entity

	// Other is the secondary entity involved in an interaction (e.g.,
	// the entity that was touched, or the entity being used).
	Other quake.Entity

	// World is the entity representing the map geometry and global state.
	// It is always entity 0.
	World quake.Entity

	// Time is the current game time in seconds.
	Time float32

	// NextEnt returns the next entity in the linked list of entities.
	NextEnt quake.Entity
)

// Dprint prints a message to the engine console only if the 'developer'
// console variable is set to a non-zero value.
//
//qgo:builtin 25
//go:noinline
func Dprint(s string) {}

// Print prints a message to the engine console.
//
//qgo:builtin 24
//go:noinline
func Print(s string) {}

// Bprint broadcasts a message to all connected clients and the console.
//
//qgo:builtin 23
//go:noinline
func Bprint(s string) {}

// Error prints a fatal error message and halts the server.
//
//qgo:builtin 10
//go:noinline
func Error(s string) {}

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

// Normalize returns a vector with the same direction as the input but with a length of 1.
//
//qgo:builtin 9
//go:noinline
func Normalize(v quake.Vec3) quake.Vec3 { return v }

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

// PrecacheModel registers a model file so it can be used by entities.
// This must be called during the 'worldspawn' or 'spawn' phase.
//
//qgo:builtin 20
//go:noinline
func PrecacheModel(s string) string { return s }

// PrecacheSound registers a sound file so it can be played.
// This must be called during the 'worldspawn' or 'spawn' phase.
//
//qgo:builtin 19
//go:noinline
func PrecacheSound(s string) string { return s }

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

// SetOrigin moves an entity to a specific position in the world.
//
//qgo:builtin 2
//go:noinline
func SetOrigin(e quake.Entity, org quake.Vec3) {}

// Ambientsound plays a looping sound at a specific position.
//
//qgo:builtin 74
//go:noinline
func Ambientsound(pos quake.Vec3, samp string, vol, atten float32) {}

// Sound plays a sound effect from an entity.
//
//qgo:builtin 8
//go:noinline
func Sound(e quake.Entity, ch int, samp string, vol, atten float32) {}

// Traceline performs a ray-cast from v1 to v2 and stores the results in
// global variables (trace_fraction, trace_endpos, etc.).
//
//qgo:builtin 16
//go:noinline
func Traceline(v1, v2 quake.Vec3, nomonsters int, e quake.Entity) {}

// Random returns a random float value between 0.0 and 1.0.
//
//qgo:builtin 7
//go:noinline
func Random() float32 { return 0 }

// Changelevel triggers a level transition to the specified map.
//
//qgo:builtin 70
//go:noinline
func Changelevel(s string) {}

// Cvar returns the current float value of a console variable.
//
//qgo:builtin 45
//go:noinline
func Cvar(s string) float32 { return 0 }

// CvarSet sets the value of a console variable.
//
//qgo:builtin 72
//go:noinline
func CvarSet(s string, v float32) {}

// Centerprint prints a message in the center of a specific client's screen.
//
//qgo:builtin 73
//go:noinline
func Centerprint(s string) {}
