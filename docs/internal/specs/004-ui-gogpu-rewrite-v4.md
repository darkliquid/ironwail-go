# IRONWAIL-SPEC-004: gogpu/ui UI Rewrite v4 — Engine-Owned Scenario A + Decoupled Input Router

**Component Identifier:** IRONWAIL-SPEC-004
**Status:** DRAFT (Stage 2) — pending Review 1
**Date:** 2026-08-21
**Branch:** `experiment/ui-rewrite-v4` (new)
**Supersedes:** IRONWAIL-SPEC-003 (v3: CPU gg.Context + custom WGSL blit + reflection)
**Language/Runtime:** Go 1.26,, `CGO_ENABLED=0`, gogpu/ui v0.1.54 / gogpu v0.53.0 / gg v0.52.3
**Primary dependencies:** `github.com/gogpu/ui`, `github.com/gogpu/gg`, `github.com/gogpu/gogpu`
**Target location:** `internal/quakeui/` (self-contained) + engine overlay pass + decoupled input router in `internal/game`
**Related research:** deep-research report `~/.local/share/crush/research/gogpu-ui-engine-owned-rendering/report.md`; research 0008 (three-branch evaluation); SPEC-001/002/003; ADRs 0001-0010

---

## 1. Metadata & Overview

### 1.1 Purpose

Rewrite ironwail-go's UI on gogpu/ui a fourth time, using the **engine-owned
Scenario A pattern** proven by the gogpu org's own g3d engine (ADR-048,
`examples/fullscreen-overlay`). The engine keeps full authority over the
frame loop (native + WASM + headless). The world renders to the swapchain
surface; the engine calls `gogpu.Context.MarkPreserveContent()`; the gogpu/ui
widget tree renders on top via a GPU-backed `ggcanvas.Canvas` +
`render.NewCanvas` + `window.DrawTo` + `canvas.RenderDirect` — `LoadOp::Load`,
alpha blend, **GPU-accelerated, no CPU readback, no custom WGSL shader, no
reflection into gogpu internals**.

Input is decoupled into a single router that splits events between the engine
and the UI by `KeyDest`: the engine gameplay path polls `gogpu.App.Input()`
(Ebiten-style, already fed by gogpu), and the UI owns the `EventSource` via
`app.WithEventSource`. This eliminates the double-delivery bug that forced v3
to rewrite its input backend.

### 1.2 Scope (from Stage 1 elicit — user decisions)

| Subsystem | MVP status |
| --- | --- |
| Main menu + submenus | **IN** — full rebuild (carryover widgets) |
| Dropdown console | **IN** — full rebuild (carryover widgets) |
| Player HUD (status bar, crosshair, centerprint) | **IN** — full rebuild (carryover widgets) |
| Demo playback controls (progress bar) | **IN** — no longer deferred (G6) |
| CSQC-driven HUD bridge | **IN (fallback)** — CSQC mods fall back to legacy HUD path |
| World render target | **Surface (Scenario A)** — offscreen world texture deferred (G1) |
| Software/screenshot renderer | **OUT** — stays legacy parity oracle (G8) |

**Out of scope (explicit non-goals):** offscreen world texture compositing
(post-FX/waterwarp reuse), GPUView-as-engine-target (Scenario B ownership),
a second "modern" design system, networking/multiplayer UI changes, packaging
as an L1 gogpu/ui module.

### 1.3 Branch & gating strategy

- New branch `experiment/ui-rewrite-v4` branched from `experiment/ui-rewrite-v3`.
- Keep the `ui_backend` cvar (0|1, default 0): 0 = legacy path (untouched
  parity oracle), 1 = gogpu/ui v4 path. **Startup-only (G11):** the cvar is
  parsed ONCE at engine startup, before any rendering, and frozen for the
  session. No runtime mid-session switching. This removes the per-frame path
  branch in the hot overlay loop, the mid-session EventSource re-registration,
  and the toggle teardown. If `ui_backend 1` is chosen but gogpu/ui init
  fails at startup, the engine fails open to the legacy path for the session.
- Software renderer / `-screenshot` / headless force legacy at startup (G8).
- Pure Go (`CGO_ENABLED=0`), zero `//go:build` tags.

### 1.4 Definition of Done (MVP)

