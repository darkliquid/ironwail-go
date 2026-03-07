# Ironwail Go port parity review

This document is the canonical, source-backed review of the current `ironwail-go` port against the original C Ironwail/Quake engine.

- Repositories compared: `ironwail`, `ironwail-go`, and `ironwail-go-docs`
- Rule of thumb: when documentation and source disagree, source wins
- Practical parity target: the CGO/OpenGL runtime (`renderer_opengl.go` + GLFW input path + SDL3/Oto audio backend) is the authoritative baseline; the gogpu/WebGPU path is still secondary

## Executive summary

The Go port is materially farther along than several older planning notes imply. It already boots real assets into an active local single-player session, parses a wide slice of the server protocol, uploads BSP world geometry with real lightmaps on the OpenGL path, renders brush and alias entities there, provides a working menu/HUD foundation, and round-trips savegames against real assets.

The biggest remaining parity problems are mostly **integration and fidelity gaps**, not total subsystem absence:

- the OpenGL renderer already has world, brush, alias, sprite, particle, decal, and viewmodel draw paths, but runtime code does not feed all of them correct data yet
- the gogpu path is still visibly behind the OpenGL path and should not be the parity gate
- the input system has Quake-style button state and binding storage, but live gameplay still uses hardcoded keys instead of bind-driven command routing
- the audio engine exists, but the main loop never dispatches parsed sound events into it
- save/load is much more complete than older notes suggested, but it still misses some exact C behavior
- demo playback exists, while demo recording is still mostly command/file scaffolding rather than full runtime integration

A useful way to think about the current tree is:

> many C algorithms have already been ported to Go, but several of them are not yet wired into the live runtime in the same way the C engine wires them together.

## High-level status

| Area | What is already implemented | Main missing / divergent behavior |
| --- | --- | --- |
| Boot, FS, QC, local runtime | real asset boot, filesystem semantics, QC VM load, local loopback single-player startup | remote connection flow is still stubbed |
| OpenGL renderer | world upload, lightmaps, lightstyle updates, brush entities, alias entities, particles, decals, viewmodel | sprite collection, decals not wired from runtime, brush angles ignored, fog/skybox/texture animation missing, render order differs from C |
| gogpu renderer | world draw path, 2D overlay, particle fallback | entity rendering is still a stub and parity should not be judged here |
| Client/input runtime | broad SVC parsing, Quake-style `KButton` handling, movement command assembly, loopback send path, demo playback primitives | prediction not called from the live frame loop, bind/unbind UX missing, hardcoded gameplay keys in `main.go` |
| Audio/music | real mixer/backend/spatialization code, sound event parsing, static sound parsing | parsed events are not dispatched to audio, listener updates are missing, music is absent |
| Menus/HUD/console/config | main menu flow, load/save/help/options/quit menus, basic HUD, console buffer/logging | multiplayer/options submenus still TODO, no full console UI, config persistence omits binds |
| Save/load | host commands, QC/global/edict/static state capture+restore, real-assets save/load test | lightstyles and some C validation rules are missing |
| Networking/multiplayer | loopback server/client and protocol work are present | `connect`, `reconnect`, and `kick` parity is missing |

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

- `Host.CmdConnect()` is still a TODO stub in `internal/host/commands.go`
- `Host.CmdReconnect()` is still a TODO stub
- `Host.CmdKick()` is still a TODO stub
- parity should currently be judged on the local/OpenGL path, not on remote multiplayer or the gogpu path

### Exact C behavior still missing

The original C engine does more here than the Go port currently exposes:

- `host_cmd.c:Host_Connect_f()` stops demo loop/playback if needed, calls `CL_EstablishConnection(name)`, then immediately calls `Host_Reconnect_f()`
- `host_cmd.c:Host_Reconnect_f()` begins the loading plaque and calls `CL_ClearSignons()` so the client re-runs the full signon process
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

#### Not yet fed from the live runtime

- `main.go` collects `BrushEntities`, `AliasEntities`, and a viewmodel, but it never populates `SpriteEntities`
- `internal/renderer/client_effects.go` contains `EmitDecalMarks()`, but `main.go` never creates a `DecalMarkSystem` or passes `DecalMarks` into `RenderFrameState`
- the renderer has a dynamic light pool (`SpawnDynamicLight`, `EvaluateLightsAtPoint`), but the live runtime never spawns gameplay lights into it

#### Implemented, but behavior still diverges from C

- brush entities currently use translation only; `BrushEntity.Angles` is collected in `main.go` but ignored by `renderBrushEntities()`
- entity alpha is only partially honored; alias draws use `entity.Alpha`, but the runtime does not comprehensively map protocol alpha/scale/effects for all entity types
- client-side fog and skybox data are parsed (`Client.FogDensity`, `FogColor`, `SkyboxName`) but are not consumed by the renderer
- sky surfaces are currently rendered through the ordinary world draw path rather than a dedicated C-style `Sky_DrawSky()` pipeline
- the texture-animation helper exists in `internal/renderer/surface.go`, but the world renderer does not currently use it when selecting animated textures
- entity effects from the network protocol are parsed, but visual behaviors such as muzzle flashes and other effect-driven lighting are not yet wired through the renderer

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

