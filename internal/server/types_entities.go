package server

// StaticSound represents a persistent ambient sound in the world signon state.
//
// Static sounds are set up once during level load and looped for the entire
// duration of the map. They are included in the signon buffer so every client
// receives them upon connecting. Unlike dynamic sounds (triggered by events),
// static sounds play continuously from a fixed position in the world.
//
// Examples: lava bubbling, wind blowing, water flowing, torches crackling.
// The client spatializes these sounds based on listener position, so they
// get louder/softer and pan left/right as the player moves.
//
// Fields:
//   - Origin: world-space position of the sound source.
//   - SoundIndex: index into the server's sound precache table.
//   - Volume: playback volume (0-255, where 255 is full volume).
//   - Attenuation: distance falloff factor. Higher values make the sound
//     fade out faster with distance. Common values: 0 = no attenuation
//     (plays everywhere equally), 1 = normal, 2 = idle (short range),
//     3 = static (very short range, e.g., a torch right next to the player).
type StaticSound struct {
	Origin      [3]float32
	SoundIndex  int
	Volume      int
	Attenuation float32
}

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
// # Baseline (Delta Compression)
//
// The Baseline field stores the entity's initial state snapshot, sent to
// clients during the signon phase. Subsequent updates only transmit fields
// that differ from this baseline, saving bandwidth.
type Edict struct {
	// Num is the entity number (index into s.Edicts and s.QCVM.Edicts).
	// Set during allocation, stable for the entity's lifetime.
	Num int

	// Free indicates this edict slot is available for reuse. When true, the
	// entity has been removed from the game world but the slot hasn't been
	// recycled yet (waiting for FreeTime delay to expire).
	Free bool

	// Area linkage for spatial partitioning. These form a doubly-linked list
	// connecting this edict to others in the same world area. The area system
	// accelerates collision queries by spatially indexing entities.
	AreaPrev *Edict
	AreaNext *Edict

	// Leaf visibility data for PVS (Potentially Visible Set) culling.
	// NumLeafs is the count of BSP visleafs this entity overlaps.
	// LeafNums stores Quake visleaf indices (BSP leaf index minus 1,
	// skipping solid leaf 0) up to MaxEntityLeafs.
	// If the entity spans more leaves than MaxEntityLeafs, it is always
	// considered visible (too large to cull precisely).
	NumLeafs int
	LeafNums [32]int

	// Baseline is the reference EntityState sent during signon for delta
	// compression. All subsequent network updates for this entity encode
	// only the differences from this baseline.
	// Alpha and Scale are engine-side overrides for rendering transparency
	// and size, sent via extended protocol bits.
	Baseline EntityState
	Alpha    uint8
	Scale    uint8

	// Physics scratch state used during the current frame's physics step.
	// ForceWater/SendForceWater handle edge cases where water state must
	// be explicitly communicated to the client.
	// SendInterval tracks whether this entity uses interpolation timing.
	// OldFrame/OldThinkTime are used to detect animation and think changes.
	ForceWater     bool
	SendForceWater bool
	SendInterval   bool
	OldFrame       float32
	OldThinkTime   float32

	// FreeTime records when this edict was freed (set to server time when
	// Free becomes true). During the first two seconds of server time free
	// slots may be reused immediately; afterwards they wait 0.5 seconds to
	// avoid client-side interpolation/trail glitches from rapid reuse.
	FreeTime float32
}


// UserCmd represents a single frame of client input sent from client to server.
//
// Each game frame, the client captures the player's input state — view angles,
// movement axes, button presses — and packages it into a UserCmd. This is sent
// to the server as part of a CLCMove message. The server applies the UserCmd to
// the player's entity via SV_RunClients → SV_ClientThink.
//
// The movement values (ForwardMove, SideMove, UpMove) are in units/second and
// are scaled by the client based on cl_forwardspeed, cl_sidespeed, etc. The
// server clamps these to sv_maxspeed (default 320) before applying them.
//
// This is the ONLY way the client can influence the server simulation. All
// player agency — movement, shooting, item use — flows through UserCmd. The
// server is fully authoritative: it validates and applies these inputs, and
// the client's local prediction must match or be corrected.
type UserCmd struct {
	// ViewAngles — the player's current look direction as Euler angles:
	//  [0] = pitch (look up/down, negative = up, positive = down)
	//  [1] = yaw (look left/right, 0 = east, 90 = north)
	//  [2] = roll (head tilt, usually 0 unless affected by damage kick)
	// These are absolute angles, not deltas. The server stores them in the
	// player entity's VAngle field.
	ViewAngles [3]float32

	// ForwardMove — forward/backward movement speed in units/second.
	// Positive = forward, negative = backward. Determined by +forward/-back
	// key bindings, scaled by cl_forwardspeed (default 200) or cl_backspeed.
	ForwardMove float32

	// SideMove — strafe movement speed in units/second.
	// Positive = right, negative = left. Determined by +moveright/+moveleft
	// key bindings, scaled by cl_sidespeed (default 350).
	SideMove float32

	// UpMove — vertical movement speed in units/second.
	// Positive = up (jump/swim up), negative = down (crouch/swim down).
	// In water, this directly controls vertical swimming. On ground, a
	// positive value triggers a jump.
	UpMove float32

	// Buttons — bitmask of button states.
	//  Bit 0: attack (fire weapon / +attack)
	//  Bit 1: jump (+jump)
	// The server unpacks these into the entity's Button0/Button2 fields.
	Buttons uint8

	// Impulse — one-shot command code. Sent once when pressed, then cleared.
	// Values 1-8 select weapons; other values are mod-specific. The server
	// copies this to the entity's Impulse field and QuakeC processes it.
	Impulse uint8
}

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
