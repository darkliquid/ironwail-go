# Aggressive Subpackage Extraction Plan

This plan details how we aggressively migrate groups of prefixed files (`renderer_gogpu_world_*`, `game_camera_*`, `game_commands_*`, `game_runtime_*`, `server_physics_*`, etc.) directly into target subpackage directories, converting top-level receiver methods into standalone subpackage component types.

---

## 1. Architectural Strategy: Prefix-to-Subpackage Mapping

Instead of keeping prefixed files in root directories, each prefix group will move to a dedicated subpackage directory:

```
internal/
├── server/
│   ├── physics/         <-- server_physics_* (frame physics loop, walkmove, unstick)
│   ├── net/             <-- server_net_* (datagrams, signon, multicast)
│   ├── qc/              <-- server_qc_* (QC VM memory sync & call tracing)
│   └── commands/        <-- server_user_commands.go (noclip, god, kill, say)
├── renderer/
│   ├── world/
│   │   └── gogpu/       <-- renderer_gogpu_world_* (WebGPU world pipeline, passes, atlas)
│   ├── alias/           <-- renderer_alias_* (Alias model lerp, skins, state)
│   ├── overlay/         <-- renderer_gogpu_overlay.go (CPU 2D overlay compositor)
│   ├── particle/        <-- renderer_gogpu_particle.go (GPU particle batcher)
│   └── warpscale/       <-- renderer_gogpu_warpscale.go (sinusoidal underwater warp)
└── game/
    ├── camera/          <-- game_camera_* (viewcalc, chase camera, zoom state)
    ├── commands/        <-- game_commands_* (console commands, bindings, debug cam)
    ├── runtime/         <-- game_runtime_* (frame tick, overlays, game loop)
    ├── audio/           <-- game_audio.go (sound precache & spatial audio)
    └── csqc/            <-- game_runtime_csqc.go (CSQC VM event hooks)
```

---

## 2. Plan for `internal/server` Prefixes

### Prefix 1: `server_physics_*` $\rightarrow$ [`internal/server/physics`](file:///home/darkliquid/Projects/ironwail-go/internal/server/physics)
- **Source Files**: `server_physics.go`, `server_physics_loop.go`, `server_physics_walk.go`
- **Target**: Move frame physics simulation (`Physics()`, `SV_Physics()`, `SV_Physics_Client()`) and monster walkmove into `physics.System`.
- **Interface & Wiring**: `Server` delegates physics loop execution to `s.PhysicsSys` via `srvtypes.PhysicsEngine`.

### Prefix 2: `server_net_*` / `message.go` $\rightarrow$ [`internal/server/net`](file:///home/darkliquid/Projects/ironwail-go/internal/server/net)
- **Source Files**: `server_net_send.go`, `server_net_main.go`, `message.go`
- **Target**: Move datagram assembly, entity delta serialization (`WriteEntitiesToClient`), and reliable packet dispatch into `net.NetworkManager`.
- **Interface & Wiring**: `Server` delegates network encoding to `s.NetManager` via `srvtypes.NetworkBroadcaster`.

### Prefix 3: `server_qc_*` $\rightarrow$ `internal/server/qc`
- **Source Files**: `server_qc_sync.go`, `qc_trace.go`
- **Target**: Move QuakeC VM state synchronization (`syncQCVMState`, `syncQCVMGlobals`) and call tracing into `qc.Binding`.
- **Interface & Wiring**: `Server` delegates VM sync to `s.QC` via `srvtypes.QCExecutor`.

### Prefix 4: `server_user_commands.go` $\rightarrow$ `internal/server/commands`
- **Source Files**: `server_user_commands.go`
- **Target**: Move server user command handlers (`noclip`, `god`, `kill`, `say`, `give`, `setpos`, `prespawn`) into `commands.Handler`.
- **Interface & Wiring**: Handlers accept `srvtypes.EntityStore` and `srvtypes.PhysicsConfig`.

---

## 3. Plan for `internal/renderer` Prefixes

### Prefix 1: `renderer_gogpu_world_*` $\rightarrow$ [`internal/renderer/world/gogpu`](file:///home/darkliquid/Projects/ironwail-go/internal/renderer/world/gogpu)
- **Source Files**: `renderer_gogpu_world_render.go`, `renderer_gogpu_world_pipelines.go`, `renderer_gogpu_world_resources.go`, `renderer_gogpu_world_shaders.go`, `renderer_gogpu_world_upload.go`, `renderer_gogpu_world_brush_render.go`, `renderer_gogpu_world_decal.go`, `renderer_gogpu_world_sprite.go`, `renderer_gogpu_world_translucent.go`, `renderer_gogpu_world_depth.go`
- **Target**: Move WebGPU BSP world pipelines, buffer uploads, lightmaps, decals, and render pass encoders into `worldgogpu.Pipeline`.
- **Interface & Wiring**: `Renderer` delegates world rendering to `worldgogpu.Pipeline` via `WorldPipeline` interface (`UploadWorld`, `DrawWorldOpaque`, `DrawWorldTranslucent`).

