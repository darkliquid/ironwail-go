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
- [x] store `svc_clientdata` viewheight/punch state on the live client so later eye-space camera/viewmodel parity slices can consume real server-driven values
- [x] raise the runtime camera to the client viewheight so normal gameplay view uses eye-space origin instead of raw player origin
- [x] apply stored client punch angles to the runtime camera outside intermission so recoil/damage kick now affects the live eye-space view
- [x] anchor runtime viewmodel placement to eye/view origin (not raw player origin), gate it behind `r_drawviewmodel`, and suppress it during intermission
- [x] restore authoritative walk/gravity collision flow by running `RunClients()` before `Physics()` in server frames, fixing `FrameTime`/time-step propagation, and running `MoveTypeWalk` through server collision/gravity movement each physics tick
- [x] make runtime camera prefer authoritative server entity origin over simplified predicted origin so manual runtime movement reflects server collision/gravity behavior
- [x] restore loopback movement intake by consuming pre-submitted `SubmitLoopbackCmd()` input in `RunClients()` (instead of parsing server reliable buffers as client input), with map-backed regression coverage that asserts authoritative server player origin moves
- [x] match C `SV_AirMove` walk parity by forcing `wishvel[2]=0` for `MoveTypeWalk`, preventing pitched forward intent from being projected into vertical velocity and clipped away by ground collision
- centralize one per-frame place where transient client events are consumed and applied

**Why now**

Rendering, audio, and viewmodel correctness all depend on the runtime using the same predicted/player state each frame.

**Done when**

- prediction code is exercised in the real runtime, not just tests
- [x] the camera and bounded runtime viewmodel path no longer rely on stale or zeroed prediction fields

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
- ✅ route the active `ViewEntity` into audio re-spatialization and combine identical static loops after spatialization, matching C self-sound/full-volume and static ambient mix behavior
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
- [x] persist archived cvars through `config.cfg` with deterministic bind/cvar write ordering and explicit host-level roundtrip coverage

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

- [x] replace remaining TODO submenu actions for join/host/controls (bounded player setup menu now supports hostname + player name + shirt/pants colors + accept/apply via `hostname`/`name`/`color`; options now route to bounded video/audio/controls submenus backed by live cvars and bind commands, including live controls sensitivity/invert/always-run/freelook and bind editing)
- [x] sync multiplayer setup text-entry fields from live `hostname`/`_cl_name`/`_cl_color` state so reopening the menu reflects current player/server settings
- [x] bounded `hud-statusbar-icons` parity slice: thread live client HUD stats/items into `hud.State` and render a base-Quake classic status bar (`sbar`/`ibar`) with weapon strip, ammo counts strip, keys/powerups/sigils, armor+face+ammo icons, and numeric readouts
- [x] bounded `intermission-cutscene-parity` slice: preserve parsed finale/cutscene strings in live client state, feed live centerprint/intermission state into runtime HUD overlay flow, and render base-Quake intermission (`gfx/complete.lmp` + `gfx/inter.lmp` + map/time/secrets/monsters) plus finale/cutscene (`gfx/finale.lmp` + timed center-text reveal) overlays on the canonical runtime path
- [x] bounded `deathmatch-scoreboard-parity` slice: wire default `TAB`/`+showscores` bindings, feed live multiplayer name/color/frags into `hud.State`, and render ranked deathmatch scoreboard overlays plus compact multiplayer frag rows
- [x] route menu key handling from key-down events only (avoid doubled cursor movement and double-fired one-shot menu actions on key release)
- [x] restore audible menu interaction feedback by wiring menu navigation/accept/cancel events to local `misc/menu*.wav` playback on the canonical runtime path
- [x] fix canonical OpenGL HUD/icon coordinate-space regression by splitting `DrawPic` (screen-space HUD/intermission) from `DrawMenuPic` (320x200 menu/loading-plaque space), restoring classic status/intermission imagery without menu regressions
- [x] render menu text/cursor glyphs through menu-space scaling so text-only prompts (for example quit y/n) align with image-backed menu layout

**Done when**