Go already ports this logic in `internal/renderer/surface.go:TextureAnimation()`, including broken/infinite cycle detection, but the live renderer does not yet apply it to world textures.

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

#### Prediction exists on paper, but is not wired into the live runtime

- `internal/client/prediction.go` contains `PredictPlayers()` and smoothing/error-correction code
- there is no live call site for `PredictPlayers()` in `main.go`
- the renderer still tries to use `gameClient.PredictedOrigin`, so prediction data exists as a concept but is not actually driven each frame

#### Input mechanics exist, but Quake-style binding UX does not

- gameplay input in `main.go` is hardcoded to keys such as `W`, `A`, `S`, `D`, `Ctrl`, `Space`, and mouse buttons
- `internal/input/types.go` already stores bindings and key names, but there are no `bind` / `unbind` console commands and no bind-driven command dispatch path
- the current runtime is therefore closer to a hardcoded control scheme than to the C engine's user-configurable input model

#### Demo playback is ahead of demo recording

- `playdemo` / `stopdemo` are implemented at the host command level
- `record` opens a file and writes the CD-track header, but live gameplay does not call `WriteDemoFrame()` per frame
- `FinishDemoFrame()` is a no-op in Go
- `stop` still has a TODO to write the final disconnect message before ending recording

### Exact C behavior still missing or not fully matched

#### `cl_input.c:CL_BaseMove()` / `CL_SendMove()`

The C engine defines an exact mapping from button state to `usercmd_t` and then to the network message:

- movement comes from `CL_KeyState()` over forward/back, strafe, moveleft/moveright, up/down
- the speed key multiplies forward/side/up movement
- `CL_SendMove()` writes `clc_move`, the last server time, three view angles, forward/side/up shorts, button bits, and the impulse byte
- attack and jump use bit 0 and bit 1 respectively, and both clear their edge-down bit after the move packet is sent

Go mirrors much of this logic in `internal/client/input.go`, but the surrounding input UX still differs because bindings are not user-driven.

#### `cl_demo.c:CL_Record_f()`

The C recording path is more capable than the current Go runtime:

- it may start recording while already connected (once signon is far enough along)
- it writes the stored signon head into the demo first
- it emits current names, frags, colors, lightstyles, stats, `svc_setview`, and a signon marker so the demo starts from a complete state snapshot

Go has the file format primitives, but not the equivalent runtime integration yet.

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

### What is missing or divergent

- multiplayer submenu selections still only emit TODO `echo` commands (`join game`, `host game`, `player setup`)
- options submenu still only has one real action (`toggle vid_vsync`); controls/video/audio are placeholders
- the HUD is much simpler than `sbar.c`; inventory, face states, sigils, keys, weapon strip, ammo icons, and other status-bar details are not yet matched
- the console core exists as a data structure, but there is no full in-game console UI/render/input path wired into `main.go`
- gameplay still uses hardcoded controls instead of bind-driven command execution
- `Host.WriteConfig()` only writes archived cvars; it does not write binds or the extra state the C engine persists

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
- `TestCmdSaveLoadRealAssetsRoundTrip` proves a real-assets session can save, reload, and recover player state

This is one of the biggest places where older status docs understated current progress.

### What is missing or divergent

- lightstyles are not included in `SaveGameState`
- the Go save path does not enforce all of the C engine's restrictions (`nomonsters`, intermission, dead-player save ban)
- load/save path behavior is simplified to `userDir/saves/<name>.sav`
- there is no equivalent of the C engine's loading plaque / broader save-file search behavior

### Exact C behavior still missing or not fully matched

#### `host_cmd.c:Host_Savegame_f()`

The C save command refuses to save when:

- there is no active local game
- `nomonsters` is enabled
- the client is in intermission
- the game is multiplayer
- any active player is dead

Go currently only enforces "active built-in server" and single-player mode.

#### `host_cmd.c:Host_Loadgame_f()`

The C load path does more than simple deserialization:

- it shows the loading plaque
- can search outside the active game dir in some cases
- restores spawn parms, skill, map, time, lightstyles, globals, and edicts
- re-enters the connection/signon flow after restoration

Go already restores most of the world/QC state, but it still lacks lightstyles and several surrounding behaviors.

## 7. Networking and multiplayer parity

### What is already implemented

- local loopback client/server plumbing is real and used by the single-player runtime
- the server emits core signon data, static entities, static sounds, and entity updates
- status/ping style host commands exist

### What is missing or divergent

- `connect`, `reconnect`, and `kick` are not feature-complete
- multiplayer menu flows are still placeholder UX
- the current runtime should be treated as single-player-first even though pieces of the multiplayer protocol are already present

### Exact C behavior still missing or not fully matched

- `Host_Connect_f()` establishes the remote connection and then forces a reconnect-style signon restart
- `Host_Reconnect_f()` begins a loading plaque and clears signons
- `Host_Kick_f()` supports name or slot-number targeting plus an optional kick message

## 8. Overall parity judgement

The current Go port is best described as:

- **strong in filesystem, QC loading, local boot, and several data-model ports**
- **surprisingly strong in OpenGL world/entity foundations and save/load serialization**
- **partially complete in menus/HUD/client parsing/demo playback**
- **not yet parity-complete in runtime integration, user-facing input/config UX, audio dispatch, remote networking, and secondary backend support**

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
