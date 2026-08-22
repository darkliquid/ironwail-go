# 005 — Single-Encoder GPU Composite — Implementation Plan

**Prerequisite:** stable baseline (`mise run test` green on
`experiment/ui-rewrite-v4`). This plan is the v4 Stage 2 follow-up (SPEC-005,
ADR-0016, ADR-0011 amendment).

**Command key** (all with `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp
CGO_ENABLED=0`):
- `FULL = go test ./...`
- `REND = go test ./internal/renderer/...`
- `QUI = go test ./internal/quakeui/...`
- `GAME = go test ./internal/game/...`
- `SMOKE = QUAKE_DIR=<dir> WAYLAND_DISPLAY= mise run smoke-menu`
- `WASM = GOOS=js GOARCH=wasm go build ./cmd/ironwailgo ./internal/game/ ./internal/renderer/ ./internal/quakeui/...`

**Estimated effort:** 2-3 focused sessions (phases M1-M3).

## 1. Phases

| Phase | Name | Outcome |
| --- | --- | --- |
| M1 | Shared-encoder plumbing | `dc.ctx.CommandEncoder()` returns the shared frame encoder; a helper records a pass into it; gogpu submits once. No renderer behavior change yet. |
| M2 | Migrate world/entity/overlay passes | All 17 encoder sites record into the shared encoder; engine issues no submits; soak test proves zero race errors. |
| M3 | Re-enable gg accelerator + offscreen-composition overlay | gg SDF accelerator (native), overlay composite via `FlushPixmap` → `PixmapTextureView` → gogpu blit; CPU-readback blit becomes fallback only. |

Sequencing rationale: M1 first (prove the shared encoder + single-submit works
without changing behavior — de-risks the whole model); M2 migrates all passes
(mechanical, ~17 sites); M3 lands the accelerator + offscreen overlay (the
goal, depends on M1/M2 being race-free).

## 2. Tasks

### M1 — Shared-encoder plumbing

**M1.1 Shared encoder accessor + pass helper (ADR-0016)**
- Files: `internal/renderer/renderer_gogpu_frame.go` (new helper),
  `internal/renderer/renderer_gogpu_runtime.go`.
- RED: `internal/renderer/renderer_gogpu_frame_test.go` — `TestSharedEncoderRecordsPass`:
  build a `DrawContext` with a stub gogpu ctx, call the new helper to record a
  trivial render pass into `dc.ctx.CommandEncoder()`, assert the encoder is
  non-nil and no `queue.Submit` was called by the engine.
