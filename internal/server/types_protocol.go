package server

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

// NetMessageType defines client-to-server (CLC = "client command") message types.
//
// These are the messages a connected client can send to the server. The server
// reads these from the client's network stream and dispatches to the appropriate
// handler. The message type byte is the first byte of each client message.
type NetMessageType int

const (
	// CLCNop — no-operation keepalive message. Sent by the client to keep the
	// connection alive when there is no other data to send. The server reads
	// and discards it. Prevents timeout disconnection.
	CLCNop NetMessageType = iota

	// CLCDisconnect — the client is disconnecting gracefully. The server
	// removes the client's entity from the world, frees the client slot,
	// and broadcasts a disconnect message to other players. This is the
	// "clean" way to leave; network errors cause timeout-based disconnection.
	CLCDisconnect

	// CLCMove — movement input from the client (the most frequent message).
	// Contains a UserCmd struct: view angles, movement speeds, button states,
	// and impulse. Sent every client frame (typically 60-72 Hz). The server
	// applies this to the player entity in SV_ClientThink. This is the core
	// of Quake's client-server input pipeline.
	CLCMove

	// CLCStringCmd — a console command string from the client. The client sends
	// this when the player types a command in the console (e.g., "say hello",
	// "kill", "name newname"). The server parses and executes allowed commands.
	// Dangerous commands (e.g., server-side file access) are blocked.
	CLCStringCmd
)

// ServerNetMessage defines server-to-client (SVC = "server command") message types.
//
// These are the messages the server sends to clients to communicate world state
// changes, entity updates, sounds, effects, and game events. The client reads
// these from the server's network stream and updates its local state accordingly.
//
// Many of these messages have "2" variants (e.g., SVCSpawnBaseline2) which are
// extended-protocol versions supporting larger index ranges (16-bit instead of
// 8-bit model/sound indices) for mods with more than 256 models or sounds.
type ServerNetMessage int

