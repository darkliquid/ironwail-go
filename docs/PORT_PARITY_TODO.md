# Ironwail Go parity todo list

This is the ordered implementation plan to reach full feature and behavior parity with the original C Ironwail/Quake engine.

## Ground rules

1. Treat the **OpenGL + GLFW + SDL3/Oto** runtime as the authoritative parity target.
2. Treat `ironwail-go-docs` as helpful context, but use C and Go source as the final authority.
3. Do not spend parity-critical time on the gogpu path until the OpenGL path is behaviorally correct.

## Ordered steps

### 1. Keep one runtime authoritative

**Goal**

Freeze the parity target around the existing CGO/OpenGL path so implementation work stops bouncing between incompatible backends.

**Primary Go files**

- `README.md`
- `mise.toml`
- `cmd/ironwailgo/main.go`

**Why this comes first**

The repo currently contains both gogpu-first and CGO-first narratives. Parity work will churn unless one runtime is treated as the acceptance target.

**Done when**

- manual parity testing and bug triage are explicitly done on the OpenGL path first
- gogpu is treated as secondary until the end of this list

### 2. Finish frame-loop integration for live client state

**Goal**

Make the live runtime actually use the client-side systems that already exist in code.

**Primary Go files**

- `cmd/ironwailgo/main.go`
- `internal/client/prediction.go`
- `internal/client/client.go`

**Work**

- ✅ call `PredictPlayers()` every frame instead of only reading `PredictedOrigin` (wired in live runtime frame loop)
- ensure camera/viewmodel logic uses the updated predicted state consistently
- centralize one per-frame place where transient client events are consumed and applied

**Why now**

Rendering, audio, and viewmodel correctness all depend on the runtime using the same predicted/player state each frame.

**Done when**

- prediction code is exercised in the real runtime, not just tests
- the camera and viewmodel no longer rely on stale or zeroed prediction fields (still in progress)

### 3. Wire audio end to end

**Goal**

Connect parsed sound data to the real audio engine.

**Primary Go files**

- `cmd/ironwailgo/main.go`
- `internal/client/parse.go`
- `internal/audio/adapter.go`
- `internal/audio/sound.go`

**Primary C references**

- `cl_parse.c:CL_ParseStartSoundPacket()`
- `snd_dma.c:S_StartSound()`
- `bgmusic.c`

**Work**

- ✅ consume `Client.SoundEvents` and call `audio.System.StartSound()`
- ✅ instantiate `Client.StaticSounds` as persistent world channels (static channel range wired; rebuilt on client/precache snapshot changes)
- ✅ implement `parseStopSound()` dispatch to `audio.System.StopSound()`
- ✅ drive listener updates from the real camera/orientation each frame
- ✅ add WAV-backed CD-track playback semantics (track change/stop/loop wiring landed; broader codec parity is still outstanding)

**Done when**

- weapon, monster, ambient, and local sounds all play from live gameplay
- sound attenuation and positioning behave like the C engine
- CD track / loop track values result in actual music playback when supported music files are present

### 4. Replace hardcoded gameplay controls with real Quake bindings

**Goal**

Move from a hardcoded control map in `main.go` to bind-driven input behavior.

**Primary Go files**

- `cmd/ironwailgo/main.go`
- `internal/input/types.go`
- `internal/client/input.go`
- `internal/host/init.go`

**Primary C references**

- `keys.c`
- `cl_input.c`
- `host.c`

**Work**

- [x] add `bind`, `unbind`, and related console commands (`unbindall`/`bindlist` included)
- [x] route key events through bindings into `+attack`, `-attack`, `+jump`, and the rest of the Quake command set
- [x] keep default binds equivalent to the current hardcoded controls until the config layer takes over
- [x] persist bindings in `config.cfg` (startup config load + config writes now restore binds across restart)

**Why before menu/console polish**

Controls, console commands, options menus, and config persistence all depend on the same binding model.

**Done when**

- [x] gameplay can be controlled entirely through the bind system
- [x] binds survive restart via `config.cfg`
- [x] hardcoded gameplay-only key mapping in `main.go` is no longer the primary control path

### 5. Add the real in-game console UX

**Goal**

Turn the existing console core into the player-facing Quake console.

**Primary Go files**

- `cmd/ironwailgo/main.go`
- `internal/console/console.go`
- rendering/draw files that will present console text
- `internal/input/types.go`