- GREEN: add `func (dc *DrawContext) sharedEncoder() *wgpu.CommandEncoder` that
  returns `dc.ctx.CommandEncoder()` (gogpu's `ws.frameEncoder`), and a
  `recordPass(encoder, desc)` helper that does `BeginRenderPass`/`End` without
  Finish/Submit.
- Verify: `REND`; `FULL`.
- Acceptance: SPEC-005 AC1 (engine passes issue no submits).

**M1.2 Soak harness (AC3)**
- Files: `internal/renderer/soak_test.go` (new).
- RED (gate, not a unit test): `TestSoakNoReleasedTextureSubmits` — run N=1000
  frames through `RenderFrame` with the menu open (using the existing
  `drawContextWithGoGPUPrimarySurface` + a real-ish renderer), assert zero
  `references released/destroyed texture` submits AND a non-black world sample
  AND an overlay draw recorded. The current multi-submit path fails
  intermittently; the test must be deterministic enough to catch it.
- GREEN: at M2.5 (this is the acceptance gate; it intentionally stays RED
  through M2.1-M2.4).
- Verify: `REND` (expected RED until M2.5), `FULL`.

### M2 — Migrate world/entity/overlay passes

**M2.1 World + depth passes → shared encoder**
- Files: `internal/renderer/renderer_gogpu_world_render.go`.
- RED: `TestWorldPassUsesSharedEncoder` — assert the world pass records into
  the shared encoder (no `CreateCommandEncoder`/`queue.Submit` in the world
  pass).
- GREEN: replace `device.CreateCommandEncoder(...)` + `queue.Submit(cmdBuffer)`
  with `dc.sharedEncoder()` + `recordPass`; remove the per-pass Finish/Submit.
- Verify: `REND`, `FULL`, `SMOKE` (world renders under menu), soak gate (no new race).
- Acceptance: SPEC-005 AC1/AC5.

**M2.2 Entity passes (alias, brush, sky, liquid, decal, sprite, translucent) → shared encoder**
- Files: `renderer_gogpu_world_alias.go`, `renderer_gogpu_world_brush_render.go`,
  `renderer_gogpu_world_decal.go`, `renderer_gogpu_world_sprite.go`,
  `renderer_gogpu_world_translucent.go`.
- RED: per-file test asserting no `CreateCommandEncoder`/`queue.Submit` in the
  pass.
- GREEN: mechanical replacement (same pattern as M2.1).
- Verify: `REND`, `FULL`, `SMOKE`, soak gate.
- Acceptance: SPEC-005 AC1/AC5.

**M2.3 Scene target + composite → shared encoder**
- Files: `internal/renderer/renderer_gogpu_warpscale.go`.
- RED: `TestSceneCompositeUsesSharedEncoder` — assert the scene clear +
  composite record into the shared encoder.
- GREEN: replace the scene-clear and scene-composite encoder+submit with
  `sharedEncoder()` + `recordPass`. Verify `MarkPreserveContent` (ADR-0011 A2)
  is still called AFTER the composite.
- Verify: `REND`, `FULL`, `SMOKE` (waterwarp path), soak gate.
- Acceptance: SPEC-005 AC1/AC5, ADR-0011 A2.

**M2.4 Overlay HAL composite → shared encoder**
- Files: `internal/renderer/overlay_composite_gogpu.go`.
- RED: `TestOverlayHALUsesSharedEncoder` — assert `renderOverlayTextureHAL`
  records into the shared encoder (no own encoder+submit).
- GREEN: replace the overlay encoder+submit with `sharedEncoder()` + `recordPass`
  (LoadOpLoad over the world). Unify `flush2DOverlay` + `DrawOverlay` blit into
  one shared pass (gap 3).
- Verify: `REND`, `FULL`, `SMOKE`, soak gate.
- Acceptance: SPEC-005 AC1/AC5.

**M2.5 Soak gate GREEN**
- Run `TestSoakNoReleasedTextureSubmits` — now passes (zero errors, world
  non-black, overlay recorded).
- Verify: `REND`, `FULL`, `SMOKE`.
- Acceptance: SPEC-005 AC3.

### M3 — Re-enable gg accelerator + offscreen-composition overlay

**M3.1 Re-add gg accelerator (native-only)**
- Files: `internal/renderer/gg_accelerator_native.go` (`//go:build !js`),
  `internal/renderer/gg_accelerator_wasm.go` (`//go:build js`).
- RED: `TestAcceleratorRegisteredNative` — assert `gg.Accelerator() != nil` on
  native, nil on wasm.
- GREEN: blank-import `github.com/gogpu/gg/gpu` in the native file; empty wasm
  stub.
- Verify: `REND`, `FULL`, `WASM`.
- Acceptance: SPEC-005 AC2/AC4/AC6.

**M3.2 Offscreen-composition overlay (path 1)**
- Files: `internal/quakeui/overlay.go`, `internal/game/game_runtime_frame.go`,
  `internal/game/quakeui_host.go`.
- RED: `TestOverlayOffscreenComposition` (QUI) — with a GPU provider, `DrawOverlay`
  renders the widget tree into the ggcanvas, calls `FlushPixmap`, exposes
  `PixmapTextureView`, and records a gogpu blit pass into the shared encoder.
  Assert (positive): `PixmapTextureView()` non-empty after DrawOverlay, and the
  overlay reached the target with content. Forbidden-call gate (grep, not unit):
  `grep -rn "canvas.Render\|RenderDirect\|FlushGPUWithView" internal/quakeui/`
  returns nothing for the overlay path.
- GREEN: implement the offscreen-composition path in `DrawOverlay`; keep the
  CPU-readback blit as the fallback when `host.RenderTarget()` is nil or the
  blit fails.
- Verify: `QUI`, `GAME`, `FULL`, `SMOKE` (overlay visible over world), `WASM`.
- Acceptance: SPEC-005 AC2/AC5/AC4.

**M3.3 Soak gate with accelerator**
- Run `TestSoakNoReleasedTextureSubmits` WITH the accelerator enabled — must
  still pass (zero errors, world non-black, overlay recorded). This is the
  definitive proof that offscreen-composition avoids the gg submit race.
- Verify: `REND`, `FULL`, `SMOKE`.
- Acceptance: SPEC-005 AC3 (with accelerator on).

## 3. Traceability Matrix

| Task | Spec § | ADR | AC | Gaps |
| --- | --- | --- | --- | --- |
| M1.1 | §2/§3 | 0016 | AC1 | — |
| M1.2 | §8 | 0016 | AC3 | 12 |
| M2.1 | §3.3 | 0016 | AC1, AC5 | — |
| M2.2 | §3.3 | 0016 | AC1, AC5 | — |
| M2.3 | §3.3/§7.3 | 0016, 0011 | AC1, AC5 | 11 |
| M2.4 | §3.3 | 0016, 0011 | AC1, AC5 | 3 |
| M2.5 | §8 | 0016 | AC3 | 12 |
| M3.1 | §2 | 0016 | AC2, AC4, AC6 | 4, 5 |
| M3.2 | §2/§5.2 | 0016, 0011 | AC2, AC5, AC4 | 8 |
| M3.3 | §8 | 0016 | AC3 | 8 |

## 4. Risks & Mitigations

| Risk | Severity | Mitigation | Gap |
| --- | --- | --- | --- |
| `FlushGPUWithView`/`canvas.Render` re-introduces the submit race | high | Hard gate: overlay uses offscreen-composition ONLY; no `canvas.Render`/`RenderDirect`/`FlushGPUWithView` for the overlay (M3.2 RED asserts this) | 8 |
| Scene-target + deferred-clear ordering in shared encoder | med | M2.3 verifies `MarkPreserveContent` after composite; soak test covers waterwarp | 11 |
| Intermittent race not caught by soak | med | Soak runs 1000+ frames + asserts world non-black + overlay recorded (not just zero errors) | 12 |
| WASM build breaks (gg/gpu pulls wgpu/core) | med | Platform-gated accelerator import (M3.1) + WASM build in every verify | 4 |
| Multi-pass single-encoder invalid (render-pass state) | med | M1 proves one pass; M2 migrates incrementally with per-pass tests | — |

## Review Log

### Stage 8 — Review 3 (2026-08-22)

Verdict: APPROVED WITH FIXES. M1.2 soak re-labeled as a gate (RED until M2.5);
M3.2 RED uses positive assertions + a grep gate for forbidden calls (not a
negative unit test); each M2.x verify now runs the soak gate so a race is
caught at introduction, not only at M2.5.
