# Subpackage Migration Plan for `server`, `renderer`, and `game`

Now that interface baseline contracts and dependency inversion have been established, this plan details the step-by-step extraction of remaining files from the monolithic root packages (`internal/server`, `internal/renderer`, `internal/game`) into dedicated, modular subpackages.

---

## 1. Goal & Architecture

Reduce top-level file counts in large packages down to 3-5 core facade files per package (`<pkg>.go`, `interfaces.go`, `doc.go`), moving domain logic into dedicated subpackages backed by interface dependency injection.

```
internal/
├── server/               (3-4 core facade files)
│   ├── collision/        (BSP spatial partitioning & area nodes)
│   ├── commands/         (Server user command handlers: noclip, god, kill, say)
│   ├── edict/            (Entity allocation & field parsing)
│   ├── net/              (Network datagrams, reliable packets, signon)
│   ├── physics/          (Per-entity physics simulation, step moves, unstick)
│   ├── qc/               (QuakeC VM sync & call tracing)
│   └── savegame/         (Save/load game state serialization)
├── renderer/             (3-4 core facade files)
│   ├── alias/            (Alias model interpolation & skin handling)
│   ├── decal/            (Decal marks & bullet impacts)
│   ├── lightmap/         (Lightmap allocation & page packing)
│   ├── oit/              (Weighted blended order-independent transparency)
│   ├── overlay/          (CPU 2D overlay compositor & font glyph cache)
│   ├── particle/         (Particle system GPU batching)
│   ├── pipeline/         (GPU device, queue, & render pipeline manager)
│   ├── scrap/            (Scrap texture packing)
│   ├── sky/              (Skybox loading & rendering)
│   ├── surface/          (Surface texture animation & lightstyles)
│   ├── warpscale/        (Underwater waterwarp sinusoidal FOV modulation)
│   └── world/            (BSP world geometry building)
│       └── gogpu/        (WebGPU world pipelines & pass encoders)
└── game/                 (3-4 core facade files)
    ├── audio/            (Audio adapter & sound effect management)
    ├── camera/           (Camera view calculations, chase cam, FOV zoom)
    ├── commands/         (Console command dispatchers & keybindings)
    ├── csqc/             (Client-Side QuakeC VM runtime hooks)
    ├── runtime/          (Frame tick dispatchers, overlays, & game loop)
    └── ui/               (Menu, HUD, 2D drawing, & UI overlays)
```

---

## 2. Package `internal/server` Migration Plan

### Target Subpackages & File Mappings

| Target Subpackage | Source Files | Extracted Domain Responsibility |
| :--- | :--- | :--- |
| **`internal/server/net`** | `sv_send.go`, `sv_main.go`, `message.go` | Network datagram building, entity delta serialization, reliable packet dispatch. |
| **`internal/server/commands`** | `user.go` | Server user command handlers (`noclip`, `god`, `kill`, `say`, `give`, `setpos`, `prespawn`). |
| **`internal/server/qc`** | `server_qc_sync.go`, `qc_trace.go` | QC VM memory sync (`syncQCVMState`, `syncQCVMGlobals`), call tracing, and field def mappings. |
| **`internal/server/physics`** | `server_physics.go`, `server_physics_loop.go`, `server_physics_walk.go` | Frame physics loop (`Physics()`, `SV_Physics()`), step movement, unsticking, pushers. |

### Core Root Files Remaining in `internal/server` (Facade)
- `server.go`: `Server` struct definition, constructor, and top-level lifecycle loops (`Frame`, `SpawnServer`).
- `interfaces.go`: Static interface assertions and getters.
- `interfaces_test.go`: Compile-time satisfaction tests.
- `doc.go`: Subsystem lineage documentation.

---

## 3. Package `internal/renderer` Migration Plan

### Target Subpackages & File Mappings

| Target Subpackage | Source Files | Extracted Domain Responsibility |
| :--- | :--- | :--- |
| **`internal/renderer/world/gogpu`** | `renderer_gogpu_world_render.go`, `renderer_gogpu_world_pipelines.go`, `renderer_gogpu_world_resources.go`, `renderer_gogpu_world_shaders.go`, `renderer_gogpu_world_upload.go`, `renderer_gogpu_world_brush_render.go`, `renderer_gogpu_world_decal.go`, `renderer_gogpu_world_sprite.go`, `renderer_gogpu_world_translucent.go`, `renderer_gogpu_world_depth.go` | WebGPU BSP world pipelines, render pass encoders, lightmap page bindings, translucent liquid sorting, skybox passes. |
| **`internal/renderer/alias`** | `renderer_gogpu_world_alias.go`, `alias_skin.go`, `alias_state.go` | Alias model lerp state tracking, skin texture upload, and drawer execution. |
| **`internal/renderer/overlay`** | `renderer_gogpu_overlay.go` | CPU-side 2D overlay quad compositor, font glyph texture cache, 2D quad flush. |

### Core Root Files Remaining in `internal/renderer` (Facade)
- `renderer_gogpu.go`: `Renderer` struct definition and main window/event loop.
- `types.go`: `RenderContext` and `Backend` interfaces.
- `interfaces.go`: Core subpackage interface contracts (`WorldPipeline`, `AliasBatcher`, `Compositor2D`, `GPUContext`).
- `doc.go`: Package lineage documentation.

---

## 4. Package `internal/game` Migration Plan

### Target Subpackages & File Mappings

| Target Subpackage | Source Files | Extracted Domain Responsibility |
| :--- | :--- | :--- |
| **`internal/game/camera`** | `game_camera.go`, `game_camera_chase.go`, `game_camera_viewcalc.go`, `game_view_state.go` | Camera positioning (`viewcalc`), chase cam placement, scope zoom, underwater FOV modulation. |
| **`internal/game/commands`** | `game_commands.go`, `game_commands_dispatch.go`, `game_commands_bindings.go`, `game_commands_camdebug.go`, `game_commands_viewpoint.go` | Console command dispatching (`exec`, `bind`, `unbind`), debug camera controls, viewpoint toggles. |
| **`internal/game/csqc`** | `game_runtime_csqc.go`, `csqc_hooks.go` | Client-Side QuakeC VM event dispatching (`CSQC_UpdateView`, `CSQC_InputEvent`). |
| **`internal/game/audio`** | `game_audio.go` | Sound effect precache tracking, spatial sound updates, music playback adapter. |
| **`internal/game/runtime`** | `game_runtime_frame.go`, `game_runtime_overlay.go`, `game_runtime_ui.go`, `game_loop.go` | Frame tick coordinator, overlay render triggers, UI event loops. |

### Core Root Files Remaining in `internal/game` (Facade)
- `game.go`: `Game` struct definition, constructor, and main run loop.
- `interfaces.go`: Subpackage interface contracts (`SessionManager`, `CameraSystem`, `AssetCache`, `UIController`).
- `doc.go`: Package lineage documentation.

---

## 5. Execution Steps

1. **Server Subpackages Phase**: Migrate `server/net`, `server/commands`, `server/qc`, and finish `server/physics`.
2. **Renderer Subpackages Phase**: Migrate `renderer/world/gogpu`, `renderer/alias`, and `renderer/overlay`.
3. **Game Subpackages Phase**: Create and migrate `game/camera`, `game/commands`, `game/csqc`, `game/audio`, and `game/runtime`.
4. **Verification**: Run `mise run verify` after each phase.
