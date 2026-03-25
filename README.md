# Ironwail Go

Ironwail Go is an exercise in porting the entire [Ironwail][1] Quake codebase
from C to Go, for the purposes of learning and education. It is an experiment
to get more experience with agentic coding and furthermore to learn more about
the Quake engine, game programming and indulge in a bit of nostalgia from my
school days of hacking together Quake mods and maps.

## Did you say agentic coding? Is this just AI slop?

Yes and no. A large portion of the codebase has been written entirely by AI
agents converting the C code to Go. However, I've been fairly hands on in
terms of planning and guiding that work, as well as reviewing and making
manual changes of my own.

In terms of tooling, mostly GitHub Copilot has been used, with a smattering of
other things, but the vast majority of agentic work has been done with
**Claude Opus 4.6** and **GPT-5.4**.

The [yggdrasil][3] repo has been invaluable for documenting and mapping the
growing codebase, providing structural overviews and cross-references that help
agents and humans alike navigate the port.

## Differences from Ironwail

Well, apart from the obvious that this is Go, rather than C, I'm building this
with the following changes:

- OpenGL/CGO as the default gameplay renderer/runtime
- gogpu/WebGPU as a secondary backend for non-parity experimentation
- Dividing the codebase up into packages
- Use Go stdlib for as much as possible, rather than custom implementations of
  things from the original C codebase

Additionally, I'm trying to build it as readable as possible, with extensive
commenting and to keep as much of the codebase in Go as practical. The
canonical OpenGL renderer currently requires CGo bindings, but the gameplay and
engine logic remain in Go and can still be understood without diving deeply
into C engine code.

## Project Status & Parity

The goal of this project is 100% behavioral parity with the original C
[Ironwail][1] engine on the OpenGL path. Regular parity audits are carried out,
but there is no concrete public tracking of gaps, differences, or known bugs at
this time.

For the Go-to-QuakeC toolchain and gameplay-language subset used by this
repository, see the [QGo / QuakeGo Guide](docs/QGO_QUAKEGO_GUIDE.md).

> **Note:** None of the original Ironwail C developers have reviewed or
> endorsed the work done in this repository.

## Building

The toolchain is built around [mise][2] which provides both the tooling and
the tasks for running tests, builds, etc.

You can see what tasks are available to run using `mise tasks`

The canonical parity/build path is the CGO/OpenGL runtime:

- `mise run test`
- `mise run build-cgo`
- `mise run smoke-map-start`
- `mise run smoke-cgo-map-start`
- `mise run parity-ref`
- `mise run parity-go`
- `mise run parity-compare`

The gogpu tasks remain available for secondary-backend work, but they are no
longer the primary parity gate.

The parity screenshot harness now targets the CGO/OpenGL path by default and
acts like a real gate:

- `mise run parity-ref` captures deterministic reference screenshots from C
  Ironwail into `testdata/parity/reference/`
- `mise run parity-go` captures the matching Go CGO/OpenGL screenshots into
  `testdata/parity/go/`
- `mise run parity-compare` writes visual diffs to `testdata/parity/diff/` and
  exits nonzero if captures are missing or if any scene exceeds the configured
  mismatch threshold

## Debug Telemetry

The server now exposes an opt-in debug telemetry mode for following trigger,
physics, and QuakeC activity from the in-game console/log output. This is aimed
at parity debugging and engine-side investigation rather than end-user
gameplay.

### Debug CVars

| CVar | Default | Purpose |
| --- | --- | --- |
| `sv_debug_telemetry` | `0` | Enables engine-side server telemetry events. |
| `sv_debug_telemetry_events` | `all` | Event mask. Accepts `all`, `none`, a numeric mask such as `0x21`, or a token list such as `trigger,touch,blocked,physics,frame,qc`. |
| `sv_debug_telemetry_classname` | `""` | Optional classname filter. Exact matches are supported, and glob patterns such as `trigger_*` also work. |
| `sv_debug_telemetry_entnum` | `-1` | Optional entity-number filter. Use `-1`/`all` for everything, or lists/ranges such as `1,4-6`. |
| `sv_debug_telemetry_summary` | `1` | Per-frame summary mode: `0` off, `1` only frames with matching events, `2` every frame. |
| `sv_debug_qc_trace` | `0` | Enables QuakeC call tracing routed through the server telemetry output. |
| `sv_debug_qc_trace_verbosity` | `1` | QuakeC trace verbosity ceiling. `1` logs function enter/leave events, `2` also includes builtin calls. |

