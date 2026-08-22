# RESEARCH-0008 follow-up: the 4th rewrite (v4) — what we learned, and how it changes the picture

**Status:** Follow-up reply to RESEARCH-0008 (the three-branch evaluation)
**Date:** 2026-08-22
**Related:** SPEC-004, ADRs 0011-0015, `experiment/ui-rewrite-v4`, gogpu discussion #468

---

## 1. tl;dr

The three-branch saga's central claim — "the embedding seam is the unsupported,
undocumented, repeatedly-reinvented part" — is now **empirically confirmed with
a fourth data point**, and the fourth attempt adds something the first three
could not: a **public API seam** that eliminated the reflection/unsafe poke and
the custom-WGSL blit, plus a **root-caused** explanation of the GPU-lifetime
crashes that killed v2 and nearly killed v4.

But v4 also landed on a humbling conclusion: after four attempts, the engine
still composites the UI with a **CPU-readback blit** — because gogpu/ui's
GPU-direct composite path (the one the org's own g3d example uses) has a
**command-buffer lifetime bug** when a host owns the frame, and the gg GPU
accelerator cannot be used with the CPU-readback path at all. The widget
toolkit is fine; the **GPU-direct composite contract is the unresolved piece**.

---

## 2. What v4 changed (the architecture)

| | **v1** | **v2** | **v3** | **v4** |
| --- | --- | --- | --- | --- |
| **Loop owner** | Engine | `desktop.Run` | Engine | Engine |
| **Widget → surface** | gg-canvas bridge | gpuview texture | CPU `gg.Context` + custom WGSL blit | CPU `gg.Context` → ggcanvas → readback blit |
| **World-preserve seam** | (fragile) | compositor | reflection/unsafe on `frameCleared`/`hasPendingClear` | **public `MarkPreserveContent()`** |
| **Input** | EventSource gateway | KeyForwarder | KeyForwarder (3-gate rewrite) | **decoupled router + poll-only backend** |
| **WASM** | works | blocked | works | works |
| **Headless** | works | mocks | works | works (fail-open) |

The three structural decisions that made v4 different:

1. **`MarkPreserveContent()` instead of reflection.** v3 poked gogpu's private
   `frameCleared`/`hasPendingClear` via `reflect`+`unsafe` to force `LoadOpLoad`
   so the overlay wouldn't clear the world. v4 deleted all of that and calls the
   **public** `Context.MarkPreserveContent()` (the g3d fullscreen-overlay
   pattern). This was the single biggest maintainability win — the reflection
   seam was a "breaks on any internal rename" landmine, and it's gone.

2. **A decoupled input router instead of a forwarding shim.** v1's EventSource
   gateway and v2/v3's KeyForwarder both fought gogpu's "one event feeds three
   sinks" model and produced double-delivery. v4 made the engine **poll
   `app.Input()`** for gameplay (Ebiten-style) and gave the UI the EventSource,
   with a single `InputRouter` policy point doing **exclusive** per-KeyDest
   routing. This eliminated the double-delivery class outright — and a
   printable-key double-print bug found in testing (each key delivered as both a
   key-down and a text char) was fixed at the router, not by rewriting the
   backend.

3. **The widget tree is drawn fresh each frame into a correctly-sized
   context.** v2/v3's retained-mode tree (RepaintBoundary scene caches) fought
   Quake's per-frame-animated menu/HUD: stale cursors, sub-menus drawing over
   each other, top-left layout from a never-refreshed window size. v4 draws the
   stack directly with a per-frame transparent clear and the engine's real
   viewport — the G4 "accept full redraws" decision, which the retained-mode
   machinery was dead weight for anyway (the honest negative finding from
   RESEARCH-0008 §5.2).

---

## 3. The new negative finding: GPU-direct composite is still broken

This is the most important thing v4 learned, and it refines RESEARCH-0008's
"root cause" section.

### 3.1 v2's Vulkan presentation crashes — now root-caused

RESEARCH-0008 §3.2 listed v2's "Vulkan command buffers referenced
retired/destroyed textures → swapchain presentation failures" as "the deepest
technical failure of the three — a GPU-lifetime bug, not a design preference."
v4 now explains **why**:

The gg GPU-direct flush (`ggcanvas.RenderDirect` → `Context.FlushGPUWithView` →
`internal/gpu/render_session.go`) does three things a host-owned frame cannot
tolerate:

```go
cmdBuf, err := encoder.Finish()      // (1) finishes THE SHARED GOGPU FRAME ENCODER
s.queue.Submit(cmdBuf)               // (2) gg submits the encoder itself
s.prevCmdBufs = append(..., cmdBuf)  // (3) retains it, frees at NEXT frame's BeginFrame
```

- It **finishes and submits the shared gogpu frame encoder itself** — gogpu's
  later `submitFrameEncoder`/world-pass recording then operates on a consumed
  encoder.
- It **retains the command buffer across frames** and frees it at the next
  `BeginFrame`, which crosses gogpu's swapchain acquire/present/`releaseFrame`
  (which releases `currentView`). The retained buffer still references the
  prior frame's surface image → `"command buffer references destroyed
  texture"` on submit.
- It sets `s.frameRendered = true`, conflicting with gogpu's per-frame
  `frameCleared`/`hasGPUWork` reset.

**This is exactly v2's failure, and it's in the gg layer, not the app.** The
g3d `fullscreen-overlay` example works because g3d/gogpu own the whole frame —
gg either records into gogpu's encoder and gogpu submits once, or flushes into
an offscreen composition texture that gogpu blits in its own submit. When a
**third-party host** owns the frame, gg's "submit the shared encoder myself"
path is the incompatibility.

### 3.2 The accelerator is a trap for the readback path

v4 also discovered the gg SDF accelerator and the CPU-readback composite are
**mutually exclusive**:

- Accelerator ON + GPU-direct flush → the swapchain lifetime race above.
- Accelerator ON + CPU-readback blit → **invisible UI**: with the accelerator
  registered, gg routes draw ops to the GPU queue and never rasterizes into the
  CPU pixmap, so `ggcanvas.Context().Image()` (the readback the blit reads) is
  empty. Inputs still worked; the UI was just blank.
- Accelerator OFF + CPU-readback blit → visible, lifetime-safe (current).

So the accelerator import had to be removed entirely. The "single-pass GPU
flush" that SPEC-003/ADR-0010 aspired to remains unimplemented — and now we
know it's not a doc/code drift problem (v3's finding), it's a **missing gg
contract**: a record-only flush that never finishes/submits the shared encoder,
or an offscreen-composition path where gogpu owns the single submit.

