# Ironwail Go port parity review

This document is the canonical, source-backed review of the current `ironwail-go` port against the original C Ironwail/Quake engine.

- Repositories compared: `ironwail`, `ironwail-go`, and `ironwail-go-docs`
- Rule of thumb: when documentation and source disagree, source wins
- Practical parity target: the CGO/OpenGL runtime (`renderer_opengl.go` + GLFW input path + SDL3/Oto audio backend) is the authoritative baseline; the gogpu/WebGPU path is still secondary

## Executive summary

The Go port is materially farther along than several older planning notes imply. It already boots real assets into an active local single-player session, parses a wide slice of the server protocol, uploads BSP world geometry with real lightmaps on the OpenGL path, renders brush and alias entities there, provides a working menu/HUD foundation, and round-trips savegames against real assets.

The biggest remaining parity problems are mostly **integration and fidelity gaps**, not total subsystem absence:

- the OpenGL renderer already has world, brush, alias, sprite, particle, decal, viewmodel, dynamic-light, animated-texture, fog, turbulent UV warp, embedded two-layer Quake sky integration, and common external cubemap skybox consumption in the live runtime; the main remaining render gaps are exact pass ordering and non-cubemap skybox edge cases
- the gogpu path is still visibly behind the OpenGL path and should not be the parity gate
- the input/command layer now uses Quake-style bindings, config persistence, command aliases (`alias`/`unalias`/`unaliasall`), and live prediction; the bigger remaining client-state gaps are special intermission/cutscene handling and remote networking flow
- the audio/music path now dispatches parsed sounds into the live mixer, maintains static sounds, updates the listener, and plays WAV-backed CD tracks; broader fidelity/format parity still remains
- save/load is much more complete than older notes suggested, and now includes C's `nomonsters` / intermission / dead-player save restrictions plus session-transition sound teardown; loading-plaque rendering/search behavior still differs
- demo recording and forward playback are now real runtime features, including connected-state snapshots, disconnect trailers, same-frame stuffed-command execution, pause semantics, and server-time pacing

A useful way to think about the current tree is:

> many C algorithms have already been ported to Go, but several of them are not yet wired into the live runtime in the same way the C engine wires them together.

## High-level status

| Area | What is already implemented | Main missing / divergent behavior |
| --- | --- | --- |
| Boot, FS, QC, local runtime | real asset boot, filesystem semantics, QC VM load, local loopback single-player startup, and local `connect`/`disconnect` session transitions | remote connection flow is still stubbed |
| OpenGL renderer | world upload, lightmaps, lightstyle updates, brush entities, alias entities, sprites, particles, decals, viewmodel, dynamic lights, brush rotation, animated textures, turbulent UV warp, live fog, dedicated embedded sky layer animation path, and common external cubemap skybox consumption | exact render-pass ordering and non-cubemap skybox edge cases still differ from C |
| gogpu renderer | world draw path, 2D overlay, particle fallback | entity rendering is still a stub and parity should not be judged here |
| Client/input runtime | broad SVC parsing, Quake-style `KButton` handling, movement command assembly, live prediction, bind-driven command routing, config persistence, loopback send path, demo record/playback integration | special intermission/finale/cutscene handling and remote connection flow still diverge |
| Audio/music | real mixer/backend/spatialization code, sound event parsing and dispatch, static sound lifecycle, listener updates, WAV CD-track playback | broader codec/fidelity parity still remains |
| Menus/HUD/console/config | main menu flow, load/save/help/options/quit menus, basic HUD, in-game console UI, history/completion, bind persistence, and Quake-style alias commands | multiplayer/options submenus still TODO and the HUD is still much simpler than `sbar.c` |
| Save/load | host commands, QC/global/edict/static state capture+restore, real-assets save/load test, lightstyles, C-style `nomonsters`/intermission/dead-player restrictions, and stop-all sound teardown on local load/map/reconnect-style transitions | broader C loading UX/search behavior is still missing |
| Networking/multiplayer | loopback server/client and protocol work are present, `connect local` now drives the existing local reconnect/signon flow, `disconnect` now cleanly tears down local session state (including stop-all sounds), `reconnect` re-runs local signon flow, and local-host `kick` supports C-style target/reason handling | remote `connect` flow is still missing, and remote networking flow remains incomplete |

## 1. Runtime baseline and core engine state

### Implemented from the C engine

The local single-player boot path is already real, not mocked.

