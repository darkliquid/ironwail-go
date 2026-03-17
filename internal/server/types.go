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

// DeadFlag defines the death state of an entity.
//
// Quake entities progress through a death state machine: DeadNo → DeadDying →
// DeadDead → (optionally) DeadRespawnable. QuakeC game logic reads this field
// to determine whether an entity should accept damage, run death animations,
// or be eligible for respawn. The server physics does not directly use DeadFlag;
// it is purely a game-logic state consumed by QuakeC.
type DeadFlag int

const (
	// DeadNo — the entity is alive. Normal AI think functions run, the entity
	// accepts damage (if TakeDamage != DamageNo), and player input is processed.
	DeadNo DeadFlag = iota

	// DeadDying — the entity is in its death animation sequence. QuakeC
	// typically sets this in the entity's th_die function. During this state,
	// the entity may still be solid (blocking movement) and may accept
	// additional damage (for gibbing). The death animation frames play out
	// via think function chaining.
	DeadDying

	// DeadDead — the entity's death sequence is complete. The corpse is
	// typically made non-solid (SolidNot) so it doesn't block movement.
	// The entity remains in the world as a visual corpse. Some mods check
	// this to determine if a corpse can be gibbed by further damage.
	DeadDead

	// DeadRespawnable — death has been fully processed and the entity is
	// eligible for respawn. In deathmatch, items use this state to track
	// that they've been picked up and are waiting for their respawn timer.
	// Players in deathmatch transition here after the death animation
	// completes, enabling the respawn button prompt.
	DeadRespawnable
)

// TakeDamage defines how an entity receives damage from attacks.
//
// This field is checked by QuakeC's T_Damage function before applying damage.
// The distinction between DamageYes and DamageAim is important for Quake's
// auto-aim system: when a player fires, the engine traces forward and slightly
// upward/downward looking for DamageAim entities to lock onto, providing the
// classic Quake "vertical auto-aim" that helps players hit monsters on
// different elevations.
type TakeDamage int

const (
	// DamageNo — the entity ignores all damage. Used for: world brushes,
	// triggers, decorations, and any entity that should be indestructible.
	// QuakeC's T_Damage returns immediately when this is set.
	DamageNo TakeDamage = iota

	// DamageYes — the entity takes damage from direct hits. When hit,
	// QuakeC's T_Damage applies the damage, triggers pain animations,
	// and may kill the entity. However, the engine's auto-aim system
	// does NOT target DamageYes entities — only DamageAim entities get
	// auto-aim lock-on.
	DamageYes

	// DamageAim — the entity takes damage AND is targetable by the
	// auto-aim system. When a player fires a weapon, the engine traces
	// from the player's view at slight vertical offsets. If a DamageAim
	// entity is found, the shot is redirected toward it. All standard
	// Quake monsters use DamageAim so players can hit them easily.
	DamageAim
)

