# AGENTS.md — Ironwail Go

Guidance for AI agents working in this repository. The first section covers
general agentic engineering principles; the rest covers what is specific to
*this* codebase that is not self-evident from opening a single file.

## Agentic Engineering Guidelines

These guidelines foster a high-quality "Senior-Junior" partnership with the
human operator. The agent should encourage the human to act as architect and
reviewer, ensuring robust, well-tested, and maintainable software.

1. **Test-First (Red/Green TDD):** For every new feature or bug fix, proactively
   suggest writing a failing test before implementing the solution. This ensures
   requirements are clear and creates an objective definition of "done."
2. **Establish a Stable Baseline:** Never build on a broken foundation. Before
   applying changes, run the existing test suite. If tests fail, report it
   immediately and suggest fixing the baseline before proceeding. Avoids
   "phantom bugs" where new changes get blamed for pre-existing issues.
3. **Architect-Reviewer Planning:** Strategy precedes execution. Before modifying
   files, present a concise plan: which files will be touched, the logic to be
   implemented, and the testing strategy. Lets the human course-correct before
   code is written.
4. **Forced Incrementalism:** Small context leads to high precision. If a request
   is broad or complex, break it down into smaller, atomic sub-tasks. Minimizes
   hallucinations and ensures each piece is thoroughly understood and verified.
5. **Be a Quality Multiplier:** Do the "boring" high-value work humans skip.
   Proactively include exhaustive edge-case handling, inline documentation, and
   comprehensive tests. Use the agent's speed to raise the quality ceiling.
6. **Empirical Verification over Trust:** "Done" means "Verified." After
   implementation, run all relevant tests. If the task involves UI or manual
   steps, provide a "Manual Verification Checklist." Move the human from trusting
   the AI to verifying the result, preventing "AI slop."
7. **Transparent Reasoning & Rubber Ducking:** Explain the *why*, not just the
   *what*. When proposing a complex change, explain trade-offs. If the human
   proposes a solution, "rubber duck" it by identifying potential edge cases or
   architectural conflicts.
8. **Code is Disposable:** Prioritize correctness over persistence. If a debugging
   session becomes convoluted, be the first to suggest reverting to the last
   stable state and trying a different, simpler approach. Treat code as cheap and
   stay focused on the cleanest solution.

## Project at a glance

