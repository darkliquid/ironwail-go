// This file belongs to the Entity/QC subsystem: edict allocation, entity accessors, QuakeC field offsets, QC call tracing, and entity state types.
//
// StaticSound and UserCmd type definitions have been moved to
// internal/server/types. Aliases below preserve backward compatibility.
//
// Edict remains in this package because it has 170+ accessor methods that
// take *Server as their first parameter, and Go does not allow defining
// methods on non-local types. TraceResult also remains here because it
// has an Entity *Edict field.
package server

import srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"

// Type aliases for types moved to the types sub-package.
type (
	StaticSound = srvtypes.StaticSound
	UserCmd     = srvtypes.UserCmd
)

// Edict represents a game entity (the engine-side "entity dictionary" entry).
//
// The name "edict" comes from id Software's original terminology: "entity
// dictionary." Every object in the Quake world — players, monsters, doors,
// triggers, rockets, gibs — is an edict. The server maintains a flat array
// of edicts (up to MaxEdicts), where edict 0 is always the worldspawn entity
// (the map geometry itself).
//
// An Edict has two layers:
//  1. Engine-side fields (this struct): managed by the C/Go engine code.
//     These include spatial partitioning links, PVS leaf data, network
//     baseline state, and physics scratch data.
//  2. QuakeC-side fields (EntVars): the "progs" data visible to QuakeC game
//     logic. Accessed via the Vars pointer.
//
// Key concepts:
//
// # Free List
//
// When an entity is removed (e.g., a rocket explodes), Free is set to true
// and FreeTime records the timestamp. The edict slot is recycled after a
// minimum delay (2 seconds) to prevent stale network references from
// pointing at a completely different entity.
//
// # Area Links (Spatial Partitioning)
//
// AreaPrev/AreaNext form a doubly-linked list for spatial partitioning.
// The world is divided into axis-aligned areas; each area maintains a list
// of edicts within it. When performing collision traces or touch checks,
// only edicts in nearby areas are tested, dramatically reducing the O(n²)
// cost of checking every entity against every other entity.
//
// # Leaf Visibility (PVS)
//
// NumLeafs/LeafNums track which BSP leaves this entity touches. The
// Potentially Visible Set (PVS) determines which entities a client can
// see. Before sending an entity update to a client, the server checks
// whether any of the entity's leaves are in the client's PVS. If none
// are visible, the entity is culled from that client's network update.
//
// Note: Edict struct definition has been moved to internal/server/types/entity.go.
// The type alias server.Edict is exported in internal/server/types.go.

// TraceResult contains the result of a collision trace (ray or hull trace).
//
// Traces are the foundation of Quake's collision detection. A trace sweeps a
// bounding box (or a ray for point-sized traces) from point A to point B through
// the world BSP and entity bounding boxes, finding the first collision.
//
// The engine uses traces for:
//   - Physics movement: sweep the entity's bounding box along its velocity vector
//     to find where it hits walls/floors/entities.
//   - Weapon fire: trace a ray from the player's eye along the aim direction to
//     find what gets hit (hitscan weapons like shotgun, lightning gun).
//   - Ground detection: trace downward from the entity to check if there's a
//     floor beneath it (for FlagOnGround).
//   - Line of sight: trace between two points to check for obstructions (monster
//     AI visibility checks).
//   - Water detection: trace to find water surface positions.
//
// A trace with Fraction == 1.0 means nothing was hit (clear path). Fraction < 1.0
// means a collision occurred at EndPos, and PlaneNormal gives the surface orientation.
type TraceResult struct {
	// AllSolid — true if the entire trace path is inside solid geometry (the
	// entity is completely stuck). This can happen if an entity is spawned
	// inside a wall or pushed into solid by a door. When AllSolid is true,
	// Fraction is 0, EndPos equals the start position, and the entity should
	// not move.
	AllSolid bool

	// StartSolid — true if the trace start point is inside solid geometry,
	// but the trace eventually exits into open space. This is a partially-stuck
	// state: the entity can still move but its starting position is invalid.
	// The engine handles this by allowing the move but flagging the condition.
	StartSolid bool

	// Fraction — how far along the trace path the first collision occurred,
	// as a fraction from 0.0 to 1.0. 0.0 = collision at the start point,
	// 1.0 = no collision (full path is clear). The actual collision point is:
	//   collision_point = start + (end - start) * Fraction
	// Values slightly less than 1.0 indicate a glancing hit near the end.
	Fraction float32

	// EndPos — the world-space position where the trace ended. If Fraction < 1.0,
	// this is the point of collision (backed off slightly from the surface by
	// DIST_EPSILON to prevent the entity from being exactly on the surface).
	// If Fraction == 1.0, this equals the desired end position.
	EndPos [3]float32

	// PlaneNormal — the outward-facing normal vector of the surface that was
	// hit. This is critical for physics response:
	//   - For floor collisions, PlaneNormal ≈ {0, 0, 1} (pointing up).
	//   - For wall collisions, PlaneNormal is horizontal.
	//   - Used by ClipVelocity to redirect the entity's velocity along the
	//     surface (slide along walls instead of stopping dead).
	//   - Dot(velocity, PlaneNormal) gives the impact speed for bounce/damage.
	PlaneNormal [3]float32

	// PlaneDist — signed plane distance for the impact surface. QC traceline
	// exposes this as trace_plane_dist.
	PlaneDist float32

	// Entity — pointer to the edict that was hit, or nil if the trace hit
	// world geometry (or nothing). When non-nil, the engine can fire touch
	// callbacks on both the moving entity and the hit entity.
	Entity *Edict

	// InOpen / InWater mirror Quake trace_t flags used by QC traceline users
	// such as monster attack checks.
	InOpen  bool
	InWater bool
}
