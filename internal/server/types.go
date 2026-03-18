// Package server implements the Quake server physics and game logic.
//
// The server handles:
//   - Entity physics simulation (movement, collision)
//   - QuakeC think/touch/blocked callbacks
//   - Client state management
//   - World state (map, models, sounds)
//
// Physics is driven by SV_Physics which iterates all entities each frame
// and dispatches to specialized physics functions based on movetype.
//
// # Architecture Overview
//
// In Quake's client-server architecture, the server is the authoritative
// simulation. Even in single-player, the game runs a local server. The server:
//
//  1. Maintains the canonical world state (all entity positions, health, etc.).
//  2. Runs physics each frame: integrates velocity, resolves collisions, applies
//     gravity, handles pushers like doors/platforms.
//  3. Executes QuakeC game logic via think/touch/blocked callbacks on entities.
//  4. Accepts client input (UserCmd) and applies it to player entities.
//  5. Sends world-state updates to connected clients for rendering.
//
// The server tick rate is controlled by the "host_framerate" or "sys_ticrate"
// cvars. Each tick, SV_Physics iterates the global entity list (edicts) and
// dispatches to a movetype-specific physics function (e.g., SV_Physics_Toss
// for grenades, SV_Physics_Pusher for doors).
package server

import "math"

// MoveType defines how an entity moves through the world.
//
// Every entity in Quake has a movetype that determines which physics code path
// runs for it each server frame. The movetype is set by QuakeC game logic
// (e.g., a grenade entity gets MoveTypeToss at spawn). The server's main
// physics loop (SV_Physics) reads each entity's movetype and dispatches to
// the corresponding SV_Physics_* function.
//
// Movetypes form a spectrum from fully static (None) to fully dynamic (Walk).
// Key distinctions:
//   - Does gravity apply? (Walk, Step, Toss, Bounce, Gib — yes; Fly, FlyMissile — no)
//   - Does it clip against world geometry? (all except None, NoClip, AngleNoClip)
//   - Does it push other entities out of the way? (only Push)
//   - Does it stop on ground contact? (Toss stops; Bounce reflects; FlyMissile triggers touch)
type MoveType int