// EntityFlags define entity behavior flags (stored in EntVars.Flags as a bitmask).
//
// These flags control physics behavior, AI navigation, and gameplay state.
// They are set/cleared by both the engine (e.g., FlagOnGround is managed by
// physics code) and QuakeC game logic (e.g., FlagFly is set when spawning
// flying monsters). Multiple flags can be active simultaneously.
//
// The flags are a critical part of the physics pipeline. For example,
// SV_Physics_Walk checks FlagOnGround to decide whether to apply gravity
// and friction. Monster AI checks FlagFly/FlagSwim to choose movement
// strategies. The network code checks FlagClient to identify player entities.
const (
	// FlagFly — entity uses flying movement. When set on a monster, the AI
	// movement code allows full 3D navigation (not restricted to ground).
	// The entity's movetype should also be MoveTypeFly. Flying monsters
	// include the Scrag (Wizard) and can move vertically to chase players.
	// This flag tells the AI pathfinding to consider vertical movement.
	FlagFly = 1 << iota

	// FlagSwim — entity is a swimmer. The AI movement code treats water
	// volumes as navigable space rather than obstacles. Swimming monsters
	// like the Rotfish have this flag set. Without it, monsters treat water
	// as impassable terrain and won't enter water volumes voluntarily.
	FlagSwim

	// FlagConveyor — entity is affected by conveyor belt movement. When set,
	// the entity's velocity is modified by the conveyor brush's movedir,
	// simulating being carried along a surface. Rarely used in standard Quake
	// but available for custom map entities.
	FlagConveyor

	// FlagClient — marks this entity as a player-controlled client. The server
	// uses this flag to identify which edicts correspond to connected players.
	// Client entities receive special treatment: UserCmd input is processed,
	// client-specific network messages are sent, and player physics (MoveTypeWalk)
	// is applied. Edict numbers 1 through MaxClients are reserved for clients,
	// and this flag is set when a client connects.
	FlagClient

	// FlagInWater — entity is currently touching a water volume. Set by the
	// physics code when the entity's origin is inside a water brush. Used to:
	//  - Trigger water entry/exit sounds and splash effects.
	//  - Modify physics (reduced gravity, swim acceleration).
	//  - Apply drowning damage when the entity's air supply runs out.
	// Cleared when the entity leaves water.
	FlagInWater

	// FlagMonster — marks this entity as a monster/NPC. Set by QuakeC when
	// spawning monsters. The engine uses this for:
	//  - Collision filtering (some traces skip monsters for performance).
	//  - Kill counting (incrementing the "killed monsters" stat).
	//  - AI-specific physics paths in MoveTypeStep.
	// All standard Quake monsters (Grunt, Knight, Ogre, Shambler, etc.) have
	// this flag.
	FlagMonster

	// FlagGodMode — entity is invulnerable to all damage. When set, QuakeC's
	// T_Damage function skips damage application entirely. Toggled by the
	// "god" console command for players. The entity still receives touch
	// callbacks and physics interactions; only damage is suppressed.
	FlagGodMode

	// FlagNoTarget — entity is invisible to monster AI targeting. Monsters
	// will not acquire this entity as an enemy, even if it's in line of sight.
	// Toggled by the "notarget" console command. Useful for debugging: the
	// player can walk past monsters without being attacked. Does not affect
	// damage — monsters already fighting will continue to attack.
	FlagNoTarget

	// FlagItem — marks this entity as a pickup item. Items have special
	// touch handling in QuakeC: when a player touches an item, it is
	// "picked up" (added to inventory), a pickup sound plays, and the item
	// becomes invisible (or removed in deathmatch). Items include weapons,
	// ammo, health, armor, powerups, and keys.
	FlagItem

	// FlagOnGround — entity is resting on a solid surface this frame. This
	// is one of the most important physics flags. When set:
	//  - Ground friction is applied to horizontal velocity (slowing the entity).
	//  - Gravity is NOT applied (the entity is supported by the ground).
	//  - Players can jump (jumping requires FlagOnGround to be set).
	//  - MoveTypeToss entities stop moving (velocity is zeroed).
	// Set by physics when a downward trace finds ground within step distance.
	// Cleared when the entity moves off the ground (falls, jumps, etc.).
	FlagOnGround

	// FlagPartialGround — some of the entity's bounding box corners are
	// unsupported (hanging over a ledge). Used by monster AI for edge/ledge
	// avoidance: when this flag is set, the monster's movement code tries to
	// steer away from the edge to prevent falling off. Players don't use
	// this flag; it's purely an AI navigation hint.
	FlagPartialGround

	// FlagWaterJump — entity is in a water-jump animation. When a player
	// swims to the surface and presses forward against a ledge, the engine
	// detects this and initiates a "water jump": a scripted upward boost
	// that launches the player out of the water onto the ledge. During this
	// animation, normal movement input is suppressed and the entity follows
	// a fixed trajectory. Cleared when the jump completes or times out.
	FlagWaterJump

	// FlagJumpReleased — the jump button has been released since the last
	// jump. This acts as a debounce latch to prevent continuous jumping
	// while the button is held down. In standard Quake, holding jump causes
	// the player to jump once, then this flag is cleared; the player must
	// release and re-press jump to jump again. Some mods modify this
	// behavior to allow "bunny hopping" by auto-clearing the flag.
	FlagJumpReleased
)