Go 1.26 port of the [Ironwail](https://github.com/andrei-drexler/ironwail) Quake
engine. Pure Go (`CGO_ENABLED=0`); canonical renderer is gogpu/WebGPU; audio via
Oto; tasks run through `mise`. Module path: `github.com/darkliquid/ironwail-go`.
Behavioral parity with C Ironwail/Quake is the project's goal and its primary
test oracle.

## C reference and Quake data

The original C Ironwail source and Quake game data do **not** live in this repo.
They are expected to be available adjacent to it and reachable via symlinks from
the repo root:

- `./ironwail` → the C Ironwail repo (source reference, parity binary)
- `./quake-data` → the Quake data directory (contains `id1/pak0.pak`, etc.)

Run `mise run setup-symlinks` to check existing symlinks, verify they resolve,
and create them if the targets exist. The symlinks are gitignored (local setup,
not committed). If `QUAKE_DIR` is unset, point it at `./quake-data`.

Package `doc.go` files have an `# Original C lineage` section naming the C
source files each package mirrors — use it to locate the C counterpart before
refactoring.

## Build, test, and lint commands

All canonical tasks are defined in `mise.toml`. `mise tasks` lists them. Key ones:

| Task | What it does |
| --- | --- |
| `mise run test` | `go test ./...` |
| `mise run build` | `go generate ./...` then `go build -o ironwailgo ./cmd/ironwailgo` |
| `mise run run` | Build then `./ironwailgo -basedir ${QUAKE_DIR}` |
| `mise run lint` | `golangci-lint run ./...` + `govulncheck ./...` |
| `mise run fmt` | `golangci-lint fmt ./...` |
| `mise run race` | `racedetector test ./...` |
| `mise run verify` | `test` + `build` |
| `mise run smoke-all` | `smoke-menu`, `smoke-headless`, `smoke-map-start` |
| `mise run parity-ref` / `parity-go` / `parity-compare` / `parity-all` | Parity screenshot harness |
| `mise run build-qgo` | Build the QGo compiler (`cmd/qgo`) |
| `mise run build-progs` | Compile QuakeGo sources into `progs.dat` |
| `mise run build-bspdiag` | Build the BSP diagnostic inspection CLI (`cmd/bspdiag`) |

### Running a single package test

`mise run test` sets `CGO_ENABLED=0` via `[env]`, but bare `go test` does not.
For a single package, match the mise environment:

```
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/client -run TestName -count=1
```

`TMPDIR` must point at the repo's `.tmp/` scratch dir (gitignored) or `go test`
can fail to invoke the build toolchain on this host.

### Renderer tests

There are **no build tags anywhere in this repo** (see gotchas below). Run
renderer tests with plain:

```
TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/renderer/... -count=1
```

Do **not** pass `-tags gogpu` or `-tags opengl` — no `.go` file carries those
constraints.

## Critical gotchas (read before editing)

### 1. No build tags

**Zero `.go` files contain `//go:build` directives.** The gogpu renderer
(`internal/renderer/*_gogpu.go`) is plain `package renderer` with unconditional
`github.com/gogpu/gogpu` imports; it is always compiled. There is no OpenGL
renderer variant. Any docs referencing `-tags gogpu` or `-tags opengl` are stale.

### 2. `pkg/qgo/*` are separate Go modules (QuakeGo source, not importable Go)

`pkg/qgo/quake` and `pkg/qgo/quakego` are **separate Go modules** with their own
`go.mod` files and no `go.work`. They are not part of the root module and cannot
be imported by it. This is intentional: `pkg/qgo/quakego` is QuakeGo source (a Go
dialect compiled to QCVM `progs.dat` bytecode by `cmd/qgo`), not regular Go
library code.

Module paths are short, non-github names reflecting their purpose:
- `pkg/qgo/quake/go.mod` → `module quake` — core QCVM-facing types (`Entity`,
  `Vec3`, `Func`) and engine builtin stubs (`pkg/qgo/quake/engine/`).
- `pkg/qgo/quakego/go.mod` → `module progs` — the gameplay source compiled to
  `progs.dat`.

`pkg/qgo/quakego/go.mod` has `replace quake => ../quake`. Source files import
`"quake"` and `"quake/engine"`.

Consequences:
- The root module does **not** require or replace `pkg/qgo/*`. From the repo
  root, gopls/LSP may report `BrokenImport` errors for `pkg/qgo/quakego/*.go`
  and `pkg/qgo/quake/engine/*.go` because they belong to separate modules. These
  errors are expected when the LSP is rooted at the main module.
- Build/test QGo packages from within their own directories:
  `cd pkg/qgo/quake && go test ./...` or `cd pkg/qgo/quakego && go test ./...`
- Compile `progs.dat`: `mise run build-progs` or
  `go build -o qgo ./cmd/qgo && cd pkg/qgo/quakego && ../../../qgo`

### 3. `.gitignore` is a whitelist — new non-`.go` files are ignored by default

`.gitignore` ignores everything (`*`) then re-allows specific patterns: `*.go`,
root config (`mise.toml`, `go.mod`, `go.sum`, `.golangci.yml`), `docs/**/*.md`,
`docs/**/*.txt`, `tasks/*.sh`, `testdata/parity/*.json`. If you add a new file
type (e.g. a `.yaml`, `.json`, script), it will be silently ignored unless you
add an allow rule. `.tmp/` is the local scratch TMPDIR.

### 4. CGO is always off

`mise.toml [env]` sets `CGO_ENABLED=0`. The project is pure Go. Never introduce
CGO dependencies. When running bare `go` commands, prefix `CGO_ENABLED=0`.

### 5. `mise run build` runs `go generate` first

`tasks.build` depends on `tasks.go-generate` (`go generate ./...`). Stringer
generated files (`internal/image/luptype_string.go`,
`pkg/qgo/quake/entityflags_string.go`) are regenerated from `//go:generate`
directives in `internal/image/wad.go:30` and `pkg/qgo/quake/quake.go:265`. If your
change touches an enum with a stringer directive, run `mise run build` (or
`go generate ./...`) rather than `go build` directly.

## Architecture and control flow

The engine preserves Quake's client/server split even in single-player: the
server is authoritative for physics, QuakeC, and entity state; the client parses
server messages and presents player-visible state; renderer/HUD/audio consume
client and game-layer projections. See `docs/LEARNING_GUIDE.md` for the package
map and `doc.go` (root) for the architectural diagram.

- `cmd/ironwailgo` — entry point. `main.go` constructs `internal/game.Game`,
  wires callbacks/backends, parses startup flags (`-headless`, `-screenshot`,
  `-width`, `-height`, `-window`, `-loglvl`, `-pprof`), and starts `Run`.
- `internal/game` — top-level coordinator. `Game` struct
  (`internal/game/game.go`) owns Host, Server, QC, CSQC, Renderer, Client,
  Particles, Menu, Input, Draw, HUD, Audio, caches, overlays. Cvars are
  registered centrally in `internal/game/game_init.go`.
- `internal/host` — startup/shutdown, command execution, frame timing, local
  sessions, demo playback, server/client sequencing.
- `internal/server` — authoritative map simulation: edicts, physics, collision
  traces, precaches, reliable/unreliable client messages, server-side QuakeC VM
  hooks.
- `internal/qc` — loads/executes `progs.dat` bytecode. Server-side QuakeC is
  driven synchronously by `internal/server`; CSQC uses separate hook structs to
  avoid renderer/client import cycles.
- `internal/client` — signon state, stats, parsed entities, temp effects,
  sounds, centerprints, demos, usercmd generation.
- `internal/renderer` — canonical GoGPU/WebGPU path for BSP world, brush
  entities, alias models, sprites, particles, decals, sky, liquids, HUD/menu/
  console overlays, parity screenshot rendering.
- `internal/fs` — Quake virtual filesystem; searches game dirs and `.pak` files
  in Quake override order, case-insensitive pack lookup.
- `pkg/qgo` — QGo compiler/toolchain and a Go port of QuakeC gameplay sources
  under `pkg/qgo/quakego` (separate module; see gotchas).

Walkthroughs: `docs/WALKTHROUGH_BOOT_TO_MENU.md`,
`docs/WALKTHROUGH_SINGLEPLAYER_FORWARD.md`,
`docs/WALKTHROUGH_MULTIPLAYER_SHOOT.md`. Parity: `docs/PARITY.md`.

## Conventions that matter

### Parity-first

Preserve behavioral parity with C Ironwail/Quake unless a change is explicitly
intended. Before designing Go-side behavior, refer to the original C as the
canonical reference. Each package's `doc.go` has an `# Original C lineage`
section naming the C source files it mirrors — use it to locate the C
counterpart before refactoring. Parity tests cite C inline, e.g.
`// Where in C: SV_WalkMove in sv_phys.c`.

### Wire formats are byte-oriented

- Server messages: `internal/server.MessageBuffer`
  (`internal/server/message.go`) — bidirectional, holds `ProtoFlags`.
- Client parsing: `internal/common.SizeBuf` (`internal/common/common.go`).
- Protocol constants/opcodes: `internal/net/protocol.go` (e.g.
  `PROTOCOL_NETQUAKE=15`, `PROTOCOL_FITZQUAKE=666`, `PRFL_*` flags, `SVC*`
  message types in C-style uppercase).

Do not mix `SizeBuf` (client) and `MessageBuffer` (server); they are deliberately
separate.

### Console/HUD text is not UTF-8

Quake console/HUD text may contain raw high-bit glyph/color bytes. Rendering
paths preserve glyph bytes; terminal/stdout paths convert through helpers such
as `console.TerminalText`.

### Cvars

Registered centrally during game init in `internal/game/game_init.go`.
`registerMirroredArchiveCvars(canonical, legacy, default, desc)` creates a
canonical+legacy alias pair with bidirectional callbacks — used to keep
compatibility with old cvar names. Renderer cvar constants often live in
`internal/renderer/types.go`.

### Filesystem paths are Quake virtual paths

Use `internal/fs` helpers; preserve mod/pak override order. Do not reach for OS
paths for game assets.

### `pkg/qgo/quakego` is a mechanical port

It intentionally mirrors original QuakeC/progs source structure. Avoid cosmetic
Go-idiom rewrites (tagged switches, merged var decls) there — they drift the
port from `progs.src` and make resync harder. `.golangci.yml` suppresses
`unused`, `SA4017`, `QF1003`, and `S1021` for that package for the same reason.

### Import aliases in use

Examples observed in real files: `cl` (`internal/client`), `inet`
(`internal/net`), `qtypes` (`pkg/types`), `surfacepkg`
(`internal/renderer/surface`), `worldgogpu` (`internal/renderer/world/gogpu`).

### Logging

`log/slog` is the universal convention (~60+ files). No `log.Printf`/
`fmt.Println` logging. Setup lives in `cmd/ironwailgo/logging.go`.

## Testing approach

### Asset-dependent tests skip via env

`internal/testutil/assets.go`:
- `LocateQuakeDir()` checks `QUAKE_DIR` env, then walks `.`/`..`/`../..`/`../../..`
  for an `id1` dir.
- `LocatePak0()` checks `QUAKE_PAK0_PATH` env, else `<QuakeDir>/id1/pak0.pak`.
- `SkipIfNoQuakeDir(t)` / `SkipIfNoPak0(t)` — `t.Helper()` + `t.Skipf` if absent.
- `CompareStructs` — `reflect.DeepEqual` with hex dump for `[]byte` mismatches.
- `AssertNoError(t, err)`.

Integration/e2e tests that need `pak0.pak` skip when assets are unavailable.
Multiplayer e2e: `internal/server/multiplayer_e2e_test.go`, targetable with
`go test -run TestMultiplayer ./internal/server/...`.

### Synthetic world model for deterministic unit tests

`internal/server/synthetic_bsp_helper.go` exposes
`CreateSyntheticWorldModel()` — a minimal single-plane BSP that lets server
physics tests run deterministically without pak assets. Prefer this for
physics/collision unit tests; reach for `SkipIfNoPak0` only when real map
data is required.

### Test helpers are package-local, not testutil constructors

Tests define local helpers prefixed `new`/`with` (e.g.
`newPhysicsTestServer()`, `withPhysicsCVars(t, s, values)`,
`newPushMoveElevatorTestServer(t)`). `testutil` is for asset location and
generic assertions, not subsystem scaffolding. Subtests use `t.Run` with
descriptive kebab/snake names.

### Parity test naming

Mix of `Test<Entity><Behavior>` and `TestPhysics<Verb>` (e.g.
`TestPhysicsWalkJump`). Existing test names usually document the parity
condition being protected — read them before changing the corresponding code.

## In-engine debug tooling

- `host_speeds 1` — frame timing logs from host/server/renderer.
- `sv_debug_telemetry 1` — server-side telemetry for trigger/physics/QC
  activity (see `internal/server/debug_telemetry.go`). Event mask via
  `sv_debug_telemetry_events`, classname/entnum filters, per-frame summary mode.
- `sv_debug_qc_trace 1` — QuakeC call tracing (enter/leave + optional builtins).
- `profile` — top 10 QC function profile counters, reset on read, local server
  only.
- `profile_cpu_start/stop`, `profile_dump_heap`, `profile_dump_allocs` — file
  pprof captures through the in-game console.

Telemetry lines emit with `[svdbg ...]` prefixes.

### Offline BSP Diagnostic Tool (`bspdiag`)

Use `bspdiag` (`cmd/bspdiag`, build with `mise run build-bspdiag`) to inspect BSP file lumps, entity definitions, leaf contents, face attributes, lightmaps, and textures offline without writing scratch scripts:

- `bspdiag [info] <quake_dir> <map.bsp> [gamedir]` — General BSP summary, texture table, texinfo flags, atlas simulation, and lightmap page estimates.
- `bspdiag entities <quake_dir> <map.bsp> [gamedir] [classname_filter]` — Parsed entity lump key-value fields (e.g., `worldspawn` fields like `wateralpha`, `fog_color`, `fog_density`, `sky`).
- `bspdiag point <x> <y> <z> <quake_dir> <map.bsp> [gamedir]` — Query BSP leaf index and contents (`CONTENTS_EMPTY`, `CONTENTS_WATER`, `CONTENTS_SLIME`, etc.) at 3D coordinates.
- `bspdiag face <face_id> <quake_dir> <map.bsp> [gamedir]` — Detailed face attributes (`PlaneNum`, `Texinfo`, `LightOfs`, vertex UVs, lightmap grid size, first lightmap sample bytes).
- `bspdiag texture <name> <quake_dir> <map.bsp> [gamedir]` — Texture dimensions, classification (`TexTypeWater`, etc.), MipLevel(0) palette indices, and converted RGBA colors.
- `bspdiag liquids <quake_dir> <map.bsp> [gamedir]` — Liquid face analysis: applies the `.lit` sidecar, enumerates every `SURF_DRAWTURB` face, reports texinfo flags (`TEX_SPECIAL`), `LightOfs`, lightmap sample statistics (`VARIED`/`UNIFORM`/`NONE`), and the resolved per-liquid alpha settings (worldspawn overrides + `TransparentWaterSafe`). Use this to diagnose lit-water and water-translucency issues.


## Reference docs to read before broad changes

- `docs/LEARNING_GUIDE.md` — package map + subsystem walkthroughs.
- `docs/WALKTHROUGH_BOOT_TO_MENU.md`, `docs/WALKTHROUGH_SINGLEPLAYER_FORWARD.md`,
  `docs/WALKTHROUGH_MULTIPLAYER_SHOOT.md` — cross-subsystem flows.
- `docs/PARITY.md` — parity expectations, current gaps, sweep workflows. Note:
  do not commit generated logs/screenshots/profiles unless a maintainer asks.
- `docs/QGO_QUAKEGO_GUIDE.md`, `QGO_SPEC.md`, `QSPEC.md` — QGo/QuakeGo behavior.