1. `ui_backend 1` renders menu, console, HUD, and demo bar over the live 3D
   world with real `gfx/*.lmp` pics + conchars text at legacy 320x200 layout,
   GPU-accelerated via the Scenario A path (no CPU readback of the UI).
2. The engine owns the frame loop: native desktop, WASM (`StepWasmFrame` /
   rAF), and headless all boot and render the UI path without `desktop.Run`.
3. Input is decoupled: the engine polls `app.Input()` for gameplay; the UI
   owns the EventSource; a single router splits by `KeyDest` (KeyGame →
   engine, KeyConsole/KeyMenu → UI, HUD-only → engine). No double-delivery;
   existing latching/mouse-look tests pass (migrated to the polling path).
4. Strict isolation boundary (AC7-style, hard): `internal/quakeui` imports
   only gogpu/ui + legacy state machines + `internal/image`; it never imports
   `internal/game` or `internal/renderer`. Verified by import-closure test.
5. Full suite green on both paths; no regressions in `internal/menu`,
   `internal/console`, `internal/hud`, `internal/game` input latching tests
   (path 0 unchanged).
6. No `//go:build` tags; `CGO_ENABLED=0` preserved.
7. The v3 reflection/unsafe hack into gogpu internals is **removed**;
   `MarkPreserveContent()` is used instead.
8. **`ui_backend` is startup-only (G11):** parsed once, frozen for the
   session; no mid-session switching. AC4 (toggle) is replaced by a
   startup-selection test.

## 2. High-Level AI-DLC / Agent Prompt

Rewrite the menu, console, HUD, and demo bar on gogpu/ui behind `ui_backend
0|1`. Follow IRONWAIL-SPEC-004, ADRs 0011-0015, and the deep-research report.
Constraints:

1. **Engine-owned Scenario A (the g3d-proven pattern).** Engine owns the
   `gogpu.App`, the frame loop, and `OnDraw`. Per frame:
   a. Pass 1: engine renders world+entities+viewmodel+polyblend to the
      swapchain surface (existing renderer passes, unchanged).
   b. Seam: `dc.MarkPreserveContent()` after the world pass (public API,
      ADR-065; NOT reflection).
   c. Pass 2: the UI widget tree draws into a GPU-backed `ggcanvas.Canvas`
      created from `gogpuApp.GPUContextProvider()`; wrap with
      `render.NewCanvas(cc, w, h)`; `uiApp.Window().DrawTo(widgetCanvas)`;
      `canvas.RenderDirect(sv, sw, sh)` — `LoadOp::Load`, alpha blend, GPU.
   d. Present: engine presents the composited surface.
2. **No `desktop.Run`.** `grep -rn "desktop.Run" internal/` must be empty.
3. **Decoupled input router.** The engine gameplay path polls
   `gogpuApp.Input()` (`Keyboard().Pressed/JustPressed`, `Mouse().Delta/
   Position/Scroll`) each frame — gogpu already feeds this state from every
   platform event. The UI owns the `EventSource` via `app.WithEventSource`.
   A single router (`internal/game/input_router.go`) is the policy point:
   per `KeyDest`, route to engine | UI | both. Backtick/binding capture
   preserved. Existing latching/mouse-look tests migrate to the polling path.
4. **Strict isolation (AC7, hard).** `internal/quakeui` imports only gogpu/ui,
   the legacy state machines (read-only via accessors), and `internal/image`.
   It NEVER imports `internal/game` or `internal/renderer`. Renderer types
   stay behind the `quakeui.Host` adapter. Import-closure test enforces it.
5. **Carryover.** Reuse the v2/v3 widget tree (menu/console/HUD roots, conchars
   atlas, LMP pic bridge, `ComputeMenuTransform`), the `Host` adapter, the
   `ui_backend` gate, the `Stack` surface model, and the `menu.Manager`
   accessor surface. Rebuild only the integration seam (Scenario A pass) and
   add the decoupled input router + demo bar widget.
6. **Visual fidelity.** Real `gfx/*.lmp` pics + conchars bitmap text at legacy
   320x200 layout. No TTF for menu/console/HUD text.
7. **Demo bar (G6).** The demo playback progress bar renders on the v4 path
   (no longer deferred). Interactive scrubbing stays out of scope (bd
   `ironwail-go-cuy`).
8. **CSQC fallback.** When a mod's CSQC_DrawHud draws, the HUD falls back to
   the legacy CSQC canvas path even with `ui_backend 1`.
