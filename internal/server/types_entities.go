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

	// Map — QC string table index for a trigger_changelevel destination map.
	// Start-map skill/episode exit triggers rely on this field being populated
	// from the BSP entity lump so QuakeC can issue the correct changelevel.
	Map int32

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