const (
	// MoveTypeNone — entity never moves. The physics loop skips velocity
	// integration entirely. QuakeC think functions still run on schedule, and
	// touch callbacks still fire if another entity moves into this one (when
	// solid != SolidNot). Used for: worldspawn (edict 0), trigger volumes,
	// point entities like info_player_start, and any decoration that never
	// needs to move after spawn.
	MoveTypeNone MoveType = iota

	// MoveTypeAngleNoClip — angular velocity (AVelocity) is integrated to
	// rotate the entity, but no collision clipping is performed. The entity's
	// position never changes. Used for purely visual rotations like spinning
	// pickup items that don't need to interact with world geometry.
	MoveTypeAngleNoClip

	// MoveTypeAngleClip — angular velocity is integrated with collision
	// clipping against the world. Rarely used in standard Quake; exists for
	// hypothetical rotating brushes that need to detect when rotation is
	// blocked by world geometry.
	MoveTypeAngleClip

	// MoveTypeWalk — full player physics. This is the most complex movetype
	// and drives all player movement. The physics pipeline:
	//  1. Apply gravity (unless on ground with FlagOnGround set).
	//  2. Integrate velocity to produce desired movement.
	//  3. Clip against world BSP and entity bounding boxes.
	//  4. Step-up logic: if blocked, try moving up by sv_stepsize (18 units)
	//     then forward, enabling stair climbing.
	//  5. Step-down: snap to ground on shallow slopes to prevent "hopping."
	//  6. Apply ground friction to horizontal velocity.
	//  7. Handle water physics: buoyancy, reduced gravity, swim acceleration.
	//  8. Air control: limited acceleration while airborne (enables strafejumping).
	// Only valid for client (player) entities; monsters use MoveTypeStep.
	MoveTypeWalk

	// MoveTypeStep — monster/NPC stepping movement. Similar to Walk but
	// designed for AI-controlled entities. Key differences from Walk:
	//  - Discrete stair-step interpolation (the client smooths the visual step
	//    so monsters don't "pop" up stairs).
	//  - Gravity is applied.
	//  - No player-specific features like air control or view bobbing.
	//  - QuakeC AI code sets velocity; the physics just integrates it.
	// Used by all ground-based monsters: Grunt, Knight, Fiend, Shambler, etc.
	MoveTypeStep

	// MoveTypeFly — flying movement with no gravity. The entity moves freely
	// in 3D space, clipping against world geometry. Velocity is set by QuakeC
	// AI code (typically to chase the player). Used by flying monsters:
	// Scrag (Wizard), Vore (Shalrath) projectiles, and any entity that should
	// hover or fly without falling.
	MoveTypeFly

	// MoveTypeToss — gravity-affected projectile movement. Each frame:
	//  1. Gravity is applied to vertical velocity (sv_gravity * frametime).
	//  2. Velocity is integrated to move the entity.
	//  3. On ground contact, velocity is zeroed and FlagOnGround is set,
	//     stopping the entity.
	//  4. On entity contact, the touch function is called.
	// Used for: grenades (they arc and come to rest), gibs in original Quake,
	// backpacks dropped on death, and any physics object that should fall and
	// stop.
	MoveTypeToss

	// MoveTypePush — kinematic pusher for brush entities. Pushers are unique:
	//  - They use LTime (local time) instead of server global time, allowing
	//    them to pause/resume independently.
	//  - When they move, they sweep all blocking entities out of the way.
	//  - If a blocking entity can't be pushed clear, the blocked callback fires
	//    (e.g., a door crushing a player triggers damage).
	//  - They don't have velocity integrated by physics; instead, QuakeC sets
	//    a destination and the think function moves toward it each frame.
	// Used for: doors (func_door), elevators/platforms (func_plat), moving
	// walls (func_wall), buttons (func_button), and trains (func_train).
	MoveTypePush

	// MoveTypeNoClip — free movement with no collision detection at all. The
	// entity passes through walls, floors, and other entities. Velocity is
	// integrated directly into position with no clipping. Used for:
	//  - Player "noclip" cheat mode (fly through the level).
	//  - Spectators in multiplayer.
	//  - Debug/development entity inspection.
	// No touch callbacks fire, no ground detection occurs.
	MoveTypeNoClip

	// MoveTypeFlyMissile — flying projectile movement (no gravity). Similar
	// to MoveTypeFly but with projectile-specific collision semantics:
	//  - Uses a point-sized or small bounding box for precise hit detection.
	//  - On any solid contact, the touch function fires (e.g., rocket explosion).
	//  - Does not stop or bounce; the touch function handles the aftermath
	//    (typically removing the projectile and spawning an explosion).
	// Used for: rockets, nails, lightning bolts, Vore homing missiles, and
	// any projectile that should fly straight and explode on impact.
	MoveTypeFlyMissile

	// MoveTypeBounce — gravity-affected movement that reflects velocity on
	// surface impact instead of stopping. Each frame:
	//  1. Gravity is applied.
	//  2. Velocity is integrated.
	//  3. On surface contact, velocity is reflected off the surface normal
	//     with energy loss (typically velocity *= 0.5 or similar).
	//  4. The touch function fires on each bounce.
	// Used for: bouncing grenades in some mods, rubber projectiles, and
	// any entity that should ricochet off surfaces. Standard Quake grenades
	// actually use MoveTypeBounce (not MoveTypeToss) while in flight.
	MoveTypeBounce

	// MoveTypeGib — lightweight bouncing fragments added for the 2021 Quake
	// rerelease. Behaves like a simplified MoveTypeBounce with less energy
	// retention, causing gib chunks to quickly come to rest. Separating this
	// from MoveTypeBounce allows the engine to apply different damping, fade-out
	// timers, and rendering optimizations (e.g., skipping gibs in low-detail mode).
	MoveTypeGib
)

// SolidType defines how an entity collides with other entities and the world.
//
// Quake's collision system is hull-based. When entity A moves, the engine
// traces A's bounding box through the world BSP and against all other entities.
// The SolidType of each entity determines whether and how it participates in
// these traces:
//
//   - SolidNot entities are invisible to traces (skipped entirely).
//   - SolidTrigger entities are tested but don't block; they fire touch callbacks.
//   - SolidBBox/SolidSlideBox entities block with an axis-aligned bounding box.
//   - SolidBSP entities use their brush model's BSP hull for precise collision.
//
// The distinction between SolidBBox and SolidSlideBox is subtle but important
// for player physics: SolidBBox sets FlagOnGround on the entity standing on it,
// while SolidSlideBox does not (used for entities you can slide off).
type SolidType int

