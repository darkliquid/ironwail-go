package server

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
