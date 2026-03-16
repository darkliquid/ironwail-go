# IRONWAIL-GO COMPREHENSIVE PORT REVIEW & PARITY TODO LIST

---

> ## ⚠️ HISTORICAL SNAPSHOT ONLY
>
> **This document is an archive.** It was generated in an early audit pass (2026-03) and is **not** current status tracking.
>
> - Completion percentages, gap tables, severity labels, and task lists are **out of date**.
> - Many gaps described here have since been resolved.
> - **Do not cite this document for current defect filing or sprint planning.**
>
> **For current state, use:**
> - [`PORT_PARITY_REVIEW.md`](PORT_PARITY_REVIEW.md) — canonical current baseline
> - [`PORT_PARITY_TODO.md`](PORT_PARITY_TODO.md) — active ordered backlog
> - [`PARITY_AUDIT_TABLE.md`](PARITY_AUDIT_TABLE.md) — source-backed audit table
>
> The OpenGL path is authoritative for parity; `gogpu` remains secondary/experimental.

---

This document is kept in the repository as **historical context only** — it provides useful background on C/Go algorithm comparison and older gap rationale. It should not be treated as a description of the current state of the port.

This report analyzed major parity gaps between the C Ironwail codebase and the Go port, focusing on the 7 primary functional areas.

---

## A. RENDERER/WORLD RENDERING PARITY GAPS

### Current Go State

**Key Files:**
- `internal/renderer/world.go` - BSP world geometry preprocessing
- `internal/renderer/world_runtime_opengl.go` - OpenGL rendering pipeline (binning, depth sorting)
- `internal/renderer/renderer_gogpu.go` - WebGPU target implementation (680+ lines)
- `internal/renderer/world_opengl.go` - OpenGL shader-based rendering
- `internal/renderer/camera.go` - View frustum and matrix setup

**What's Implemented:**
- Basic BSP geometry loading into CPU data structures (WorldGeometry, WorldVertex)
- World geometry binning by render pass (sky, opaque, alphaTest, translucent)
- Distance-based depth sorting for translucent faces
- Liquid alpha override system (water/lava/slime/teleport with per-map settings)
- Light styles (animated color ramps) basic parsing
- Dynamic light system with distance attenuation (glLightPool in OpenGL)
- Frustum culling at BSP leaf level
- Multiple texture passes (texture binding exists)
- Simple particle and sprite batching frameworks

**Critical Gaps:**

1. **Lightmaps Not Processed** - `world.go:153` sets `LightmapIndex: -1` with TODO comment
   - C code: `r_brush.c` manages `lightmap_t lightmaps[]`, `lightmap_count`, `lightmap_chart`, `lightmap_data[MAX_SANITY_LIGHTMAPS]`
   - C path: BSP faces → lightmap generation → Chart_Init/Chart_AllocBlock → packed atlas → GPU upload
   - Go gap: Lightmap coordinate calculation skipped, lightmap texture never uploaded, surface lighting completely dark
   - Behavior loss: All dynamic lighting, surface detail, and shadows entirely missing from world geometry

2. **Water Warp Animation Missing** - No implementation of `gl_warp.c` effects
   - C code: `R_TextureAnimation()`, wave offset functions for turbulent surfaces
   - Expected: Water surfaces show time-based vertex displacement
   - Current: Water renders flat without wave deformation

3. **Sky Rendering Incomplete**
   - C code: `gl_sky.c` implements 3 modes: cubemap, stencil, layered
   - Only basic sky pass binning exists in Go (separation from opaque)
   - Missing: Actual sky geometry/cubemap texture upload and rendering
   - Behavior: Sky probably renders as black or missing entirely

4. **Texture Animation Not Wired**
   - C code: `R_TextureAnimation(texture_t *base, int frame)` animates with `cl.time*10 % anim_total`
   - Go state: No frame animation logic for alternating texture sequences
   - Effect: Torch flames, scrolling water textures remain static

5. **Fog System Incomplete**
   - C code: Parses `svc_fog` messages (density, color[3], time) → updates fog state globally
   - Go gap: Fog parsing exists in client but never feeds to renderer
   - OpenGL fog: Missing uniforms and fragment shader fog calculation

6. **No GPU Buffer Abstraction Yet**
   - C code: All geometry uploaded once, reused with matrix transforms
   - Go issue: `world.go:1006` has TODO "GPU buffer handles (when gogpu exposes buffer API)"
   - Current: Likely uploading geometry per-frame (major performance issue)