const (
	// SolidNot — non-solid entity. The collision system completely ignores this
	// entity during traces. Other entities pass through it freely, and no touch
	// callbacks fire from collision. Used for: purely decorative entities,
	// corpses (after death, solidity is cleared to prevent blocking),
	// particle emitters, and any entity that should have no physical presence.
	// Note: SolidNot entities can still be hit by explicit QC traceline calls.
	SolidNot SolidType = iota

	// SolidTrigger — trigger volume. The entity has a bounding box (or BSP hull
	// if it has a brush model), but it does not physically block movement.
	// Instead, when another entity's trace overlaps this volume, the trigger's
	// touch function is called. This is the foundation of Quake's trigger system:
	// trigger_once, trigger_multiple, trigger_teleport, trigger_hurt, etc.
	// The entity must also have a valid touch function set in QuakeC, or the
	// overlap is silently ignored.
	SolidTrigger

	// SolidBBox — axis-aligned bounding box collision. The entity blocks
	// movement using a simple box defined by Mins/Maxs. When another entity
	// collides with this box:
	//  - The moving entity is clipped (stopped or redirected along the surface).
	//  - If the moving entity lands on top, FlagOnGround is set on it, enabling
	//    friction and jump logic.
	//  - Touch callbacks fire on both entities.
	// Used for: monsters, players, most interactive entities. The bounding box
	// is always axis-aligned (never rotated), which is why monster collision
	// boxes don't rotate when the monster turns.
	SolidBBox

	// SolidSlideBox — axis-aligned blocking box that does NOT set FlagOnGround
	// on entities standing on it. This prevents entities from "resting" on this
	// object. In practice, this means:
	//  - Entities slide off rather than standing firmly.
	//  - Players cannot jump while standing on a SolidSlideBox entity.
	// Used for: specific gameplay entities where resting should not be allowed,
	// and in some movement edge cases during step detection.
	SolidSlideBox

	// SolidBSP — brush model collision using the entity's BSP hull. Instead of
	// a simple bounding box, collision traces are performed against the full
	// brush geometry of the entity's model (doors, platforms, the worldspawn
	// itself). This enables precise collision with complex shapes:
	//  - Doors that only block when closed.
	//  - Platforms with irregular geometry.
	//  - The world (edict 0) uses SolidBSP with the map's full BSP tree.
	// BSP collision is more expensive than BBox but necessary for architectural
	// brush entities. The engine uses pre-computed clip hulls (hull 0 for point
	// traces, hull 1 for player-sized, hull 2 for shambler-sized) to accelerate
	// these traces.
	SolidBSP
)

// ServerState defines the current state of the server's map/level lifecycle.
//
// The server transitions through these states during level loading:
//
//	ServerStateLoading → ServerStateActive
//
// During Loading, the server is parsing the BSP, spawning entities from the
// entity lump, and building the signon buffer (initial state snapshot). No
// client input is processed and no physics runs. Once all entities are spawned
// and the signon data is ready, the server transitions to Active. In Active
// state, physics runs each frame, client input is accepted, and the game is
// playable.
type ServerState int

const (
	// ServerStateLoading — the server is loading a new map. Entity spawning
	// is in progress. The signon buffer is being filled with baseline entity
	// states, static sounds, and lightstyles. Clients receive signon data
	// but cannot send input yet.
	ServerStateLoading ServerState = iota

	// ServerStateActive — the map is fully loaded and the server is running
	// the game simulation. Physics runs, QuakeC thinks execute, client input
	// is processed, and network updates are sent. This is the normal
	// "playing the game" state.
	ServerStateActive
)

