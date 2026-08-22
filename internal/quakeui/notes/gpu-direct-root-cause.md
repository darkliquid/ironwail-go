# GPU-Direct Composite Root Cause Investigation (gg/gpu accelerator + gogpu)

**Status:** Root cause identified (2026-08-22)
**Owner:** v4 Scenario A composite (ADR-0011)
**Outcome:** The engine overlay stays on the CPU-readback blit for now; GPU-direct
requires gogpu to own all submits (gg records only). Documented here so the
accelerator path can be revisited with the correct integration.

## 1. Symptom

Enabling the gg/gpu SDF accelerator (blank import) and compositing the overlay
via `ggcanvas.RenderDirect(sv, w, h)` produced, every frame:

```
submit: wgpu: Submit: command buffer at index 0 references released texture:
wgpu: Submit: command buffer references destroyed texture
```

Affecting gogpu's OWN submits (`renderWorldInternal: Failed to submit ...`),
`renderOverlayTextureHAL: queue.Submit failed ("destination texture retired
before submit")`, and the gg directly path alike. The 3D world went black and
menu-quit produced error spam.

## 2. What was ruled out

- **The accelerator binding into the engine's wgpu device is NOT the cause.**
  `GPUShared.SetDeviceProvider` sets `externalDevice=true` and `Close()` never
  Releases a shared device (gpu_shared.go:301-342). Verified: accelerator
  registered + overlay using ONLY the CPU-readback blit runs clean (0 errors,
  1200+ frames).
- The overlay's own texture cache (`DrawRGBA`'s `uiOverlayTexture`) — fixed
  separately by `DrawRGBAFresh`.

## 3. Root cause (definitive)

`ggcanvas.RenderDirect(sv, w, h)` → gg `Context.FlushGPUWithView` →

```go
// internal/gpu/render_session.go:2850-2895 (GPU-direct flush)
cmdBuf, err := encoder.Finish()      // finishes THE SHARED GOGPU FRAME ENCODER
encoderConsumed = true
s.queue.Submit(cmdBuf)               // gg SUBMITS THE ENCODER ITSELF
s.prevCmdBufs = append(s.prevCmdBufs, cmdBuf)
s.frameRendered = true
```

Even through `renderDirectToTarget` (which borrows gogpu's shared frame
encoder), gg does NOT stop at recording — it **Finish()es + Submit()s the
shared gogpu frame encoder itself** and retains the command buffer in
`prevCmdBufs`, freeing it at the NEXT frame's `BeginFrame`:

- The shared encoder gogpu owns is finished by gg mid-frame; gogpu's later
  `submitFrameEncoder`/world-pass recording then operate on a finished or
  re-recorded encoder → invalid command buffers.
- gg's deferred `prevCmdBufs` free (next `BeginFrame`) crosses gogpu's
  swapchain acquire/present/release — the retained command buffer references
  the prior frame's surface image after `releaseFrame()` released
  `currentView` → "references released/destroyed texture" on submit.
- `s.frameRendered=true` marks the surface as rendered from gg's perspective,
  conflicting with gogpu's per-frame `frameCleared`/`hasGPUWork` reset state.

The g3d `fullscreen-overlay` pattern avoids this because g3d/gogpu **own the
entire frame**: either g3d records into gogpu's encoder and gogpu submits once
at endFrame, or g3d uses the offscreen-composition path where gg's flush
targets an intermediate texture (not the swapchain image) and gogpu blits it
in its own submit. Submitting the shared encoder from inside the gg layer
mid-frame is the incompatibility.

## 4. Why the readback blit is safe

The engine's CPU-readback composite (`blitToTarget` → `DrawRGBAFresh` →
`renderOverlayTextureHAL`) targets `dc.currentWGPURenderTargetView()` with its
own encoder+submit, but runs inside gogpu's OnDraw AFTER the world pass and
AFTER the frame is started; the fresh per-frame texture is created in the
current frame and submitted before gogpu's present/release. The world pass
and the overlay are the only two submits, both ordered before present, so no
deferred cross-frame lifetime exists. Not zero-copy, but lifetime-correct.

## 5. What it would take to use the GPU-direct path

1. gg's GPU-direct flush must STOP finishing+submitting the shared gogpu
   encoder: it should record passes into the encoder gogpu hands it and
   return, letting gogpu `submitFrameEncoder` finish+submit once. This is
   the "gg records only" contract — needs a gg-side change
   (FlushGPUWithView gaining a record-only mode that never calls
   `encoder.Finish()`/`queue.Submit` and never manages `prevCmdBufs`).
2. Alternatively: route the overlay through the same offscreen-composition
   path g3d uses where gg flushes into a canvas-owned intermediate texture
   (ggcanvas `FlushPixmap` → `PixmapTextureView` → gogpu's own blit pass),
   so the swapchain image is only ever referenced by gogpu's single submit.
3. `s.frameRendered`/frame-state interactions with gogpu's `frameCleared`
   reset must be reconciled (or suppressed when using a shared encoder).

## 6. Follow-up finding (accelerator + CPU-blit readback)

Re-enabling the accelerator while the overlay composites via the CPU-readback
blit made the UI INVISIBLE (inputs still worked). Cause: with the SDF
accelerator registered, gg routes draw ops to the GPU queue and does NOT
rasterize into the CPU pixmap, so `ggcanvas.Context().Image()` (the readback
the blit reads) is empty. The accelerator and the CPU-blit composite are
mutually exclusive:

- Accelerator ON + GPU-direct flush → swapchain lifetime race (section 3).
- Accelerator ON + CPU-blit readback → empty pixmap, invisible UI.
- Accelerator OFF + CPU-blit readback → visible UI, lifetime-safe (current).

So the accelerator import is removed from the engine for now. Re-enabling
GPU-direct requires one of the section-5 changes (record-only gg flush, or
offscreen-composition where gogpu owns the single submit).

## 7. Cross-references

- ADR-0011 (Scenario A composite); redraw-model-spike.md (M0.3)
- gogpu shared frame encoder: `context.go` `CommandEncoder()`, `ensureFrameEncoder()`
- gg render session GPU-direct flush: `internal/gpu/render_session.go:2850-2895`
- g3d `examples/fullscreen-overlay` (record-only / offscreen-composition)