Relevant Go files:

- `cmd/ironwailgo/main.go`
- `internal/host/init.go`
- `internal/host/commands.go`
- `internal/fs/fs.go`
- `internal/server/sv_main.go`
- `internal/host/commands_realassets_test.go`

What already works:

- Quake-style filesystem mounting and pack search precedence are implemented in `internal/fs/fs.go`
- `progs.dat` is loaded into the Go QC VM during startup
- the host/server/client loopback path can boot a real map and reach an active local client state
- `TestCmdMapStartRealAssetsReachesCaActive` proves a real-assets local session can complete the signon sequence

### Missing or divergent

The runtime is still biased toward **local loopback play**.

- `Host.CmdConnect()` now handles local-loopback parity slices (`demonum=-1`, demo-playback stop/reset, `connect local` handoff) and emits explicit unsupported messaging for remote targets
- `Host.CmdKick()` now supports local name/slot targeting, optional reasons, and self-kick protection
- parity should currently be judged on the local/OpenGL path, not on remote multiplayer or the gogpu path

### Exact C behavior still missing

The original C engine does more here than the Go port currently exposes:

- `host_cmd.c:Host_Connect_f()` stops demo loop/playback if needed, calls `CL_EstablishConnection(name)`, then immediately calls `Host_Reconnect_f()`; Go now mirrors this sequence for local loopback targets and explicit disconnect/reset behavior
- `host_cmd.c:Host_Reconnect_f()` begins the loading plaque and calls `CL_ClearSignons()` so the client re-runs the full signon process; Go now mirrors the signon reset/restart and stop-all sound teardown on the local loopback path but still lacks the loading plaque and wider remote path
- `host_cmd.c:Host_Kick_f()` supports kicking either by player name or by `# <slot>`, accepts an optional message, and refuses to kick the caller

## 2. Rendering parity

### 2.1 OpenGL path: what is already implemented

The OpenGL path is significantly more complete than some older status notes claimed.

Relevant Go files:

- `internal/renderer/stubs_opengl.go`
- `internal/renderer/world_opengl.go`
- `internal/renderer/world_runtime_opengl.go`
- `internal/renderer/decal_opengl.go`
- `internal/renderer/particle_runtime_opengl.go`
- `internal/renderer/entity_types.go`

What is already present:

- BSP world geometry extraction in `BuildWorldGeometry()` / `BuildModelGeometry()`
- real texture classification for sky, water, lava, slime, teleporter, cutout, and default surfaces
- lightmap page allocation and upload
- per-frame lightstyle evaluation flowing into `setLightStyleValues()` and `updateUploadedLightmapsLocked()`
- world draw bucketing for sky / opaque / alpha-test / translucent faces
- brush submodel rendering via `renderBrushEntities()`
- alias model rendering via `renderAliasEntities()` and `renderViewModel()`
- sprite rendering code via `renderSpriteEntities()`
- particle rendering via `renderParticles()`
- projected decal rendering via `renderDecalMarks()`

In other words, the OpenGL `RenderFrame()` path already dispatches far more than just world geometry.

### 2.2 OpenGL path: what is missing or divergent

The gaps are mostly about **runtime collection, exact behavior, and fidelity**.

#### Runtime-fed render state is much more complete now

- `main.go` now populates `SpriteEntities`, maintains a `DecalMarkSystem`, and passes active decal marks into `RenderFrameState`
- the live runtime spawns temp-entity and effect-driven dynamic lights into the renderer's light pool
- brush entities now honor rotation, and the runtime feeds protocol alpha/scale/effect lighting through the active entity paths
- animated world textures are evaluated against live client time, and client fog state is consumed by the shared runtime renderer
- turbulent (`SurfDrawTurb`) world/brush surfaces now apply C-style time-varying UV warp on the canonical OpenGL path

#### Remaining divergences from C

- embedded BSP sky surfaces now render through a dedicated animated sky-layer shader/path (instead of the ordinary world shader), and the common external cubemap skybox path is consumed (including partial square face sets with zero-filled missing faces) with fallback to embedded sky for unsupported face sets
- particle rendering now has explicit opaque/translucent subpass plumbing in the top-level OpenGL frame path, but the broader scene ordering is still simpler than `R_RenderScene()` and world/brush water/viewmodel sequencing still diverges

### 2.3 gogpu path: current status

Relevant Go file:

