# RESEARCH-0006: desktop.Run Integration with Engine World (GPUView)

- **Status:** Delivered
- **Owner:** ironwail-go-5fi (spike bead)
- **Date:** 2026-08-19
- **Blocks:** `.ai-dlc/ui-rewrite-v2/gaps.md #3` (R1.3)

---

## Research Question

How does `desktop.Run(gogpuApp, uiApp)` take over the window/render loop
without breaking the engine's world render, and how does a `core/gpuview`
widget receive the world so the UI composites over it (never clearing it)?

## Background & Constraints

- SPEC-002 §3.3 / §5.1 requires Architecture B: `desktop.Run` owns the
  swapchain/render loop; the world renders into a `core/gpuview` texture; the
  desktop compositor blits it as the base layer with UI widgets on top.
- Hard constraint G10: the UI must never clear the world.
- The engine currently renders the world directly to `dc.ctx.SurfaceView()`
  (the swapchain) via `renderWorld` → `renderWorldInternal`
  (`renderer_gogpu_frame.go:642`). It already has offscreen-target machinery
  (`WorldRenderTextureView`, `enableSceneRenderTarget`,
  `compositeSceneRenderTarget` in `renderer_gogpu_warpscale.go`) used for
  waterwarp, proving the world can target an offscreen texture.

## Investigation Findings

### 1. desktop.Run owns the loop; it cannot coexist with the engine's own OnDraw

`desktop.Run(gogpuApp, uiApp)` (desktop/desktop.go:58) calls `gogpuApp.OnDraw(rl.draw)`
and runs `gogpuApp.Run()`. `gogpu.App.OnDraw` is a **single-slot** setter
(app.go:182) — the engine's own `Renderer.OnDraw` registration would be
overwritten. Therefore on the `ui_backend=1` path, **desktop.Run must be the
sole OnDraw owner**; the engine's world render must be invoked from inside the
ui render loop, not from a separate engine OnDraw.

### 2. The gpuview widget owns an offscreen texture and calls OnRender(view)

`core/gpuview.Widget` (core/gpuview/widget.go):
- On first `Draw`, allocates an offscreen `gpucontext.TextureView` via the
  Context's `widget.GPUTextureProvider` (`CreateGPUTexture`).
- When dirty or `Continuous(true)`, calls `cfg.onRender(w.texture)` — the
  external renderer (the engine) issues GPU commands into that texture view.
- Implements `externalTextureWidget` (Texture() + ViewportSize()), so the
  desktop layer tree appends an `ExternalTextureLayer` (app/layer_tree.go:322)
  that the compositor blits via `cc.DrawGPUTexture` (desktop/desktop.go:876).

### 3. Composite path: gpuview texture is the base, UI widgets on top

The `ExternalTextureLayer` is a sibling of the widget tree's `PictureLayer`
inside the same `OffsetLayer`. The compositor draws the external texture
first (base), then the UI widgets. Because the gpuview texture is an
offscreen render target the engine fills, and the compositor blits it with
`DrawGPUTexture` (no LoadOpClear of the surface), **the world is never
cleared by the UI composite** — satisfying G10.

### 4. The engine must retarget the world render into the gpuview view

The engine's `renderWorld` currently targets `dc.ctx.SurfaceView()`. To feed
the gpuview, the engine's `OnRender(view)` callback must render the world into
`view` (a `gpucontext.TextureView`) instead of the surface. The existing
waterwarp scene-target machinery (`enableSceneRenderTarget` sets
`dc.sceneRenderTarget = r.resources.WorldRenderTextureView`) is the precedent:
the world can render into an offscreen view. The work is to make the render
path accept an arbitrary external texture view (the gpuview's) as the target
for the world/entities/polyblend passes, then present nothing (the compositor
blits the gpuview texture).

### 5. Screenshot / parity / software paths

`-screenshot` and the software renderer path are legacy-only (no gpuview
canvas). They stay on `ui_backend=0`. AC5 compares via in-window captures at
`ui_backend=1`; the screenshot command remains the legacy parity oracle.

### 6. WASM: desktop.Run is native-desktop-only

`desktop.Run` calls `gogpuApp.Run()` (desktop/desktop.go:100), which starts
the blocking main loop. The engine's wasm path does NOT use `App.Run`'s loop:
it uses `StepWasmFrame` (a requestAnimationFrame driver) as the single driver
and disables `App.Run`'s `onUpdate` to avoid double-stepping
(`renderer_gogpu_runtime.go:283-289`). The browser platform's `WaitEvents` is
a no-op — the loop must cooperate with rAF, not block
(`platform_browser.go:79-95`). Therefore `desktop.Run` + GPUView is
**native-desktop-only**; the wasm build stays on `ui_backend=0` (legacy),
consistent with ADR-0006's negative consequence and SPEC-002 AC3.

## Recommended Resolution

- On `ui_backend=1`, the engine does NOT register its own `OnDraw`. Instead it
  builds the ui app with a `core/gpuview` widget as the base of the root, and
  calls `desktop.Run(gogpuApp, uiApp)`.
- The engine implements `gpuview.OnRender(view)`: it retargets the world
  render (world + entities + polyblend) into `view` via a new renderer method
  (e.g. `Renderer.RenderIntoView(view gpucontext.TextureView, state)`), reusing
  the offscreen-target machinery. `Continuous(true)` drives per-frame render.
- The UI widgets (menu/console/HUD) composite above the gpuview in the same
  root; the compositor blits the gpuview texture as the base (never cleared).
- The `quakui.Host` adapter (SPEC-002 §3.1) exposes the world texture as a
  `gpucontext.TextureView` and a `RenderIntoWorldTexture(view)` callback
  implemented in `internal/renderer` and routed through `internal/game`.
- **WASM:** `ui_backend=1` is gated off on `GOOS=js` (log + legacy path), per
  ADR-0006 and SPEC-002 AC3.

## Open Questions / Follow-ups

- Whether the engine's polyblend/waterwarp/scene-composite passes can all
  target the gpuview view without per-pass changes, or whether only the world
  pass is retargeted and the rest stay surface-bound. This is implementation
  detail for the plan; the waterwarp precedent suggests full retargeting is
  feasible.

## Source Index

- gogpu/ui v0.1.54: desktop/desktop.go:58,212,876,1056; app/layer_tree.go:10-30,322-360;
  core/gpuview/widget.go:24-31,63-91,129-153.
- ironwail-go: internal/renderer/renderer_gogpu_frame.go:642;
  internal/renderer/renderer_gogpu_warpscale.go:301-469;
  internal/renderer/renderer_gogpu_world_resources.go:65.
- gogpu v0.53.0: app.go:182 (single-slot OnDraw).
