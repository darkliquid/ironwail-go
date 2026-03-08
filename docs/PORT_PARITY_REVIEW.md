# Ironwail Go port parity review

This document is the canonical, source-backed review of the current `ironwail-go` port against the original C Ironwail/Quake engine.

- Repositories compared: `ironwail`, `ironwail-go`, and `ironwail-go-docs`
- Rule of thumb: when documentation and source disagree, source wins
- Practical parity target: the CGO/OpenGL runtime (`renderer_opengl.go` + GLFW input path + SDL3/Oto audio backend) is the authoritative baseline; the gogpu/WebGPU path is still secondary

## Executive summary

The Go port is materially farther along than several older planning notes imply. It already boots real assets into an active local single-player session, parses a wide slice of the server protocol, uploads BSP world geometry with real lightmaps on the OpenGL path, renders brush and alias entities there, provides a working menu/HUD foundation, and round-trips savegames against real assets.

The biggest remaining parity problems are mostly **integration and fidelity gaps**, not total subsystem absence:

- the OpenGL renderer already has world, brush, alias, sprite, particle, decal, viewmodel, dynamic-light, animated-texture, fog, turbulent UV warp, embedded two-layer Quake sky integration, and external skybox consumption (cubemap plus non-cubemap per-face path, including mixed-case lowercase asset fallback); the main remaining render gaps are now mostly broader polish beyond the bounded pass-order/skybox/particle staging slices
- the gogpu path is still visibly behind the OpenGL path and should not be the parity gate
- the input/command layer now uses Quake-style bindings, config persistence, command aliases (`alias`/`unalias`/`unaliasall`), and live prediction; the bigger remaining client-state gaps are special intermission/cutscene handling and remote networking flow
- the audio/music path now dispatches parsed sounds into the live mixer, maintains static sounds, updates the listener, and plays WAV-backed CD tracks; broader fidelity/format parity still remains
- save/load is much more complete than older notes suggested, and now includes C's `nomonsters` / intermission / dead-player save restrictions, session-transition sound teardown, and local load/reconnect loading-plaque visibility; broader search behavior and remote connect UX still differ
- demo recording and forward playback are now real runtime features, including connected-state snapshots, disconnect trailers, same-frame stuffed-command execution, pause semantics, and server-time pacing

A useful way to think about the current tree is:

> many C algorithms have already been ported to Go, but several of them are not yet wired into the live runtime in the same way the C engine wires them together.

## High-level status

| Area | What is already implemented | Main missing / divergent behavior |
| --- | --- | --- |
| Boot, FS, QC, local runtime | real asset boot, filesystem semantics, QC VM load, local loopback single-player startup, and local/remote `connect`/`disconnect` session transitions | remote transport edge-case polish still remains |
| OpenGL renderer | world upload, lightmaps, lightstyle updates, brush entities, alias entities, sprites, particles, decals, viewmodel, dynamic lights, brush rotation, animated textures, turbulent UV warp, live fog, dedicated embedded sky layer animation path, and external skybox consumption via cubemap/per-face paths (with lowercase fallback for mixed-case names) | remaining divergences are now mostly broader visual polish rather than the previously-bounded particle-pass and per-face skybox edge slices |
| gogpu renderer | world draw path, 2D overlay, particle fallback | entity rendering is still a stub and parity should not be judged here |
| Client/input runtime | broad SVC parsing, Quake-style `KButton` handling, movement command assembly, live prediction, bind-driven command routing, config persistence, loopback send path, demo record/playback integration, bounded intermission/finale/cutscene + centerprint runtime overlay wiring plus deathmatch scoreboard hold/overlay support, and remote signon auto-reply progression (`prespawn`/`spawn`/`begin`) | broader netgame depth still diverges |
| Audio/music | real mixer/backend/spatialization code, sound event parsing and dispatch, static sound lifecycle, listener updates, WAV CD-track playback | broader codec/fidelity parity still remains |
| Menus/HUD/console/config | main menu flow, load/save/help/options/quit menus, bounded options video/audio/controls submenus (including live controls cvars for sensitivity/invert-mouse/always-run/freelook plus bind editing), in-game console UI, history/completion, bind persistence, Quake-style alias commands, menu-space text scaling for text-only prompts, and a base-game classic status-bar HUD path with weapon strip, ammo strip, key/powerup/sigil icons, and armor/face/ammo icon helpers | HUD parity still lacks special-case overlays/expansion-pack variants and broader C menu polish |
| Save/load | host commands, QC/global/edict/static state capture+restore, real-assets save/load test, lightstyles, C-style `nomonsters`/intermission/dead-player restrictions, stop-all sound teardown on local load/map/reconnect-style transitions, and loading-plaque overlay visibility for load/reconnect across local and remote transition paths | broader C search behavior and long-tail transition UX polish are still missing |
| Networking/multiplayer | loopback server/client and protocol work are present, local+remote `connect` now drive signon-capable sessions, `disconnect` tears down session state (including stop-all sounds), `reconnect` re-runs signon flow (including remote reconnect command dispatch), local-host `kick` supports C-style target/reason handling, and join/host/setup menus route through live host commands | broader remote networking flow depth remains incomplete |

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