- `internal/renderer/renderer_gogpu.go`

The gogpu path still contains major parity gaps:

- `DrawContext.RenderFrame()` explicitly labels entity rendering as a stub
- `renderEntities()` is still TODO
- particle rendering is a simplified 2D fallback, not a full world-space parity implementation
- the path includes backend-specific state hacks to preserve a HAL world render beneath the overlay; this is not a parity-complete gameplay renderer yet

This makes the gogpu path a secondary backend, not the right place to measure parity.

### Exact C behavior still missing or not fully matched

#### `gl_rmain.c:R_RenderScene()`

The C engine's scene ordering is:

1. `R_SetupScene()`
2. `R_Clear()`
3. `Fog_EnableGFog()`
4. `S_ExtraUpdate()`
5. draw non-alpha entities
6. draw opaque particles
7. `Sky_DrawSky()`
8. draw opaque water
9. begin translucency
10. draw translucent water
11. draw alpha entities
12. draw alpha particles
13. end translucency
14. `R_DrawViewModel()`

The Go OpenGL path currently clears, then draws world, brush entities, alias entities, sprite entities, decals, particles, and viewmodel in a simpler sequence. It has the pieces, but not yet the same top-level pass structure.

#### `r_brush.c:R_TextureAnimation()`

The C behavior is precise:

- if `frame != 0` and `alternate_anims` exists, use the alternate chain
- compute `relative = int(cl.time * 10) % base->anim_total`
- walk `anim_next` until `anim_min <= relative < anim_max`
- error out on broken or infinite cycles

Go already ports and consumes this logic in the live OpenGL renderer (`internal/renderer/surface.go:TextureAnimation()` + `internal/renderer/world_runtime_opengl.go`), including broken/infinite cycle detection.

## 3. Client runtime, input, and demos

### 3.1 What is already implemented

Relevant Go files:

- `internal/client/client.go`
- `internal/client/parse.go`
- `internal/client/input.go`
- `internal/client/prediction.go`
- `internal/client/demo.go`
- `cmd/ironwailgo/main.go`

What already works:

- broad server-message parsing, including entity updates, lightstyles, temp entities, static sounds, fog, and skybox name
- Quake-style `KButton` handling via `KeyDown`, `KeyUp`, and `KeyState`
- command assembly through `AdjustAngles()`, `BaseMove()`, and `AccumulateCmd()`
- loopback command submission from client to server
- demo file open/read/write primitives in `internal/client/demo.go`
- live demo playback integration in `main.go` by reading frames and parsing them back through the client parser

### 3.2 What is missing or divergent

#### Core prediction, bindings, and demos are now wired into the live runtime

- `main.go` now calls `PredictPlayers()` and uses the updated predicted state for camera/viewmodel work
- gameplay input routes through live `bind` / `unbind` / `unbindall` / `bindlist` handling, and `config.cfg` persists those bindings
- demo recording writes live gameplay frames, connected-state snapshots, and a disconnect trailer; playback applies recorded view angles, flushes `stufftext` in the same frame, honors pause state, and paces reads against recorded server time

#### Remaining client/runtime divergences

- special intermission / finale / cutscene handling is parsed into client state but not yet turned into full C-style runtime/UI flow
- remote `connect` flow is still incomplete (Go now prints an explicit unsupported message rather than pretending success), and `reconnect` still lacks the C loading-plaque UX outside the local loopback path even though local transition audio teardown is now in place

### Exact C behavior still missing or not fully matched

#### `cl_input.c:CL_BaseMove()` / `CL_SendMove()`

The C engine defines an exact mapping from button state to `usercmd_t` and then to the network message:

- movement comes from `CL_KeyState()` over forward/back, strafe, moveleft/moveright, up/down
- the speed key multiplies forward/side/up movement
- `CL_SendMove()` writes `clc_move`, the last server time, three view angles, forward/side/up shorts, button bits, and the impulse byte
- attack and jump use bit 0 and bit 1 respectively, and both clear their edge-down bit after the move packet is sent

Go mirrors this logic in `internal/client/input.go` and now drives it from configurable runtime bindings in the live loopback path.

#### `cl_demo.c:CL_Record_f()` / `CL_FinishDemoFrame()`

The forward recording path now matches the C runtime much more closely:

- recording may begin after connection and emits the initial signon/state snapshot the demo needs
- live gameplay appends raw server-message demo frames as play continues
- `stop` writes the final disconnect trailer
- playback consumes one frame per host frame, flushes stuffed commands in-frame, and waits on recorded server time instead of host FPS alone