### Scope

Current engine-side telemetry focuses on server execution paths that are useful
when debugging map logic and parity issues:

- frame boundaries and `StartFrame`
- entity `think` execution
- touch/impact callbacks
- trigger `touchLinks` scans and callback dispatch
- pusher/blocker physics callbacks
- QuakeC call chains executed through the server's QC wrapper

QC profiling counters are already implemented and available through the
`profile` host command (top 10 functions, reset-on-read, local server only).
That command is separate from telemetry tracing. This means QC profiling is in
scope and considered implemented for parity purposes; there is no current plan
to add a full statement-by-statement VM profiler as part of this telemetry
feature.

QC tracing is not a generic whole-engine instruction trace. It follows QuakeC
function entry/leave activity and optional builtin calls for server-side paths
that execute through `executeQCFunction`.

### Output Behavior

Telemetry lines are emitted with a `[svdbg ...]` prefix through the normal
console/log path. Event lines include frame/time metadata plus the best current
entity snapshot:

```text
[svdbg frame=12 time=4.200 kind=trigger] ent=57 classname="trigger_once" targetname="door1" target="door1" model="*3" origin=(256.0 128.0 32.0) touchlinks callback begin other=1 fn=42
```

QC trace lines add call depth, phase, and the resolved function name/index:

```text
[svdbg frame=12 time=4.200 kind=qc depth=2 phase=enter fn=trigger_relay[#17]] ent=57 classname="trigger_once" targetname="door1" target="door1" model="*3" origin=(256.0 128.0 32.0) self=57 other=1 other_classname="player"
```

Per-frame summaries are controlled by `sv_debug_telemetry_summary`:

- `0`: no summary line
- `1`: summary only when at least one matching event was logged
- `2`: summary for every frame, including quiet frames

Example summary:

```text
[svdbg frame=12 time=4.200 dt=0.050] summary total=7 qc=2 counts=frame=2,trigger=2,think=1,qc=2
```

### Filters and Common Usage

Common filters can be combined:

```text
sv_debug_telemetry 1
sv_debug_telemetry_events trigger,qc,frame
sv_debug_telemetry_classname trigger_*
sv_debug_telemetry_entnum 57,60-62
sv_debug_telemetry_summary 1
sv_debug_qc_trace 1
sv_debug_qc_trace_verbosity 2
```

Notes:

- token separators for `sv_debug_telemetry_events` include commas, pipes,
  plus signs, and whitespace
- classname matching is case-insensitive
- entity filters are explicit lists/ranges, not glob patterns
- QC trace output is still subject to the `qc` event mask, so masking out
  `qc` disables trace output even if `sv_debug_qc_trace` is `1`
- the `use` event token is part of the mask parser, but the current server-side
  instrumentation is centered on frame/trigger/touch/think/blocked/physics/qc
  paths

### Limitations and Noise Caveats

- This is intentionally verbose and can produce a lot of output in busy maps,
  especially with `sv_debug_telemetry_summary 2` or QC builtin tracing enabled.
- The current coverage is server-centric. It does not attempt to trace every
  renderer, client, or filesystem path.
- QC trace output reports function-oriented events (`enter`, `leave`, and
  optional `builtin`) rather than a full statement-by-statement VM trace.
- Trigger-heavy maps can still be noisy even with classname filters, because
  begin/end bookkeeping and callback messages are logged around each observed
  path.
- Output is emitted to the console/log stream only; if you want to keep a
  capture, redirect stdout/stderr or save the console log externally.

[1]:https://github.com/andrei-drexler/ironwail
[2]:https://mise.jdx.dev
[3]:https://github.com/krzysztofdudek/Yggdrasil
