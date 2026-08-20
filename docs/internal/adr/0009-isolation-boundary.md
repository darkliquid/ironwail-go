# ADR-0009: Self-Contained `internal/quakeui` with a Narrow Engine Adapter

**Status:** Accepted (2026-08-19)
**Deciders:** darkliquid (v2 Stage 1: self-contained), lifecycle driver
**Date:** 2026-08-19
**Related:** IRONWAIL-SPEC-002 §1.4/§3.1/AC7; gap log row 5

## Context and Problem Statement

SPEC-001 leaked UI wiring into `internal/game`: host construction, `ui_backend`
branching, input raw sinks, CSQC fallback, and `UIHost`/root fields all lived in
game code. SPEC-002 requires the UI subsystem to be self-contained with clean
package boundaries (AC7): `internal/quakeui` must not import `internal/game`,
`internal/renderer`, or `internal/renderer/*`.

## Decision Drivers

- Clean package boundaries (the user's explicit v2 requirement).
- The UI needs the legacy state machines (`menu.Manager`, `console.Console`,
  `hud.State`) read-only, and the engine's world texture + cvars + command
  text + sound.
- `internal/quakeui` must not import engine internals; the world texture is a
  `gpucontext.TextureView` (a gogpu type, not an engine type).

## Considered Options

1. **UI code in `internal/game` (v1)** — rejected: it is the failed SPEC-001
   approach; game wiring leaks.
2. **Self-contained `internal/quakeui` + a `quakeui.Host` adapter** — chosen.
   `internal/quakeui` imports only gogpu/ui, the legacy state machines
   (read-only via accessors), and `internal/image`. It defines a `Host`
   adapter interface whose types are gogpu/gpucontext types (a
   `gpucontext.TextureView` for the world, plain value reads for cvars, and
   `func(string)` sinks for command text/sound). `internal/game` implements
   `quakeui.Host` and calls `quakeui.Run(host)`. The engine renderer exposes
   `RenderIntoWorldTexture(view)` implemented in `internal/renderer` and
   routed through `internal/game`'s adapter — never imported by
   `internal/quakeui`. Pros: clean boundary; the UI is testable in isolation;
   the engine touches one function. Cons: the adapter must abstract enough of
   the engine surface (world texture, cvars, input keydest, command text,
   sound) without leaking engine types.
3. **Adapter package `internal/uiadapter`** — rejected: an extra package with
   no benefit; the `Host` interface inside `internal/quakeui` is the adapter.

## Decision Outcome

`internal/quakeui` is self-contained. It defines `quakeui.Host` (world texture
as `gpucontext.TextureView`, cvar reads as values, command text/sound sinks as
`func(string)`, input keydest as a plain enum) and `quakeui.Run(host)`.
`internal/game` implements the adapter and calls `quakeui.Run`. No `internal/game`
or `internal/renderer` import inside `internal/quakeui`; the engine renderer's
`RenderIntoWorldTexture(view)` is routed through the adapter.

- **Positive:** clean boundary; UI testable in isolation; one engine touch
  point; AC7 verifiable by import graph.
- **Negative:** the adapter must abstract the engine surface without leaking
  engine types; some plumbing to route the world texture.

## Links

- IRONWAIL-SPEC-002 §1.4/§3.1/AC7; gap log 5; ADR-0006 (desktop/GPUView),
  ADR-0007 (input)

## Review Log

### Stage 5 — Review 2 (2026-08-19)

Verdict: APPROVED. The adapter contract is pinned (world texture as
`gpucontext.TextureView`, no engine types). AC7 is verifiable by import graph
check. Consistent with SPEC-002 §3.1 and the R1.1 fix. No further findings.
