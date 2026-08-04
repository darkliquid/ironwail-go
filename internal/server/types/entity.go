// This file belongs to the Entity/QC subsystem: entity state, user commands, static sounds, and server state types.
package types

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