const (
	// SVCNop — no-operation keepalive. Server sends this to prevent client
	// timeout when there's no real data to transmit.
	SVCNop ServerNetMessage = iota

	// SVCDamage — notifies the client that their player took damage. Contains
	// the armor-absorbed amount, health-lost amount, and the direction the
	// damage came from. The client uses this to show the red screen flash,
	// directional damage indicator, and view kick.
	SVCDamage

	// SVCDisplayDisconnect — instructs the client to display a disconnect
	// message (e.g., "Server shutting down"). The client shows the message
	// and returns to the main menu.
	SVCDisplayDisconnect

	// SVCLevelName — sends the human-readable level name (e.g., "The Slipgate
	// Complex"). Displayed in the loading screen and intermission scoreboard.
	SVCLevelName

	// SVCLoaded — signals that the server has finished loading the level and
	// the client can proceed with signon stages.
	SVCLoaded

	// SVCMove — server-side acknowledgment of a client move, potentially
	// including corrections. Used for server-authoritative position updates.
	SVCMove

	// SVCEnterServer — signals the client to transition from signon to active
	// gameplay. The client starts its render loop and begins sending UserCmd.
	SVCEnterServer

	// SVCSound — plays a sound effect. Contains: sound index, volume,
	// attenuation, channel, entity number, and origin. The client spatializes
	// the sound relative to the listener (distance, direction, Doppler).
	SVCSound

	// SVCPrint — prints a text message to the client's console/HUD. Used for
	// chat messages, kill notifications, and server announcements. The text
	// appears in the top-left message area and scrolls into the console log.
	SVCPrint

	// SVCSinglePrecisionFrame — sends entity frame numbers as single-precision
	// values (standard protocol). Sufficient for models with < 256 frames.
	SVCSinglePrecisionFrame

	// SVCDoublePrecisionFrame — sends entity frame numbers with extended
	// precision for models with large frame counts (> 255).
	SVCDoublePrecisionFrame

	// SVCCreateBaseline — sends a complete entity baseline during signon.
	// The baseline is the reference state for delta compression. Standard
	// protocol version with 8-bit model/sound indices.
	SVCCreateBaseline

	// SVCCreateBaseline2 — extended-protocol baseline with 16-bit indices,
	// supporting more than 256 models/sounds.
	SVCCreateBaseline2

	// SVCLightStyle — defines a lightstyle pattern. Contains a style index
	// and a string of brightness characters ('a' = dark, 'z' = bright).
	// The client cycles through the string to animate lights. Style 0 is
	// normal, style 1 is flicker, etc.
	SVCLightStyle

	// SVCTempEntity — spawns a temporary visual effect entity (explosion,
	// blood splash, lightning beam, spark shower, etc.). Temp entities are
	// client-side only; they have no server-side edict. Each type has its
	// own data format (position, direction, color, etc.).
	SVCTempEntity

	// SVCCenterPrint — displays a message in the center of the screen.
	// Used for story text, item pickup messages, and mod notifications.
	// The message fades after a few seconds.
	SVCCenterPrint

	// SVCKillMonster — increments the client's killed-monsters count.
	// Sent when a monster dies. The client updates its HUD stat display.
	// The server also tracks total monsters and sends that separately.
	SVCKillMonster

	// SVCSpawnBaseline — sends entity baseline state during signon (standard
	// protocol). Equivalent to SVCCreateBaseline but may use a different
	// encoding path in some engine variants.
	SVCSpawnBaseline

	// SVCSpawnBaseline2 — extended-protocol spawn baseline with 16-bit indices.
	SVCSpawnBaseline2

	// SVCSpawnStatic — spawns a static entity (a world decoration that never
	// moves, like a torch or armor model on a pedestal). Static entities are
	// rendered but have no edict; they consume no server entity slots. Standard
	// protocol with 8-bit model index.
	SVCSpawnStatic

	// SVCSpawnStatic2 — extended-protocol static entity with 16-bit model index.
	SVCSpawnStatic2

	// SVCSpawnStaticSound — creates a looping ambient sound at a fixed position
	// (wind, lava bubbling, etc.). Standard protocol with 8-bit sound index.
	SVCSpawnStaticSound

	// SVCSpawnStaticSound2 — extended-protocol static sound with 16-bit sound index.
	SVCSpawnStaticSound2

	// SVCClientData — per-frame client-specific data update. Contains: view
	// height, ideal pitch, punch angle, velocity, items bitmask, weapon model,
	// weapon frame, ammo counts, and active weapon. This is the "HUD update"
	// message — everything the client needs to draw the status bar.
	SVCClientData

	// SVCDownload — file download data. Used when the client needs to download
	// custom content (models, sounds, maps) from the server. Contains a data
	// chunk and progress information.
	SVCDownload

	// SVCUpdatePing — updates a player's ping display in the scoreboard.
	// Contains the client index and their current ping in milliseconds.
	SVCUpdatePing

	// SVCUpdateFrags — updates a player's frag (kill) count. Contains the
	// client index and new frag count. All clients receive this for scoreboard.
	SVCUpdateFrags

	// SVCUpdateStat — updates a single client stat value (health, armor, ammo,
	// etc.). Used for incremental stat changes between full SVCClientData updates.
	SVCUpdateStat

	// SVCParticle — spawns a burst of particles at a position with a direction.
	// Used for bullet impact sparks, blood sprays, and other particle effects.
	// Legacy message type; modern engines may use SVCTempEntity instead.
	SVCParticle

	// SVCCDTrack — commands the client to play a CD audio track. Contains the
	// track number. Originally played from the game CD; modern source ports
	// play equivalent music files (OGG/MP3).
	SVCCDTrack

	// SVCLocalSound — plays a sound that only this client hears (not spatialized
	// from a world position). Used for UI sounds, pickup sounds, and other
	// feedback that doesn't need 3D positioning.
	SVCLocalSound

	// SVCSetAngle — forces the client's view angles to specific values. Used
	// after teleportation to face the destination's target direction, and
	// during intermission to look at the camera angle.
	SVCSetAngle

	// SVCSetView — sets which entity the client's camera follows. Normally
	// this is the player's own entity, but can be changed for spectator
	// cameras, cutscenes, or intermission views.
	SVCSetView

	// SVCUpdateUserInfo — updates a client's user info (name, colors, etc.)
	// for all other clients. Sent when a player changes their name or colors.
	SVCUpdateUserInfo

	// SVCSignOnNum — advances the client's signon stage. The server sends
	// this to tell the client to proceed to the next stage of the connection
	// handshake. Contains the signon stage number.
	SVCSignOnNum

	// SVCStuffText — sends a console command for the client to execute locally.
	// The server can force clients to run commands (e.g., "reconnect", "bf"
	// for bonus flash). Security-sensitive: modern engines restrict which
	// commands the server can stuff.
	SVCStuffText

	// SVCTime — sends the current server time to the client. The client uses
	// this for interpolation timing, prediction, and animation. Sent at the
	// start of each server frame's client update.
	SVCTime

	// SVCSetInfo — sets a key-value pair in the client's server info. Used
	// for server-wide settings like hostname, maxclients, deathmatch mode.
	SVCSetInfo

	// SVCServerInfo — sends the server's info string containing map name,
	// game directory, and protocol version. Sent during the first signon
	// stage so the client knows which map to load.
	SVCServerInfo

	// SVCUpdateEnt — updates an entity's state using delta compression against
	// its baseline. Contains a bitmask of changed fields followed by the new
	// values for those fields. This is the primary entity update message and
	// constitutes the majority of network bandwidth.
	SVCUpdateEnt

	// SVCLocalSound2 — extended-protocol local sound with 16-bit sound index.
	SVCLocalSound2
)

