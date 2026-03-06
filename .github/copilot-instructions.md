# Copilot Instructions

## Project Overview

Ironwail Go is a pure-Go (no CGO) port of the [Ironwail Quake engine](https://github.com/andrei-drexler/ironwail). It targets full Quake gameplay parity using WebGPU or OpenGL for rendering and SDL3 for input/audio. The project is an in-progress port; many subsystems have stubs or partial implementations.

## Build Commands

The project uses [mise](https://mise.jdx.dev) as its task runner. `CGO_ENABLED=0` is always set.

```bash
# Build
mise run build-gogpu       # WebGPU backend → ironwailgo-wgpu
mise run build-gl          # OpenGL backend → ironwailgo-gl

# Run directly (sdl3 required for real input)
go run -tags=gogpu,sdl3 ./cmd/ironwailgo -basedir <quake_dir>

# Code generation (required before first build)
mise run go-generate       # or: go generate ./...
```

## Test Commands

```bash
# Unit tests
mise run test
# Equivalent: QUAKE_DIR=/path/to/quake go test ./internal/testutil

# Run a single test package
go test ./internal/<package>

# Run a specific test
go test ./internal/<package> -run TestName

# Smoke tests (require QUAKE_DIR with real Quake assets)
mise run smoke-menu        # Verify menu loads
mise run smoke-headless    # Verify headless mode
mise run smoke-map-start   # Verify map spawning
mise run smoke-all         # All smoke tests
```

Smoke tests verify deterministic log markers: `"FS mounted"`, `"QC loaded"`, `"menu active"`, `"frame loop started"`, `"map spawn started"`, `"map spawn finished"`.

Set `WAYLAND_DISPLAY=` (empty) when running tests that start a window.

## Architecture

```
cmd/ironwailgo/main.go
  └── internal/host         # Main game loop; coordinates all subsystems
        ├── internal/server  # World simulation, entity physics, QuakeC VM
        ├── internal/client  # Player input, server message parsing, prediction
        ├── internal/renderer# GPU rendering (WebGPU or OpenGL backends)
        ├── internal/audio   # Sound mixing and spatial attenuation
        ├── internal/input   # SDL3 keyboard/mouse
        ├── internal/net     # Networking (loopback + UDP)
        ├── internal/fs      # Virtual filesystem (PAK files + loose files)
        ├── internal/qc      # QuakeC VM (executes progs.dat)
        ├── internal/bsp     # BSP map loader and collision tree
        ├── internal/model   # MDL/SPR model loaders
        ├── internal/console # In-game console
        ├── internal/cvar    # Console variables
        ├── internal/cmdsys  # Command system
        ├── internal/menu    # Menu UI
        ├── internal/draw    # 2D drawing (menu/HUD)
        └── internal/hud     # HUD status bar
pkg/types/                   # Public math types (Vec3, angles) — safe for external import
cmd/wadgen/                  # Dev utility: generates dummy WAD files for testing without real assets
```

The renderer has three files per backend: `renderer_gogpu.go`, `renderer_opengl.go`, and `renderer_stub.go`. Build tags (`gogpu` or `opengl,egl`) select which is compiled.

## Key Conventions

### Build Tags
- WebGPU backend: `-tags=gogpu,sdl3`
- OpenGL backend: `-tags=opengl,egl,sdl3`
- Without `sdl3`: input handling is a no-op stub (headless/testing only)

The `sdl3` tag is independent of the renderer tag and must be added explicitly to enable real keyboard/mouse/gamepad input via `internal/input/sdl3_backend.go`. Without it, `internal/input/sdl3_stub.go` is compiled instead and all input is silently dropped.

### Package Organization
- `internal/` — all game subsystems; not importable externally
- `pkg/` — public API (currently only `pkg/types`)
- `cmd/` — executables only; subsystem init logic lives in `cmd/ironwailgo/main.go`

### Error Handling
- Standard Go error returns; wrap with `fmt.Errorf("context: %w", err)`
- No panics in critical paths; errors are logged and handled gracefully

### Logging
- Uses `log/slog` for structured logging
- Deterministic startup log markers are load-bearing (smoke tests grep for them)

### Testing
- Tests use standard `testing` package with mock structs (e.g., `mockServer`, `mockClient`)
- `internal/testutil` provides asset-loading helpers; requires `QUAKE_DIR` for asset-dependent tests
- `cmd/wadgen` generates synthetic WAD files for tests that need textures but not real assets

### Quake Terminology Preserved
The codebase uses original Quake/QuakeC naming where it maps to well-known concepts: `Edict` (entity), `Hull` (collision volume), `Datagram` (network packet), `Progs` (QuakeC bytecode), `Lump` (WAD entry).

## Development Status

The port follows a milestone roadmap (see `finish-up-port.md`):
- **Done:** Architecture, basic compilation, menu rendering, QuakeC VM, basic server physics
- **In progress:** PAK filesystem integration, client signon/parsing, renderer world drawing, input
- **Not started:** Full audio spatial mixing, save/load

When working on unimplemented areas, check `finish-up-port.md` for context on what's planned and `port-guide.md` for C→Go porting decisions and patterns.