7. **Texture Coordinate Calculation Stubbed**
   - C code: `R_BuildLightmap()` + surface TexInfo provides UV mapping
   - Go: `world.go:269` hardcodes `lightmapCoord := [2]float32{0.0, 0.0}`
   - Effect: Lightmaps don't map correctly even when added

8. **WebGPU (gogpu) 3D Rendering Stub** - `renderer_gogpu.go:1025`
   - Only 2D drawing implemented (pic/fill/character)
   - 3D world, entities, particles marked as TODO/stub
   - GoGPU has render pipeline setup but no world geometry rendering code
   - Fallback: Only works in headless stub mode or GLES fallback

---

## B. ENTITY/MODEL/SPRITE/PARTICLE RENDERING GAPS

### Current Go State

**Key Files:**
- `internal/renderer/entity_types.go` - Type definitions (BrushEntity, AliasModelEntity, SpriteEntity, DecalMarkEntity)
- `internal/renderer/alias_opengl.go` - Alias model frame interpolation (pose blending)
- `internal/renderer/sprite_opengl.go` - Sprite setup
- `internal/renderer/particle.go` - Particle system
- `internal/renderer/particle_runtime_opengl.go` - Runtime particle updates
- `internal/renderer/dynamic_light_opengl.go` - Dynamic light evaluation

**What's Implemented:**
- Particle allocation, update, and vertex building (interpolation between ramps)
- Frame interpolation for alias models (pose1/pose2 blending with factor)
- Sprite frame lookup (SPR_SINGLE, SPR_ANGLED, SPR_GROUP)
- Dynamic light contribution per vertex (attenuation, color)
- Particle color ramps (ramp1, ramp2, ramp3)
- Decal mark entity type (bullet holes, scorch marks)

**Critical Gaps:**

1. **No Entity Rendering Pipeline** - `renderer_gogpu.go:733, 780, 1025`
   - C code: `R_RenderScene()` → `R_DrawBrushModels()` / `R_DrawAliasModels()` / `R_DrawSpriteModels()` / `R_DrawParticles()`
   - All called in strict sequence after world with entity state merged
   - Go gap: No equivalent top-level function that accepts Entity list and produces render calls
   - Integration issue: Renderer never receives entity data from client
   - Behavior: All monsters, players, items, effects completely invisible

2. **No Alias Model Rendering** - OpenGL implementation exists but no integration
   - C path: Entity → model pointer → `aliashdr_t` → pose data → vertex transform → lighting → GPU draw
   - Go code: `alias_opengl.go` has `buildAliasVerticesInterpolated()` but never called from frame render
   - Missing: Model loading, pose index calculation, batching, shader programs
   - Behavior: Player and monsters render as invisible unless explicitly added

3. **Sprite Rendering Incomplete**
   - C code: `R_DrawSpriteModels()` → `R_GetSpriteFrame()` → billboard quad gen
   - Go: `sprite_opengl.go` has frame lookup but no geometry generation or batching
   - Missing: Quad construction, rotation handling, distance sorting for proper overlap
   - Behavior: Flares, explosions, projectiles invisible

4. **Particle Rendering Missing Integration**
   - C code: `R_DrawParticles()` takes `qboolean opaqueonly` flag
   - Renders two passes: opaque (before translucent world) and transparent (after)
   - Go issue: Particle system exists but never called from render loop
   - Behavior: Blood, sparks, explosions all missing

5. **No Lighting Calculation for Alias Models**
   - C code: `R_SetupAliasLighting()` evaluates surface lighting + dynamic lights for each model
   - Uses lightmaps (if available) + light styles + dlights for final vertex color
   - Go gap: Dynamic light contribution exists but no surface lighting integration
   - Behavior: Models are either too bright or completely unlit

6. **Brush Submodel (Inline BSP) Rendering Missing**
   - C code: `R_DrawBrushModels()` iterates entities with brush models (doors, platforms, etc.)
   - Uses `entity_t.model` → submodel BSP tree from `worldmodel->submodels[]`
   - Applies entity-local origin + angles transform before rendering geometry
   - Go gap: BrushEntity type defined but no rendering path connects them to world geometry
   - Behavior: Doors, rotating platforms invisible; map playability broken