// EntityEffects define visual effects attached to entities (stored in EntVars.Effects
// as a bitmask). These flags are read by the client rendering code to add dynamic
// lights, particle effects, and muzzle flashes to entities. Multiple effects can
// be combined. Effects are transmitted as part of the entity state in each server
// update, so the client always knows which effects to render.
const (
	// EffectBrightField — renders a swirling cloud of yellow-gold particles
	// around the entity, creating a "bright field" halo effect. Used on items
	// of significance like the Sigils (runes) in single-player Quake. The
	// particles orbit the entity's origin in a spherical pattern.
	EffectBrightField = 1 << iota

	// EffectMuzzleFlash — renders a one-frame muzzle flash at the entity's
	// weapon position. Set by QuakeC when a monster or player fires a weapon.
	// The client renders a bright flash emanating from the entity's gun barrel.
	// This flag is typically cleared by the server after one network update
	// cycle, so the flash only appears for a single frame.
	EffectMuzzleFlash

	// EffectBrightLight — attaches a strong dynamic point light to the entity
	// (radius ~400 units). The light moves with the entity and illuminates
	// nearby surfaces. Used for: rockets in flight (bright orange glow),
	// explosion effects, and any entity that should strongly illuminate its
	// surroundings.
	EffectBrightLight

	// EffectDimLight — attaches a weaker dynamic point light to the entity
	// (radius ~200 units). Less intense than EffectBrightLight. Used for:
	// player entities carrying a light source, torches, and subtle ambient
	// glow effects.
	EffectDimLight

	// EffectQuadLight — adds the Quad Damage powerup glow effect. When a
	// player picks up Quad Damage, this effect is set on their entity,
	// rendering a distinctive blue dynamic light that signals to other players
	// (in deathmatch) that this player has quad damage active. Added for the
	// 2021 Quake rerelease.
	EffectQuadLight

	// EffectPentaLight — adds the Pentagram of Protection (invulnerability)
	// glow effect. Renders a red dynamic light on the entity, signaling that
	// the player is currently invulnerable. Like EffectQuadLight, this provides
	// a visual gameplay cue visible to other players.
	EffectPentaLight

	// EffectCandleLight — renders a flickering candle-style point light. The
	// light intensity varies randomly each frame to simulate a candle flame.
	// Used on torch and candle entities for atmospheric lighting. The flicker
	// pattern is generated client-side so it doesn't require network bandwidth.
	EffectCandleLight
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
	// NumLeafs is the count of BSP leaves this entity overlaps.
	// LeafNums stores the leaf indices (up to MaxEntityLeafs).
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
	// Free becomes true). The slot is not reused until at least 2 seconds
	// have elapsed, preventing "ghost" entity references on clients that
	// haven't yet received the removal message.
	FreeTime float32

	// Vars points to the QuakeC-visible entity fields. This is the bridge
	// between the engine and game logic: QuakeC reads and writes these
	// fields, and the engine's physics code also reads/modifies them.
	Vars *EntVars
}

