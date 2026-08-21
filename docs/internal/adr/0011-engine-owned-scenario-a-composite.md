# ADR-0011: Engine-Owned Scenario A Composite (MarkPreserveContent) — Supersedes ADR-0010

**Status:** Accepted (2026-08-21)
**Deciders:** darkliquid (v4 Stage 1: surface Scenario A), lifecycle driver
**Date:** 2026-08-21
**Supersedes:** ADR-0010 (v3: CPU gg.Context + custom WGSL blit + reflection)
**Related:** IRONWAIL-SPEC-004 §1.1/§3.3/§5.1/AC9-AC10; deep-research report §2.1-2.3;
g3d ADR-048; gap log rows 1-4, 11, 17

## Context and Problem Statement

All three prior UI-rewrite branches struggled to composite gogpu/ui over the
engine's 3D world inside an engine-owned loop:

- v1 used a CPU `ggcanvas.Canvas` bridged into the engine loop and composited
  manually; it fought the framework's canvas lifecycle and could clear the
  world.
- v2 adopted `desktop.Run` + GPUView; `desktop.Run` inverts control (single-slot
  OnDraw) and GPUView's texture lifecycle churned on resize, producing Vulkan
  presentation failures.
- v3 kept the engine loop but rasterized the UI into a CPU `gg.Context`, read
  it back with `dc.Image()`, uploaded it as a texture, and blitted with a
  hand-written WGSL pipeline plus reflection/unsafe pokes into gogpu's private
  `frameCleared`/`hasPendingClear` fields.

The deep research found the gogpu org's own 3D engine (g3d) hit the identical
wall (g3d#5: "both g3d and ui currently assume exclusive surface ownership")
and shipped a documented solution (ADR-048, `examples/fullscreen-overlay`):
render the 3D pass, call the **public** `MarkPreserveContent()`, then render
the UI pass on top with `LoadOp::Load`. The v3 reflection hack is a
reinvention of this public API.

## Decision Drivers

- Engine owns the frame loop (native + WASM + headless); no `desktop.Run`.
- The UI must never clear the world (hard constraint G10 from v2).
- GPU-accelerated UI: no CPU readback of the widget tree.
- No reflection/unsafe into gogpu internals (maintainability, upgrade safety).
- The pattern must be proven, not invented — g3d's `fullscreen-overlay` is the
  reference implementation.

## Considered Options

1. **CPU gg.Context + custom WGSL blit (v3)** — rejected: CPU readback of the
   UI each frame, a hand-written shader + pipeline + bind-group to blit, and
   reflection/unsafe into gogpu internals that breaks on any internal rename.
2. **`desktop.Run` + GPUView (v2)** — rejected: inverts control, breaks WASM,
   GPUView texture lifecycle churn and Vulkan presentation failures.
3. **Engine-owned Scenario A (g3d-proven)** — chosen. Engine renders the world
   to the swapchain surface; calls `gogpu.Context.MarkPreserveContent()`
   (public, ADR-065); the UI widget tree draws into a GPU-backed
   `ggcanvas.Canvas` (`render.NewCanvas` + `window.DrawTo`) and flushes with
   `canvas.RenderDirect(sv, sw, sh)` — `LoadOp::Load`, alpha blend, GPU.
   Pros: engine-owned loop, GPU-accelerated, no CPU readback, no custom
   shader, no reflection, proven by g3d. Cons: the world must render to the
   surface (not a reusable offscreen texture); the gg GPU path falls back to
   CPU on software adapters (graceful degradation).

## Decision Outcome

Engine-owned Scenario A. The engine keeps the `gogpu.App`, the frame loop, and
`OnDraw`. Per frame: Pass 1 world → `dc.MarkPreserveContent()` → Pass 2 UI via
`ggcanvas.Canvas` + `render.NewCanvas` + `window.DrawTo` + `canvas.RenderDirect`
→ present. The v3 reflection/unsafe `markGoGPUFrameContentForOverlay` is
removed and replaced with the direct `MarkPreserveContent()` call (verified:
the reflection function already calls `MarkPreserveContent()`, so the poke is
redundant).

**Scene-target caveat (A2):** when waterwarp/translucent-liquid rendering is
active, the v3 frame retargets the world into an offscreen scene texture
(`sceneTargetActive`). In that case `MarkPreserveContent()` must be called
AFTER the scene composite (post-composite), not after the naive world pass —
the surface content is the composited scene, and that is what must be
preserved. The spec §3.3 step 3 is clarified accordingly.

- **Positive:** engine-owned loop (native/WASM/headless); GPU-accelerated UI;
  no CPU readback; no custom WGSL; no reflection; proven by g3d
  `fullscreen-overlay`; uses the shared `dc.CommandEncoder()` for single-submit
  multi-pass frames.
- **Negative:** world must render to the surface (offscreen world texture
  deferred, G1); gg GPU path falls back to CPU on software adapters (software
  renderer stays legacy anyway, G8).

## Links

- IRONWAIL-SPEC-004 §1.1, §3.3, §5.1, AC9-AC10
- Deep-research report §2.1-2.3; g3d ADR-048, `examples/fullscreen-overlay`
- gogpu `context.go` `MarkPreserveContent` (ADR-065)
- Gap log rows 1-4, 11, 17

## Review Log

### Stage 5 — Review 2 (2026-08-21)

Verdict: APPROVED WITH ONE FIX. Supersedes ADR-0010 honestly (the v3 approach
failed and is replaced, not amended). Negative consequences (surface-only
world, software fallback) are specific and consistent with SPEC-004 G1/G8. The
MarkPreserveContent-vs-reflection finding is verified against the v3 source.
Fix A2 applied: the scene-target caveat (waterwarp retargets the world
offscreen; MarkPreserveContent must come after the scene composite) is now
pinned in the Decision Outcome and SPEC-004 §3.3.