7. **No Effect Particles** - Entity effects like EF_MUZZLEFLASH
   - C code: `R_EntityParticles()` generates particles for entity effects
   - `entity.effects` field triggers: EF_BRIGHTFIELD, EF_MUZZLEFLASH, EF_BRIGHTLIGHT, EF_DIMLIGHT
   - Go gap: No effect parsing or particle emission from entity state
   - Behavior: Weapon flashes, teleport glows missing

8. **ViewFrame/Gun Model Missing**
   - C code: Player's held weapon rendered separately in eye space with `r_drawviewmodel` cvar
   - Uses `cl.viewent` (entity 0 with special transform)
   - Applied before HUD/menu overlay with special FOV scaling (`cl_gun_fovscale`)
   - Go gap: No gun model rendering at all
   - Behavior: First-person weapon model invisible

9. **No Stencil/Decal System**
   - C code: `gl_refrag.c` projects decal geometry onto surfaces
   - Go type exists (DecalMarkEntity) but no rendering implementation
   - Behavior: Bullet holes, scorch marks missing

10. **Sprites Don't Orient to Camera Properly**
    - C code: SPR_ANGLED sprites compute facing angle via dot products with camera basis
    - Expected: Sprites rotate to always face camera
    - Go gap: No orientation logic in sprite pipeline

---

## C. MENUS/HUD/CONSOLE/INPUT/CONFIG UX GAPS

### Current Go State

**Key Files:**
- `internal/menu/manager.go` - Menu state machine and navigation
- `internal/hud/hud.go` - HUD overlay (status bar, centerprint)
- `internal/hud/statusbar.go` - Status bar rendering
- `internal/hud/centerprint.go` - Centered message display
- `internal/console/console.go` - Console buffer and scroll
- `internal/input/types.go` - Input structure definitions
- `internal/cvar/cvar.go` - Console variable system
- `internal/cmdsys/cmd.go` - Command system

**What's Implemented:**
- Menu state machine (MenuMain, MenuSinglePlayer, etc.)
- Basic menu navigation (cursor movement, item selection)
- Pause/resume functionality
- Save/load game menu stubs
- HUD status bar rendering (health, armor, ammo)
- Centerprint message display with timeout
- Console input buffer
- CVar registration and access
- Command registration and execution
- Input system type definitions

**Critical Gaps:**

1. **Submenu Systems Are Stubs** - `menu/manager.go:324-354`
   - C code: Full hierarchical menus (M_Menu_Net_f, M_Menu_LanConfig_f, M_Menu_GameOptions_f, M_Menu_Search_f, M_Menu_ServerList_f)
   - Each menu has custom drawing, input handling, item enumeration
   - Go state: 6 submenus echo "TODO" when selected instead of opening
     - Join Game → network server discovery/connection UI
     - Host Game → local server options, player setup
     - Player Setup → name, color, skin selection
     - Controls → key binding UI
     - Video → resolution, brightness, filtering options
     - Audio → volume, sound quality settings
   - Behavior loss: Cannot join multiplayer servers, launch local games, or configure hardware

2. **No Key Binding System** - Missing from keybinds architecture
   - C code: `keys.c` implements `Key_Event()`, command bindings (+forward, +attack, etc.), key binding UI
   - Keys maintain state with edge triggers (state bits 0=current, 1=edge-down, 2=edge-up)
   - `cl_input.c` implements `KeyDown()` / `KeyUp()` for state tracking
   - Go gap: Input system only has basic key event types, no command binding or state machine
   - Missing: `in_mlook`, `in_klook`, `in_left`, `in_right`, `in_forward`, `in_back`, `in_lookup`, `in_lookdown`, `in_strafe`, `in_speed`, `in_use`, `in_jump`, `in_attack`
   - Behavior: Most movement controls unmapped; players must use UI buttons only (extremely limited)

3. **Console Input Limited**
   - C code: Full line editing (insert, backspace, delete, home, end, arrow keys), history (up/down), auto-completion
   - C implementation: `console.c` with scroll buffer, layout, command completion from registered commands
   - Go implementation: Basic Read/Write but no editing mode or completion
   - Behavior: Console hard to use; typos cannot be fixed mid-line

