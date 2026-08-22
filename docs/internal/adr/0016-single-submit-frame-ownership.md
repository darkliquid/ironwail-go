# ADR-0016: Single-Submit Frame Ownership (g3d Model)

**Status:** Proposed
**Deciders:** darkliquid (v4 Stage 2: single-encoder), lifecycle driver
**Date:** 2026-08-22
**Related:** SPEC-005 §2/§3/§5/AC1-AC3; ADR-0011 (Scenario A composite);
research 0009 (v4 findings); `internal/quakeui/notes/single-encoder-refactor.md`;
`internal/quakeui/notes/gpu-direct-root-cause.md`

## Context and Problem Statement

The engine-owned UI overlay composites the widget tree over the 3D world inside
gogpu's `OnDraw`. The renderer issues **17 independent
`CreateCommandEncoder` + `queue.Submit` pairs** per frame (world, aliases,
brushes, sky, liquid, decals, sprites, translucent, scene composite, overlay),
and gogpu's shared frame encoder is never used — the engine is the sole
submitter and gogpu only presents.

This multi-submit model is the root of the swapchain-lifetime race: an
independent submit can reference a surface image that gogpu has already
released (present + `releaseFrame()` → `currentView.Release()`), producing
`"command buffer references destroyed texture"` on submit. The gg GPU-direct
flush (`FlushGPUWithView`) makes it worse: it finishes+submits the shared
encoder itself and defers command-buffer free across frames.

## Decision Drivers

- Eliminate the swapchain-lifetime race structurally (one submit, one owner).
- Re-enable the gg SDF accelerator + GPU-direct overlay composite (the goal of
  the v4 rewrite, previously abandoned for the CPU-readback blit).
- Keep the engine-owned loop (native + WASM + headless); no `desktop.Run`.
- Preserve the world-preserve seam (`MarkPreserveContent`, ADR-0011) and the
  decoupled input router (ADR-0012).

## Considered Options

1. **Single-submit frame ownership (g3d model)** — chosen. All engine passes
   record into gogpu's shared frame encoder (`dc.ctx.CommandEncoder()`);
   gogpu's `endFrameForSurface` → `submitFrameEncoder` finishes, submits,
   presents, and releases once. The overlay composite uses offscreen
   composition (gg renders to a canvas-owned texture; gogpu blits it in its
   own submit). Pros: one submit, one owner, no race; re-enables the
   accelerator; gogpu-native. Cons: renderer-wide refactor (~17 sites);
   gg's `FlushGPUWithView` must not be used (it submits).
2. **Keep multi-submit + CPU-readback blit (status quo)** — rejected: the
   world race remains (research 0009) and the accelerator stays disabled.
3. **Keep multi-submit + record-only gg flush** — rejected: requires a gg
   upstream change that does not exist; forking/patching gg is a maintenance
   burden.
4. **`desktop.Run` + GPUView (v2)** — rejected (ADR-0011): inverts control,
   breaks WASM, GPUView texture churn.

## Decision Outcome

Adopt the single-submit model. The engine records all render passes into
gogpu's shared frame encoder; gogpu owns finish/submit/present/release. The
overlay composite uses offscreen composition (gg `FlushPixmap` →
`PixmapTextureView` → gogpu blit into the shared encoder), re-enabling the gg
SDF accelerator without any gg submit. The CPU-readback blit remains only as
the software/headless fallback.

- **Positive:** structurally race-free; accelerator re-enabled; gogpu-native;
  engine-owned loop preserved.
- **Negative:** renderer-wide refactor (~17 sites, 11 files); gg's
  `FlushGPUWithView`/`canvas.Render`/`RenderDirect` must be avoided for the
  overlay (they submit the shared encoder); offscreen-composition adds one
  texture blit per frame (a small copy, not a readback).

## Links

- SPEC-005 §2/§3/§5/AC1-AC3
- ADR-0011 (Scenario A composite — amended for offscreen composition)
- Research 0009; `notes/single-encoder-refactor.md`; `notes/gpu-direct-root-cause.md`
- gogpu `context.go` `CommandEncoder()` / `ensureFrameEncoder()` /
  `submitFrameEncoder` / `endFrameForSurface`

## Review Log

### Stage 5 — Review 2 (2026-08-22)

Verdict: APPROVED. Consistent with SPEC-005 (offscreen-composition, no
`canvas.Render`/`RenderDirect`/`FlushGPUWithView` for the overlay). The
"one blit per frame" negative is a full-screen texture sample + composite (a
real pass, not a readback) — acceptable. ADR-0011 amended to supersede its
body's stale `canvas.RenderDirect` composite description.