9. **Platforms (G7, hard AC).** Native + WASM + headless all boot the UI
   path. WASM uses `StepWasmFrame`/rAF (engine-owned, no `desktop.Run`);
   headless uses the software fallback for the UI path or fails open to
   legacy (G8).
10. **Redraw model (G4).** Default: **accept full redraws per frame** (all
    three prior branches did). An M-spike (M0.3) evaluates a continuous-render
    mode (skip retained-mode invalidation bookkeeping); it is adopted ONLY if
    the spike shows material overhead reduction without breaking the widget
    tree. Alignment: the plan's M0.3 default decision matches this spec.
11. **No reflection/unsafe into gogpu internals.** `MarkPreserveContent()` is
    the seam. If a private field is needed, that is a gap → research, not a
    hack.
12. **`ui_backend` is startup-only (G11).** Parse once at startup; freeze for
    the session. No mid-session switching. Software/headless force legacy.
13. TDD red/green per task; run
    `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/quakeui/...` and
    `mise run test` before closing a task.

## 3. Information Architecture / Topology

### 3.1 Package layout

```
internal/quakeui/              SELF-CONTAINED gogpu/ui subsystem (v4)
  adapter.go                   Host interface (CVar, PlaySound, ExecuteCommandText, Quit)
  backend.go                   ui_backend cvar reader
  overlay.go                   OverlayRenderer — engine-owned Scenario A driver
                               (DrawOverlay into a ggcanvas.Canvas + RenderDirect)
  stack.go                     Stack container (MenuRoot, ConsoleRoot, HUDRoot, DemoBarRoot)
  gfx/                         QPicToImage, ConcharsAtlas, skin translation
  menu/                        MenuRoot (real LMP pics, submenu layouts, navigation)
  console/                     ConsoleRoot (single-image batched compositing, dropdown, input)
  hud/                         HUDRoot (status bar, crosshair, centerprint)
  demobar/                     DemoBarRoot (progress bar, NEW in v4)
internal/game/                 WIRING ONLY
  game_runtime_frame.go        Scenario A pass: world → MarkPreserveContent → overlay
  game_input.go                KeyDest router (unchanged)
  input_router.go              NEW — decoupled input router (engine polling | ui EventSource)
  quakeui_host.go              implements quakeui.Host
internal/renderer/             MINIMAL CHANGE: overlay phase hook; REPLACE the
                               v3 reflection/unsafe markGoGPUFrameContentForOverlay
                               with a direct dc.ctx.MarkPreserveContent() call (R1.7)
```

**Isolation boundary (AC7, hard):** `internal/quakeui` imports only gogpu/ui,
the legacy `internal/menu|console|hud` state machines (read-only via
accessors), and `internal/image`. It NEVER imports `internal/game` or
`internal/renderer`. The `quakeui.Host` adapter (defined in `internal/quakeui`)
abstracts cvar reads, command text, sound, and quit as plain values /
`func(string)` sinks — no engine types.

### 3.2 Widget tree per surface

```
Root (window-sized; host-managed via engine-owned DrawTo)
├── [hud surface]    HUDRoot (non-interactive overlay)
├── [console surface] ConsoleRoot (captures input when active)
├── [menu surface]   MenuRoot (captures input when active; real LMP pics)
└── [demo surface]   DemoBarRoot (display-only progress bar)
```

The engine world is the base (already on the surface via Scenario A); the UI
widgets composite above with `LoadOp::Load`. The UI never clears the world.

### 3.3 Layering in the frame (Scenario A)

```
Engine Frame (game_runtime_frame.go):
  1. Engine updates physics, client state, camera
  2. Pass 1: engine renders 3D world into Swapchain View (existing passes).
     NOTE (A2): when waterwarp/translucent-liquid rendering is active
     (sceneTargetActive), the world renders into an offscreen scene texture
     and is composited back; MarkPreserveContent must come AFTER that
     composite.
  3. dc.MarkPreserveContent()                    // public seam, ADR-065
  4. If ui_backend == 1:
       OverlayRenderer.DrawOverlay(cc, w, h):
         a. Layout Stack to (w, h)
         b. widgetCanvas := render.NewCanvas(cc, w, h)
         c. uiApp.Window().DrawTo(widgetCanvas)  // widget tree draws into gg canvas
         d. canvas.RenderDirect(sv, sw, sh)      // GPU flush, LoadOp::Load
     Else (ui_backend == 0):
       drawRuntimeOverlayFrame(legacy)
  5. Present
```

