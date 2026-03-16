# Ironwail-Go Parity Audit Table

Source-backed audit comparing `darkliquid/ironwail-go` against `andrei-drexler/ironwail`.

**Rules:**
- Parity target: **CGO/OpenGL runtime** (`renderer_opengl.go` + GLFW + SDL3/Oto)
- **gogpu/WebGPU** is a secondary backend, not a parity gate — see its own section below
- When this doc and source code disagree, **source code wins**
- Canonical docs: [`PORT_PARITY_REVIEW.md`](PORT_PARITY_REVIEW.md) · [`PORT_PARITY_TODO.md`](PORT_PARITY_TODO.md)

**Status vocabulary:**
`Implemented` · `Mostly implemented` · `Partial` · `Missing` · `Non-parity backend`

**Gap type vocabulary:**
`Missing feature` · `Incomplete integration` · `Behavioral inaccuracy` · `UX/product gap` · `Backend divergence` · `Historical doc stale`

---

## Renderer: baseline and pipeline

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Renderer baseline | CGO/OpenGL as authoritative runtime | Implemented | `internal/renderer/renderer_opengl.go`; `PORT_PARITY_REVIEW.md §2.1` | — | — |
| Renderer baseline | gogpu/WebGPU as secondary/experimental | Non-parity backend | `PORT_PARITY_REVIEW.md §2.3`; `PORT_PARITY_TODO.md §12` | Backend divergence | Low (intentional) |
| Scene ordering | `R_RenderScene()` pass order: setup → clear → fog → non-alpha entities → opaque particles → sky → opaque water → begin translucency → translucent water → alpha entities → alpha particles → end translucency → viewmodel | Implemented | `PORT_PARITY_REVIEW.md §2.2`; `internal/renderer/world_runtime_opengl.go` | — | — |
| Render frame loop | `S_ExtraUpdate()` integration between render stages | Mostly implemented | Called in runtime loop; exact per-stage placement vs C may differ | Behavioral inaccuracy | Low |

---

## World rendering

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| BSP geometry | Extract, bin, and depth-sort BSP surfaces | Implemented | `internal/renderer/world.go`; `BuildWorldGeometry()` | — | — |
| Lightmaps | Lightmap allocation, atlas upload, filtering | Implemented | `PORT_PARITY_REVIEW.md §2.1`; linear min/mag filtering for lightmap pages | — | — |
| Lightmap updates | Per-frame lightstyle evaluation and lightmap re-upload | Implemented | `setLightStyleValues()`, `updateUploadedLightmapsLocked()` | — | — |
| Lightmap filtering | Linear min/mag for lightmap textures vs nearest for world/sky textures | Implemented | `PORT_PARITY_TODO.md §8 (fix-lightmap-block-artifacts slice)` | — | — |
| Lightmap overbright | No unintended overbright scaling on world diffuse | Implemented | `PORT_PARITY_TODO.md §8 (lighting-diffuse-parity slice)` | — | — |
| Animated textures | `R_TextureAnimation()` — alternate anim chains, `anim_total`, cycle walk | Implemented | `internal/renderer/surface.go:TextureAnimation()`; broken/infinite cycle detection | — | — |
| Turbulent UV warp | Time-varying UV displacement for `SurfDrawTurb` surfaces | Implemented | `PORT_PARITY_REVIEW.md §2.2`; `internal/renderer/world_runtime_opengl.go` | — | — |
| Fog | Live client fog state consumed by world shader | Implemented | `PORT_PARITY_REVIEW.md §2.2` | — | — |
| Visual polish | Broader world visual fidelity beyond bounded slices | Mostly implemented | `PORT_PARITY_REVIEW.md §2.2` notes remaining broader divergences | Behavioral inaccuracy | Medium |

---

