# Copilot instructions for ironwail-go

## Highest priority

When planning or writing any changes to Go code, first refer to the original C Ironwail/Quake implementation as the canonical reference. Use the C behavior to resolve ambiguity, preserve parity, and guide tests before designing Go-side behavior.

Plan, structure, and write code to be educational. Application code and test code should be well documented with comments explaining complex or non-obvious behavior and the rationale behind it. Tests should make clear what behavior they verify, how they verify it, and why that behavior matters.

## Build, test, and lint commands

This project is a Go 1.26 workspace using `mise` for canonical tasks. `mise.toml` sets `CGO_ENABLED=0` for normal task runs.

- List tasks: `mise tasks`
- Full test suite: `mise run test` (`go test ./...`)
- Single package test: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/client -run TestName -count=1`
- Renderer-focused tests: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test -tags gogpu ./internal/renderer/... -count=1`
- OpenGL renderer compatibility tests: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp go test -tags opengl ./internal/renderer/... -count=1`
- Build game binary: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp mise run build`
- Run binary after build: `./ironwailgo -basedir "${QUAKE_DIR}"`
- Smoke map startup: `QUAKE_DIR=/path/to/quake mise run smoke-map-start`
- Parity screenshot flow: `mise run parity-ref`, `mise run parity-go`, `mise run parity-compare`
- Lint/security: `mise run lint` (runs `golangci-lint run ./...` and `govulncheck ./...`)
- Format: `gofmt -w <files>` for edited Go files; `mise run fmt` runs `golangci-lint fmt ./...`

Some integration/e2e tests require Quake assets (`pak0.pak`) and skip when assets are unavailable. Multiplayer e2e tests live in `internal/server/multiplayer_e2e_test.go` and can be targeted with `CGO_ENABLED=0 go test -run TestMultiplayer ./internal/server/...`.

## High-level architecture

The engine keeps Quake's client/server architecture even in single-player: the server is authoritative for physics, QuakeC, and entity state; the client parses server messages and presents player-visible state; the renderer/HUD/audio consume client and game-layer projections of that state.

- `cmd/ironwailgo` is the executable entry point. It constructs `internal/game.Game`, wires callbacks/backends, and starts `Run`.
- `internal/game` is the top-level coordinator. `Game` owns host, server, client, renderer, audio, input, menu, HUD, caches, entity collection, per-frame view/camera state, and command registration.
- `internal/host` schedules startup/shutdown, command execution, frame timing, local sessions, demo playback, and server/client sequencing through small subsystem interfaces.
- `internal/server` owns the authoritative map simulation: edicts, physics, collision traces, precaches, reliable/unreliable client messages, and server-side QuakeC VM hooks.
- `internal/qc` loads and executes `progs.dat` bytecode. Server-side QuakeC is driven synchronously by `internal/server`; CSQC uses separate hook structs to avoid renderer/client import cycles.
- `internal/client` owns signon state, stats, parsed entities, temp effects, sounds, centerprints, demos, and user command generation. Protocol constants and wire formats live in `internal/net`.
- `internal/renderer` is the canonical GoGPU/WebGPU rendering path for BSP world, brush entities, alias models, sprites, particles, decals, sky, liquids, HUD/menu/console overlays, and parity screenshot rendering.
- `internal/fs` is the Quake virtual filesystem. It searches game directories and `.pak` files in Quake override order, with case-insensitive pack lookup.
- `pkg/qgo` contains the QGo compiler/toolchain and a Go port of QuakeC gameplay sources under `pkg/qgo/quakego`.

Reference docs worth checking before broad changes:

- `docs/LEARNING_GUIDE.md` for package map and subsystem walkthroughs.
- `docs/WALKTHROUGH_BOOT_TO_MENU.md`, `docs/WALKTHROUGH_SINGLEPLAYER_FORWARD.md`, and `docs/WALKTHROUGH_MULTIPLAYER_SHOOT.md` for cross-subsystem flows.
- `docs/PARITY.md` for parity expectations, current gaps, and focused sweep workflows.
- `docs/QGO_QUAKEGO_GUIDE.md`, `QGO_SPEC.md`, and `QSPEC.md` for QGo/QuakeGo behavior.
- `.github/agents/junior.agent.md` for the repository's existing agent workflow expectations around planning, baselines, and empirical verification.

## Key conventions

- Preserve behavioral parity with C Ironwail/Quake unless a change is explicitly intentional. Many package `doc.go` files name the original C counterparts; use them to understand lineage before refactors.
- GoGPU is the canonical gameplay renderer. Normal development should not introduce alternate renderer/audio/input runtime paths unless the task specifically requires them.
- Build tasks run `go generate ./...` before the game build. If generated artifacts are affected, run the appropriate `mise` task instead of only `go build`.
- Keep Quake wire formats byte-oriented. Server messages use `internal/server.MessageBuffer`; client parsing uses `internal/common.SizeBuf`; protocol bits/opcodes belong in `internal/net/protocol.go`.
- Quake console/HUD text may contain raw high-bit glyph/color bytes and is not always UTF-8. Rendering paths should preserve glyph bytes; terminal/stdout paths should convert through console helpers such as `console.TerminalText`.
- Cvars are registered centrally during game initialization (`internal/game/game_init.go`) and often have constants in subsystem packages (for example renderer cvars in `internal/renderer/types.go`).
- Filesystem paths are Quake virtual paths. Use `internal/fs` helpers and preserve mod/pak override order instead of using direct OS paths for game assets.
- `pkg/qgo/quakego` intentionally mirrors original QuakeC/progs source structure. Avoid cosmetic Go-idiom rewrites there; `.golangci.yml` suppresses some linters for this package to keep mechanical resync practical.
- Prefer package-local tests near the subsystem. Existing test names usually document the parity condition being protected; add focused regressions for protocol, renderer, physics, and QuakeC behavior changes.