- `Host.CmdConnect()` now handles local-loopback parity slices (`demonum=-1`, demo-playback stop/reset, `connect local` handoff) and also establishes remote transport-backed sessions
- `Host.CmdKick()` now supports local name/slot targeting, optional reasons, and self-kick protection
- parity should currently be judged on the OpenGL path first; remote multiplayer is now functional but still in bounded parity scope

### Exact C behavior still missing

The original C engine does more here than the Go port currently exposes:

- `host_cmd.c:Host_Connect_f()` stops demo loop/playback if needed, calls `CL_EstablishConnection(name)`, then immediately calls `Host_Reconnect_f()`; Go now mirrors this sequence for local loopback targets and explicit disconnect/reset behavior
- `host_cmd.c:Host_Reconnect_f()` begins the loading plaque and calls `CL_ClearSignons()` so the client re-runs the full signon process; Go now mirrors local and remote signon reset/restart plus transition sound teardown/loading-plaque visibility in bounded scope
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
- dedicated lightmap texture upload/filtering path now uses linear min/mag filtering for world/brush lightmaps (and fallback lightmap), avoiding generic nearest-filter world texture sampling artifacts
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

- embedded BSP sky surfaces now render through a dedicated animated sky-layer shader/path (instead of the ordinary world shader), and the external skybox path now consumes both cubemap-eligible sets and loaded non-cubemap face sets (per-face external sky draw) with embedded fallback when no external faces load or upload fails
- particle rendering now has explicit opaque/translucent subpass plumbing in the top-level OpenGL frame path, sky now runs in its own post-opaque stage, world/brush liquid surfaces bucket into dedicated opaque/translucent liquid bins and are staged so all opaque liquid draws happen before any translucent liquid draws, alias-model entities now split into explicit opaque/translucent frame stages, translucent brush-entity non-liquid work has moved out of the early opaque stage, the late-frame translucent pass is now wrapped in an explicit begin/end translucency state block before viewmodel rendering, runtime sprites now render in that late translucency stage (instead of the early opaque entity block), runtime sprite-frame selection now mirrors C more closely (`SPR_GROUP` uses client-time intervals while `SPR_ANGLED` chooses directional subframes from the current camera basis), runtime sprite quad orientation now mirrors C sprite-type behavior (`SPR_VP_PARALLEL_UPRIGHT` / `SPR_FACING_UPRIGHT` / `SPR_VP_PARALLEL` / `SPR_ORIENTED` / `SPR_VP_PARALLEL_ORIENTED`), runtime particles now honor `r_particles` mode when selecting opaque vs translucent passes, lowercase fallback resolves mixed-case per-face external skybox lookups, and the runtime viewmodel path now anchors to eye origin with `r_drawviewmodel`/intermission/invisibility/death gating while staying on the dedicated post-translucency depth-range pass

### 2.3 gogpu path: current status

Relevant Go file:

- `internal/renderer/renderer_gogpu.go`

The gogpu path still contains major parity gaps:

- `DrawContext.RenderFrame()` now runs a bounded entity-marker baseline instead of a full model pipeline
- `renderEntities()` projects entity origins as screen-space markers; full model/sprite/decal parity is still TODO
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