## Sky / skybox

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Embedded BSP sky | Dedicated animated two-layer sky shader (not ordinary world shader) | Implemented | `PORT_PARITY_TODO.md §8`; dedicated sky stage after opaque entities/particles | — | — |
| Sky fog mix | `r_skyfog` + worldspawn `skyfog` semantics gated by fog density | Implemented | `PORT_PARITY_TODO.md §8` | — | — |
| External skybox (cubemap) | Load and render square same-size face sets as a cubemap | Implemented | `PORT_PARITY_REVIEW.md §2.2`; `gfx/env/<name><suffix>.{png,tga,jpg}` | — | — |
| External skybox (per-face) | Non-cubemap per-face external sky path with embedded BSP fallback | Implemented | `PORT_PARITY_REVIEW.md §2.2`; per-face path used when faces load without forming a cubemap | — | — |
| Skybox mixed-case fallback | Lowercase path retry for mixed-case skybox names on case-sensitive FS | Implemented | `PORT_PARITY_TODO.md §8 (skybox-per-face-lowercase-fallback slice)` | — | — |
| Sky pass ordering | Sky renders after opaque entities/particles, before liquid | Implemented | `PORT_PARITY_TODO.md §8 (post-e061c6a slice)` | — | — |
| Sky edge-case polish | Additional non-cubemap content packs, further fidelity validation | Mostly implemented | `PORT_PARITY_REVIEW.md §2.2` — keep validating additional packs | Behavioral inaccuracy | Low |

---

## Water / turbulence / liquid rendering

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Liquid surface bucketing | Separate opaque-liquid / translucent-liquid bins | Implemented | `PORT_PARITY_TODO.md §8 (post-5800311 / post-c2233bc slices)` | — | — |
| Liquid pass ordering | Opaque liquid before translucent liquid, all after opaque entities/sky | Implemented | `PORT_PARITY_TODO.md §8 (final-opengl-pass-order slice)` | — | — |
| Transparent water VIS safety | Gate liquid translucency by `transwater`/`watervis` worldspawn; force opaque on unsafe maps | Implemented | `PORT_PARITY_TODO.md §8 (transparent-water-vis-safety slice)` | — | — |
| Underwater visual warp | Screen-space view distortion (`r_waterwarp`, C `R_WarpScaleView()`) | Missing | Audio intensity is wired; visual blue-shift/warp not yet implemented | Missing feature | Medium |

---

## Entity rendering

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Brush entities | Brush submodel rendering with origin + rotation | Implemented | `renderBrushEntities()`; brush-entity rotation parity landed | — | — |
| Brush entity translucency | Translucent brush non-liquid surfaces move to late stage | Implemented | `PORT_PARITY_TODO.md §8 (post-46f3ca3 slice)` | — | — |
| Alias models | Pose interpolation, skin textures, alpha/scale, muzzle/bright/dim effect lights | Implemented | `renderAliasEntities()`; `PORT_PARITY_REVIEW.md §2.1` | — | — |
| Alias model pass split | Opaque aliases before water; non-opaque aliases in late translucent stage | Implemented | `PORT_PARITY_TODO.md §8 (post-e74f73b slice)` | — | — |
| Sprites | Billboard rendering, pass-order, frame selection | Implemented | `renderSpriteEntities()`; sprite-quad-orientation and sprite-pass-order slices landed | — | — |
| Sprite frame selection | `SPR_GROUP` interval timing; `SPR_ANGLED` directional subframes | Implemented | `PORT_PARITY_TODO.md §8 (post-19b917a slice)` | — | — |
| Sprite quad orientation | `SPR_VP_PARALLEL_UPRIGHT` / `SPR_FACING_UPRIGHT` / `SPR_VP_PARALLEL` / `SPR_ORIENTED` / `SPR_VP_PARALLEL_ORIENTED` | Implemented | `PORT_PARITY_TODO.md §8 (sprite-quad-orientation-fidelity slice)` | — | — |
| Particles | Opaque/translucent pass split; `r_particles` mode routing | Implemented | `renderParticles()`; `PORT_PARITY_TODO.md §8 (runtime-particle-pass-mode slice)` | — | — |
| Particle visual fidelity | Full visual polish matching C particle appearance | Mostly implemented | `PORT_PARITY_REVIEW.md §2.2` — broader polish remains | Behavioral inaccuracy | Medium |
| Decals | Projected decal rendering | Implemented | `renderDecalMarks()`; `DecalMarkSystem` wired from live client state | — | — |
| Dynamic lights | Protocol-driven temp entity + effect lights | Implemented | `PORT_PARITY_TODO.md §7`; effect flags, quad/penta mapped | — | — |
| Viewmodel | Eye-origin anchor, `r_drawviewmodel`, intermission/invisibility/death gating | Implemented | `PORT_PARITY_TODO.md §8 (viewmodel-origin-and-gating slice)`; dedicated post-translucency depth-range pass | — | — |
| Late translucency block | Explicit begin/end translucency state wrapping the late entity/decal/particle/liquid stage | Implemented | `PORT_PARITY_TODO.md §8 (post-f735cc1 slice)` | — | — |

