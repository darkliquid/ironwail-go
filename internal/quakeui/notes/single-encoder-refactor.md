# Single-Encoder Refactor: Using a Shared Finish/Submit (g3d model)

**Status:** Analysis (2026-08-22)
**Owner:** v4 Scenario A composite (ADR-0011)
**Question:** Can we refactor the engine's encoder loop so a single finish/submit
(gg's `FlushGPUWithView` or gogpu's `submitFrameEncoder`) owns the frame, instead
of the engine doing 17 independent encoder+submit pairs?

## 1. The current engine model

The renderer creates **17 command encoders across 11 files**, each paired with
its own `queue.Submit`:

```
world_render.go        (world, external sky, shared depth clear)
world_alias.go         (alias models)
world_brush_render.go  (brush entities, sky, liquid)
world_decal.go         (decals)
world_sprite.go        (sprites)
world_translucent.go   (alpha-test, late translucent)
warpscale.go           (scene clear, scene composite)
overlay_composite_gogpu.go (overlay HAL composite)
```

gogpu's shared `ws.frameEncoder` is **never used** by the engine. gogpu's
`submitFrameEncoder` (called in `endFrameForSurface`) submits nothing (encoder
nil) — the engine is the **sole submitter**; gogpu only presents the swapchain
and releases `currentView` after present.

## 2. Why gg's `FlushGPUWithView` is NOT the right finish/submit point

`FlushGPUWithView` flushes **gg's own GPU shapes** (SDF paths, glyph masks,
image quads) that gg recorded into its session. It is designed to be called
*mid-frame*, possibly **multiple times per frame** (the `prevCmdBufs` comment
explicitly handles "multiple FlushGPUWithView calls within a single frame").
Each call finishes + submits the shared encoder.

If the engine recorded its world passes into the shared encoder and then
called `FlushGPUWithView`:
- gg would finish+submit the encoder (including the world passes — fine),
- but gogpu's `submitFrameEncoder` would then try to finish+submit an
  **already-finished encoder** (invalid), and
- gg's `prevCmdBufs` deferred-free (next frame boundary) would still apply.

`FlushGPUWithView` is a "flush my gg drawing" primitive, not a "submit the
whole frame" primitive.

## 3. The correct refactor: gogpu's `submitFrameEncoder`, not gg's flush

The g3d model is: **one encoder, one submit, one owner**. The engine should
record ALL passes into gogpu's shared `ws.frameEncoder` (via
`dc.ctx.CommandEncoder()`), and let gogpu's `endFrameForSurface` →
`submitFrameEncoder` finish+submit+present+release in the correct order.

Concretely:

1. Replace each `encoder, err := device.CreateCommandEncoder(...)` +
   `queue.Submit(cmdBuffer)` pair with:
   ```go
   encoder := dc.ctx.CommandEncoder()   // gogpu's shared frame encoder
   renderPass, err := encoder.BeginRenderPass(...)
   ... renderPass.End() ...
   // NO Finish/Submit — gogpu's submitFrameEncoder does it once at frame end.
   ```
2. The passes are already self-contained (`BeginRenderPass`...`End`), so they
   can be re-targeted to the shared encoder without changing their render-pass
   structure.
3. gogpu's `endFrameForSurface` already calls `submitFrameEncoder` (finish +
   submit) then `present()` then `releaseFrame()` — the correct order, no race.
4. The overlay composite (`renderOverlayTextureHAL`) also becomes a pass into
   the shared encoder instead of its own encoder+submit.

## 4. What this fixes

- **The GPU-lifetime race:** one encoder, one submit, one owner (gogpu). No
  independent submit can reference a surface image that gogpu released.
- **The gg GPU-direct path becomes usable:** with the engine recording into
  the shared encoder, gg's `FlushGPUWithView` could ALSO record into it (via
  the shared-encoder path) and gogpu submits once — the g3d fullscreen-overlay
  contract. (Still requires gg's flush to not finish+submit the shared encoder
  itself, or the engine simply never calls gg's GPU-direct flush and uses the
  readback/offscreen path.)

## 5. Effort / risk

- ~17 encoder sites to re-target; mechanical but touches every render path.
- Render passes must not assume a fresh encoder per pass (e.g. no
  `encoder.Finish()` between passes; `BeginRenderPass`/`End` on the same
  encoder is valid WebGPU).
- The scene-target path (offscreen texture for waterwarp) uses its own
  target — those passes still target the scene texture, but can share the
  frame encoder.
- The `flushClear`/`hasPendingClear` deferred-clear path in gogpu's
  `CommandEncoder()` must be respected (it flushes a pending clear into the
  shared encoder before external passes).

## 6. Recommendation

This is the correct long-term architecture (single-encoder host, gogpu owns
submit/present). It is a renderer-wide refactor (~17 sites), not a small
change. It is the prerequisite for re-enabling gg's GPU-direct composite
without the swapchain race. Until then, the CPU-readback blit stays.

## 7. Cross-references

- gogpu `Context.CommandEncoder()` / `ensureFrameEncoder()` /
  `submitFrameEncoder` / `endFrameForSurface`
- gg `render_session.go` `FlushGPUWithView` GPU-direct flush + `prevCmdBufs`
- g3d `examples/fullscreen-overlay` (single-submit model)
- `notes/gpu-direct-root-cause.md` (the race that this refactor eliminates)
