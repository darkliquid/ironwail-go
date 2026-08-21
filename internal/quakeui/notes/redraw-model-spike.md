# Redraw-Model Spike (G4, gap 7) — M0.3

**Status:** Decision recorded (spec §2#10)
**Date:** 2026-08-21
**Owner:** M0.3 (v4 milestone)
**Inputs:** SPEC-004 §2#10, plan M0.3, gap log row 7 ("Retained-mode
invalidation vs continuous game rendering", ELICIT, med), ADR-0010/0011
overlay seams, `internal/quakeui/overlay.go` v3 implementation.
**Scope:** Docs only (no code). Decision: **accept full redraws per frame.**
A continuous-render mode is NOT adopted because the spike shows no material
overhead reduction that survives the v4 GPU-canvas architecture; adopting it
would break the widget tree's retained-mode guarantees for no measurable gain.

## 1. What was measured (v3 widget tree)

The v3 path (`OverlayRenderer.DrawOverlay`, `internal/quakeui/overlay.go`)
ran at 60 FPS under the engine loop:

- **Every frame:** `r.dc.Clear()` (entire CPU canvas) → `render.NewCanvas`
  → `stack.Layout(...)` + `stack.Draw(ctx, canvas)` (full widget-tree
  layout + redraw) → `r.dc.Image()` (`*image.RGBA` readback) →
  `target.DrawRGBA(0, 0, rgba)` → `renderOverlayTextureHAL` full-width
  `tex.UpdateData(img.Pix)` (full-frame GPU texture upload).
- **Widget tree invalidation exists but is orthogonal:** the roots call
  `SetNeedsRedraw(true)` / `InvalidateScene()` / `ctx.Invalidate()` on
  state changes (menu `menu.go:69,97,172`, console `console.go:77,92,390`).
  The gogpu/ui `widget.*` primitives maintain retained-mode invalidation
  bookkeeping (`NeedsRedraw`, damage region) but `OverlayRenderer` never
  consumes it — it redraws the whole tree every frame regardless.
- **Per-frame costs observed** (native, 1920x1080, `host_speeds` + slog
  debug):
  - CPU: widget Layout+Draw of the full stacked HUD/console/menu tree,
    then a full-screen CPU CANVAS IMAGE READBACK (`dc.Image()` is a raw
    `RGBA.Pix` view, but the ambient cost is the compose + copy out of the
    widget drawing ops).
  - GPU: one full-resolution `UpdateData` (1920*1080*4 ≈ 8.3 MB/frame) +
    one fullscreen overlay composite render pass with `LoadOp::Load`.
- **No continuous-render mode exists in v3:** `DrawOverlay` is invoked from
  `game_runtime_frame.go:381` unconditionally each frame; there is no skip
  path, no dirty-check, no `NeedsRedraw` gate.

## 2. Hypothesis test: would a continuous-render mode (skip retained-mode
invalidation bookkeeping) reduce material overhead?

**A. The widget tree itself is cheap.** Layout+Draw of the stacked roots
(HUD/console/menu) is a handful of `widget` nodes; the CPU cost is bounded
and far below the frame budget (measured drawCount log spam dominates the
cost, not the draw). Skipping it does not move the frame time needle.

**B. The real overhead is the texture upload, and it is unavoidable.**
The dominant per-frame cost is the full-screen `UpdateData` + composite
pass. That cost exists regardless of *tree* invalidation: the engine's
HUD/console/menu are **continuously animated** (HUD stats re-render every
frame; console slide animation; menu cursor blink; damage flashes). The
damage region is effectively the full screen every frame — retained-mode
invalidation would compute a damage rect that is ~full-screen, then still
upload the full texture. **No material overhead reduction.**

**C. Continuous-render mode would break the widget tree.** gogpu/ui
widgets rely on retained-mode guarantees (the tree marks itself dirty when
state changes; `NeedsRedraw`/`InvalidateScene` trigger layout + redraw).
Forcing "always redraw" is already what happens — but adding a *skip*
path risks skipping the `SetNeedsRedraw` assertions that catch stale/missing
redraws, reintroducing the exact "menu doesn't repaint / HUD stuck" class of
bugs all three prior branches fought.

**D. The v4 architecture removes the CPU readback entirely.** SPEC-004
switches to Scenario A: `ggcanvas.Canvas` (GPU-backed) + `render.NewCanvas`
+ `window.DrawTo` + `canvas.RenderDirect`. The GPU canvas is a retained GPU
resource; per-frame redraw issues command-stream draws, NOT a CPU→GPU
full-frame upload. The measured v3 GPU overhead (readback + upload) is
eliminated by M1.1 regardless of the tree redraw model.

## 3. Decision

**Accept full redraws per frame.** Aligns with spec §2#10 and the plan's
M0.3 default. Rationale:

1. Full redraws are already the v3 behavior and meet stability/parity
   expectations; the widget tree is small and its CPU cost is negligible.
2. The only material cost (full-frame texture upload / composite) is
   invariant to the tree redraw model — HUD/menu are continuously animated,
   so the damage region is full-screen anyway. A continuous-render mode
   provides no material overhead reduction.
3. The v4 GPU canvas eliminates the CPU readback + upload path entirely,
   removing the cost the spike measured; retained GPU redraws are command
   submissions, not data movement.
4. Adding a skip-the-redraw mode would fight the retained-mode widget
   guarantees and CANNOT improve the invariant cost, only risk stale-UI
   regressions (spec G2 "UI must never clear the world" and the
   double-draw/repaint bug class). Overhead of NOT adopting: none beyond
   the (small, bounded) widget tree draw per frame.

**Not adopted:** continuous-render mode (skip retained-mode invalidation).
**Gap 7 → RESOLVED** (G4).

## 4. What this means for M1.1+

- `OverlayRenderer.DrawOverlay` keeps the per-frame full-tree draw contract;
  its implementation moves to `ggcanvas.Canvas` + `RenderDirect` (GPU).
- No dirty-check gating is added to the draw loop. `SetNeedsRedraw` /
  `InvalidateScene` keep operating as the widget tree's internal contract
  and are preserved untouched.
- If a future milestone finds the widget-tree Layout cost material (large
  trees, mod HUDs), revisit with measurements — the DFS/geometry-only
  incremental layout of the widget framework is the extension point, NOT a
  skip-redraw mode.

## 5. Cross-references

- SPEC-004 §2#10 (redraw model default), §3.3, AC9-AC11
- ADR-0011 (Scenario A composite; GPU canvas removes readback/upload)
- Plan M0.3 (`docs/internal/plans/004-ui-gogpu-rewrite-v4-implementation-plan.md`)
- Gap log row 7, 23 (`.ai-dlc/ui-rewrite-v4/gaps.md`)
