# ADR-0002: Engine-Owned Integration (Architecture A) + Per-Surface Widget Roots

**Status:** Accepted (2026-08-18)
**Deciders:** darkliquid (Stage 1: A+C), lifecycle driver
**Date:** 2026-08-18
**Related:** IRONWAIL-SPEC-001 §1.2/§3.1-3.3/§5.1; research 0002 §5-6, 0004 §3;
gap log rows 6, 7, 14

## Context and Problem Statement

gogpu/ui's `desktop.Run` owns the window and render loop. ironwail-go already
owns its `gogpu.App` (swapchain, `OnDraw`, input backend), and has a
software/screenshot/wasm fallback that must keep working. Reworking the whole
renderer to render the 3D world into a GPUView texture (Architecture B) is the
#468-endorsed ideal but major surgery. How do we integrate without breaking
the engine's ownership and its renderer/screenshot/wasm paths?

## Decision Drivers

- Engine owns `gogpu.App`, swapchain, render loop, input pipeline
  (research 0004 §1-2; single-slot OnDraw/EventSource).
- Parity, screenshot, headless, and wasm paths must keep working.
- Input latching/mouse-look regression tests must keep passing.
- Real tradeoff exists vs Architecture B — see open question in plan.

## Considered Options

1. **Architecture B: `desktop.Run` + GPUView** — pros: full layer-tree
   compositing, GPUView 3D-in-texture, #468's exact BYO-kit pattern. Cons:
   world currently renders directly to the surface; retargeting world +
   entities + waterwarp + polyblend into the gpuview texture is significant
   renderer surgery; `desktop.Run` blocks (must decompose to per-frame);
   screenshot/parity/software paths and input single-slot conflict all need
   rework; wasm can't use `desktop.Run`.
2. **Architecture D: ui owns EventSource** — rejected: overwrites the
   engine's input backend; no "was event consumed" API today
   (research 0004 §3-D).
3. **Architecture A (engine-owned) + C (engine input router forwards into
   ui tree)** — chosen. The engine keeps window/swapchain/render loop; per
   frame: `uiHost.Frame()` → `uiApp.Frame()` +
   `Window().DrawTo(render.NewCanvas(cc, w, h))` over a gg canvas bridged
   from the engine's `gogpu.Context`, then composited onto the engine
   surface. Input: the engine's `input.System`/KeyDest remains
   authoritative; a gateway EventSource shim feeds the uiApp; unconsumed
   events stay in the game.

## Decision Outcome

Architecture A+C, with per-surface widget roots for stacking (gap G.14):
each surface (menu, console, HUD) is a root widget keyed by active state;
when surfaces overlap (console forced-up over menu at boot; menu over frozen
world + HUD), the roots are layered (either stacked in one root Box or via
`OverlayManager`), never mutually exclusive. This matches the legacy overlay
order (console below menu; HUD below both) and preserves `RenderModeHostManaged`
(no double clear).

- **Positive:** renderer/screenshot/wasm paths untouched; input tests keep
  passing; GPUView deferred to a later spike without blocking MVP; matches
  the #468 BYO-kit imports (`app`, `widget`, no `core/*`).
- **Negative vs B:** an extra CPU/GPU copy (gg canvas → engine surface) and
  two retained-mode systems per frame; the escape hatch
  (`Context() *gg.Context`) is needed for precise draws until
  `ImageRegionDrawer` lands. B remains the documented goal architecture for
  the 3D viewport case (plan "stretch" task, out of MVP).

## Links

- research 0004 §3 (architectures A-D), 0002 §5-6 (BYO-kit, GPUView)
- IRONWAIL-SPEC-001 §3.1-3.3, §5.1; gap log 6, 7, 14

## Review Log

### Stage 5 — Review 2 (2026-08-18)

Verdict: APPROVED WITH ONE CLARIFICATION. A vs B tradeoffs honest (extra
copy, two retained systems vs renderer surgery). Clarification: GPUView "goal
architecture" must not silently become MVP scope — plan marks it explicit
stretch task with its own spike (G.15). "Only one active typically" stacking
model fixed to layered roots (gap 14). Consistent with spec §3.1-3.3.