**Primary C references**

- `console.c`
- `gl_screen.c`
- `keys.c`

**Work**

- [x] add console toggle behavior and key-destination switching
- [x] render console contents and notify lines in the live renderer
- [x] support text entry, history, and scrollback from the running game
- [x] add tab completion from the running game
- [x] expose Quake-style `alias` / `unalias` / `unaliasall` commands through the live console

**Done when**

- the user can open the console, type commands, inspect output, and close it without leaving gameplay

### 6. Finish menu and HUD parity as one UX workstream

**Goal**

Close the player-facing UI gaps together instead of as isolated stubs.

**Primary Go files**

- `internal/menu/manager.go`
- `internal/hud/hud.go`
- `internal/hud/status.go`
- `cmd/ironwailgo/main.go`

**Primary C references**

- `menu.c`
- `sbar.c`

**Work**

- replace TODO submenu actions for join/host/player setup/controls/video/audio
- wire real menu actions into cvars/commands
- extend the HUD toward `sbar.c` behavior: inventory, face state, keys, weapon strip, ammo icons, powerup indicators, and other missing status elements

**Done when**

- all visible menu entries lead to real functionality
- the HUD exposes the same gameplay-critical information the C engine does

### 7. Feed the remaining renderable state into the OpenGL renderer

**Goal**

Use the render backends that already exist but are not yet driven by the live runtime.

**Primary Go files**

- `cmd/ironwailgo/main.go`
- `internal/renderer/client_effects.go`
- `internal/renderer/mark_system.go`
- `internal/renderer/stubs_opengl.go`

**Work**

- [x] collect sprite entities from client state and pass them into `RenderFrameState.SpriteEntities`
- [x] create and maintain a `DecalMarkSystem`; call `EmitDecalMarks()` and pass active marks into `RenderFrameState.DecalMarks`
- [x] map temp entities into dynamic lights
- [x] map alias-entity muzzle/bright/dim effect flags into dynamic lights
- [x] map alias-entity quad/penta effect flags into dynamic lights
- [x] keep defined-but-unrendered effect-light flags aligned with C Ironwail
- [x] honor protocol alpha / scale on alias and sprite entities
- [x] honor protocol alpha / scale on brush entities
- [x] honor brush-entity rotation parity
- [x] honor remaining entity effect flags with EF_BRIGHTFIELD particles

**Done when**

- sprites, projected marks, and gameplay lights appear in the live game
- protocol fields that are already parsed are no longer silently ignored by rendering

### 8. Close the OpenGL renderer's fidelity gaps

**Goal**

Make the authoritative renderer behave like the C renderer, not just draw approximately the right things.

**Primary Go files**

- `internal/renderer/world_runtime_opengl.go`
- `internal/renderer/world_opengl.go`
- `internal/renderer/surface.go`
- `internal/renderer/stubs_opengl.go`

**Primary C references**

- `gl_rmain.c:R_RenderScene()`
- `r_brush.c:R_TextureAnimation()`
- `gl_warp.c`
- sky/fog-related renderer files in the C tree

**Work**

- [x] apply brush entity angles, not just origin offsets
- [x] integrate animated texture selection into world rendering
- [x] apply C-style turbulent UV warp on `SurfDrawTurb` world + brush surfaces (OpenGL path)
- [x] consume client fog state
- [x] render embedded BSP sky via dedicated animated two-layer sky shader/path on canonical OpenGL runtime
- [x] align embedded-sky fog mix semantics with C (`r_skyfog` + worldspawn `skyfog`, gated by general fog density)
- [x] consume client skybox state on canonical OpenGL path and load common external cubemap skyboxes from Quake FS search paths (`gfx/env/<name><suffix>.{png,tga,jpg}`), including partial square face sets (zero-filling missing faces) with fallback to embedded BSP sky for inconsistent/non-square cases
- [x] bounded post-38de7f3 parity slice: split particle rendering into explicit opaque/translucent subpasses so the top-level OpenGL frame path can separate those passes before the larger sky/water/translucency ordering refactor (with current runtime particles still landing in the opaque side)
- [x] bounded post-5800311 parity slice: split world/brush liquid surfaces into explicit opaque-liquid/translucent-liquid buckets in canonical OpenGL runtime and draw those buckets separately from general opaque/translucent world buckets (no full-frame reorder yet)
- bring sky, water, translucent ordering, and viewmodel ordering closer to C pass sequencing (remaining larger pass-order refactor, including full frame-level sequencing across entities/particles/viewmodel)