// Physics constants used in collision detection and movement clipping.
const (
	// MoveEpsilon is the minimum movement distance threshold. Movements smaller
	// than this are discarded to prevent infinite loops in the clipping code.
	// When a trace returns a fraction that would result in less than MoveEpsilon
	// units of movement, the entity is considered "stuck" and movement stops.
	// This prevents floating-point precision issues from causing entities to
	// jitter or slide infinitely along surfaces.
	MoveEpsilon = 0.01

	// StopEpsilon is the velocity threshold below which an entity is considered
	// stopped. When an entity's velocity component drops below StopEpsilon, it
	// is snapped to zero. This prevents entities from slowly drifting forever
	// due to floating-point accumulation in friction calculations. Applied
	// independently to each axis (X, Y, Z) of the velocity vector.
	StopEpsilon = 0.1
)

// EntityState represents the baseline (reference) state of an entity for
// network delta compression.
//
// When a client first connects, the server sends a full EntityState for every
// visible entity as a "baseline." On subsequent frames, only fields that differ
// from the baseline are transmitted, dramatically reducing bandwidth. The client
// reconstructs the full state by overlaying the delta on top of the stored
// baseline.
//
// This struct mirrors the C engine's entity_state_t. Each field here corresponds
// to a bit flag in the network update header: if a flag is set, that field
// follows in the packet. If not, the client uses the baseline value.
//
// Fields:
//   - Origin: world-space position (X, Y, Z). Changed almost every frame for
//     moving entities; static entities match their baseline indefinitely.
//   - Angles: orientation as Euler angles (pitch, yaw, roll) in degrees.
//   - ModelIndex: index into the server's model precache table. Determines
//     which 3D model the client renders (0 = invisible/no model).
//   - Frame: current animation frame number within the model's frame list.
//   - Colormap: player color translation index. For players, this maps to
//     shirt/pants color pairs. Non-player entities typically use 0 (no remap).
//   - Skin: model skin (texture variant) index. Allows the same model to
//     have multiple appearances (e.g., different colored armor).
//   - Effects: bitmask of EntityEffects (dynamic lights, muzzle flash, etc.).
//   - Alpha: entity transparency (0 = fully transparent / use default,
//     255 = fully opaque). Added by extended protocols for fade effects.
//   - Scale: entity render scale (0 = use default 1.0). Added by extended
//     protocols for size variation effects.
type EntityState struct {
	Origin     [3]float32
	Angles     [3]float32
	ModelIndex int
	Frame      int
	Colormap   int
	Skin       int
	Effects    int
	Alpha      uint8
	Scale      uint8
}

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

	// Entity — pointer to the edict that was hit, or nil if the trace hit
	// world geometry (or nothing). When non-nil, the engine can fire touch
	// callbacks on both the moving entity and the hit entity.
	Entity *Edict
}

// ClientState tracks the current state of a connected client in the server's
// client management lifecycle.
//
// A client progresses through: Disconnected → Connected → Spawned.
//
//   - Disconnected: the client slot is empty, no player is using it.
//   - Connected: a network connection has been established and the signon
//     handshake is in progress. The client is receiving baseline entity data,
//     precache lists, and server info, but cannot yet interact with the game.
//   - Spawned: the signon handshake is complete. The client's player entity
//     is active in the world, input is accepted, and game state updates are
//     sent each frame.

// Default physics/sound constants used when entity-specific values are not set.
const (
	// DefaultSoundVolume — default volume for sound playback (0-255 scale).
	// 255 is maximum volume. QuakeC can specify lower volumes for quieter
	// effects, but most sounds use this default (full volume at the source,
	// attenuated by distance).
	DefaultSoundVolume = 255

	// DefaultSoundAttenuation — default distance falloff for sounds. Controls
	// how quickly a sound fades with distance from the listener. Standard
	// attenuation values:
	//  0.0 = no attenuation (plays at equal volume everywhere, like music)
	//  1.0 = normal (standard falloff, good for most game sounds)
	//  2.0 = idle (shorter range, for ambient/idle sounds)
	//  3.0 = static (very short range, for point-source effects like torches)
	DefaultSoundAttenuation = 1.0

	// ViewHeight — default eye-level offset from the entity's origin, in
	// Quake units. The player's origin is at their feet; the camera is
	// ViewHeight (22) units above. This produces a camera height of roughly
	// eye-level for Quake's player model. QuakeC can override this via
	// the ViewOfs field for crouching or special camera effects.
	ViewHeight = 22

	// OneEpsilon — general-purpose small epsilon for floating-point
	// comparisons. Matches C ON_EPSILON (0.1) from quakedef.h, used in
	// movement stair-stepping and plane distance checks.
	OneEpsilon = 0.1
)