4. **Status Bar Missing Details**
   - C code: `sbar.c` displays weapons (7 types × 8 states), ammo (4 types), armor (3 types), items (32 types), face (7 health × 2 states + powerups)
   - Complex layout with score overlay, frags, weapon flashing
   - Go state: Basic health/armor/ammo numbers only, no weapon icons, no face
   - Behavior: Player doesn't know which weapons they have

5. **No Intermission Screen**
   - C code: `M_Menu_Quit_f()` and intermission state rendering
   - After map completion: shows kills, secrets, time, advance button
   - Go gap: Intermission state exists (`client.Intermission`) but not rendered
   - Behavior: Level completion invisible; players can't see stats or advance

6. **Menu Drawing Uses Placeholder Images**
   - C code: Loads menu pictures from `gfx/menu/` (mainmenu, plaque, cursor, etc.)
   - Each menu item has custom sprite/pic positioning
   - Go: Stubs return echo messages instead of rendering UI
   - Behavior: Menu looks like debug console output, not professional UI

7. **No Video Mode Selection UI**
   - C code: `M_Menu_Video_f()` enumerates resolutions, refresh rates, allows live preview
   - Saves `vid_width`, `vid_height`, `vid_fullscreen` cvars
   - Go gap: No mode enumeration or testing interface
   - Behavior: Users cannot change resolution without config file editing

8. **No Mod Loading UI** - `M_Menu_Mods_f()` stub
   - C code: Lists .pak files, shows mod info, allows launch with mod activated
   - Go gap: Menu exists but lists nothing, provides no mod activation
   - Behavior: Mod support completely broken from UI perspective

9. **Center Print Not Wired to Game Events**
   - C code: Game sends centerprint messages via `svc_centerprint` with timeout
   - Shows level objectives, time warnings, etc.
   - Go gap: Type exists but client never parses `svc_centerprint` correctly
   - Behavior: Game messages won't display during play

10. **No Quit Confirmation Dialog**
    - C code: `M_Menu_Quit_f()` shows confirmation menu before exit
    - Go gap: Direct quit without confirmation
    - Behavior: Easy to accidentally exit during play

11. **CVar Callback System Incomplete**
    - C code: Many cvars register callbacks (e.g., `Cvar_SetCallback(&r_clearcolor, R_ClearColor_f)`)
    - When cvar changes, callback fires (e.g., video reset, texture reload)
    - Go gap: No callback mechanism in CVar struct
    - Behavior: Video setting changes require manual restart instead of live update

12. **Menu Music Not Wired**
    - C code: `M_Music_f()` plays theme.ogg while menu active
    - Pauses game music, resumes on map load
    - Go gap: No menu music system
    - Behavior: Silent menu instead of iconic Quake theme

---

## D. AUDIO/MUSIC GAPS

### Current Go State

**Key Files:**
- `internal/audio/sound.go` - Main audio system (System, channels, cache)
- `internal/audio/backend.go` - Backend interface
- `internal/audio/backend_sdl3.go` / `backend_oto.go` - Implementation backends
- `internal/audio/cache.go` - Sound sample cache
- `internal/audio/wav.go` - WAV file loading
- `internal/audio/spatial.go` - Spatial audio (volume, panning by position)
- `internal/audio/mix.go` - Sample mixing

**What's Implemented:**
- Audio system init/shutdown with configurable sample rate
- SFX cache with optional 8-bit reduction
- Backend abstraction (SDL3 or Oto)
- WAV file decoding
- Spatial sound calculations (volume attenuation by distance)
- Channel allocation
- Raw audio sample buffer for engine mixing
- Ambient sound support (background loop channels)

**Critical Gaps:**

1. **Sound Events Never Dispatched** - `client/parse.go:180-181` has TODO
   - C code: `CL_ParseStartSoundPacket()` reads svc_sound, queues to audio system
   - Parses: entity/channel, sound index, volume, attenuation, position, pitch
   - Go state: Parser has `parseStartSound()` but never calls audio system
   - Behavior: Weapons, explosions, footsteps, ambient loops completely silent

2. **No Stop Sound Integration** - `client/parse.go:180` comment
   - C code: `CL_ParseStopSound()` → `S_StopSound(entity, channel)`
   - Cancels ongoing sound before new one starts (e.g., reload cuts firing)
   - Go state: Parser recognizes message but audio system never receives command
   - Behavior: Sounds don't cut off; overlapping sounds create noise

