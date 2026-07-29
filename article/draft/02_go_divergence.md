# Chapter 2: The Go Divergence — From C Hunk to GC, From OpenGL to WebGPU

Chapter 1 described the Quake engine as it was built in 1996: a manual-memory,
single-threaded, immediate-mode-GL, shared-memory-VM architecture. This chapter
is about what happens when you port that to Go in 2026. The `ironwail-go`
README states the intent:

> Apart from the obvious that this is Go, rather than C, I'm building this
> with the following changes: gogpu/WebGPU as the canonical gameplay
> renderer/runtime; dividing the codebase up into packages; use Go stdlib
> for as much as possible, rather than custom implementations of things
> from the original C codebase. [#README](#readme)

This is not a transliteration. It is a deliberate re-architecture that preserves
behavioral parity while changing the substrate. Each divergence has a reason,
and each reason has a cost.

---

## Memory: from Hunk/Zone/Cache to the garbage collector

The most fundamental change. Chapter 1 covered Quake's three-tier memory model:
the Hunk (a single pre-allocated arena, bump-allocated and nuked on map change),
the Zone (a free-list heap for individual-lifetime objects), and the Cache
(LRU-evictable resource storage). All three assume the engine owns a contiguous
block and manages it with pointer arithmetic.

Go replaces all of this with the runtime garbage collector. `Hunk_Alloc` becomes
`make()` or `new()`. Raw pointer arrays become slices. Manual `Z_Free` becomes
implicit GC. The comparison doc states it plainly: *"Replaces `Hunk_Alloc` with
standard `make()` or `new()` and utilizes slices instead of raw pointers for
collections."* [#Comparison](#comparison) The boot sequence doc adds: *"The C
version's `parms.membase = malloc(parms.memsize)` is entirely absent in Go."*
[#BootSeq](#bootseq)

### The cost: GC pressure in hot paths

The garbage collector is a trade. You get safety and simplicity; you lose
deterministic deallocation and control over memory layout. In a game engine
running at 250 FPS, the GC pressure from per-frame allocations is real. The
project's git history shows a direct response: commit `5a04a01` ("Optimize
renderer allocations in hot paths") introduced:

- **Scratch buffers** on the `Renderer` struct for brush entity rendering,
  eliminating per-frame `make()` in `renderOpaqueBrushEntitiesHAL`.
- **`sync.Pool`** for the dynamic lights slice, reusing allocations across
  frames.
- **`unsafe.Slice`** for `float32ToBytes` conversions, avoiding per-call heap
  allocation.
- Elimination of per-frame map copying (holding an `RLock` instead).

These are not premature optimizations — they came from profiling the renderer
under the qbj3 brutalist jam map's 1,002 visible faces and 750 models. The
GC pressure is the Go tax on Quake's arena model, and the mitigation is pooling
and reuse rather than reverting to manual memory.

### The long-term question: arena allocators

The prologue mentioned that one of the project's future plans is investigating
Go-based arena/region allocators. The Hunk's "allocate linearly, nuke
everything on map change" pattern maps cleanly onto Go's experimental `arena`
proposal or custom region allocators: allocate a large `[]byte`, sub-allocate
into it, and discard the whole backing array when the map changes. This would
give deterministic cleanup for the bulk of per-map allocations (BSP data,
models, textures) without the GC tax, while still being memory-safe. It is an
open question, not a settled decision. [#AGENTS](#agents)

---

## Concurrency: from single-threaded + SDL threads to goroutines

The C engine is primarily single-threaded. `Host_Frame` runs on one thread.
SDL mutexes and threads are used only for specific tasks: async loading,
background music, and (in Ironwail) the renderer thread. The comparison doc
notes: *"Primarily single-threaded, with some use of SDL mutexes and threads
for specific tasks like async loading or background music."*
[#Comparison](#comparison)

Go replaces this with goroutines and channels, but the project does not naively
parallelize the engine. The core simulation remains single-threaded — the
server physics loop, the QCVM execution, and the client update are sequential,
matching C's `SV_Physics` iteration. Where Go's concurrency model shines is in
the *periphery*:

### The async queue

The `internal/async` package provides a bounded FIFO work queue that marshals
work from background goroutines back to the main frame pump. Its doc explains
the parity rationale:

> This matches the semantics of the original C Ironwail's `host.c`
> AsyncQueue. In the context of a game engine like Quake, many systems (like
> save workers or mod downloaders) run in the background but need to update
> the game state safely without racing against the client or server state.
> [#AsyncDocs](#asyncdocs)

The queue uses `sync.Mutex` and `sync.Cond` for blocking behavior, and is drained
once per frame in `Host.Frame`. The async doc is candid about the trade-off:
*"While idiomatic Go might use an unbounded channel for this purpose,
`async.Queue` mirrors the C implementation's bounded, blocking behavior and
atomic drain semantics."* [#AsyncDocs](#asyncdocs)

### Dedicated render thread

The GoGPU renderer runs on its own thread, coordinated through the `gogpu.App`
event loop. The `OnDraw` callback (`renderer_gogpu_runtime.go:149`) registers
the frame draw callback; `OnUpdate` (`:199`) registers the game logic update.
The `MainThreadQueue` in `internal/host/mainthread.go` ensures that OS-sensitive
operations (window management, renderer calls) execute on the correct thread.
[#HostDocs](#hostdocs)

### Audio streaming

The `internal/audio` package uses the Oto library as its backend, replacing C's
DMA/sound-card drivers. Audio mixing still uses the same DMA-style buffer model
(mirroring classic sound card behavior), but the output device is abstracted
behind a `Backend` interface. The audio doc notes the mixer uses 24.8 fixed-point
arithmetic in `SamplePair` for precision without floating-point overhead.
[#AudioDocs](#audiodocs)

### Parallel asset loading

The `internal/engine` package provides `ParallelLoad[T]` and `LoadPipeline[T]`
using a worker-pool pattern with a buffered-channel semaphore for concurrency
limiting. This is used during level loading to fetch multiple sounds, models,
and textures concurrently. [#EngineDocs](#enginedocs)

---

## Packaging: from flat `Quake/*.c` to `internal/*` packages

The C Ironwail source is a flat directory of `*.c` and `*.h` files under
`Quake/`. There are no packages, no visibility control, no import boundaries.
Everything is global. `extern` declarations and header files are the only
interface contracts. The `COM_*` functions in `common.c` are called from
everywhere. The `SV_*` functions in `sv_phys.c` call `CL_*` functions in
`cl_main.c` directly.

Go cannot work this way. The `internal/` package convention enforces visibility
boundaries. The project divides the engine into packages with specific
responsibilities:

| C area | Go package | Responsibility |
| --- | --- | --- |
| `host.c`, `main_sdl.c` | `internal/host` | Main loop, timing, session lifecycle |
| `common.c` (VFS) | `internal/fs` | Virtual filesystem, PAK files |
| `gl_*.c`, `r_*.c` | `internal/renderer` | WebGPU rendering pipeline |
| `in_sdl.c`, `keys.c` | `internal/input` | Input abstraction |
| `pr_exec.c`, `pr_edict.c` | `internal/qc` | QuakeC VM |
| `sv_main.c`, `sv_phys.c` | `internal/server` | Authoritative simulation |
| `cl_main.c`, `cl_parse.c` | `internal/client` | Client state, prediction |
| `cmd.c` | `internal/cmdsys` | Command system |
| `cvar.c` | `internal/cvar` | Console variables |
| `console.c` | `internal/console` | Console buffer |
| `common.c` (math) | `pkg/types` | Vec3, Mat4, angle math |
| — | `internal/engine` | Generic data structures (Cache, Registry, Queue) |
| — | `internal/async` | Thread-safe work queue |
| — | `internal/game` | Top-level coordinator wiring everything together |

The `internal/game` package is the Go equivalent of the C `main()` wiring —
it owns the `Game` struct (`internal/game/game.go`) that holds Host, Server, QC,
CSQC, Renderer, Client, Particles, Menu, Input, Draw, HUD, Audio, caches, and
overlays. Cvars are registered centrally in `internal/game/game_init.go`.

### The `doc.go` lineage convention

Every package has a `doc.go` file with an `# Original C lineage` section naming
the C source files it mirrors. For example, `internal/server/doc.go` names
`sv_main.c`, `sv_phys.c`, `world.c`, and `pr_cmds.c`. This is not decoration —
it is a navigation tool. Before refactoring a Go package, you read its lineage
section to find the C counterpart, then study the C to understand the canonical
behavior. [#AGENTS](#agents)

### The `pkg/qgo` exception

`pkg/qgo/quake` and `pkg/qgo/quakego` are **separate Go modules** with their own
`go.mod` files, intentionally outside the root module. They are not importable by
the engine. This is by design: `pkg/qgo/quakego` is QuakeGo source (a Go dialect
compiled to QCVM `progs.dat` bytecode), not regular Go library code. The root
module does not require or replace `pkg/qgo/*`. [#AGENTS](#agents) Chapter 5
covers QuakeGo in detail.

---

## stdlib adoption: replacing custom Quake utilities

The README states: *"Use Go stdlib for as much as possible, rather than custom
implementations of things from the original C codebase."* [#README](#readme)
In practice this means:

- **String handling**: Quake's custom `COM_Parse` tokenizer and string utilities
  are replaced with Go `strings` / `strconv` where the semantics are compatible.
  The command tokenizer still has custom logic (it must respect Quake's
  quote/semicolon rules), but generic string manipulation uses stdlib.
- **I/O**: `io.Reader` / `io.Writer` / `io.NewSectionReader` replace C's raw
  `FILE *` and byte-pointer I/O. The filesystem package uses `io.NewSectionReader`
  to provide a standard `io.Reader` over a portion of a `.pak` file. [#FSDocs](#fsdocs)
- **Containers**: `sync.Map`, `sync.Pool`, generic slices replace C's manual
  linked lists and arrays.
- **Math**: `pkg/types` provides `Vec3` and `Mat4` as Go structs with both
  procedural (`Vec3Add`, `Vec3Dot`) and method (`v.Add`, `v.Dot`) APIs. The
  procedural functions follow C Quake's style for parity; the methods provide
  idiomatic Go. The doc notes: *"Both produce identical results."*
  [#TypesPkg](#typespkg)

### Where custom code remains

Some C utilities cannot be replaced by stdlib because they encode Quake-specific
semantics:

- **The command tokenizer** (`internal/cmdsys/cmd_buffer.go`) replicates
  Quake's specific rules for whitespace, quotes, and semicolons.
- **The QCVM** (`internal/qc`) must bit-match C's bytecode interpretation,
  including IEEE divide-by-zero behavior and the `0x1000000` runaway-loop limit.
- **The filesystem** must preserve Quake's case-insensitive PAK lookup and
  mod override order.
- **The network protocol** must produce byte-identical wire format.

---

## The CGO policy: pure Go, always

`mise.toml` sets `CGO_ENABLED = "0"`. The project is pure Go. AGENTS.md states
this as a hard rule: *"CGO is always off. The project is pure Go. Never
introduce CGO dependencies."* [#AGENTS](#agents)

This policy was not always in place. The git history tells a story:

### The cgo-GLFW detour and return

The project started with GoGPU as the intended renderer (commit `064c027`,
2026-02-24: *"renderer: port WebGPU core initialization"*). But early gogpu
issues — naga shader compilation bugs, Wayland input failures, crashes —
forced a detour. Commit `15b888e` (2026-02-25, one day later) added *"alternate
cgo gl renderer"*. For over a month, the engine ran on a cgo-based OpenGL
renderer using GLFW for windowing and SDL for input.

The gogpu issue #157 opening body records the frustration:

> I first attempted to tackle things using GoGPU as the rendering backend,
> but eventually hit enough issues that I sadly switched to cgo GLFW code.
> [#GogpuIssues](#gogpuissues)

The return came with commit `b2fb6e9` (2026-04-05: *"Retire gl+sdl (#11)"*),
which removed the OpenGL renderer, the SDL input backend, and made Oto the
canonical audio backend, with GoGPU as the sole renderer. The commit was
co-authored with Copilot. After that, commit `889f797` (2026-04-24) dropped
the renderer shims and cleaned up the game loop.

### The current stack

The canonical gameplay stack is now:
- **Renderer**: GoGPU/WebGPU (`github.com/gogpu/gogpu`)
- **Audio**: Oto (`github.com/ebitengine/oto/v3`)
- **Input**: renderer-provided backend (gogpu's input adapter in
  `internal/renderer/gogpu/input_backend.go`)
- **Native bindings**: `github.com/ebitengine/purego` (indirect, for
  cgo-free native function calls)

`purego` appears as an indirect dependency — it is used by the gogpu stack
for cgo-free FFI to platform libraries where needed, but the engine itself
compiles with `CGO_ENABLED=0`. [#Comparison](#comparison)

---

## Input: from SDL to backend injection

The C engine uses `in_sdl.c` to interface with SDL2 for keyboard, mouse, and
gamepad events. The input handling comparison doc explains the divergence:
*"Go uses `internal/input/` as a backend-neutral abstraction layer. The active
runtime backend is supplied by the executable/renderer integration rather than
by a package-local SDL implementation."* [#InputHandling](#inputhandling)

In practice, this means:
- `internal/input` defines a `Backend` interface and `System` type that
  normalize keyboard, mouse, and gamepad events into Quake key codes and
  movement commands.
- The gogpu renderer provides its own input adapter
  (`internal/renderer/gogpu/input_backend.go`) that bridges gogpu window events
  to the `input.Backend` interface.
- The input system decides which callback to trigger based on `KeyDest`
  (console, menu, game), matching C's `key_dest` dispatch.

The Go implementation maintains identical Quake keycodes (`KMWheelUp`,
`KMouse1`, etc.) to ensure compatibility with `config.cfg` and
`autoexec.cfg`. Gamepad support is currently initial (deadzones only), compared
to C Ironwail's extensive gyro/rumble support. [#InputHandling](#inputhandling)

The gogpu input bugs that forced the cgo detour (issues #129, #173, #175) are
covered in Chapter 6.

---

## Parity-first discipline

The comparison doc states the goal: *"high-fidelity parity,"* meaning
identical `progs.dat` execution, identical physics and movement, visual parity
with the GoGPU renderer, and support for standard Quake data files.
[#Comparison](#comparison)

This is enforced through several mechanisms:

### The `// Where in C:` convention

Tests cite the C function they mirror. For example, in
`internal/cmdsys/cmd_test.go`:

```go
// Where in C: Cmd_TokenizeString in cmd.c
```

This appears throughout the test suite. Every parity test is anchored to a
specific C function, so a reader can open both side by side.

### Parity test naming

Test names document the invariant being protected:

- `TestPhysicsSendIntervalMatchesFitzQuakeParity` — the send-interval lerp
  timing matches FitzQuake's protocol extension.
- `TestWriteEntityUpdate_FieldOrderMatchesCProtocol` — the wire format field
  order matches C exactly.
- `TestRandomBuiltinMatchesCompatSequence` — the `random()` QC builtin produces
  the same sequence as C's compatrand.
- `TestLoadExternalSkyboxWindMatchesCIronwailConfig` — the external skybox wind
  config parsing matches C.

The names are documentation. A reader scanning test names learns the parity
contract.

### The parity screenshot harness

`mise run parity-ref` captures deterministic reference screenshots from C
Ironwail. `mise run parity-go` captures matching GoGPU screenshots.
`mise run parity-compare` writes visual diffs and exits nonzero if any scene
exceeds the configured mismatch threshold. This is a real CI gate, not a
manual eyeball check. [#README](#readme)

### The brutalist jam maps as integration tests

As the prologue established, the Quake Brutalist Jam (qbj) map packs are the
project's de facto integration test suite. The qbj2 mod's `start` map — a
BSP2-format large map — surfaced the texture atlas overflow (the materials
buffer is hardcoded to 256 entries but the map has more), the lit-water fallback
mismatch, and the QCVM entity-sync pusher/non-pusher bug chain (the lift trigger
stack). The qbj3 mod's `qbj3_stickflip` map is the current priority stress case:
85,936 raw faces, 750 models, 106 textures, 1,295 lit-water faces, 228 sky
faces. [#Parity](#parity)

These maps are the unforgiving test. If a parity claim survives a qbj sweep,
it is real.

---

## The technology stack

| Layer | C Ironwail | ironwail-go |
| --- | --- | --- |
| Language | C99 | Go 1.26 |
| Renderer | OpenGL 1.x–3.x (legacy/core mix) | WebGPU via `gogpu` |
| Audio | SDL2 / DMA drivers | Oto (`ebitengine/oto/v3`) |
| Input | SDL2 (`in_sdl.c`) | gogpu input adapter |
| Windowing | SDL2 | gogpu `App` (Wayland/X11/native) |
| Math | `mathlib.c` + assembly (`math.s`) | `pkg/types` (pure Go `float32`) |
| Memory | Hunk / Zone / Cache | GC + slices + `sync.Pool` |
| Concurrency | SDL threads + mutexes | Goroutines + `sync` + `internal/async` |
| Data structures | Manual linked lists, arrays | `internal/engine` (generics: `Cache[T]`, `Registry[T]`, `Queue[T]`) |
| Platform | Win32 / DOS / Linux | Linux (Wayland/X11) via gogpu |

The Go runtime no longer carries parallel legacy renderer/input/audio variants.
The canonical gameplay stack is GoGPU rendering, renderer-provided input, and
Oto audio. There are no build tags selecting between renderers — the gogpu
renderer is always compiled. [#Comparison](#comparison) [#AGENTS](#agents)

---

## What this sets up

The divergences in this chapter — GC instead of Hunk, packages instead of flat
files, goroutines instead of SDL threads, WebGPU instead of OpenGL, a sync
layer instead of shared VM memory — are the root causes of every bug the rest
of this article covers. The renderer chapters (3 and 4) cover the
OpenGL-to-WebGPU leap. Chapter 5 covers the QCVM dual-storage sync problem.
Chapter 6 covers the gogpu-specific bugs that the pure-Go stack surfaced.

But first, Chapter 3 begins the renderer story: what the C renderer does, and
what replacing it with WebGPU means.

---

## References

<a name="readme"></a>[README] `README.md`, ironwail-go repository.

<a name="agents"></a>[AGENTS] `AGENTS.md`, ironwail-go repository.

<a name="comparison"></a>[Comparison] `docs/COMPARISON.md`, ironwail-go
repository.

<a name="bootseq"></a>[BootSeq] `docs/BOOT_SEQUENCE.md`, ironwail-go repository.

<a name="inputhandling"></a>[InputHandling] `docs/INPUT_HANDLING.md`,
ironwail-go repository.

<a name="hostdocs"></a>[HostDocs] `docs/internal/host.md`, ironwail-go
repository.

<a name="asyncdocs"></a>[AsyncDocs] `docs/internal/async.md`, ironwail-go
repository.

<a name="audiodocs"></a>[AudioDocs] `docs/internal/audio.md`, ironwail-go
repository.

<a name="enginedocs"></a>[EngineDocs] `docs/internal/engine.md`, ironwail-go
repository.

<a name="fsdocs"></a>[FSDocs] `docs/internal/fs.md`, ironwail-go repository.

<a name="typespkg"></a>[TypesPkg] `pkg/types/types.go`, ironwail-go repository.

<a name="parity"></a>[Parity] `docs/PARITY.md`, ironwail-go repository.

<a name="gogpuissues"></a>[GogpuIssues] `article/gogpu_issues.md` (transcript
of gogpu/gogpu issues, fetched 2026-07-27).