- all visible menu entries lead to real functionality
- the HUD exposes the same gameplay-critical information the C engine does, including bounded intermission/finale/cutscene and deathmatch-scoreboard overlays

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
- [x] consume client skybox state on canonical OpenGL path and load external skyboxes from Quake FS search paths (`gfx/env/<name><suffix>.{png,tga,jpg}`): use cubemap upload for square same-size face sets and a dedicated per-face external sky path for loaded non-cubemap sets, with embedded BSP fallback only when no external faces load or upload fails
- [x] bounded post-38de7f3 parity slice: split particle rendering into explicit opaque/translucent subpasses so the top-level OpenGL frame path can separate those passes before the larger sky/water/translucency ordering refactor (with current runtime particles still landing in the opaque side)
- [x] bounded post-5800311 parity slice: split world/brush liquid surfaces into explicit opaque-liquid/translucent-liquid buckets in canonical OpenGL runtime and draw those buckets separately from general opaque/translucent world buckets (no full-frame reorder yet)
- [x] bounded post-aabf5ca parity slice: stage world + brush non-liquid buckets first, then alias/sprite + opaque particles, then world + brush liquid-only buckets in their own top-level frame step (while still deferring the broader sky/translucency/viewmodel reorder)
- [x] bounded post-e74f73b parity slice: split alias-model entity rendering into explicit opaque/translucent frame stages so fully opaque aliases stay before water and non-opaque aliases move later in the frame without changing the still-pending brush/sprite/translucency state behavior
- [x] bounded post-e061c6a parity slice: move the dedicated sky pass out of the early non-liquid world/brush stage and run it after opaque entities/particles but before liquid surfaces so the canonical OpenGL scene order is closer to C
- [x] bounded post-c2233bc parity slice: split opaque and translucent liquid world/brush draws into separate top-level frame stages so all opaque liquid work now precedes all translucent liquid work on the canonical OpenGL path
- [x] bounded post-46f3ca3 parity slice: split brush entities into opaque/translucent frame groups so translucent brush non-liquid work moves later in the frame while opaque brush work stays in the earlier entity/liquid stages
- [x] bounded post-f735cc1 parity slice: add an explicit late-frame translucency state block on the canonical OpenGL path, wrapping the translucent-liquid/entity/decal/particle stage and ending before viewmodel rendering
- [x] bounded post-19b917a parity slice: resolve runtime sprite-frame selection so `SPR_GROUP` sprites advance by client-time intervals and `SPR_ANGLED` sprites choose directional subframes from the current camera basis instead of time-stepping through those frames
- [x] bounded parity slice (`sprite-quad-orientation-fidelity`): pass runtime entity angles through sprite collection/render prep and mirror C `R_DrawSpriteModel_Real` quad-orientation behavior for `SPR_VP_PARALLEL_UPRIGHT`, `SPR_FACING_UPRIGHT`, `SPR_VP_PARALLEL`, `SPR_ORIENTED`, and `SPR_VP_PARALLEL_ORIENTED`
- [x] bounded parity slice (`sprite-pass-order-fidelity`): move runtime sprite draws out of the early opaque-entity block and into the explicit late translucency stage so sprite rendering is staged with the rest of late translucent entity content
- [x] bounded post-ef3bee6 parity slice (`viewmodel-origin-and-gating`): anchor runtime viewmodel origin to the active eye/view origin, suppress the viewmodel during intermission, and honor `r_drawviewmodel`-style visibility gating (including invisibility/death suppression) on the canonical runtime path
- [x] bounded parity slice (`fix-lightmap-block-artifacts`): route world + brush lightmap page uploads (and fallback lightmap texture) through a dedicated lightmap texture path using linear min/mag filtering while leaving generic world/sky/alias texture upload filtering unchanged
- [x] bounded parity slice (`lighting-diffuse-parity`): remove unintended world lightmap overbright scaling and feed per-surface dynamic-light accumulation into the canonical OpenGL world shader so diffuse lighting matches C behavior more closely without regressing linear-filter artifact fixes
- [x] bounded parity slice (`final-opengl-pass-order-and-viewmodel-placement`): keep the canonical OpenGL frame schedule aligned with the scoped C `R_RenderScene()` ordering by staging sky after opaque entities/particles, drawing opaque liquid before the late translucency block, and rendering the viewmodel after late translucent liquid/entity/decal/sprite/particle work via the dedicated viewmodel depth-range path
- [x] bounded parity slice (`transparent-water-vis-safety`): gate liquid translucency by map VIS/worldspawn compatibility (`transwater`/`watervis`) and force unsafe maps to opaque liquid fallback
- [x] bounded parity slice (`runtime-particle-pass-mode`): honor `r_particles` pass mode during runtime particle rendering so particles route through the opaque or late translucent pass instead of always resolving through opaque staging
- [x] bounded parity slice (`skybox-per-face-lowercase-fallback`): keep per-face non-cubemap skybox mode but retry lowercase file paths when mixed-case sky names fail against lowercase asset packs on case-sensitive filesystems
- [x] bounded parity slice (`sky-water-translucency-regressions`): add focused renderer regression coverage for transparent-water VIS safety, `r_particles` pass routing, and mixed-case per-face skybox load fallback

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
- [x] add local load/reconnect loading plaque visibility in the runtime overlay path (bounded host-managed state/timer, no remote connect work)
- [x] make sure menu/UI entry points reach the same save/load system
- [x] scan load/save slots for menu labels and fall back to legacy install-dir saves when the user save dir has no match

**Done when**

- a saved game restores the same gameplay-relevant state as the C engine
- unsupported save situations fail the same way the C engine does
- remaining legacy save import edge cases and remote connect/reconnect loading UX are still tracked separately as remaining parity work

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
- [x] stop active gameplay sounds during local disconnect/reconnect-style session transitions
- [x] add local reconnect loading-plaque visibility in runtime overlay flow
- [x] implement remote transport-backed `connect` flow (transport client now establishes remote sessions and auto-progresses signon via `prespawn`/`spawn`/`begin` replies)
- [x] connect the multiplayer menus to real behavior (join/host/setup dispatch live `connect`/host setup/hostname+player options through the same remote-capable host command path)

**Done when**

- remote server connection and basic multiplayer host control work from both console and menus

### 12. Only then decide how far to push gogpu parity

**Goal**

Keep the secondary renderer at a consciously bounded scope so it does not block or redefine the main parity path.

**Primary Go files**

- `internal/renderer/renderer_gogpu.go`

**Work**

- [x] explicit scope decision: `gogpu` is an **experimental, secondary backend** and is **not** a parity acceptance target
- expected support in this phase: keep the `gogpu` path buildable/runnable for bounded smoke usage (world draw + 2D overlay + particle fallback) without regressing the canonical OpenGL path
- explicitly out of scope in this phase: full render-fidelity parity, entity-rendering parity, or using `gogpu` behavior to judge overall port parity
- revisit deeper `gogpu` catch-up only after authoritative OpenGL parity milestones are met, via separately scoped follow-up planning

**Done when**

- the repo has a clear, honest statement about gogpu's role and non-goals
- secondary-backend notes no longer imply that `gogpu` is a parity gate

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