3. **No Music/BGM System**
   - C code: `bgmusic.c` with support for: OGG Vorbis, FLAC, Opus, ModPlug (XM/IT/S3M), libxmp
   - Separate from SFX with own volume cvar (`bgmvolume`)
   - Tracks: `music/track##.ogg` for levels + `music/theme.ogg` for menu
   - Goes through `S_Music()` / `CL_StartMusic()` / `CL_StopMusic()`
   - Go gap: No music codec support, no playback system
   - Behavior: Game completely silent except HUD/menu events

4. **CD Audio Not Implemented** - `cdaudio.h`, `cd_null.c`
   - C code: Legacy CD audio for Quake soundtrack (tracks 2-11)
   - svc_cdtrack message controls playback
   - Go gap: Type defined but never connected to backend
   - Behavior: CD tracks don't play (less critical; music takes priority)

5. **Sound Attenuation Incomplete**
   - C code: Calculates 3D position, listener origin/angles, applies attenuation formula
   - Distance-based volume, stereo panning based on relative angle
   - Go state: `spatial.go` has basic distance calculation but never applied to channel output
   - Missing: Pan calculation from listener angle + sound position angle
   - Behavior: All sounds are mono at origin, no directional cues

6. **No Per-Channel Pitch Control**
   - C code: Some sounds play at entity velocity-based pitch (doppler effect simulation)
   - `snd_dma.c` applies pitch offset before mixing
   - Go gap: No pitch parameter in sound playback
   - Behavior: All sounds at fixed pitch; no pitch variety effects

7. **Ambient Sounds Not Positioned**
   - C code: `S_AmbientSound()` places ambient loops at world locations with attenuation
   - E.g., water drip, lava bubble, wind
   - Go gap: No integration with world position; ambient sounds ignore entity location
   - Behavior: Ambient loop plays everywhere at same volume regardless of player location

8. **No Sound Effect Resampling**
   - C code: Source samples at various rates (11kHz, 22kHz, 44kHz) → converted to output rate
   - Can load at 8-bit mono or 16-bit stereo with automatic conversion
   - Go gap: Backend only supports one output rate; input rate mismatch causes silent/distorted audio
   - Behavior: Sounds with wrong sample rate don't play or sound corrupted

9. **Water/Filtering Effects Missing** - `snd_waterfx` cvar
   - C code: When player in water (contents), applies low-pass filter to SFX
   - Reduces treble, adds muffling effect
   - Go gap: No filter infrastructure
   - Behavior: Underwater audio doesn't sound underwater

10. **No Sound List/Precache Display**
    - C code: `S_SoundList()` command shows memory usage
    - Go gap: Can't audit loaded sounds
    - Behavior: Debug info missing

---

## E. NETWORKING/MULTIPLAYER HOST-COMMAND GAPS

### Current Go State

**Key Files:**
- `internal/net/net.go` - Network interface (Connect, SendMessage, GetMessage)
- `internal/net/loopback.go` - Loopback for local play
- `internal/net/udp.go` - UDP transport
- `internal/net/socket.go` - Socket abstraction
- `internal/host/commands.go` - Host commands (map, skill, load, save, kick, etc.)
- `internal/server/` - Server physics and message handling

**What's Implemented:**
- Socket abstraction (loopback + UDP)
- Datagram protocol with sequence/acknowledgment
- Message sending/receiving
- Local server hosting (loopback)
- Basic host commands (map, skill, pause, status, etc.)
- Save/load game (JSON-based state serialization)
- Player connection tracking
- God/noclip/fly cheats

**Critical Gaps:**

1. **Connect/Reconnect Commands Unimplemented** - `host/commands.go:69-72, 89-94`
   - C code: `Host_Connect_f()` / `Host_Reconnect_f()` establish TCP/UDP to remote server
   - Handshake sequence: version check → model/sound precache → entity baseline → begin signon
   - Go code: Function signatures exist but body is TODO
   - Behavior: Cannot join remote multiplayer games at all

2. **Kick Command Incomplete** - `host/commands.go:89-95`
   - C code: `Host_Kick_f()` boots player by number
   - Sends disconnect message, removes from server
   - Go state: Signature exists, body is TODO
   - Behavior: Server admin cannot remove disruptive players