**Done when**

- rotating/moving brush submodels render correctly
- animated water/other texture sequences update the same way as C
- fog and embedded sky animation behavior come from live client/runtime state instead of being ignored

### 9. Finish save/load behavior parity

**Goal**

Take the already-strong save/load system the rest of the way to C parity.

**Primary Go files**

- `internal/host/commands.go`
- `internal/server/savegame.go`
- menu integration files

**Primary C references**

- `host_cmd.c:Host_Savegame_f()`
- `host_cmd.c:Host_Loadgame_f()`

**Work**

- [x] save and restore lightstyles
- [x] enforce save restrictions (`nomonsters`, intermission, dead-player)
- [x] stop active gameplay sounds on local session transitions (`disconnect`/`reconnect`/`load`/`map`) to mirror the C engine's stop-all behavior
- align user-visible save/load flow with the C engine where practical
- make sure menu/UI entry points reach the same save/load system

**Done when**

- a saved game restores the same gameplay-relevant state as the C engine
- unsupported save situations fail the same way the C engine does
- loading-plaque rendering and broader save-file search UX are still tracked separately as remaining parity work

### 10. Finish demo recording parity

**Goal**

Make demo recording a real runtime feature, not just file I/O scaffolding.

**Primary Go files**

- `internal/client/demo.go`
- `internal/host/commands.go`
- `cmd/ironwailgo/main.go`

**Primary C references**

- `cl_demo.c:CL_Record_f()`
- `cl_demo.c:CL_Stop_f()`
- `cl_demo.c:CL_FinishDemoFrame()`

**Work**

- [x] write demo frames during live gameplay
- [x] support recording while already connected by emitting the initial state snapshot the C engine writes
- [x] implement the missing stop/disconnect trailer behavior
- [x] align timing/frame-finalization behavior with the C path

**Done when**

- a live session can be recorded, stopped, and replayed with the correct initial state and progression

### 11. Finish remote networking and multiplayer commands

**Goal**

Close the host-command and UI gaps that keep the port local-only.

**Primary Go files**

- `internal/host/commands.go`
- network transport code
- `internal/menu/manager.go`

**Primary C references**

- `host_cmd.c:Host_Connect_f()`
- `host_cmd.c:Host_Reconnect_f()`
- `host_cmd.c:Host_Kick_f()`

**Work**

- [~] implement bounded `connect`/`disconnect` parity for local loopback (`demonum=-1`, demo playback stop/reset, `connect local` re-entry into local signon flow)
- [x] implement local-loopback `reconnect` signon restart behavior
- [x] implement local host `kick` parity by name or slot with optional message
- [x] stop active gameplay sounds during local disconnect/reconnect-style session transitions (without implementing full loading-plaque rendering)
- implement remote transport-backed `connect` flow (currently explicit unsupported path with console messaging)
- connect the multiplayer menus to real behavior

**Done when**

- remote server connection and basic multiplayer host control work from both console and menus

### 12. Only then decide how far to push gogpu parity

**Goal**

Bring the secondary renderer up to a consciously chosen level instead of letting it block the main parity path.

**Primary Go files**

- `internal/renderer/renderer_gogpu.go`

**Work**

- either port the now-correct OpenGL/runtime behavior forward into gogpu
- or explicitly document gogpu as a non-parity experimental/fallback path

**Done when**

- the repo has a clear, honest statement about gogpu's role
- gogpu no longer creates ambiguity about whether parity has actually been reached

## Final acceptance checklist

The port should not be considered at parity until all of the following are true on the authoritative OpenGL runtime:

- local single-player campaign flow works start-to-finish
- controls are bind-driven and persisted
- console, menus, and HUD are all fully usable from inside the game
- world, brush models, alias models, sprites, particles, decals, fog, sky, and viewmodel all render with the expected C behavior
- sound effects, static sounds, stop-sound handling, and music all work
- save/load restores gameplay state accurately, including lightstyles
- demo recording and playback both behave like the C engine
- remote `connect`/`reconnect` flow and multiplayer menu control behave like the C engine (local loopback `connect`/`disconnect` parity slices alone are not sufficient)