---

## gogpu backend gaps

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| gogpu entity rendering | Full alias/sprite/brush/decal/viewmodel pipeline | Missing | `PORT_PARITY_REVIEW.md §2.3` — still entity-marker stub | Backend divergence | Low (secondary backend) |
| gogpu particle rendering | Full world-space particle parity | Missing | Simplified 2D fallback only | Backend divergence | Low (secondary backend) |
| gogpu scene composition | Parity-complete gameplay renderer | Missing | Backend-specific hacks preserve world draw; not a parity-complete path | Backend divergence | Low (secondary backend) |
| gogpu world rendering | Basic world draw + 2D overlay | Partial | World path + 2D overlay + particle fallback are present | Backend divergence | Low (secondary backend) |

---

## Input / bindings / config / console

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Key bindings | `bind`/`unbind`/`unbindall`/`bindlist` with Quake `KButton` model | Implemented | `internal/input/types.go`; `PORT_PARITY_REVIEW.md §3` | — | — |
| Config persistence | `config.cfg` saves/restores bindings and archived cvars on restart | Implemented | `Host.WriteConfig()` with deterministic bind/cvar ordering | — | — |
| Console UI | Text buffer, scrollback, history, notify lines, debug logging | Implemented | `internal/console/console.go`; `PORT_PARITY_REVIEW.md §5` | — | — |
| Console tab completion | Commands, cvars, and aliases | Implemented | `PORT_PARITY_REVIEW.md §5` | — | — |
| Command aliases | `alias`/`unalias`/`unaliasall`; command-over-alias execution precedence | Implemented | `PORT_PARITY_REVIEW.md §5` | — | — |
| Menu key routing | Key-down only, no double-fire on key-up release | Implemented | `PORT_PARITY_TODO.md §6` | — | — |
| Bind/config long-tail | Extra trailing state commands in `WriteConfigurationToFile()` (e.g. `+mlook`) | Mostly implemented | `PORT_PARITY_REVIEW.md §5` — trailing state commands depend on features not yet fully exposed | Incomplete integration | Low |

---