3. **No Deathmatch Game Type**
   - C code: Game mode selection (single-player, coop, deathmatch)
   - Affects scoring, respawning, player names on scoreboard
   - Go gap: Single-player only; no DM spawning logic
   - Behavior: Cannot play competitive multiplayer

4. **No Server Config/Settings UI** - Submenu in gap C above
   - C code: M_Menu_GameOptions_f() → sets deathmatch, skill, timelimit, fraglimit, maxplayers, etc.
   - Go gap: No submenu for server launch parameters
   - Behavior: Server launched with hardcoded defaults; no customization

5. **No Network Stats Display**
   - C code: `status` command shows ping, packets sent/received, packet loss
   - NET layer tracks: packetsSent, packetsReSent, packetsReceived, receivedDuplicateCount
   - Go gap: `status` command shows basic info but not network stats
   - Behavior: Can't diagnose lag or connection issues

6. **No Ban System** - `net_dgrm.c` has BAN_TEST code
   - C code: IP address filtering, ban/unban commands
   - Go gap: No ban list or filtering logic
   - Behavior: Griefers cannot be permanently blocked

7. **No NAT Traversal/Port Forwarding Help**
   - Expected in modern ports: UPnP or alternative for LAN discovery
   - Go gap: Manual port forwarding only
   - Behavior: Behind-NAT players can't host visible servers

8. **CDTrack Server Command Stubbed** - `host/commands.go:67`
   - C code: `Host_CDTrack_f()` queues CD track change to all clients
   - Go: Comment says "TODO: Get actual CD track from client if available"
   - Behavior: Music track changes don't sync to clients

9. **No Message Filtering Debug Mode**
   - C code: `cl_shownet` cvar shows network message parsing
   - Go gap: No debug output for message flow
   - Behavior: Network bugs hard to diagnose

10. **No Server Pause Notification**
    - C code: Sends `svc_setpause` to all clients, shows pause UI
    - Go gap: Server can pause but clients don't know
    - Behavior: Other players see player standing still without pause indicator

11. **Signon Handshake Incomplete**
    - C code: `Host_ClientBegin()` → version, serverinfo, model/sound precache, entity baselines, lightstyles, cshifts, signonnum
    - Multi-stage signon with client responses (prespawn, spawn, begin)
    - Go state: Basic structure exists but not all messages sent
    - Behavior: Client may not receive full initial state; map may not load correctly

---

## F. CLIENT PREDICTION/DEMO/RUNTIME INTEGRATION GAPS

### Current Go State

**Key Files:**
- `internal/client/prediction.go` - Movement prediction
- `internal/client/demo.go` - Demo recording/playback
- `internal/client/client.go` - Client state machine
- `internal/client/parse.go` - Server message parsing
- `internal/client/tent.go` - Temporary entities
- `internal/client/input.go` - Input processing

**What's Implemented:**
- Basic movement prediction (forward/side/up velocity, acceleration, friction)
- Prediction error correction with lerp
- Demo structure definitions (frames, sound events, rewinding)
- Client state machine (Disconnected, Connected, Active)
- Entity state parsing from delta updates
- Light style and color updates
- Temporary entity system
- User command building

**Critical Gaps:**

1. **Prediction Missing Core Physics** - `client/prediction.go` is incomplete
   - C code: `CL_PredictMove()` calls `PM_PlayerMove()` from server DLL
   - Full physics: ground detection, step handling, slope walking, air control, collision
   - Go state: Simplified model with constant acceleration/friction, no collision
   - Behavior: Prediction diverges from server; player appears to stutter/snap as corrections apply

2. **No View Weapon Prediction** - `client/prediction.go:96`
   - C code: Separate prediction for view entity (gun model)
   - Uses different height/offset calculation
   - Go gap: Gun prediction completely missing
   - Behavior: Gun model jitters relative to view

3. **Prediction Doesn't Loop Commands** - `client/prediction.go:89`
   - C code: Loops through accumulated commands (e.g., 10 frames of queued inputs), applies each
   - Allows smooth multi-frame prediction even with packet loss
   - Go state: `TODO: loops through c.CommandBuffer`; only applies current frame
   - Behavior: Prediction lags; noticeable delay between input and response

4. **Demo Rewinding Unimplemented**
   - C code: `demo_rewind` structure tracks frame offsets, light styles, csifts
   - `CL_SeekDemoFrame()` / `CL_TimeDemo_f()` for fast-forward/rewind
   - Go state: Frame list defined but seek/rewind logic not written
   - Behavior: Can't scrub through demos; linear playback only