---

## 4. What v4 confirms from the first three

1. **The embedding seam is the whole problem.** Four architectures, four
   hand-built seams, all touching gogpu internals or the single-slot
   `OnDraw`/`EventSource`. The widget layer (menu/console/HUD/demo bar) carried
   over nearly unchanged — v4 added only a demo bar. RESEARCH-0008's §4
   conclusion is now proven four times over.

2. **The BYO-kit path works** (all four branches, zero `core/*` imports), and
   **custom widgets on `WidgetBase`** is what a game actually needs — the
   Painter pattern remains the wrong abstraction for bespoke game UIs.

3. **`ImageRegionDrawer`/`DrawImageSrcRect` is still the highest-value Canvas
   addition** — the conchars atlas + status-bar sub-rects are still the
   hottest draw path.

4. **The bitmap-font gap is still blocking.** v4 reused the hand-rolled
   per-glyph conchars atlas (as v2/v3 did); a public font-registration /
   bitmap-strike API remains the P0 ask.

5. **`desktop.Run` ownership is still the sharpest edge**, and the standalone
   engine path (widget tree → caller-owned texture, caller loop) is still
   undocumented — v4 hit the exact gap.

6. **Retained-mode invalidation is dead weight for continuous 3D games** — v4
   drew fresh every frame and the RepaintBoundary caches were the *source* of
   the stale-cursor/leftover-page bugs, not a win.

---

## 5. What v4 adds to the recommendations

RESEARCH-0008 §5.3's list stands, with one addition and one reframing:

- **P0 (new): a record-only GPU-direct flush contract.** The org's own
  `fullscreen-overlay` works because gogpu owns the frame; a host-owned frame
  cannot use gg's GPU-direct path without racing the swapchain. Either
  `FlushGPUWithView` gains a mode that records into the caller's encoder and
  **never** calls `Finish()`/`Submit()`/manages `prevCmdBufs`, or the
  offscreen-composition path (gg flushes to an intermediate texture, gogpu
  blits in its own submit) is documented as **the** standalone-engine path.
  This is the concrete reason every branch ended up on a CPU readback.
- **P0 (reframed): the standalone-engine embedding contract** (from §5.3 P1)
  is now confirmed as the *first* thing to standardize — it's not just
  undocumented, it's **unimplementable** today without either the record-only
  flush or the offscreen path.
- Everything else (font path, `ImageRegionDrawer`, `AdvancedCanvas.GGContext()`,
  palette escape, `Continuous`/`AlwaysDirty` mode) is unchanged and still
  validated.

---

## 6. Where v4 stands now

- **All four surfaces render** (menu, console, HUD, demo bar) with real LMP
  art + conchars, on the engine-owned loop, native + WASM + headless.
- **No reflection/unsafe** into gogpu internals; `MarkPreserveContent()` is the
  seam.
- **Input is decoupled** (engine polls `app.Input()`, UI owns EventSource,
  exclusive router) with no double-delivery.
- **Composite is a CPU-readback blit** — the pragmatic, lifetime-safe choice
  until the gg record-only flush or offscreen path exists. The GPU-direct
  crash is root-caused and documented (`internal/quakeui/notes/
  gpu-direct-root-cause.md`).

The honest bottom line after four attempts: **the widget toolkit is not the
problem, and the CPU readback is not the answer — it's the stopgap.** The
thing that would end this saga is a supported "render my widget tree into a
texture I own, on my own loop, with gogpu owning the single submit" contract.
That's the ask.