The Go OpenGL path now uses explicit staged ordering (opaque world/entities/particles, dedicated sky, split liquid opaque/translucent, late translucent block, then the depth-ranged viewmodel pass) for the currently scoped `R_RenderScene()` parity slice, with runtime particles honoring `r_particles` pass routing and per-face skybox loads handling mixed-case lowercase fallback.

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
- the client now preserves `svc_clientdata` viewheight and punch state instead of discarding those server-driven eye-space inputs, narrowing the remaining camera/viewmodel fidelity gap
- server frame ordering now matches C host flow for movement (`RunClients()` before `Physics()`), `FrameTime` is set from the outer frame step, and server time advances once per frame through physics
- `MoveTypeWalk` entities now run authoritative server gravity/collision movement (`AddGravity` + `FlyMove` + `LinkEdict`) instead of only `RunThink`, restoring bounded walk/gravity collision behavior
- the runtime camera now raises predicted/view-entity origin by the parsed client viewheight, so normal gameplay renders from eye space instead of raw player origin
- runtime camera origin now prefers authoritative server entity origin and only falls back to simplified prediction when no entity origin is available
- the runtime camera now applies stored punch angles during normal gameplay (while still skipping them in intermission), and the bounded `v_gunkick` path now supports off/instant/interpolated kick behavior in the live runtime
- the runtime viewmodel now uses the active eye/view origin and honors bounded C-style visibility gates (`r_drawviewmodel`, intermission suppression, invisibility/death checks)
- gameplay input routes through live `bind` / `unbind` / `unbindall` / `bindlist` handling, and `config.cfg` persists those bindings
- demo recording writes live gameplay frames, connected-state snapshots, and a disconnect trailer; playback applies recorded view angles, flushes `stufftext` in the same frame, honors pause state, and paces reads against recorded server time; bounded `timedemo` and `rewind` tooling now runs on top of that forward path

#### Remaining client/runtime divergences

- bounded intermission / finale / cutscene overlay flow now runs in the live HUD path (`gfx/complete.lmp` + `gfx/inter.lmp` stats overlay and `gfx/finale.lmp` + timed center-text reveal), while remaining HUD divergence is now mostly expansion-pack special-casing and broader menu polish
- remote `connect` now establishes transport-backed sessions and auto-progresses signon replies (`prespawn`/`spawn`/`begin`), while broader netgame/deathmatch polish still remains

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

Bounded rewind/timedemo tooling is now wired, while deeper C demo tooling/benchmark polish remains future work.

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

- a real audio system with channel selection, precache, start/stop sound, mixing, listener spatialization, and view-entity full-volume handling
- backend selection via SDL3 first, then Oto, then NullBackend fallback
- parsed client-side `SoundEvent`, `StopSoundEvent`, and `StaticSound` data structures consumed into the live mixer/runtime
- identical static-world loops are re-spatialized then combined during `audio.System.Update()`, matching C's `S_Update()` static-sound combine behavior
- WAV-backed CD-track playback now follows live `CDTrack` / `LoopTrack` changes
- server-side sound message emission in `Server.StartSound()`

### What is missing or divergent

- broader codec/music parity beyond the current WAV/OGG CD-track support is still missing
- underwater visual blue-shift and broader audiovisual polish remain outside the current bounded parity slice

### Exact C behavior still missing or not fully matched

#### `snd_dma.c:S_Update()`

The Go runtime now mirrors the main C runtime behavior in this area:

- sounds tied to the active `viewentity` stay full volume during spatialization
- identical static sound loops are re-spatialized then combined so clustered torches/drips do not all mix independently every frame
- per-frame runtime code now samples the current BSP leaf and calls `UpdateAmbientSounds()` from `cmd/ironwailgo/main.go`, and `internal/audio/sound.go` applies ambient fade plus underwater-intensity behavior

Coverage now includes `internal/audio/audio_test.go` for ambient/underwater behavior.

#### `sv_main.c:SV_StartSound()`

The client parser accepts both compact and large sound/entity encodings, and the built-in Go server now emits `SND_LARGEENTITY` / `SND_LARGESOUND` packets when entity/channel/sound index ranges require them, matching the C edge-case packet behavior for this bounded slice.

Coverage now includes `internal/server/server_test.go` for sound-packet encoding edges.

#### Music / CD behavior

