# Spike S1: GPUView 3D viewport feasibility (stretch scope)

**Status:** RESOLVED (2026-08-19)
**Bead:** ironwail-go-teh.5 (M1.2)
**ADR:** 0002 (stretch milestone S)
**Gap:** 16

## Question

Can the engine's 3D world be rendered into a `core/gpuview` texture so the
gogpu/ui layer can composite UI over a live 3D viewport (the #468 BYO-kit
ideal)? This is explicitly a stretch milestone, NOT MVP scope.

## Findings

1. **The engine already has offscreen scene-target machinery.** When waterwarp
   is active, the world renders into `WorldRenderTextureView` and is composited
   back to the surface (`enableSceneRenderTarget` /
   `compositeSceneRenderTarget` in `renderer_gogpu_warpscale.go`, gated by
   `sceneTargetActive` in `renderer_gogpu_frame.go`). This proves the world
   can target an offscreen texture without retargeting the whole renderer.
2. **GPUView owns an offscreen texture and calls `OnRender(view)`.** The
   widget creates a `gpucontext.TextureView` via `GPUTextureProvider` and
   hands it to the external renderer; `Continuous(true)` schedules per-frame
   re-render (research 0002 §6).
3. **The compositing caveat is the blocker for the engine-owned loop.** GPUView
   is only blitted by the `desktop` render loop (which owns the whole window),
   not by the standalone compositor or the engine's own render loop. Since
   ironwail-go keeps its own `gogpu.App`/render loop (Architecture A, ADR-0002),
   the engine would have to blit the GPUView texture itself via
   `DrawGPUTexture`/`DrawGPUTextureBase` — the engine already has
   `SurfaceView`, `DrawTextureEx`, and full `gpucontext` access, so this is
   feasible but requires wiring the widget's texture into the engine's
   composite pass.
4. **Alternative without GPUView:** the engine's existing scene-target
   machinery already produces an offscreen world texture for waterwarp; the
   same texture could be composited under the widget canvas today (the
   `canvas_bridge` present path) without adopting `core/gpuview`.

## Decision

**Feasible, but deferred as stretch milestone (S).** The engine can render the
world into an offscreen texture (proven by the waterwarp scene target), and a
`core/gpuview` widget can present it; the remaining work is blitting the
widget's texture into the engine's own composite pass (the desktop-loop caveat).
This is NOT in the MVP: the MVP composites the widget canvas over the already-
rendered engine surface, which is simpler and sufficient for behavioral parity.
If adopted later, either (a) wire `core/gpuview`'s texture into the engine
composite, or (b) reuse the existing waterwarp scene target under the widget
canvas.

## Acceptance

- Feasibility note: DONE (this note).
- Stretch milestone (S) definition: DONE — "GPUView 3D viewport" is a
  documented stretch, not MVP scope (ADR-0002).
- No code changes: DONE.