5. **Temporary Entity Spawning Incomplete** - `tent.go` is 60 lines, very basic
   - C code: `CL_ParseTempEntity()` spawns 20+ entity types
   - Types: TE_SPIKE, TE_SUPERSPIKE, TE_GUNSHOT, TE_EXPLOSION, TE_TAREXPLOSION, TE_LIGHTNING1, TE_LIGHTNING2, TE_LIGHTNING3, TE_LAVASPLASH, TE_TELEPORT, TE_BLOOD, TE_LIGHTNINGBLOOD, TE_BEAM, TE_IMPLOSION, TE_RAILTRAIL, TE_EXPLOSION3, TE_EXPLOSION2, TE_BEAM, TE_SMALLFLASH, TE_FLAMETHROWER, TE_TELEPORTOTHER, TE_TELEPORTSWAP, TE_LASERBEAM
   - Go gap: Only defines skeleton
   - Behavior: Special effects (explosions, beam attacks, gibs) missing

6. **Water Shift Effect Missing**
   - C code: When player in water, applies view angle pitch shift (`v_waterpitch` cvar)
   - Darkens screen slightly (`cshift_water`)
   - Go gap: No screen effect for water
   - Behavior: Underwater looks same as normal

7. **Damage Flash Effect Missing**
   - C code: `V_ParseDamage()` applies screen red shift and damage direction indicator
   - Intensity based on damage taken, fades over time
   - Go gap: Damage state parsed but not rendered
   - Behavior: Player doesn't see visual damage feedback

8. **Quad Damage Flash Missing**
   - C code: When player has quad damage, applies blue screen flash
   - Pulses with powerup timer
   - Go gap: Not implemented in HUD/view
   - Behavior: Quad powerup invisible to player

9. **Intermission Not Triggered**
   - C code: `svc_intermission` sets `cl.intermission` flag, changes to special view
   - Shows frag counter, waits for player input to advance
   - Go state: Flag parsed but state machine never transitions to intermission mode
   - Behavior: Level ends without visible result; no map/frag display

10. **Demo Recording / Forward Playback Wired**
    - C code: `record`, `stop`, and `CL_FinishDemoFrame()` manage the header, signon snapshot, frame stream, stuffed-command timing, and disconnect trailer
    - Go state: `record` / `playdemo` / `stop` are implemented; live gameplay writes demo frames, connected-state record emits the initial snapshot, and playback applies recorded view angles plus server-time pacing
    - Remaining gap: advanced rewind / timedemo tooling is still absent, but the forward record/playback path now works end to end

11. **View Angles Not Updated from Mouse/Gamepad** - Gap B.2 key binding issue
    - C code: `CL_AdjustAngles()` applies mouse delta + key input to view angles
    - Go state: Input received but not accumulated into view angles
    - Behavior: Cannot look around; view stuck facing forward

---

## G. SAVE/LOAD GAPS (if any remain)

### Current Go State

**Key Files:**
- `internal/host/commands.go` - Save/Load commands
- `internal/server/` - Server state save

**What's Implemented:**
- CmdLoad / CmdSave (160 lines in commands.go)
- JSON serialization of server state
- Save file validation and naming
- Player state persistence
- Entity state preservation (partial)

**Critical Gaps:**

1. **QuakeC Interpreter State Not Saved** - `internal/qc/` exists but not integrated
   - C code: `Host_SaveGameToFile()` saves server time, entity count, entities array, strings heap, globals, field values
   - QuakeC program state (function stack, globals, entity fields) must be exact match
   - Go gap: QC VM execution state not connected to save system
   - Behavior: QC state (monster positions, triggers, counters) lost on reload; triggers can fire twice

2. **Save Restriction Parity Updated**
   - C code: `Host_Savegame_f()` rejects saves when `nomonsters` is enabled, during intermission, or when players are dead
   - Go state: save now rejects those same `nomonsters`/intermission/dead-player cases and persists lightstyles
   - Remaining divergence: load/save UX still differs (loading plaque and broader save-file search behavior)

3. **Player Inventory Not Preserved**
   - C code: Saves `items` bitmask, ammo counts, weapon selection
   - Go state: Parsed from entity but may not round-trip correctly
   - Behavior: Player might lose weapons/ammo on reload