## Menus / controls / multiplayer setup / mods menu / mouse UI / menu polish

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Main menu flow | Main, load, save, help, options, quit | Implemented | `internal/menu/manager.go` | — | — |
| Options submenus | Video, audio, controls (with live cvars + bind editing) | Mostly implemented | `PORT_PARITY_REVIEW.md §5` | — | — |
| Multiplayer setup | Hostname + player name + shirt/pants colors + accept | Implemented | `PORT_PARITY_TODO.md §6`; syncs from live `hostname`/`_cl_name`/`_cl_color` cvars | — | — |
| Multiplayer join/host | Menu-driven join + host with live command dispatch | Mostly implemented | Bounded `PORT_PARITY_TODO.md §11`; broader remote netgame depth still pending | Incomplete integration | Medium |
| Player setup preview | Translated player preview / text-box art as in C | Partial | Simplified color-swatch preview used; C-style translated art not yet ported | UX/product gap | Low |
| Mods menu | Ironwail-style installed add-on browser | Missing | Not found in `internal/menu/manager.go`; Ironwail advertises this as a product feature | Missing feature | Medium |
| Mouse-driven UI | Mouse movement in menus (`M_Mousemove` equivalent) | Partial | Menu key routing is functional; comprehensive mouse-driven menu navigation is incomplete | UX/product gap | Medium |
| Weapon bind UI | Ironwail richer weapon key-binding UI in menus | Partial | Base bind editing exists; Ironwail-specific weapon bind UI not fully matched | UX/product gap | Low |
| Broader menu polish | Full C menu behavior, C-style UX details | Mostly implemented | `PORT_PARITY_REVIEW.md §5` — some C-polish details still missing | UX/product gap | Low |
| Menu audio feedback | `misc/menu*.wav` on navigation/accept/cancel | Implemented | `PORT_PARITY_TODO.md §6` | — | — |
| Menu text scaling | Text-only prompts use same 320×200 scaling as image-backed menus | Implemented | `PORT_PARITY_TODO.md §6` | — | — |

---

## HUD / intermission / scoreboard / alternate HUD styles

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Classic status bar | Weapon strip, ammo strip, keys/powerups/sigils, armor/face/ammo icons | Implemented | `internal/hud/status.go`; `PORT_PARITY_TODO.md §6` | — | — |
| Intermission overlay | `gfx/complete.lmp` + `gfx/inter.lmp` stats + `gfx/finale.lmp` + timed center-text | Implemented | `PORT_PARITY_TODO.md §6` | — | — |
| Deathmatch scoreboard | `+showscores` hold, ranked frag rows, multiplayer mini-frag strip | Implemented | `PORT_PARITY_TODO.md §6` | — | — |
| DrawPic / DrawMenuPic split | Screen-space HUD vs 320×200 menu/loading-plaque coordinate spaces | Implemented | `PORT_PARITY_TODO.md §6` | — | — |
| Expansion-pack HUD variants | Special-case overlays for missionpack/episode content | Partial | `PORT_PARITY_REVIEW.md §5` — expansion-pack special-casing not yet ported | Missing feature | Low |
| Alternate HUD styles (Q64) | Ironwail-specific Q64-layout HUD mode | Missing | `PORT_PARITY_REVIEW.md §5`; Ironwail advertises alternate HUD styles | UX/product gap | Medium |
| Pickup flash timing | Item pickup animation timing polish | Mostly implemented | `PORT_PARITY_REVIEW.md §5` notes this as remaining polish | Behavioral inaccuracy | Low |

---

## Audio / music / CD tracks / fidelity

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Sound system init | Backend selection (SDL3 → Oto → NullBackend) | Implemented | `internal/audio/backend.go` | — | — |
| Sound dispatch | `svc_sound` → `S_StartSound()` with entity/channel/index/volume/attenuation | Implemented | `PORT_PARITY_REVIEW.md §4`; `PORT_PARITY_TODO.md §3` | — | — |
| Stop sound | `svc_stopsound` dispatch to `S_StopSound()` | Implemented | `PORT_PARITY_TODO.md §3` | — | — |
| Static sounds | World persistent sound channels rebuilt on precache snapshot changes | Implemented | `PORT_PARITY_TODO.md §3` | — | — |
| Listener updates | Camera origin + orientation fed to spatializer each frame | Implemented | `PORT_PARITY_TODO.md §3` | — | — |
| View-entity spatialization | Active `ViewEntity` routes to full-volume; identical static loops combined | Implemented | `PORT_PARITY_REVIEW.md §4`; `audio_test.go` coverage | — | — |
| Ambient sounds | BSP leaf ambient fade + underwater intensity | Implemented | `PORT_PARITY_REVIEW.md §4`; `UpdateAmbientSounds()` | — | — |
| Session-transition teardown | `StopAllSounds(true)` on disconnect/reconnect/load/map transitions | Implemented | `PORT_PARITY_TODO.md §9` | — | — |
| CD track / music | WAV/OGG CD-track playback with `CDTrack`/`LoopTrack` change | Mostly implemented | `PORT_PARITY_TODO.md §3`; bounded search-path parity | — | — |
| Sound packet encoding | `SND_LARGEENTITY`/`SND_LARGESOUND` edge-case packets from server | Implemented | `PORT_PARITY_REVIEW.md §4`; `server_test.go` coverage | — | — |
| Broader codec parity | Full `bgmusic.c` codec breadth (OGG beyond CD tracks, etc.) | Partial | `PORT_PARITY_REVIEW.md §4` — broader codec parity still missing | Incomplete integration | Medium |
| Underwater visual blue-shift | Screen visual effect when camera is underwater | Missing | Audio intensity wired; visual coupling remains open | Missing feature | Medium |
| Music/BGM fidelity | Full C `bgmusic.c` behavior, track-control polish | Mostly implemented | `PORT_PARITY_REVIEW.md §4` — broader codec breadth and track-control polish remain | Behavioral inaccuracy | Medium |

