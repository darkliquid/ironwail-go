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

// Note: TraceResult struct definition has been moved to internal/server/types/trace.go.
// The type alias server.TraceResult is exported in internal/server/types.go.