---

## PRIORITY ORDER FOR CLOSING REMAINING GAPS

### TIER 1 (Playability Critical - Blocks core gameplay)
1. **Entity Rendering Pipeline** (B.1) - Players/monsters/items invisible
2. **Connect/Reconnect/Kick Parity** (E.1) - Remote multiplayer flow still incomplete
3. **Save/Load UX Parity** (G.2) - Loading plaque and broader save-file search behavior still differ
4. **Alias Model Rendering** (B.2) - Player and monsters invisible
5. **Lightmaps Processing** (A.1) - World entirely unlit/dark

### TIER 2 (Multiplayer/Advanced - Needed for complete feature parity)
6. **Connect/Reconnect Commands** (E.1) - Cannot join online
7. **Particle Effects Integration** (B.4) - Visual feedback missing
8. **Brush Submodel Rendering** (B.6) - Doors/platforms invisible
9. **Movement Prediction Physics** (F.1) - Input lag/stutter
10. **Menu System Completion** (C.1) - UI completely broken

### TIER 3 (Polish - Enhances experience but not required)
11. **Music System** (D.3) - No background music
12. **Skybox Runtime Integration** (B.3) - Skybox state still not fully consumed by the renderer
13. **Water Warp Effects** (A.2) - Visual enhancement
14. **Weapon Model Rendering** (B.8) - First-person weapon invisible
15. **Advanced Demo Tooling** (F.10) - Timedemo/rewind style tooling is still missing

---

## STALE/MISLEADING PLANNING ASSUMPTIONS IN DOCS

### README.md Issues

The README has since been corrected to reflect the current renderer/build reality:

- OpenGL/CGO is the default gameplay renderer and parity target.
- gogpu/WebGPU remains a secondary backend while its runtime bugs are addressed.
- CGo is required by the canonical OpenGL path even though most gameplay logic remains pure Go.

### Repository Structure Assumptions

**Assumption:** Package boundaries reflect functional domains (client, server, renderer, audio)

**Reality:** Package organization is correct but integration gaps mean most are isolated stubs. No integration tests verify data flow between packages.

**Assumption:** Each package is independently testable

**Reality:** Unit tests exist but lack integration tests showing end-to-end: server → client → renderer → window

---

## SUMMARY TABLE: Parity Status by Area

| Area | Completion | Blockers | Priority |
|------|-----------|----------|----------|
| World Rendering | 40% | Lightmaps, sky, water warp | TIER 1 |
| Entity Rendering | 5% | Rendering pipeline, alias models, integration | TIER 1 |
| Particles | 30% | Integration, rendering pass | TIER 2 |
| Sprites | 20% | Geometry gen, rotation, batching | TIER 2 |
| Menus | 20% | All submenus are stubs | TIER 2 |
| HUD | 50% | Weapon icons, face, score overlay | TIER 2 |
| Audio | 50% | Event dispatch, music system | TIER 1 |
| Networking | 60% | Connect, server config UI, DM mode | TIER 2 |
| Prediction | 30% | Full physics, looped commands | TIER 2 |
| Demos | 20% | Rewinding, recording | TIER 3 |
| Save/Load | 80% | QuakeC state, inventory | TIER 2 |

---

## KEY FILE REFERENCES FOR DEEP DIVES

### Critical C Functions to Reference
- **Rendering:** `gl_rmain.c:R_RenderScene()`, `r_alias.c:R_DrawAliasModel()`, `r_brush.c:R_TextureAnimation()`
- **Audio:** `snd_dma.c:S_StartSound()`, `cl_parse.c:CL_ParseStartSoundPacket()`
- **Input:** `cl_input.c:CL_AdjustAngles()`, `keys.c:Key_Event()`
- **Menus:** `menu.c:M_Draw()`, `menu.c:M_Keydown()`
- **Network:** `net_main.c:NET_SendMessage()`, `host_cmd.c:Host_Map_f()`

### Critical Go Functions to Wire
- **Renderer:** `renderer_opengl.go:Frame()` → add entity rendering call
- **Client:** `client.go:Update()` → integrate prediction loop
- **Audio:** `parse.go:parseStartSound()` → dispatch to audio system
- **Menu:** `manager.go:Draw()` → render actual menu graphics
- **Host:** `commands.go:CmdConnect()` → implement network connection