The original engine keeps a separate music path (`bgmusic.c`) distinct from one-shot SFX. The Go port now responds to `CDTrack` / `LoopTrack` with WAV/OGG playback and C-style search-path priority across those supported formats, but broader codec parity from `bgmusic.c` still remains.

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
- menu key routing now fires on key-down events only, matching C's `Key_Event(..., down=true)` menu path and preventing release-edge double navigation/actions
- a basic Quake-style HUD with status bar and centerprint support
- a real console text buffer with scrollback, resize behavior, notify lines, and debug logging
- input destination routing (`KeyGame`, `KeyMenu`, `KeyConsole`, `KeyMessage`)
- key name conversion and binding storage in `input.System`
- user-facing `alias`, `unalias`, and `unaliasall` commands backed by cmdsys aliases, with command-over-alias execution precedence
- console tab completion now includes alias names alongside commands and cvars

### What is missing or divergent

- multiplayer submenu now has bounded `join game` and `host game` flows: join supports menu text entry and dispatches `connect <address>`, while host applies bounded local options (`maxplayers`/`coop`/`deathmatch`/`skill`/`map`) through queued commands; bounded `player setup` now syncs hostname/player name/colors from live cvars and applies changes through `hostname`/`name`/`color`
- options submenu now has bounded video/audio/controls submenus wired to supported cvars; controls now exposes live sensitivity/invert-mouse/always-run/freelook settings plus existing bind editing
- setup menu still uses a simplified color-swatch preview instead of the C menu's translated player preview/text-box art in this bounded slice
- the HUD now has a base-game classic `sbar.c`-style strip (weapon/ammo inventory, keys/powerups/sigils, armor/face/ammo icons, and live client-driven numbers), bounded deathmatch scoreboard overlays (`+showscores` hold and multiplayer frag rows), and a bounded live intermission/finale/cutscene overlay path with timed center-text reveal; it still omits expansion-pack special cases and pickup flash timing polish
- the menu/console layer still lacks a number of C-polish details even though the baseline in-game console UI/render/input path is now wired
- the HUD and option/menu surfaces still expose much less functionality than the C engine
- `Host.WriteConfig()` now writes binds plus archived cvars in deterministic order, and startup `config.cfg` execution now round-trips archived cvar values through cmdsys cvar fallback handling

### Exact C behavior still missing or not fully matched

#### `host.c:Host_WriteConfigurationToFile()`

The C engine writes more than archived cvars:

- `Key_WriteBindings(f)` writes all key bindings
- `Cvar_WriteVariables(f)` writes archived cvars
- it then appends extra state-preserving commands such as `vid_restart` and `+mlook` when needed

Go now writes key bindings plus archived cvars in deterministic order and verifies cvar state reload through `config.cfg`; the remaining gap is the extra trailing state commands that depend on runtime features not yet exposed here.

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
- `TestCmdSaveLoadRealAssetsRoundTrip` proves a real-assets session can save, reload, and recover player state, and `TestSaveGameStateRoundTripsGameplayState` adds focused inventory/gameplay-state restore coverage (ammo/weapon/items/armor, server flags, pause/time, and client spawn parms)

This is one of the biggest places where older status docs understated current progress.

### What is missing or divergent

- load/save path behavior is simplified to `userDir/saves/<name>.sav`
- there is still no equivalent of the C engine's broader save-file search behavior; loading-plaque rendering now covers local load/reconnect plus remote reconnect hold-until-signon with a bounded failsafe timeout

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

Go already restores most of the world/QC state, including lightstyles, transition sound teardown, post-load signon re-entry, and local/remote reconnect loading-plaque transition visibility; it still lacks the broader save-file search behavior.

## 7. Networking and multiplayer parity

### What is already implemented

- local loopback client/server plumbing is real and used by the single-player runtime
- the server emits core signon data, static entities, static sounds, and entity updates
- status/ping style host commands exist

### What is missing or divergent

- remote transport-backed `connect`/`reconnect` is now wired and signon-completing; bounded deathmatch rules now enforce `fraglimit`/`timelimit` and delayed respawn, while broader game-options/netgame depth is still pending
- multiplayer menu flows now route join/host/setup through the same live remote-capable host command path
- runtime remains parity-scoped and still lacks full C netgame breadth despite real remote session transitions

### Exact C behavior recently closed in this slice

- `Host_Connect_f()` parity: remote connect now mirrors C sequencing by establishing transport and then forcing reconnect-style signon reset state
- `Host_Reconnect_f()` parity: reconnect starts loading plaque and clears signons for both local and remote flows, matching the bounded C behavior targeted here
- `Host_Kick_f()` supports name or slot-number targeting plus an optional kick message (mirrored on the local host path)

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