Advanced tooling such as rewind/timedemo remains outside the currently matched forward-playback path.

## 4. Audio and music parity

### What is already implemented

Relevant Go files:

- `internal/audio/adapter.go`
- `internal/audio/backend.go`
- `internal/audio/backend_oto.go`
- `internal/audio/backend_sdl3.go`
- `internal/audio/sound.go`
- `internal/audio/mix.go`
- `internal/audio/spatial.go`
- `internal/client/parse.go`

What already exists:

- a real audio system with channel selection, precache, start/stop sound, mixing, and listener spatialization
- backend selection via SDL3 first, then Oto, then NullBackend fallback
- parsed client-side `SoundEvent` and `StaticSound` data structures
- server-side sound message emission in `Server.StartSound()`

### What is missing or divergent

- `main.go`'s `UpdateAudio()` callback is a stub, so the listener origin/orientation are never updated each frame
- `Client.SoundEvents` are accumulated by the parser but never consumed into `audio.System.StartSound()`
- `Client.StaticSounds` are parsed but never instantiated as persistent ambient/static channels
- `parseStopSound()` still contains a TODO to dispatch the stop request into the audio system
- there is no background-music / CD-track playback path analogous to `bgmusic.c`
- server sound sending currently only uses the compact packet form; the client parser supports larger sound/entity encodings, but the built-in server does not emit them

### Exact C behavior still missing or not fully matched

#### `cl_parse.c:CL_ParseStartSoundPacket()`

The C parser:

- reads the field mask
- optionally reads explicit volume and attenuation
- reads either packed entity/channel or the large-entity form
- reads either 8-bit or 16-bit sound index depending on flags
- reads three coordinates
- calls `S_StartSound()` immediately with the decoded values

Go already decodes the packet into `SoundEvent`, but it stops short of the final runtime dispatch.

#### Music / CD behavior

The original engine keeps a separate music path (`bgmusic.c`) distinct from one-shot SFX. The Go port currently parses `CDTrack` / `LoopTrack` values into client state, but there is no corresponding playback system.

## 5. Menus, HUD, console, bindings, and config persistence

### What is already implemented

Relevant Go files:

- `internal/menu/manager.go`
- `internal/hud/hud.go`
- `internal/hud/status.go`
- `internal/console/console.go`
- `internal/input/types.go`
- `internal/host/init.go`

What is already present:

- menu state machine for main, single-player, load, save, multiplayer, options, help, and quit screens
- real menu navigation and command queuing for new game / load / save / help / quit flows
- a basic Quake-style HUD with status bar and centerprint support
- a real console text buffer with scrollback, resize behavior, notify lines, and debug logging
- input destination routing (`KeyGame`, `KeyMenu`, `KeyConsole`, `KeyMessage`)
- key name conversion and binding storage in `input.System`
- user-facing `alias`, `unalias`, and `unaliasall` commands backed by cmdsys aliases, with command-over-alias execution precedence
- console tab completion now includes alias names alongside commands and cvars

### What is missing or divergent

- multiplayer submenu selections still only emit TODO `echo` commands (`join game`, `host game`, `player setup`)
- options submenu still only has one real action (`toggle vid_vsync`); controls/video/audio are placeholders
- the HUD is much simpler than `sbar.c`; inventory, face states, sigils, keys, weapon strip, ammo icons, and other status-bar details are not yet matched
- the menu/console layer still lacks a number of C-polish details even though the baseline in-game console UI/render/input path is now wired
- the HUD and option/menu surfaces still expose much less functionality than the C engine
- `Host.WriteConfig()` now writes binds plus archived cvars, but it still does not append the extra state-preserving commands the C engine can emit

### Exact C behavior still missing or not fully matched

#### `host.c:Host_WriteConfigurationToFile()`

The C engine writes more than archived cvars:

- `Key_WriteBindings(f)` writes all key bindings
- `Cvar_WriteVariables(f)` writes archived cvars
- it then appends extra state-preserving commands such as `vid_restart` and `+mlook` when needed

Go currently writes only `cvar.ArchiveVars()`.

#### `keys.c` binding behavior

The C key system is not just a map from key to string; it is part of the live command-routing model. `Key_Event()` handles autorepeat filtering, key-destination routing, bind execution, and user-facing unbound-key behavior. Go has the storage pieces, but not the full runtime UX.