No offscreen texture, no CPU readback, no custom WGSL — the g3d-proven
pattern.

## 4. Data Models & Interface Contracts

### 4.1 OverlayRenderer Contract

```go
type OverlayRenderer struct {
    host     Host
    stack    *Stack
    menuRoot *quakuimenu.MenuRoot
    conRoot  *quakeuiconsole.ConsoleRoot
    hudRoot  *quakeuihud.HUDRoot
    demoRoot *quakeuidemobar.DemoBarRoot
    canvas   *ggcanvas.Canvas   // GPU-backed, created from GPUContextProvider
    uiApp    *app.App
}

func NewOverlayRenderer(host Host, mgr *legacymenu.Manager, con *console.Console,
    hudProv hud.Provider, drawMgr *draw.Manager, conchars, palette []byte,
    provider gpucontext.DeviceProvider) *OverlayRenderer
func (r *OverlayRenderer) DrawOverlay(cc *gg.Context, width, height int) error
func (r *OverlayRenderer) Event(e event.Event) bool
func (r *OverlayRenderer) Close() error   // releases canvas + uiApp
```

### 4.2 Input Router Contract

```go
// internal/game/input_router.go
type InputRouter struct {
    engine *EngineInput   // polls gogpuApp.Input() for gameplay
    ui     *quakeui.Forwarder  // forwards to uiApp.HandleEvent / EventSource
}

// RouteInput is the single policy point. Called once per platform event.
// keyDest decides (exclusive for keys, R1.2):
//   KeyGame/HUD-only → engine only (polling app.Input())
//   KeyConsole/KeyMenu → UI only (EventSource → widget tree)
//   KeyMessage → engine (chat input)
//   backtick/binding-capture → engine pre-route
func (r *InputRouter) RouteInput(e platform.Event)
```

The engine gameplay path polls `gogpuApp.Input()` (`Keyboard().Pressed`,
`JustPressed`, `Mouse().Delta/Position/Scroll`) each frame — gogpu already
feeds this from every platform event (dispatchKeyEvent → dispatchKeyToInputState,
dispatchPointerEvent → updateMouseStateFromPointer). The UI owns the
`EventSource` via `app.WithEventSource(gogpuApp.EventSource())`.

**Isolation note (R1.4):** the `quakeui.Host` adapter and
`OverlayRenderer.DrawOverlay(cc *gg.Context, ...)` may pass gogpu/gg types
(`gg.Context`, `gpucontext.TextureView`) — these are gogpu ecosystem types,
not engine internals. AC7 forbids only `internal/game` and
`internal/renderer` in the `internal/quakeui` import closure.

### 4.3 Retained legacy sources of truth

- `menu.Manager` state/actions (accessors from M3.1 v1) — consumed read-only.
- `console.Console` ring buffer/input/completion.
- `hud.State` + scoreboard.
- `renderer.CanvasTransformParams` / `internal/game/ui` dims math → translated
  to widget geometry; `scr_menuscale`/`conscale`/`sbarscale` as transforms.

### 4.4 cvar

- `ui_backend` 0|1 default 0 (kept from v1).

## 5. State Machines / Flows

### 5.1 Boot / gating

```
init: registered ui_backend (0 legacy / 1 gogpu/ui)
Run frame:
  path = ui_backend==1 ? quakeui : legacy
  path==legacy → existing host/screen/menu/console/HUD (untouched)
  path==gogpu-ui → engine-owned Scenario A:
        world pass → MarkPreserveContent → OverlayRenderer.DrawOverlay
        (widget tree into ggcanvas + RenderDirect) → present
```