---

## Client parsing / prediction

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Server message parsing | Broad SVC parsing: entity updates, lightstyles, temp entities, static sounds, fog, skybox | Implemented | `internal/client/parse.go`; `PORT_PARITY_REVIEW.md §3` | — | — |
| `svc_clientdata` viewheight | Parsed and applied to runtime camera origin | Implemented | `PORT_PARITY_TODO.md §2` | — | — |
| `svc_clientdata` punch angles | Runtime camera kick + `v_gunkick` 0/1/2 interpolation | Implemented | `PORT_PARITY_TODO.md §2` | — | — |
| Client prediction | `PredictPlayers()` called every frame | Implemented | `PORT_PARITY_TODO.md §2` | — | — |
| Authoritative camera origin | Authoritative server entity origin first; predicted fallback only when unavailable | Implemented | `PORT_PARITY_TODO.md §2` | — | — |
| Command accumulation | Per-frame command accumulation parity (`AccumulateCmd()`) | Implemented | `PORT_PARITY_TODO.md §2` | — | — |
| Transient event consumption | One per-frame place: `ConsumeTransientEvents()` in `runRuntimeFrame()` | Implemented | `PORT_PARITY_TODO.md §2`; covered by test | — | — |
| Movement feel long-tail | Deeper edge-case movement feel tuning beyond bounded walk/gravity slices | Mostly implemented | `PORT_PARITY_REVIEW.md §3` — bounded slices landed, long-tail polish remains | Behavioral inaccuracy | Medium |

---

## Local loopback vs remote networking

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Local loopback | Full local single-player session lifecycle | Implemented | `PORT_PARITY_REVIEW.md §1`; passes `TestCmdMapStartRealAssetsReachesCaActive` | — | — |
| Disconnect | Session teardown including stop-all sounds | Implemented | `PORT_PARITY_TODO.md §11` | — | — |
| Reconnect | Signon reset/restart + loading plaque (local + remote) | Implemented | `PORT_PARITY_TODO.md §11` | — | — |
| Kick | Name or slot-number targeting + optional message + self-protection | Implemented | `PORT_PARITY_TODO.md §11` | — | — |
| Remote connect | Transport-backed remote sessions + auto-progress signon | Implemented | `PORT_PARITY_TODO.md §11`; `PORT_PARITY_REVIEW.md §7` | — | — |
| Remote netgame depth | Full C netgame breadth (game options, server list, deathmatch polish, etc.) | Partial | `PORT_PARITY_REVIEW.md §7` — broader netgame depth still incomplete | Incomplete integration | High |
| Deathmatch rules | `fraglimit`/`timelimit` checks + delayed respawn flow | Mostly implemented | `PORT_PARITY_TODO.md §11` bounded slice | Incomplete integration | Medium |
| Multiplayer server browser / list | Ironwail server list / LAN scan UI | Partial | Menu join/host flows wired; deeper network UI not fully matched | UX/product gap | Medium |