// Max constants for server limits.
//
// These constants define hard upper bounds on various server resources. They
// originate from the original Quake protocol and engine design, where fixed-size
// arrays were used for performance. Some have been increased from original Quake
// values (e.g., MaxModels was 256 in the original protocol) by extended protocols
// like FitzQuake Protocol 666 and its derivatives.
const (
	// MaxClients — maximum number of simultaneous player connections. Original
	// Quake supported up to 16 players in deathmatch. Each client occupies an
	// edict slot (edicts 1 through MaxClients) and a client_t structure that
	// tracks connection state, signon progress, and network stats.
	MaxClients = 16

	// MaxModels — maximum number of precached models (maps, sprites, aliases).
	// Models are referenced by index in network messages. The original protocol
	// used 8-bit indices (max 256); extended protocols use 16-bit indices,
	// allowing 2048. Includes the world BSP (always index 1), all monster/weapon
	// models, brush entity models (doors, platforms), and sprite effects.
	MaxModels = 2048

	// MaxSounds — maximum number of precached sound effects. Like models, sounds
	// are referenced by index. Original protocol: 256; extended: 2048. Includes
	// all weapon sounds, monster sounds, ambient sounds, and UI sounds.
	MaxSounds = 2048

	// MaxEdicts — maximum number of entities that can exist simultaneously. This
	// is the size of the server's edict array. Every player, monster, projectile,
	// door, trigger, and item consumes an edict. Original Quake: 600; modern
	// engines: 8192 or more. Running out of edicts causes "ED_Alloc: no free
	// edicts" errors, which is a common problem in complex maps.
	MaxEdicts = 8192

	// MaxDatagram — maximum size in bytes of a single network datagram (UDP
	// packet payload). Quake uses unreliable datagrams for real-time updates.
	// If a frame's entity updates exceed this size, entities are dropped from
	// that frame's update (the client uses its previous data). Larger values
	// allow more entities per update but risk UDP fragmentation.
	MaxDatagram = 32000

	// MaxSignonBuffers — maximum number of signon buffer segments. The signon
	// data (baselines, static entities, static sounds, lightstyles) is split
	// into multiple buffers that are sent as reliable messages. If the total
	// signon data exceeds MaxSignonBuffers * buffer_size, the level cannot load.
	MaxSignonBuffers = 16

	// MaxEntityLeafs — maximum number of BSP leaves an entity can touch for
	// PVS visibility tracking. If an entity overlaps more leaves than this,
	// it is marked as "always visible" to prevent PVS culling from hiding
	// large entities. Matches the LeafNums array size in Edict.
	MaxEntityLeafs = 32
)
