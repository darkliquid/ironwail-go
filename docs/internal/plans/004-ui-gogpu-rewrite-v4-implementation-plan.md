# Implementation Plan 31: gogpu/ui UI Rewrite v4 (IRONWAIL-SPEC-004)

**Priority**: Experiment (branch `experiment/ui-rewrite-v4`); the reference
BYO-kit validation case for gogpu discussion #468.
**Status**: PLANNED (2026-08-21) — pending Review 3.
**Prerequisite**: stable baseline (`mise run test` green on
`experiment/ui-rewrite-v3` before branching).
**Spec**: `docs/internal/specs/004-ui-gogpu-rewrite-v4.md`
**ADRs**: `docs/internal/adr/0011..0015`
**Research**: deep-research report
`~/.local/share/crush/research/gogpu-ui-engine-owned-rendering/report.md`;
`docs/internal/research/0008-ui-rewrite-branches-evaluation.md`
**Gaps**: `.ai-dlc/ui-rewrite-v4/gaps.md` (rows 7, 8, 10 open → resolved by
tasks M0.1/M5.3 below)
**Estimated effort**: 8-12 focused sessions (phases M0-M5)

---

## 1. Executive Summary

Rewrite the menu, console, HUD, and demo bar on gogpu/ui a fourth time, using
the **engine-owned Scenario A pattern** proven by the gogpu org's own g3d
engine (ADR-048). The engine keeps the frame loop (native + WASM + headless);
the world renders to the swapchain surface; the engine calls
`MarkPreserveContent()`; the UI widget tree draws into a GPU-backed
`ggcanvas.Canvas` via `render.NewCanvas` + `window.DrawTo` +
`canvas.RenderDirect` (LoadOp::Load, GPU-accelerated, no CPU readback, no
custom WGSL, no reflection). Input is decoupled: the engine polls
`gogpuApp.Input()` for gameplay; the UI owns the EventSource; a single router
splits by KeyDest (exclusive for keys). The v3 widget tree carries over
(ADR-0014); only the seam is rebuilt. `ui_backend 0|1` gate kept; software/
screenshot stays legacy. All work on `experiment/ui-rewrite-v4`; no
commit/push unless asked (conservative policy).

## 2. Milestones & Phases

| Milestone | Theme | Deliverable |
| --- | --- | --- |
| M0 | Baseline + de-risk | branch from v3; remove v3 reflection/unsafe seam; engine polling spike (latching test to polling, RED first) |
| M1 | Scenario A composite | OverlayRenderer with GPU ggcanvas + MarkPreserveContent + RenderDirect; uiApp lazy lifecycle; desktop.Run gone |
| M2 | Decoupled input router | input_router.go; engine polls app.Input(); UI owns EventSource; exclusive key routing |
| M3 | Menu + console on v4 | MenuRoot + ConsoleRoot render via Scenario A; input routed; visual fidelity verified |
| M4 | HUD + demo bar on v4 | HUDRoot + DemoBarRoot render via Scenario A; CSQC fallback |
| M5 | Cross-platform + verification | WASM + headless ACs; full suite; parity captures |

Sequencing rationale: M0 first (remove the fragile v3 seam + de-risk the
polling migration before building on it); M1 builds the Scenario A composite
everything hangs off (the M-spike for the redraw model lands here); M2 adds
the router; M3-M4 land surfaces on the proven seam; M5 verifies cross-platform
(AC3a/b/c) and closes the open gaps.

## 3. Tasks (Red-Green TDD, file paths, test commands)

**Command key** (all with `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp
CGO_ENABLED=0`):
- `FULL = go test ./...`
- `GAME = go test ./internal/game/...`
- `REND = go test ./internal/renderer/...`
- `QUI = go test ./internal/quakeui/...`
- `WASM = GOOS=js GOARCH=wasm go build ./...`

### M0 — Baseline + de-risk

**M0.1 Branch + remove v3 reflection seam (ADR-0011, R1.7)**
- Files: branch `experiment/ui-rewrite-v4` from `experiment/ui-rewrite-v3`;
  `internal/renderer/renderer_gogpu_frame.go` — replace
  `markGoGPUFrameContentForOverlay` reflection/unsafe with a direct
  `dc.ctx.MarkPreserveContent()` call (the reflection fn already calls it, so
  the poke is redundant); delete `goGPUFrameStateFields`, `writableReflectValue`,
  reflection/unsafe helpers.
