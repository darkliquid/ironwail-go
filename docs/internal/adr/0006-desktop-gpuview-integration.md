# ADR-0006: Desktop/GPUView Integration (Architecture B) — Supersedes ADR-0002

**Status:** Accepted (2026-08-19)
**Deciders:** darkliquid (v2 Stage 1: desktop/GPUView), lifecycle driver
**Date:** 2026-08-19
**Supersedes:** ADR-0002 (Architecture A engine-owned bridge — abandoned)
**Related:** IRONWAIL-SPEC-002 §1.3/§3.1-3.3/§5.1; research 0006;
gap log rows 3, 10

## Context and Problem Statement

SPEC-001 used Architecture A (engine-owned window/render loop; a gg-canvas
bridge composited inside the engine's RenderFrame). It failed: canvas sizing
fought the framework, the composite could clear the world, and input
invalidation was fragile. SPEC-002 pivots to Architecture B: `desktop.Run`
owns the window/render loop and the world renders into a `core/gpuview`
texture the desktop compositor blits as the base layer under the UI widgets.
The engine currently renders the world directly to the swapchain surface via
its own `OnDraw`.

## Decision Drivers

- Visual fidelity and framework-native behavior (the #468 BYO-kit path).
- The UI must never clear the world (hard constraint G10).
- `gogpu.App.OnDraw` is single-slot; `desktop.Run` must be the sole owner on
  the `ui_backend=1` path (research 0006 §1).
- The engine already has offscreen-target machinery (waterwarp scene target)
  proving the world can render into an offscreen texture (research 0006 §4).
- Screenshot/parity/software paths stay legacy-only on `ui_backend=0`.

## Considered Options

1. **Architecture A (v1 engine-owned bridge)** — rejected: it is the failed
   SPEC-001 approach; canvas sizing/composite/input fought the framework.
2. **Architecture B: `desktop.Run` + GPUView** — chosen. `desktop.Run`
   owns the swapchain/render loop; the engine builds the ui app with a
   `core/gpuview` widget as the base of the root; the engine implements
   `gpuview.OnRender(view)` to render the world into the gpuview's offscreen
   `gpucontext.TextureView`; the desktop compositor blits the gpuview texture
   as an `ExternalTextureLayer` (base) with UI widgets on top. The world is
   never cleared (the compositor blits, it does not LoadOpClear the surface).
   Pros: framework-native compositing, visual fidelity, #468 pattern. Cons:
   the engine's world render must be retargeted from the surface to the
   gpuview view (renderer surgery, reusing the waterwarp scene-target
   machinery); `desktop.Run` is the sole OnDraw owner on path 1; wasm can't
   use `desktop.Run` (path 1 is desktop-only, wasm stays legacy).
3. **Hybrid (engine loop for world, desktop.Run for UI overlay)** — rejected:
   the single-slot OnDraw conflict means `desktop.Run` would still overwrite
   the engine's OnDraw; there is no clean split without the gpuview texture.

## Decision Outcome

Architecture B: `desktop.Run(gogpuApp, uiApp)` on `ui_backend=1`. The engine
does not register its own OnDraw on path 1; it builds the ui app with a
`core/gpuview` widget as the base, implements `gpuview.OnRender(view)` to
render the world into the gpuview texture (reusing the offscreen-target
machinery), and lets the desktop compositor blit the gpuview texture under the
UI widgets. The `quakui.Host` adapter exposes the world texture as a
`gpucontext.TextureView` and a `RenderIntoWorldTexture(view)` callback
implemented in `internal/renderer` and routed through `internal/game`.

- **Positive:** framework-native compositing; UI never clears the world;
  visual fidelity with real LMP pics; #468 validation case.
- **Negative:** renderer surgery to retarget the world render; `desktop.Run`
  owns the loop on path 1 (wasm/software stay legacy); a CPU/GPU copy of the
  gpuview texture each frame.

## Links

- IRONWAIL-SPEC-002 §1.3/§3.1-3.3/§5.1; research 0006; ADR-0007 (input),
  ADR-0009 (isolation); gap log 3, 10

## Review Log

### Stage 5 — Review 2 (2026-08-19)

Verdict: APPROVED. Supersedes ADR-0002 honestly (the v1 approach failed and is
replaced, not amended). Negative consequences (renderer surgery, loop
ownership) are specific. Consistent with SPEC-002 §3.3 and research 0006.
 No further findings.
