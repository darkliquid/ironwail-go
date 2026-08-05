# Deep Subpackage Decomposition Plan

This document details an aggressive strategy to move files from the root directories of `internal/server`, `internal/renderer`, and `internal/game` directly into existing and new subpackages, bringing each root directory down to just a handful of core facade files.

---

## 1. `internal/server` Subpackage File Relocation

Move root server files into existing subpackages:

| Subpackage | Files to Move | Domain Responsibility |
| :--- | :--- | :--- |
| **`internal/server/debug`** | `debug_telemetry.go`, `debug_telemetry_test.go`, `debug_trigger.go`, `svdbg.go`, `svdbg_test.go` | Server telemetry, trigger logging, debug commands (`sv_debug_telemetry`). |
| **`internal/server/collision`** | `collision_system.go`, `world.go`, `world_math.go`, `world_leafs_test.go` | BSP world collision tree, leaf querying, and bounding box math. |
| **`internal/server/net`** | `network_manager.go`, `server_net_main.go`, `server_net_send.go`, `sv_client.go`, `sv_pvs.go`, `sv_stats.go`, `message.go`, `message_test.go` | Client networking, PVS visibility calculation, datagram serialization, packet dispatch. |
| **`internal/server/physics`** | `physics_system.go`, `server_physics.go`, `server_physics_loop.go`, `server_physics_walk.go`, `movement.go`, `movement_test.go` | Physics step simulation, monster pathfinding, unsticking, pusher movement. |
| **`internal/server/edict`** | `edict_compat.go` | Edict pool compatibility helpers. |
| **`internal/server/commands`** | `server_user_commands.go`, `user_spawn.go`, `spawn_parms.go`, `rules.go`, `rules_test.go`, `skill.go` | Server user commands (`noclip`, `god`, `kill`), spawn parameter parsing, game rules (coop/deathmatch). |
| **`internal/server/qc`** | `qc_fields.go`, `qc_trace.go`, `server_qc_sync.go`, `qc_trace_test.go`, `qcvm_regression_test.go` | QuakeC VM field offset maps, execution tracing, and Go/VM memory mirroring. |

---

## 2. `internal/renderer` Subpackage File Relocation

Move root renderer files into existing subpackages:

| Subpackage | Files to Move | Domain Responsibility |
| :--- | :--- | :--- |
| **`internal/renderer/alias`** | `renderer_alias_skin.go`, `renderer_alias_skin_test.go`, `renderer_alias_skin_variants.go`, `renderer_alias_skin_variants_test.go`, `renderer_alias_state.go`, `renderer_gogpu_world_alias.go` | Alias model pose interpolation, skin texture variants, and drawer execution. |
| **`internal/renderer/sky`** | `renderer_skybox_external.go`, `renderer_skybox_external_test.go`, `renderer_gogpu_world_external_sky.go` | Skybox loading, external sky textures, and sky rendering pass. |
| **`internal/renderer/overlay`** | `renderer_gogpu_overlay.go`, `renderer_gogpu_overlay_test.go`, `overlay_composite_gogpu.go` | CPU 2D overlay compositor, font glyph texture cache, and quad flusher. |
| **`internal/renderer/particle`** | `renderer_gogpu_particle.go`, `particle.go`, `particle_test.go` | GPU particle scratch buffer, texture atlas, and particle pipeline encoder. |
| **`internal/renderer/warpscale`** | `renderer_gogpu_warpscale.go`, `renderer_gogpu_warpscale_test.go`, `warpscale_shared.go` | Underwater sinusoidal waterwarp post-process pipeline. |
| **`internal/renderer/decal`** | `renderer_gogpu_world_decal.go`, `renderer_gogpu_world_decal_test.go`, `decal_shared.go`, `mark_system.go` | Decal mark system and bullet impact renderer. |
| **`internal/renderer/lightmap`** | `renderer_gogpu_world_lightmap.go`, `lightmap_samples.go`, `renderer_gogpu_world_dynamic_light_quant.go` | Dynamic light quantization, lightmap surface updates, sample expansion. |

---

## 3. `internal/game` Subpackage File Relocation

Move root game files into existing subpackages:

| Subpackage | Files to Move | Domain Responsibility |
| :--- | :--- | :--- |
| **`internal/game/camera`** | `game_camera.go`, `game_camera_chase.go`, `game_camera_viewcalc.go`, `game_view_state_camera_test.go`, `game_view_state_test.go` | Camera positioning (`viewcalc`), chase camera placement, scope zoom, underwater FOV modulation. |
| **`internal/game/commands`** | `game_commands.go`, `game_commands_bindings.go`, `game_commands_camdebug.go`, `game_commands_camdebug_test.go`, `game_commands_dispatch.go`, `game_commands_profile_test.go`, `game_commands_viewpoint.go`, `game_commands_viewpoint_test.go`, `perf_commands_test.go` | Console command dispatching (`exec`, `bind`, `unbind`), debug camera controls, viewpoint toggles. |
| **`internal/game/csqc`** | `game_runtime_csqc.go`, `game_runtime_csqc_test.go`, `csqc_input_test.go` | Client-Side QuakeC VM event dispatching (`CSQC_UpdateView`, `CSQC_InputEvent`). |
| **`internal/game/audio`** | `game_audio.go`, `game_audio_visual_sprites_test.go`, `game_audio_visual_test.go`, `game_music_test.go` | Sound effect precache tracking, spatial sound updates, music playback adapter. |
| **`internal/game/runtime`** | `game_runtime_frame.go`, `game_runtime_frame_parity_test.go`, `game_runtime_overlay.go`, `game_runtime_overlay_test.go`, `game_runtime_ui.go`, `game_runtime_ui_test.go`, `game_loop.go`, `game_loop_runtime_test.go` | Frame tick coordinator, overlay render triggers, UI event loops. |

---

## 4. Migration Execution Steps

1. **Step A (`internal/server` relocation)**: Move server debug, collision, net, physics, edict, commands, and qc files into subpackages.
2. **Step B (`internal/renderer` relocation)**: Move renderer alias, sky, overlay, particle, warpscale, decal, and lightmap files into subpackages.
3. **Step C (`internal/game` relocation)**: Move game camera, commands, csqc, audio, and runtime files into subpackages.
4. **Verification**: Run `mise run verify` after each step.