// Vector math helper functions.
//
// These operate on [3]float32 vectors representing 3D positions, velocities,
// normals, and directions in Quake's coordinate system:
//   - X axis: east (+) / west (-)
//   - Y axis: north (+) / south (-)
//   - Z axis: up (+) / down (-)
//
// Quake uses a right-handed coordinate system with Z-up, which differs from
// some 3D engines that use Y-up. All positions are in "Quake units" where
// 1 unit ≈ 1 inch (a player is about 56 units tall, door frames are 128 units).

// VecAdd returns the component-wise sum of two vectors: result[i] = a[i] + b[i].
//
// Physics use: computing new positions from current position + velocity * dt,
// combining force vectors, offsetting positions (e.g., origin + view offset).
func VecAdd(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

// VecSub returns the component-wise difference of two vectors: result[i] = a[i] - b[i].
//
// Physics use: computing displacement vectors between entities (target - self),
// finding velocity change (new_velocity - old_velocity), and computing relative
// positions for distance/direction calculations.
func VecSub(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

// VecScale returns a vector scaled by a scalar: result[i] = v[i] * s.
//
// Physics use: applying frametime to velocity (velocity * dt), scaling normals
// for backoff calculations, applying friction multipliers, and attenuating
// forces. For example, gravity application: velocity[2] -= sv_gravity * dt
// is equivalent to VecAdd(velocity, VecScale({0,0,-1}, gravity * dt)).
func VecScale(v [3]float32, s float32) [3]float32 {
	return [3]float32{v[0] * s, v[1] * s, v[2] * s}
}

// VecLen returns the Euclidean length (magnitude) of a vector:
// sqrt(v[0]² + v[1]² + v[2]²).
//
// Physics use: computing entity speed (length of velocity vector), measuring
// distances between points (len(VecSub(a, b))), and as a precursor to
// normalization. Note: uses float64 math internally for precision, matching
// the original C engine's use of double-precision sqrt.
func VecLen(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

// VecNormalize normalizes a vector in-place to unit length and returns its
// original length. If the vector has zero length, it remains unchanged.
//
// Physics use: converting displacement vectors to direction vectors for
// AI aiming (aim_direction = normalize(target - self)), computing surface
// normals, and preparing vectors for dot product angle calculations.
// The returned length is useful for simultaneous distance+direction queries
// (e.g., "how far is the enemy and which direction?").
func VecNormalize(v *[3]float32) float32 {
	len := VecLen(*v)
	if len > 0 {
		v[0] /= len
		v[1] /= len
		v[2] /= len
	}
	return len
}

// VecDot returns the dot product of two vectors: a[0]*b[0] + a[1]*b[1] + a[2]*b[2].
//
// Physics use: the dot product is fundamental to collision response and physics.
//   - Dot(velocity, plane_normal) gives the speed of impact against a surface.
//     Used in ClipVelocity to compute the velocity component to remove.
//   - Dot(direction, normal) gives cos(angle) between them. Used for line-of-sight
//     angle checks (is the player in the monster's field of view?).
//   - Dot(point - plane_point, plane_normal) gives signed distance from a plane.
//     Used extensively in BSP traversal and collision detection.
func VecDot(a, b [3]float32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

// VecCopy copies the source vector's components into the destination vector.
//
// Physics use: saving entity positions before movement (for rollback on
// collision), copying positions between entity fields (e.g., OldOrigin = Origin
// before physics runs), and initializing trace result positions.
func VecCopy(src [3]float32, dst *[3]float32) {
	dst[0] = src[0]
	dst[1] = src[1]
	dst[2] = src[2]
}

// VecCross returns the cross product of two vectors, producing a vector
// perpendicular to both inputs. The result follows the right-hand rule.
//
// Physics use: computing surface normals from two edge vectors (for dynamic
// geometry), calculating torque or rotational forces, and finding perpendicular
// directions. Less commonly used in Quake's physics than VecDot, but essential
// for certain geometric operations like building coordinate frames from angles.
func VecCross(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}