The engine's `gogpu.App` is created by the renderer as today. The UI app is
created **once at startup when `ui_backend 1` is selected** (G11 — startup-only,
supersedes R1.6's lazy-on-first-frame) with
`app.WithWindowProvider/WithPlatformProvider/WithEventSource/WithTheme` —
safe before `gogpuApp.Run()` (g3d examples create it before Run). On
`ui_backend 0` (or software/headless), the UI app is never created; the legacy
path is used for the whole session. If UI init fails at startup with
`ui_backend 1`, fail open to legacy for the session. Teardown happens on
engine shutdown via `gogpuApp.OnClose` (releases the `ggcanvas` + `uiApp`).

### 5.2 Input

- **KeyDest router** (engine) unchanged: KeyGame/KeyConsole/KeyMenu/KeyMessage.
- **Decoupled router** (`input_router.go`): the single policy point.
  - KeyGame / HUD-only → engine gameplay (polling `app.Input()`).
  - KeyConsole / KeyMenu → UI only (EventSource → widget tree). The engine
    does NOT also process these keys on path 1 (guards the v3
    double-dispatch bug `TestUIRoutingMenuKeyDoesNotDoubleDispatch`).
  - KeyMessage → engine (chat input stays engine-side).
  - Backtick/binding capture → engine pre-route (before the router).
  - "Both" applies only to the mouse where KeyDest genuinely needs it; by
    KeyDest the split is exclusive for keys (R1.2).
- Engine gameplay input migrates from the callback backend to polling
  `app.Input()`. Latching/mouse-look tests migrate to the polling path,
  de-risked by the M0 spike (R1.3).

### 5.3 CSQC

- CSQC active → HUD falls back to legacy CSQC canvas path (mod compat).

### 5.4 Demo bar

- `DemoBarRoot` renders the playback progress bar (previously deferred).
- **Display-only** (R1.5): mirrors the legacy `drawRuntimeDemoControls`
  (research 0001 §7 — "not clickable/draggable; no mouse interaction").
  Interactive scrubbing stays out of scope (bd `ironwail-go-cuy`).

## 6. Security Model

No new attack surface: all input is local; text/images come from local Quake
assets (palette.lmp, conchars, gfx/*.lmp). The UI composites over the engine
surface only. No network/auth/secrets. A11y roles inert on desktop.
CSP/eBPF N/A.

## 7. Edge Cases & Failure Handling

| Edge case | Behavior |
| --- | --- |
| UI init failure (app.New/ggcanvas.New error) at startup with ui_backend 1 | log, freeze path to legacy (fail-open) for the session |
| `ui_backend 1` + software/screenshot renderer | legacy path always (no gpu canvas); screenshots stay comparable (G8) |
| `ui_backend 1` + WASM (`GOOS=js`) | engine-owned loop works (StepWasmFrame/rAF); no desktop.Run |
| `ui_backend 1` + headless | **fail open to legacy** (no UI render attempt, no panic); headless smoke asserts clean fallback (G8, R1.1) |
| CSQC mod active | HUD falls back to legacy path |
| Demo playback active | DemoBarRoot renders progress bar on path 1 |
| UI never clears world | MarkPreserveContent → LoadOp::Load; assert in capture/parity |
| window resize / DPI | ggcanvas.Canvas.Resize; widget tree re-laid-out |
| GPU resource lifetime | ggcanvas.Canvas + uiApp released on Close/Unmount; no per-frame leaks |
| input decoupling | engine polls app.Input(); UI owns EventSource; router splits by KeyDest; no double-delivery |
| ui_backend is startup-only (G11) | parsed once at startup, frozen; path never switches mid-session; init failure fails open to legacy |
| gg GPU path unavailable (software adapter) | fall back to CPU pixmap path (ggcanvas.Render universal path) or legacy |

## 8. Acceptance Criteria

| # | Criterion | Verifiable by |
| --- | --- | --- |
| AC1 | cvar/command surface identical on both paths; same menu structure | grep diff of registered cvars/commands; functional menu walkthrough |
| AC2 | full `mise run test` green; menu/console/hud/game tests pass on path 0 | `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./...` |
| AC3a | path 1 boots native and renders menu/submenus/console/HUD/demo bar over the live world without panic, no GPU leak growth | `mise run smoke-*` + `+ui_backend 1`; renderer resource counters |
| AC3b | path 1 boots WASM (`GOOS=js`) and renders the UI via `StepWasmFrame`/rAF (no `desktop.Run`) | WASM build + runtime smoke |
| AC3c | path 1 in headless mode fails open to legacy cleanly (no panic, no UI render attempt) | headless smoke at `ui_backend 1` |
| AC4 | ui_backend is startup-only (G11): parsed once, frozen; init failure at path 1 fails open to legacy; software/headless force legacy | startup-selection test: parse at boot, assert path frozen + fail-open on init error |
| AC5 | in-window captures at ui_backend 1 visually match legacy layout (real LMP pics, 320x200 geometry) | in-window capture at 1 vs 0; `-screenshot` stays legacy oracle |
| AC6 | no //go:build tags; CGO off; pure Go | grep empty; `go build` with CGO off |
| AC7 | internal/quakeui self-contained; internal/game only via narrow adapter; NO internal/game or internal/renderer in quakeui closure | import graph check (go list -deps) |
| AC8 | no stock core/* widgets used by quakeui widgets (BYO-kit) | import graph check |
| AC9 | UI never clears the world; input decoupled (engine polls, UI owns EventSource); no double-delivery | capture/parity assert world visible under UI; input tests for decoupling |
| AC10 | `desktop.Run` eliminated; no reflection/unsafe into gogpu internals; MarkPreserveContent used | `grep -rn "desktop.Run" internal/` empty; grep for unsafe/reflect in the seam; code review |
| AC11 | demo bar renders on path 1 (no longer deferred) | in-game demo playback at ui_backend 1 |

## 9. Cross-references

- Deep-research report: `~/.local/share/crush/research/gogpu-ui-engine-owned-rendering/report.md`
- Research 0008 (three-branch evaluation): `docs/internal/research/0008-ui-rewrite-branches-evaluation.md`
- Prior (superseded): SPEC-001/002/003, ADRs 0001-0010.
- g3d prior art: `gogpu/g3d` issue #5, ADR-048, `examples/fullscreen-overlay`.
- Gap log: `.ai-dlc/ui-rewrite-v4/gaps.md`.

## Review Log

### Stage 3 — Review 1 (spec defense), 2026-08-21

**Verdict: APPROVED WITH FINDINGS** (7 raised; 3 high, 3 med, 1 info — all
resolved in-spec).

| # | Finding | Seve. | Resolution |
| --- | --- | --- | --- |
| R1.1 | AC3 conflated "boots without panic" with "renders correctly"; headless has no surface for Scenario A | high | fixed: split AC3a (native renders) / AC3b (WASM renders) / AC3c (headless fails open to legacy); §7 headless row pinned to fail-open |
| R1.2 | Input router "ui + engine where legacy needs it" was ambiguous and risked the v3 double-dispatch bug | high | fixed: §4.2/§5.2 pin exclusive key routing (KeyGame→engine, KeyConsole/KeyMenu→UI only, KeyMessage→engine, backtick→engine pre-route); "both" only for mouse where KeyDest needs it |
| R1.3 | Polling migration (G2) asserted but not de-risked; callback-vs-polling double-delivery is subtle (v3 input backend rewrite) | high | fixed: added explicit M0 spike task (RED test first) migrating one latching test to polling before full migration |
| R1.4 | AC7 import-closure test only forbids internal/game+renderer; gg leak not caught | med | clarified: adapter/DrawOverlay may pass gogpu/gg types (not engine internals); AC7 forbids only engine packages |
| R1.5 | Demo bar scope ambiguous (render vs interactive scrubbing) | med | fixed: DemoBarRoot is display-only, mirroring legacy drawRuntimeDemoControls (research 0001 §7); scrubbing stays in bd cuy |
| R1.6 | uiApp lifecycle undefined (creation, teardown, before-Run safety) | med | fixed: lazy creation on first path-1 frame; teardown via gogpuApp.OnClose; g3d precedent noted (superseded by G11 — startup-only, no toggle-to-0) |
| R1.7 | "renderer UNCHANGED" contradicted AC10 no-reflection; v3 has the reflection hack | info | fixed: renderer change pinned as replacing markGoGPUFrameContentForOverlay reflection/unsafe with direct dc.ctx.MarkPreserveContent() (verified: the reflection fn already calls MarkPreserveContent, so the poke is redundant) |

## Change Log

| Date | What | Why |
| --- | --- | --- |
| 2026-08-21 | Initial spec draft (Stage 2) | After Stage 1 elicit (decisions G1-G10) + deep-research findings |
| 2026-08-21 | Stage 3 revision: folded Review 1 findings R1.1-R1.7 | Review 1 (spec defense) |
| 2026-08-21 | Scope revision: ui_backend startup-only (G11) | User decision — removes mid-session toggle complexity (per-frame branch, EventSource re-registration, teardown); AC4 rewritten; supersedes R1.6 lazy-creation |