- Tests: `REND` — RED: a test asserting the overlay pass preserves surface
  content fails while the reflection helper still exists? (No — write a GREEN
  test first: `TestRenderFramePreservesSurfaceAfterMarkPreserveContent`
  asserting `RenderFrame` with `Draw2DOverlay` does not clear the surface);
  RED before the rename: existing test references the old symbol; GREEN after.
- Acceptance: AC10 (no reflection/unsafe in the seam; MarkPreserveContent used).

**M0.2 Engine polling spike (ADR-0012, R1.3, gap 8)**
- Files: new `internal/game/input_polling.go` (polling adapter over
  `gogpuApp.Input()`), new `internal/game/input_polling_test.go`.
- Tests: `GAME` — RED: write `TestPollingAdapterEdgeSemantics` against a stub
  `app.Input()` with known key/mouse events (press/release edges, mouse delta
  accumulation) and assert the ADAPTER produces identical edge semantics to
  the callback backend; GREEN: implement the adapter over the real
  `gogpuApp.Input()`. Then migrate ONE real latching test
  (`game_movement_input_test.go:145-329` family) onto the adapter and verify
  it passes; the callback-based original still passes (both paths work).
- Acceptance: AC2/AC9 (latching preserved on both paths; polling de-risked).

**M0.3 Redraw-model spike (G4, gap 7)**
- Spark: file `internal/quakeui/notes/redraw-model-spike.md` — measure the
  cost of the full-redraw-per-frame model on the v3 widget tree
  (drawCount/setNeedsRedraw per frame) and evaluate whether a
  continuous-render mode (skip retained-mode invalidation bookkeeping) is
  cheap to add. **Default decision: accept full redraws** (spec §2#10); a
  continuous-render mode is adopted ONLY if the spike shows material overhead
  reduction without breaking the widget tree.
- No code. Gaps: 7 → RESOLVED.

### M1 — Scenario A composite

**M1.1 OverlayRenderer with GPU canvas (ADR-0011)**
- Files: `internal/quakeui/overlay.go` — rework `DrawOverlay` to take a
  `*ggcanvas.Canvas` (GPU-backed) + `gg.Context`; create `render.NewCanvas`;
  `uiApp.Window().DrawTo(widgetCanvas)`; `canvas.RenderDirect(sv, sw, sh)`.
  The uiApp is created lazily on first path-1 frame (R1.6) with
  `app.WithWindowProvider/WithPlatformProvider/WithTheme` (EventSource per
  ADR-0012); teardown on `gogpuApp.OnClose`.
  **WASM guard (P2):** the GPU provider is only available after
  `gogpuApp.Run()`. The canvas/provider must be resolved LAZILY on first draw
  (v1's `ProviderFunc` pattern), not at uiApp construction, so the WASM path
  (`StepWasmFrame`/rAF, M5.1) resolves the provider after Run. Headless/
  software: nil provider → software `gg.NewContext` fallback or fail-open
  (AC3c).
- Tests: `QUI` — RED: `DrawOverlay` with a fake canvas renders the stack and
  calls RenderDirect; GREEN: implement. Headless: assert nil-provider path
  renders to a software gg context without panic.
- Acceptance: AC3a, AC9.

**M1.2 MarkPreserveContent seam in engine frame (ADR-0011, A2)**
- Files: `internal/game/game_runtime_frame.go` — after the world pass (and
  AFTER the scene composite when `sceneTargetActive`), call
  `dc.ctx.MarkPreserveContent()` before invoking the overlay callback.
- Tests: `REND`/`GAME` — RED: capture assert world visible under UI fails
  without the call; GREEN: add the call.
- Acceptance: AC9 (UI never clears the world), AC10.

**M1.3 uiApp startup lifecycle (G11, ADR-0012 A1)**
- Files: `internal/game/quakeui_host.go` + `game_runtime_frame.go` — create
  the uiApp ONCE at startup when `ui_backend 1` is selected; teardown on
  `gogpuApp.OnClose`; EventSource ownership follows the startup path (UI when
  1, engine backend when 0); software/headless force legacy.
- Tests: `GAME` — RED: startup-selection test asserting the path is frozen
  (no mid-session flip), EventSource ownership matches the startup path, and
  a forced uiApp init failure fails open to legacy; GREEN.
- Acceptance: AC4 (startup-only path), R1.6 (superseded — created once at
  startup).

### M2 — Decoupled input router

**M2.1 input_router.go (ADR-0012)**
- Files: `internal/game/input_router.go` — the single policy point; takes
  `KeyDest`; exclusive key routing (KeyGame/HUD→engine, KeyConsole/KeyMenu→UI
  only, KeyMessage→engine, backtick/binding→engine pre-route); mouse routing
  by KeyDest.
- Tests: `GAME` — RED: `TestInputRouterExclusiveKeyRouting` (each KeyDest →
  exactly one sink), `TestInputRouterNoDoubleDispatch` (surface keys never hit
  engine on path 1); GREEN.
- Acceptance: AC9, R1.2.

**M2.2 Engine gameplay polling migration (ADR-0012)**
- Files: `internal/game/game_input.go` + `input_polling.go` — migrate the
  gameplay input path (KeyGame) from the callback backend to polling
  `gogpuApp.Input()`; KeyDest router unchanged.
- Tests: `GAME` — RED: full latching/mouse-look suite fails on the polling
  path; GREEN: migrate all latching tests; assert both paths pass.
- Acceptance: AC2, AC9.

### M3 — Menu + console on v4

**M3.1 MenuRoot on Scenario A (carryover + ADR-0014)**
- Files: carry over `internal/quakeui/menu/*` (MenuRoot, pages, transform);
  wire into `OverlayRenderer`; input via router (KeyMenu → UI only).
- Tests: `QUI` + `GAME` — RED: menu renders at 320x200 transform on the
  Scenario A canvas; navigation keys reach the UI only; GREEN.
- Acceptance: AC1, AC5, AC9.

**M3.2 ConsoleRoot on Scenario A**
- Files: carry over `internal/quakeui/console/*` (ConsoleRoot, completion);
  wire into `OverlayRenderer`; input via router (KeyConsole → UI only).
- Tests: `QUI` + `GAME` — RED: console dropdown renders; input/execution/
  tab-completion work via the router; GREEN.
- Acceptance: AC1, AC9.

### M4 — HUD + demo bar on v4

**M4.1 HUDRoot on Scenario A**
- Files: carry over `internal/quakeui/hud/*`; wire into `OverlayRenderer`;
  HUD is draw-only (fallthrough to engine).
- Tests: `QUI` + `GAME` — RED: status bar/crosshair/centerprint render; HUD
  keys fall through (KeyGame → engine); GREEN.
- Acceptance: AC1, AC9.

**M4.2 DemoBarRoot (ADR-0015, gap 9)**
- Files: new `internal/quakeui/demobar/demobar.go` — display-only progress
  bar mirroring legacy `drawRuntimeDemoControls` (research 0001 §7); wire
  into `OverlayRenderer` + stack.
- Tests: `QUI` — RED: progress bar renders from `DemoState` (track/cursor/
  status glyph/speed label/name/M:SS); GREEN.
- Acceptance: AC11.

**M4.3 CSQC fallback (gap 10)**
- Files: `internal/game/game_runtime_csqc.go` — when a mod's CSQC_DrawHud
  draws, HUD falls back to legacy CSQC canvas path on path 1.
- Tests: `GAME` — RED: stub CSQC DrawHud active + `ui_backend 1` → native HUD
  widget skipped; GREEN.
- Acceptance: §7 CSQC row.

### M5 — Cross-platform + verification

**M5.1 WASM (AC3b)**
- Files: verify `internal/game/game_runtime_frame.go` WASM path
  (`StepWasmFrame`/rAF) drives the Scenario A composite; no `desktop.Run`.
- Tests: `WASM` build green; `mise run smoke-*` WASM smoke at `ui_backend 1`.
- Acceptance: AC3b, AC6.

**M5.2 Headless fail-open (AC3c)**
- Files: `internal/game/game_runtime_frame.go` — headless `ui_backend 1`
  fails open to legacy cleanly (no panic, no UI render attempt).
- Tests: `GAME` — RED: headless boot at `ui_backend 1` panics; GREEN: fail-open.
- Acceptance: AC3c.

**M5.3 Full verification + parity (AC2, AC5)**
- Run `FULL` + `REND` + parity captures; in-window captures at `ui_backend 1`
  vs 0; `-screenshot` stays legacy oracle.
- Acceptance: AC2, AC5, AC6.

## 4. Traceability Matrix

| Task | Spec § | ADR | AC | Gaps |
| --- | --- | --- | --- | --- |
| M0.1 | §3.1/§8 | 0011 | AC10 | 17 |
| M0.2 | §5.2/§8 | 0012 | AC2, AC9 | 8, 13 |
| M0.3 | §2#10 | — | — | 7 |
| M1.1 | §3.3/§5.1/§8 | 0011 | AC3a, AC9 | 1-4 |
| M1.2 | §3.3/§8 | 0011 | AC9, AC10 | 2, 19 |
| M1.3 | §5.1/§8 | 0012 | AC4 | 16, 18 |
| M2.1 | §4.2/§5.2/§8 | 0012 | AC9 | 12 |
| M2.2 | §5.2/§8 | 0012 | AC2, AC9 | 8, 13 |
| M3.1 | §3.2/§5.2/§8 | 0014 | AC1, AC5, AC9 | 14 |
| M3.2 | §3.2/§5.2/§8 | 0014 | AC1, AC9 | 14 |
| M4.1 | §3.2/§5.2/§8 | 0014 | AC1, AC9 | 14 |
| M4.2 | §5.4/§8 | 0015 | AC11 | 9, 15 |
| M4.3 | §5.3/§7 | — | §7 CSQC | 10 |
| M5.1 | §7/§8 | 0011 | AC3b, AC6 | — |
| M5.2 | §7/§8 | 0011 | AC3c | 11 |
| M5.3 | §8 | — | AC2, AC5, AC6 | — |

## 5. Risks & Mitigations

| Risk | Mitigation | Gap |
| --- | --- | --- |
| Polling migration changes latching semantics | M0.2 spikes ONE latching test to polling first; both paths verified; rollback = keep callbacks | 8, 13 |
| MarkPreserveContent placement wrong (scene-target active) | M1.2 places it post-composite when `sceneTargetActive` (A2) | 19 |
| gg canvas GPU path falls back to CPU on software adapters | graceful degradation; software renderer stays legacy (G8); ggcanvas.Render universal path | — |
| uiApp/EventSource ownership startup-only | M1.3 startup-selection test asserts frozen path; router tests cover no-double-dispatch | 16, 18 |
| Full-redraw model cost | M0.3 spike measures; adopt full-redraw unless continuous mode proven cheap | 7 |
| CSQC fallback regression | M4.3 explicit stub test | 10 |
| WASM path breaks (desktop.Run residue) | M5.1 WASM build + smoke; `grep desktop.Run` empty enforced | — |
| Isolation boundary weakens again | AC7 import-closure test forbids BOTH internal/game and internal/renderer (hard) | 6, 14 |

## 6. Post-MVP follow-ups (bd candidates)

- Offscreen world texture composite (deferred G1; post-FX/waterwarp reuse).
- Interactive demo scrubbing (bd `ironwail-go-cuy`).
- Upstream feedback to gogpu #468 (deliberately out of this run, G10).
- PARITY.md UI deviations section for v4.

## Review Log

### Stage 8 — Review 3 (2026-08-21)

Verdict: APPROVED WITH FIXES. Five findings processed:
- P1 (high): M0.2 RED/GREEN was backwards — reworded to test the polling
  adapter against a stub `app.Input()` first, then integrate + migrate one
  latching test.
- P2 (med): M1.1 lacked a WASM guard — added lazy provider resolution
  (v1 `ProviderFunc` pattern) so the WASM path resolves the provider after
  Run; nil-provider → software fallback or fail-open (AC3c).
- P3 (med): redraw-model decision misaligned — spec §2#10 and plan M0.3 now
  agree: default is accept full redraws; continuous mode only if the spike
  shows material overhead reduction without breaking the widget tree.
- P4 (info): `QUI` test command covers new subpackages — no action.
- P5 (info): parity captures use the `-screenshot` legacy oracle — no action.

Self-check: tasks ≤ ~2 days each; TDD ordering survivable (M0 → M1 → M2 →
M3-M4 → M5; no task needs later code); test commands are the repo's real
commands (`TMPDIR=.../.tmp CGO_ENABLED=0 go test ./...`); file paths exist or
are new under `internal/quakeui|game`; acceptance criteria staged per task;
traceability matrix covers gaps 1-20.