// EntVars contains the QuakeC-exported entity fields — the "progs" side of an entity.
//
// This struct is the bridge between the engine (Go/C code) and QuakeC game logic.
// QuakeC programs read and write these fields to control entity behavior, and the
// engine's physics and networking code also reads them to drive simulation.
//
// # Why float32 for everything?
//
// QuakeC has only one numeric type: a 32-bit float. All numeric values — positions,
// velocities, health, flags, even boolean-like fields — are stored as float32 to
// match the QuakeC VM's native representation. The engine converts to/from int as
// needed (e.g., casting Flags to int for bitmask operations). This is a direct
// consequence of Quake's original design where the QC VM used a single "float"
// register type for all arithmetic.
//
// # Why int32 for some fields?
//
// Fields typed as int32 are NOT numeric values — they are indices into QuakeC VM
// tables:
//   - String references (ClassName, Model, NetName, Target, etc.): indices into
//     the QC string table (pr_strings). The engine resolves these to Go strings
//     when needed.
//   - Entity references (Enemy, Owner, GroundEntity, etc.): edict numbers. Entity
//     0 is worldspawn, entities 1-MaxClients are players.
//   - Function references (Think, Touch, Use, Blocked): indices into the QC
//     function table. The engine calls these by looking up the function pointer
//     and invoking the QC interpreter.
//
// # Field Layout
//
// The field order matters: it must match the QuakeC progs.dat field definition
// table exactly. The QC VM accesses fields by byte offset from the start of the
// EntVars struct. If the field order doesn't match, QC reads/writes the wrong
// fields, causing bizarre gameplay bugs.
type EntVars struct {
	// ModelIndex — precache index of the entity's visual model. Set by QuakeC's
	// setmodel() builtin. The server sends this to clients so they know which
	// model to render. 0 means no model (invisible).
	ModelIndex float32

	// AbsMin/AbsMax — the entity's world-space axis-aligned bounding box,
	// computed by the engine from Origin + Mins/Maxs. Updated whenever the
	// entity moves. Used for broad-phase collision detection: if two entities'
	// AbsMin/AbsMax boxes don't overlap, detailed collision is skipped.
	AbsMin [3]float32
	AbsMax [3]float32

	// LTime — local time for MoveTypePush entities (doors, platforms). Unlike
	// other entities which use the global server time, pushers track their own
	// timeline. This allows a door to pause mid-movement (e.g., when blocked)
	// without affecting the global clock. NextThink for pushers is relative to
	// LTime, not server time.
	LTime float32

	// MoveType — determines which physics function runs for this entity each
	// frame. See the MoveType constants for detailed descriptions. Stored as
	// float32 because QuakeC sets it as a numeric value.
	MoveType float32

	// Solid — determines collision behavior. See SolidType constants. Controls
	// whether and how this entity participates in collision traces.
	Solid float32

	// Origin — world-space position of the entity (center of bounding box for
	// point entities, brush origin for brush models). This is THE position
	// used for physics, rendering, and networking.
	Origin [3]float32

	// OldOrigin — position from the previous physics frame. Used by the client
	// for interpolation: the renderer blends between OldOrigin and Origin to
	// produce smooth movement between discrete server ticks.
	OldOrigin [3]float32

	// Velocity — current movement velocity in units/second. Physics integrates
	// this each frame: new_origin = origin + velocity * frametime. Gravity
	// adds to velocity[2] (Z axis). Friction reduces horizontal components.
	Velocity [3]float32

	// Angles — entity orientation as Euler angles (pitch, yaw, roll) in degrees.
	// For players, Angles[0] is look pitch, Angles[1] is look yaw. For brush
	// models (doors), rotation around these axes produces visual rotation.
	Angles [3]float32

	// AVelocity — angular velocity in degrees/second for each axis. Integrated
	// each frame: new_angles = angles + avelocity * frametime. Used for
	// spinning pickup items, rotating fans, etc.
	AVelocity [3]float32

	// PunchAngle — temporary view angle offset applied to the player's view
	// after taking damage or firing a weapon. Creates a "kick" effect. Decays
	// back to zero over several frames, producing the classic Quake screen
	// shake when hit or shooting.
	PunchAngle [3]float32

	// ClassName — QC string table index for this entity's class name (e.g.,
	// "player", "monster_ogre", "func_door"). Set during entity spawning from
	// the BSP entity lump. Used for debugging and by QC game logic.
	ClassName int32

	// Model — QC string table index for the model path (e.g., "progs/soldier.mdl").
	// Set by QuakeC's setmodel() builtin. The engine resolves this to a precache
	// index (ModelIndex) for network transmission.
	Model int32

	// Frame — current animation frame within the model's frame list. QuakeC
	// sets this each think cycle to drive animations (walking, attacking, dying).
	// The client interpolates between frames for smooth animation.
	Frame float32

	// Skin — selects which skin (texture variant) to use on the model. Models
	// can have multiple skins; QuakeC sets this to change appearance (e.g.,
	// different colored shirts for different enemy types).
	Skin float32

	// Effects — bitmask of EntityEffects (dynamic lights, particles, etc.).
	// See the EntityEffects constants. Set by QuakeC to add visual effects.
	Effects float32

	// Mins/Maxs — entity bounding box relative to Origin. Mins is the bottom-
	// left-front corner offset, Maxs is the top-right-back corner offset.
	// For a player, typical values are {-16,-16,-24} / {16,16,32}. The engine
	// computes AbsMin/AbsMax as Origin + Mins / Origin + Maxs.
	Mins [3]float32
	Maxs [3]float32

	// Size — bounding box dimensions: Size = Maxs - Mins. Computed by the
	// engine when setsize() is called. Convenience field to avoid recomputing.
	Size [3]float32

	// Touch — QC function table index for the touch callback. Called when this
	// entity is touched by another entity during physics movement. The function
	// receives the touching entity as a parameter. Examples: item pickup logic,
	// trigger activation, projectile impact.
	Touch int32

	// Use — QC function table index for the use callback. Called when another
	// entity "uses" this one (e.g., a button press triggers a door's use
	// function). QuakeC chain: player touches button → button.touch fires →
	// button uses its target → target.use fires.
	Use int32

	// Think — QC function table index for the think callback. Called when
	// server time >= NextThink. This is the primary mechanism for entity AI
	// and behavior: monsters check for enemies, doors check if they should
	// close, projectiles check lifetime expiry. After thinking, the entity
	// typically sets a new NextThink and Think to schedule the next callback.
	Think int32

	// Blocked — QC function table index for the blocked callback. Called on
	// MoveTypePush entities (doors, platforms) when they are blocked by another
	// entity during movement. Typically triggers crush damage on the blocker
	// and/or reverses the pusher's direction.
	Blocked int32

	// NextThink — server time at which the Think function should fire. For
	// MoveTypePush entities, this is relative to LTime instead. Set to 0 or
	// a past time to disable thinking. QuakeC typically sets this in each
	// think function to schedule the next think (e.g., NextThink = time + 0.1
	// for 10 Hz AI updates).
	NextThink float32

	// GroundEntity — edict number of the entity this one is standing on.
	// Set by the physics code when FlagOnGround is set. 0 means standing
	// on the world (map geometry). Used to inherit platform movement: when
	// standing on a MoveTypePush entity, the player moves with it.
	GroundEntity int32

	// Health — entity hit points. QuakeC's T_Damage subtracts from this.
	// When Health drops to 0 or below, the entity's th_die function fires.
	// Players start with 100 health; monsters vary (Grunt=30, Shambler=600).
	Health float32

	// Frags — kill count for players in deathmatch. Incremented by QuakeC
	// when a player kills another player. Sent to all clients for the
	// scoreboard display. Not used for monsters.
	Frags float32

	// Weapon — bit flag indicating the player's currently selected weapon.
	// QuakeC uses IT_* item flags (e.g., IT_SHOTGUN=1, IT_ROCKET_LAUNCHER=32).
	// Combined with Items to determine available weapons.
	Weapon float32

	// WeaponModel — QC string table index for the player's viewmodel (the
	// first-person weapon model, e.g., "progs/v_shot.mdl"). Set by QuakeC
	// when the player switches weapons. The client renders this model in the
	// lower portion of the screen.
	WeaponModel int32

	// WeaponFrame — current animation frame of the weapon viewmodel. QuakeC
	// advances this during firing/reloading animations.
	WeaponFrame float32

	// CurrentAmmo — ammo count for the currently selected weapon. Displayed
	// in the HUD. QuakeC updates this when the player switches weapons or
	// fires.
	CurrentAmmo float32

	// AmmoShells/AmmoNails/AmmoRockets/AmmoCells — ammo pools for each ammo
	// type. Shells are used by shotguns, nails by nailguns, rockets by the
	// rocket/grenade launcher, cells by the lightning gun. QuakeC manages
	// these; the engine just stores and transmits them.
	AmmoShells  float32
	AmmoNails   float32
	AmmoRockets float32
	AmmoCells   float32

	// Items — bitmask of items/weapons the entity possesses. Uses IT_* flags
	// defined in QuakeC (e.g., IT_SHOTGUN, IT_KEY1, IT_QUAD). The client
	// reads this to draw the HUD inventory bar and detect powerup states.
	Items float32

	// TakeDamage — determines if and how this entity receives damage. See
	// TakeDamage constants. QuakeC's T_Damage checks this before applying
	// damage.
	TakeDamage float32

	// Chain — edict number used for building temporary linked lists of
	// entities in QuakeC. Functions like findradius() chain matching entities
	// together so QC can iterate over them. Not used by the engine itself.
	Chain int32

	// DeadFlag — current death state. See DeadFlag constants. Managed by
	// QuakeC game logic to track death animation progression.
	DeadFlag float32

	// ViewOfs — offset from Origin to the entity's "eye" position. For
	// players, this is typically {0, 0, 22} (22 units above feet). The
	// engine adds this to Origin when computing the player's view position
	// for rendering and for aiming traces (traceline from eyes).
	ViewOfs [3]float32

	// Button0 — primary attack button state (1.0 = pressed, 0.0 = released).
	// Mapped from the client's UserCmd.Buttons field. QuakeC reads this to
	// trigger weapon firing.
	Button0 float32

	// Button1 — secondary button state. In original Quake, used for jumping
	// in some configurations. Most mods use Impulse for non-attack actions.
	Button1 float32

	// Button2 — tertiary button state (jump). QuakeC checks this in
	// PlayerPreThink to initiate jumps when FlagOnGround is set.
	Button2 float32

	// Impulse — one-shot command value from the client. Unlike buttons (which
	// have press/release states), impulses fire once and are cleared to 0.
	// Used for: weapon switching (impulse 1-8), and custom mod commands.
	// The "impulse 9" cheat gives all weapons; "impulse 255" is quad damage
	// in some mods.
	Impulse float32

	// FixAngle — when set to 1.0 by QuakeC, the server sends a SetAngle
	// message to the client, forcing the player's view to VAngle. Used after
	// teleportation to face the teleport destination's target direction.
	// Cleared to 0 after the message is sent.
	FixAngle float32

	// VAngle — view angles to force on the client when FixAngle is set.
	// Also used to read the client's current view angles (updated from
	// UserCmd.ViewAngles each frame).
	VAngle [3]float32

	// IdealPitch — target pitch angle for auto-pitch on slopes. When a
	// player walks up/down stairs, the engine can automatically tilt the
	// view to match the slope. IdealPitch is the computed target; the view
	// smoothly interpolates toward it.
	IdealPitch float32

	// NetName — QC string table index for the entity's network name. For
	// players, this is their chosen name (shown in chat and scoreboard).
	// For other entities, typically empty.
	NetName int32

	// Enemy — edict number of this entity's current enemy/target. For
	// monsters, this is the player (or other entity) they are chasing and
	// attacking. QuakeC AI code sets this in FindTarget and clears it when
	// the target is lost or killed.
	Enemy int32

	// Flags — bitmask of EntityFlags. See the EntityFlags constants above.
	// Both the engine (physics) and QuakeC read/write these flags. Cast to
	// int for bitmask operations since QC stores everything as float.
	Flags float32

	// Colormap — player color translation index (1-based, matching the
	// player's edict number). The client uses this to remap the model's
	// shirt and pants colors. 0 means no color remapping.
	Colormap float32

	// Team — team number for team-based game modes. QuakeC sets this during
	// player spawn. Used by teamplay rules to prevent friendly fire and
	// enable team scoring.
	Team float32

	// MaxHealth — maximum health cap for this entity. QuakeC uses this to
	// limit health pickups. Megahealth (100+ HP) can temporarily exceed this
	// but decays back down over time.
	MaxHealth float32

	// TeleportTime — timestamp of the last teleportation. For a brief window
	// after teleporting, the player's velocity is preserved but certain
	// physics checks (like fall damage) are suppressed. Also used to disable
	// touch callbacks briefly to prevent teleporter loops.
	TeleportTime float32

	// ArmorType — damage absorption fraction of current armor. Standard Quake
	// values: 0.3 (green/100 armor), 0.6 (yellow/150 armor), 0.8 (red/200
	// armor). When the player takes damage, ArmorType * damage is absorbed
	// by armor, and (1 - ArmorType) * damage hits health.
	ArmorType float32

	// ArmorValue — current armor points. Reduced when absorbing damage.
	// When ArmorValue reaches 0, no more damage is absorbed.
	ArmorValue float32

	// WaterLevel — how deep the entity is in water. 0 = not in water,
	// 1 = feet wet (wading), 2 = waist deep, 3 = fully submerged (can drown).
	// Computed by the engine each frame by testing points at the entity's
	// feet, waist, and head against water brushes.
	WaterLevel float32

	// WaterType — the content type of the liquid the entity is in. Matches
	// BSP content types: CONTENTS_WATER (-3), CONTENTS_SLIME (-4),
	// CONTENTS_LAVA (-5). QuakeC uses this to apply different damage rates
	// (slime hurts slowly, lava hurts fast).
	WaterType float32

	// IdealYaw — target yaw angle for monster AI turning. When a monster
	// detects a target, IdealYaw is set to face the target. The AI turning
	// code smoothly rotates the monster's actual yaw toward IdealYaw at
	// YawSpeed degrees per second.
	IdealYaw float32

	// YawSpeed — rotation speed in degrees per second for AI turning. Higher
	// values make the monster snap to face targets faster. Typical: 20-40
	// for slow turners (Shambler), 40-60 for agile monsters (Fiend).
	YawSpeed float32

	// AimEnt — edict number of the entity this one is aiming at. Used by
	// some QuakeC AI routines for targeting logic separate from Enemy.
	AimEnt int32

	// GoalEntity — edict number of this entity's navigation goal. For
	// monsters, this is the path_corner or waypoint they are walking toward.
	// Separate from Enemy: a monster can be walking toward a GoalEntity while
	// also chasing an Enemy.
	GoalEntity int32

	// SpawnFlags — flags from the BSP entity lump, parsed during level load.
	// These are the checkboxes set in the map editor. Examples:
	//  - Bit 0: SPAWNFLAG_NOT_EASY (don't spawn on Easy difficulty).
	//  - Bit 1: SPAWNFLAG_NOT_MEDIUM.
	//  - Bit 2: SPAWNFLAG_NOT_HARD.
	//  - Entity-specific flags (e.g., door starts open, light starts off).
	SpawnFlags float32

	// Target — QC string table index for this entity's target name. When the
	// entity fires (triggers/doors/buttons), it activates all entities whose
	// TargetName matches this Target. This is the core of Quake's entity I/O
	// system: cause → effect chains.
	Target int32

	// TargetName — QC string table index for this entity's own name. Other
	// entities reference this name in their Target field to form trigger chains.
	// Example: a button's Target = "door1", a door's TargetName = "door1".
	TargetName int32

	// DmgTake — damage taken this frame (after armor absorption). QuakeC sets
	// this in T_Damage. The client reads it to determine screen tint intensity
	// (red flash proportional to damage taken).
	DmgTake float32

	// DmgSave — damage absorbed by armor this frame. Sent to the client
	// alongside DmgTake. The client can show a different color tint or icon
	// for absorbed damage.
	DmgSave float32

	// DmgInflictor — edict number of the entity that caused the damage (e.g.,
	// a rocket entity, or the world for fall damage). QuakeC uses this for
	// death attribution and kill credit in deathmatch.
	DmgInflictor int32

	// Owner — edict number of this entity's owner/creator. A rocket's Owner
	// is the player who fired it. Owned entities cannot collide with their
	// owner (prevents rockets from hitting the player who fired them). This
	// exclusion is hard-coded in the engine's collision trace code.
	Owner int32

	// MoveDir — normalized direction vector for entity movement. For brush
	// entities, QuakeC computes this from the "angle" key in the map editor.
	// A door's MoveDir determines which direction it opens. For trigger_push,
	// MoveDir determines the push direction.
	MoveDir [3]float32

	// Message — QC string table index for a text message associated with this
	// entity. For triggers, this message is displayed to the player when
	// activated. For intermission, this is the end-of-level text. Sent to
	// the client as a center-print message.
	Message int32

	// Sounds — numeric sound selection for this entity. Map editors use this
	// field to choose between preset sound sets (e.g., a door can have
	// different open/close sound styles based on this value).
	Sounds float32

	// Noise/Noise1/Noise2/Noise3 — QC string table indices for sound file
	// paths associated with this entity. QuakeC sets these during entity
	// spawn to configure the sounds the entity makes during various actions
	// (e.g., Noise = open sound, Noise1 = close sound for a door).
	Noise  int32
	Noise1 int32
	Noise2 int32
	Noise3 int32
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
type ClientState int

const (
	// ClientStateDisconnected — this client slot is available. No player is
	// connected. The server skips this slot during entity updates and input
	// processing.
	ClientStateDisconnected ClientState = iota

	// ClientStateConnected — a player has connected and the signon handshake
	// is underway. The server is sending signon data (server info, model/sound
	// precache lists, entity baselines, static sounds, lightstyles). The client
	// cannot send movement commands yet.
	ClientStateConnected

	// ClientStateSpawned — the signon handshake is complete and the player
	// entity is fully active in the world. The server processes this client's
	// UserCmd input each frame and includes their entity in world state updates.
	// This is the normal "playing the game" state.
	ClientStateSpawned
)

// SignonStage tracks the connection handshake progress between client and server.
//
// The Quake signon handshake is a multi-step protocol where the server sends
// initial world state in stages and the client acknowledges each stage. This
// ensures the client has all necessary data (models, sounds, entity baselines)
// before the game begins.
//
// The sequence is:
//
//	SignonNone → SignonPrespawn → SignonSignonBufs → SignonSignonMsg → SignonFlush → SignonDone
//
// At each stage, the server waits for the client's acknowledgment before
// proceeding to the next. This prevents the client from being overwhelmed
// by data and ensures reliable delivery over the unreliable Quake network
// protocol.
type SignonStage int

const (
	// SignonNone — initial state. No signon data has been sent yet. The
	// client has just connected and the server is preparing the signon
	// buffer.
	SignonNone SignonStage = iota

	// SignonPrespawn — server has sent ServerInfo (map name, model/sound
	// precache lists). The client loads the map BSP and all precached
	// resources during this stage. Once loaded, the client sends a
	// "prespawn" command to request entity baselines.
	SignonPrespawn

	// SignonSignonBufs — server is sending signon buffers containing entity
	// baselines, static entities, and static sounds. These may span multiple
	// network packets due to size. The client stores this data as the
	// reference state for delta compression.
	SignonSignonBufs

	// SignonSignonMsg — server has sent additional signon messages (lightstyles,
	// etc.). The client processes these to configure the visual environment.
	SignonSignonMsg

	// SignonFlush — server has sent the final signon data and is waiting for
	// the client to acknowledge all data has been received and processed.
	// The client flushes its buffers and sends an acknowledgment.
	SignonFlush

	// SignonDone — signon handshake is complete. The client is ready to
	// enter the game. The server transitions the client to ClientStateSpawned
	// and begins sending real-time game state updates. From this point, the
	// client can send UserCmd input and participate in the game.
	SignonDone
)

// NetMessageType defines client-to-server message types.
type NetMessageType int

const (
	CLCNop NetMessageType = iota
	CLCDisconnect
	CLCMove
	CLCStringCmd
)

// ServerNetMessage defines server-to-client message types.
type ServerNetMessage int

const (
	SVCNop ServerNetMessage = iota
	SVCDamage
	SVCDisplayDisconnect
	SVCLevelName
	SVCLoaded
	SVCMove
	SVCEnterServer
	SVCSound
	SVCPrint
	SVCSinglePrecisionFrame
	SVCDoublePrecisionFrame
	SVCCreateBaseline
	SVCCreateBaseline2
	SVCLightStyle
	SVCTempEntity
	SVCCenterPrint
	SVCKillMonster
	SVCSpawnBaseline
	SVCSpawnBaseline2
	SVCSpawnStatic
	SVCSpawnStatic2
	SVCSpawnStaticSound
	SVCSpawnStaticSound2
	SVCClientData
	SVCDownload
	SVCUpdatePing
	SVCUpdateFrags
	SVCUpdateStat
	SVCParticle
	SVCCDTrack
	SVCLocalSound
	SVCSetAngle
	SVCSetView
	SVCUpdateUserInfo
	SVCSignOnNum
	SVCStuffText
	SVCTime
	SVCSetInfo
	SVCServerInfo
	SVCUpdateEnt
	SVCLocalSound2
)

// Max constants for server limits.
const (
	MaxClients       = 16
	MaxModels        = 2048
	MaxSounds        = 2048
	MaxEdicts        = 8192
	MaxDatagram      = 32000
	MaxSignonBuffers = 16
	MaxEntityLeafs   = 32
)

// Default physics/sound constants.
const (
	DefaultSoundVolume      = 255
	DefaultSoundAttenuation = 1.0
	ViewHeight              = 22
	OneEpsilon              = 0.01
)

// Vector math helper functions.

// VecAdd adds two vectors.
func VecAdd(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]}
}

// VecSub subtracts two vectors.
func VecSub(a, b [3]float32) [3]float32 {
	return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

// VecScale scales a vector by a scalar.
func VecScale(v [3]float32, s float32) [3]float32 {
	return [3]float32{v[0] * s, v[1] * s, v[2] * s}
}

// VecLen returns the length of a vector.
func VecLen(v [3]float32) float32 {
	return float32(math.Sqrt(float64(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])))
}

// VecNormalize normalizes a vector, returning its original length.
func VecNormalize(v *[3]float32) float32 {
	len := VecLen(*v)
	if len > 0 {
		v[0] /= len
		v[1] /= len
		v[2] /= len
	}
	return len
}

// VecDot returns the dot product of two vectors.
func VecDot(a, b [3]float32) float32 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

// VecCopy copies a vector.
func VecCopy(src [3]float32, dst *[3]float32) {
	dst[0] = src[0]
	dst[1] = src[1]
	dst[2] = src[2]
}

// VecCross returns the cross product of two vectors.
func VecCross(a, b [3]float32) [3]float32 {
	return [3]float32{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}
