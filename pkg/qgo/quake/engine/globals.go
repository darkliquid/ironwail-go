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