---

## Save / load

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Save command | Map, time, paused, flags, precaches, static entities/sounds, client parms, edicts, QC globals | Implemented | `PORT_PARITY_REVIEW.md §6`; `HOST_PARITY_TODO.md §9` | — | — |
| Save restrictions | Reject when: no active local game, `nomonsters`, intermission, multiplayer, dead player | Implemented | `PORT_PARITY_REVIEW.md §6` | — | — |
| Load command | Restore edicts, relink world, sync QC VM, restore globals, lightstyles, signon re-entry | Implemented | `PORT_PARITY_REVIEW.md §6` | — | — |
| Gameplay-state restore | Ammo/weapon/items/armor, server flags, pause/time, client spawn parms | Implemented | `PORT_PARITY_TODO.md §9`; covered by `TestSaveGameStateRoundTripsGameplayState` | — | — |
| Loading plaque | Visible on local load/reconnect + remote reconnect with failsafe timeout | Implemented | `PORT_PARITY_TODO.md §9` | — | — |
| Broader save-file search | C engine search outside active game dir in some cases | Partial | `PORT_PARITY_REVIEW.md §6` — broader search behavior still missing | Missing feature | Low |
| Legacy save import | Import saves from install dir when user dir has no match | Partial | `PORT_PARITY_TODO.md §9` — scan/fallback wired; legacy edge cases remain | Incomplete integration | Low |
| Transition UX polish | Longer-tail session/audio/plaque parity on unusual transition paths | Mostly implemented | `PORT_PARITY_REVIEW.md §6` — some long-tail polish remains | Behavioral inaccuracy | Low |

---

## Demos

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| Demo recording | Write connected-state snapshot + live gameplay frames + disconnect trailer | Implemented | `PORT_PARITY_TODO.md §10`; `internal/client/demo.go` | — | — |
| Demo playback | One frame per host frame; flush stufftext in-frame; honor pause; pace by recorded server time | Implemented | `PORT_PARITY_TODO.md §10`; `PORT_PARITY_REVIEW.md §3` | — | — |
| `timedemo` / rewind | Bounded timing benchmark and rewind tooling | Implemented | `PORT_PARITY_TODO.md §10` | — | — |
| Demo long-tail accuracy | Full edge-case / benchmark / metadata accuracy vs C `cl_demo.c` | Mostly implemented | `PORT_PARITY_TODO.md §10` — deeper C demo tooling polish remains | Behavioral inaccuracy | Low |

---

## Ironwail-specific performance / product-surface differences

| Subsystem | Ironwail feature / expectation | Go status | Evidence / notes | Gap type | Priority |
|---|---|---|---|---|---|
| GPU performance architecture | Compute shaders, instancing, persistent buffer mapping, indirect multi-draw, bindless textures | Missing | Go port prioritizes readability/portability over GPU-heavy optimization | Backend divergence | Low (intentional) |
| Decoupled renderer/server timing | Ironwail decouples renderer timing from server tick for smoother visuals | Partial | Not a current parity priority; Go port uses simpler frame loop | Backend divergence | Low |
| Mods menu / add-on browser | Ironwail new Mods menu for installed add-ons | Missing | Not present in Go menu system | Missing feature | Medium |

---

## Historical doc accuracy

| Doc | Status | Notes |
|---|---|---|
| `PORT_PARITY_REVIEW.md` | Current | Canonical current baseline; source-backed |
| `PORT_PARITY_TODO.md` | Current | Active ordered backlog with done/open markers |
| `PARITY_AUDIT_TABLE.md` | Current | This document |
| `PARITY_SUMMARY.md` | Current | Rewritten executive summary |
| `QUICK_REFERENCE.txt` | Current | Rewritten operational card |
| `PARITY_ANALYSIS_INDEX.md` | Current | Rewritten index with current/historical separation |
| `parity_report.md` | Historical snapshot only | Early 2026-03 audit; many gaps since resolved; see warning banner |