## 6. Save/load parity

### What is already implemented

Relevant Go files:

- `internal/host/commands.go`
- `internal/server/savegame.go`
- `internal/host/commands_realassets_test.go`

What already works:

- `CmdSave()` and `CmdLoad()` are real host commands, not placeholders
- save files include map name, time, paused state, server flags, model/sound precaches, static entities, static sounds, client spawn parms, edicts, and QC globals
- restore recreates edicts, relinks the world, syncs the QC VM, and restores saved globals
- lightstyles are saved/restored and resent through the restored loopback signon flow
- the save command now rejects `nomonsters`, intermission, and dead-player states to match the C engine's restrictions
- `TestCmdSaveLoadRealAssetsRoundTrip` proves a real-assets session can save, reload, and recover player state

This is one of the biggest places where older status docs understated current progress.

### What is missing or divergent

- load/save path behavior is simplified to `userDir/saves/<name>.sav`
- there is still no equivalent of the C engine's loading-plaque rendering / broader save-file search behavior (despite matching stop-all transition-audio behavior more closely)

### Exact C behavior still missing or not fully matched

#### `host_cmd.c:Host_Savegame_f()`

The C save command refuses to save when:

- there is no active local game
- `nomonsters` is enabled
- the client is in intermission
- the game is multiplayer
- any active player is dead

Go now enforces the active built-in server, single-player, `nomonsters`, intermission, and dead-player cases.

#### `host_cmd.c:Host_Loadgame_f()`

The C load path does more than simple deserialization:

- it shows the loading plaque
- can search outside the active game dir in some cases
- restores spawn parms, skill, map, time, lightstyles, globals, and edicts
- re-enters the connection/signon flow after restoration

Go already restores most of the world/QC state, including lightstyles, transition sound teardown, and the post-load signon re-entry, but it still lacks the loading plaque and broader search/UX behavior.

## 7. Networking and multiplayer parity

### What is already implemented

- local loopback client/server plumbing is real and used by the single-player runtime
- the server emits core signon data, static entities, static sounds, and entity updates
- status/ping style host commands exist

### What is missing or divergent

- `connect` is not feature-complete, and `reconnect` is still only wired through the local loopback path
- multiplayer menu flows are still placeholder UX
- the current runtime should be treated as single-player-first even though pieces of the multiplayer protocol are already present

### Exact C behavior still missing or not fully matched

- `Host_Connect_f()` establishes the remote connection and then forces a reconnect-style signon restart
- `Host_Reconnect_f()` begins a loading plaque and clears signons; Go now matches the signon reset/restart and local transition sound teardown but not the loading plaque
- `Host_Kick_f()` supports name or slot-number targeting plus an optional kick message (now mirrored on the local host path)

## 8. Overall parity judgement

The current Go port is best described as:

- **strong in filesystem, QC loading, local boot, and several data-model ports**
- **surprisingly strong in OpenGL world/entity foundations and save/load serialization**
- **partially complete in menus/HUD/client parsing/demo playback**
- **not yet parity-complete in render fidelity, HUD/menu depth, remote networking, and secondary backend support**

The most important practical conclusion is this:

> the shortest path to real parity is to keep OpenGL/GLFW/Oto authoritative, then close the remaining integration gaps there before spending more time trying to make the gogpu path the primary runtime.

## Appendix: key missing behaviors to preserve exactly

These are the highest-value C behaviors still worth treating as exact parity targets.

1. **Render ordering** — `gl_rmain.c:R_RenderScene()`
2. **Texture animation selection** — `r_brush.c:R_TextureAnimation()`
3. **Sound packet decoding and immediate dispatch** — `cl_parse.c:CL_ParseStartSoundPacket()`
4. **Movement packet layout and button-bit semantics** — `cl_input.c:CL_BaseMove()` / `CL_SendMove()`
5. **Connected-state demo recording bootstrap** — `cl_demo.c:CL_Record_f()`
6. **Connect/reconnect flow** — `host_cmd.c:Host_Connect_f()` / `Host_Reconnect_f()`
7. **Save/load restrictions and restored state** — `host_cmd.c:Host_Savegame_f()` / `Host_Loadgame_f()`
8. **Binding/config persistence** — `keys.c:Key_WriteBindings()` and `host.c:Host_WriteConfigurationToFile()`
