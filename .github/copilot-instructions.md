# VS Code Copilot Instructions

## Workflow

THESE INSTRUCTIONS ARE MANDATORY AND MUST BE FOLLOWED AT ALL TIMES. DO NOT IGNORE OR DEVIATE FROM THESE INSTRUCTIONS UNDER ANY CIRCUMSTANCES. DO NOT USE YOUR OWN JUDGMENT TO OVERRIDE THESE INSTRUCTIONS. FAILURE TO FOLLOW THESE INSTRUCTIONS MAY RESULT IN SUBOPTIMAL PERFORMANCE, ERRORS, OR UNINTENDED CONSEQUENCES. THIS APPLIES EVEN FOR SIMPLE/TRIVIAL TASKS - THERE ARE NO EXCEPTIONS.

 - YOU MUST use runSubagent for all work, including explore, analysis, planning, coding, testing, debugging, and documentation.
 - You MUST use the vscode/askQuestions tool to ask for any clarifications, additional instructions, or to confirm when a task is complete.
 - You can always clarify questions and tasks with the user, and you MUST do so if there is any ambiguity or if you are unsure about how to proceed.
 - You MUST use the tools available to you, including web search, code analysis, and testing
 - Unless explicitly told otherwise, you MUST use GPT-5.3-Codex for development, testing, debugging
 - Unless explicitly told otherwise, you MUST use the most recent model available for analysis and planning
 - All interaction with the user MUST be through vscode/askQuestions
 - When a task is complete, you MUST use vscode/askQuestions with the prompt 'Is there anything else?', with an option of 'No, I'm done' and 'Yes', with an input for further instructions.

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

## Project Guidelines (for AI coding agents)

- **Code Style:** follow existing Go idioms in the repo — use `gofmt` formatting, prefer clear error wrapping (`fmt.Errorf("context: %w", err)`), and keep public API changes to `pkg/` minimal. See `pkg/types/types.go` and `internal/*` files for style examples.
- **Architecture:** top-level entry is [cmd/ironwailgo/main.go](cmd/ironwailgo/main.go). Major subsystems live under `internal/` (host, server, client, renderer, audio, input, qc, fs, bsp, model). Renderer backends: `internal/renderer/renderer_gogpu.go`, `internal/renderer/renderer_opengl.go`, `internal/renderer/renderer_stub.go`.
- **Build & Test:** use `mise` task runner. Run `mise run go-generate` before builds. Typical commands:

```bash
mise run go-generate
mise run build-gogpu     # build WebGPU binary
mise run build-gl        # build OpenGL binary
mise run test            # run unit tests
# Smoke tests require real Quake assets (set QUAKE_DIR)
mise run smoke-menu
```

- **Build Tags:** use `-tags=gogpu,sdl3` for WebGPU+SDL3, `-tags=opengl,egl,sdl3` for OpenGL+SDL3. Without `sdl3` the input backend is a no-op (`internal/input/sdl3_stub.go`).
- **Testing Conventions:** smoke tests look for deterministic log markers (e.g. "FS mounted", "QC loaded", "menu active"). Use `internal/testutil` and `cmd/wadgen` for asset-less tests.

## Project Conventions & Patterns

- Keep subsystem code inside `internal/` (not importable). Public types and stable math helpers live in `pkg/types` — treat those as low-change surface area. Examples: [pkg/types/types.go](pkg/types/types.go).
- Use feature build tags to compile backends; search for `_gogpu` and `_opengl` suffixes to find backend-specific files.
- Avoid introducing CGO; `CGO_ENABLED=0` is enforced in CI and local builds.

## Integration Points

- Task runner: `mise` (see `mise.toml`).
- Rendering: WebGPU or OpenGL backends in `internal/renderer`.
- Input/audio: SDL3 backends (`internal/input/sdl3_backend.go`, `internal/audio/backend_sdl3.go`) with stub fallbacks for headless tests.
- Assets: many tests require a `QUAKE_DIR` with original Quake assets; `cmd/wadgen` can synthesize WADs for asset-free tests.

## Security & Sensitive Areas

- The repo does not store secrets; avoid hardcoding paths or credentials. Asset directories (QUAKE_DIR) can contain copyrighted game data — do not commit or distribute them.
- CI and smoke tests depend on deterministic logs; do not remove or rename those markers without updating tests.

## Quick Links (examples to inspect)

- [cmd/ironwailgo/main.go](cmd/ironwailgo/main.go)
- [internal/renderer/renderer_gogpu.go](internal/renderer/renderer_gogpu.go)
- [internal/input/sdl3_backend.go](internal/input/sdl3_backend.go)
- [pkg/types/types.go](pkg/types/types.go)
- [cmd/wadgen/main.go](cmd/wadgen/main.go)

## Useful directories and reference files

- `/home/darkliquid/Projects/ironwail` - the original C version of the engine, for reference
- `/home/darkliquid/Projects/ironwail-go` - the Go port, where you will be working
- `/home/darkliquid/Games/Heroic/Quake` - a directory containing the original Quake assets, which may be needed for testing and development (typically set as `QUAKE_DIR`)