### Prefix 2: `renderer_alias_*` $\rightarrow$ [`internal/renderer/alias`](file:///home/darkliquid/Projects/ironwail-go/internal/renderer/alias)
- **Source Files**: `renderer_alias_skin.go`, `renderer_alias_skin_variants.go`, `renderer_alias_state.go`, `renderer_gogpu_world_alias.go`
- **Target**: Move Alias model pose interpolation, skin texture cache, and drawer execution into `alias.Batcher`.
- **Interface & Wiring**: `Renderer` delegates alias rendering to `alias.Batcher` via `AliasBatcher` interface (`PrepareAliasDraws`, `DrawAliasModels`).

### Prefix 3: `renderer_gogpu_overlay*` $\rightarrow$ `internal/renderer/overlay`
- **Source Files**: `renderer_gogpu_overlay.go`
- **Target**: Move CPU 2D overlay compositor, font glyph texture cache, and quad flusher into `overlay.Compositor2D`.
- **Interface & Wiring**: `Renderer` delegates 2D compositing via `Compositor2D` interface (`DrawPic`, `DrawCharacter`, `DrawString`, `FlushOverlay`).

### Prefix 4: `renderer_gogpu_particle.go` $\rightarrow$ [`internal/renderer/particle`](file:///home/darkliquid/Projects/ironwail-go/internal/renderer/particle)
- **Source Files**: `renderer_gogpu_particle.go`
- **Target**: Move GPU particle scratch buffer and pipeline encoder into `particle.System`.

### Prefix 5: `renderer_gogpu_warpscale.go` $\rightarrow$ [`internal/renderer/warpscale`](file:///home/darkliquid/Projects/ironwail-go/internal/renderer/warpscale)
- **Source Files**: `renderer_gogpu_warpscale.go`
- **Target**: Move sinusoidal underwater waterwarp post-process pipeline into `warpscale.Pipeline`.

---

## 4. Plan for `internal/game` Prefixes

### Prefix 1: `game_camera_*` $\rightarrow$ `internal/game/camera`
- **Source Files**: `game_camera.go`, `game_camera_chase.go`, `game_camera_viewcalc.go`, `game_view_state.go`
- **Target**: Move view matrix calculation (`viewcalc`), chase camera positioning, FOV waterwarp modulation, and scope zoom state into `camera.System`.
- **Interface & Wiring**: `Game` delegates camera management via `CameraSystem` interface (`ComputeView`, `UpdateZoom`).

### Prefix 2: `game_commands_*` $\rightarrow$ `internal/game/commands`
- **Source Files**: `game_commands.go`, `game_commands_dispatch.go`, `game_commands_bindings.go`, `game_commands_camdebug.go`, `game_commands_viewpoint.go`
- **Target**: Move console command dispatchers (`exec`, `bind`, `unbind`), debug camera controls, and viewpoint toggles into `commands.Dispatcher`.

### Prefix 3: `game_runtime_*` $\rightarrow$ `internal/game/runtime`
- **Source Files**: `game_runtime_frame.go`, `game_runtime_overlay.go`, `game_runtime_ui.go`, `game_loop.go`
- **Target**: Move frame tick coordination, overlay render triggers, and UI event loops into `runtime.Engine`.

### Prefix 4: `game_audio_*` $\rightarrow$ `internal/game/audio`
- **Source Files**: `game_audio.go`
- **Target**: Move sound effect precache tracking, spatial sound updates, and music playback adapter into `audio.Manager`.

### Prefix 5: `game_csqc_*` $\rightarrow$ `internal/game/csqc`
- **Source Files**: `game_runtime_csqc.go`
- **Target**: Move Client-Side QuakeC VM event dispatchers into `csqc.Engine`.

---

## 5. Execution Roadmap

1. **Step 1 (`game` prefixes)**: Extract `game_camera_*` into `internal/game/camera` and `game_commands_*` into `internal/game/commands`.
2. **Step 2 (`renderer` prefixes)**: Extract `renderer_alias_*` into `internal/renderer/alias` and `renderer_gogpu_overlay*` into `internal/renderer/overlay`.
3. **Step 3 (`server` prefixes)**: Extract `server_user_commands.go` into `internal/server/commands` and `server_qc_*` into `internal/server/qc`.
4. **Verification**: Run `mise run build` and `go test ./...` after each step.
