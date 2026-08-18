# RESEARCH-0004: gogpu/ui Integration Points in ironwail-go (Ownership, Input, Callbacks)

- **Status:** Integrated
- **Owner:** research (source deep-dive, 2026-08-18)
- **Date:** 2026-08-18
- **Blocks:** gap log #4 (ai-dlc/ui-gogpu-rewrite)

---

## Research Question

Where exactly does gogpu/ui plug into ironwail-go's existing gogpu ownership:
who owns the gogpu.App, who owns OnDraw/EventSource, what are the single-slot
callback conflicts, and what are the viable integration architectures for the
UI rewrite?

## Background & Constraints

- The engine already creates its own `gogpu.App` in
  `internal/renderer/renderer_gogpu_runtime.go:84` (`NewRenderer` →
  `gogpu.NewApp(gpuCfg)`) and runs it via `Renderer.Run()` →
  `r.app.Run()` (`renderer_gogpu_runtime.go:575-582`).
- `Renderer.OnDraw(callback)` (`:178`) registers `r.drawCallback` and calls
  `r.app.OnDraw(...)` — it owns the **single OnDraw slot**.
- The input backend `gogpuimpl.NewInputBackend(app, sys)`
  (`internal/renderer/input_backend_gogpu.go:10-12`) registers the **single
  EventSource slots** (`OnKeyPress/OnKeyRelease/OnTextInput/OnPointer/
  OnMouseMove/OnMousePress/OnMouseRelease/OnScroll/OnFocus`) in
  `initCallbacks` (`internal/renderer/gogpu/input_backend.go:217-330`).
- Cursor mode: `iinput.CursorModeGrabbed` ↔ `gogpu.Context.CursorMode() ==
  gpucontext.CursorModeLocked` (used in `input_backend.go` mouse-move
  accumulation). The engine grabs/ungrabs pointers itself
  (`syncGameplayInputMode`, mouse grab released on menu open).

## Investigation Findings

### 1. gogpu.App callback API is single-slot

From `github.com/gogpu/gogpu@v0.52.1` (`app.go`):
- `OnDraw(fn func(*Context)) *App` (:182) — sets `a.onDraw` plus
  `primaryWindow.onDraw`. A second caller overwrites the first.
- `OnUpdate(fn func(float64))` (:196), `OnResize` (:207), `OnClose` (:221?)
  likewise single-slot.
- `eventSourceAdapter` (`event_source.go:45-120`): one callback field per
  event kind (`onKeyPress`, `onPointer`, `onScroll`, `onMouseMove`, ...);
  `OnX` setters overwrite.
- Dispatch: platform event → `a.dispatchKeyEvent` → `dispatchKeyToWindow`
  (window-manager callback) → `dispatchKeyToEventSource` (eventSource
  adapter) → `dispatchKeyToInputState` (polling state). The input-state
  polling path (`a.Input()`, `Keyboard().Pressed(...)`) is independent of
  the EventSource callbacks and is what the legacy poll path in the engine's
  backend uses as fallback.

Consequences:
- **gogpu/ui's `app.New(app.WithEventSource(gogpuApp.EventSource()))` would
  overwrite the engine's input backend callbacks** (they register on the same
  adapter). The engine's `initCallbacks` guards with `callbacksInited` — once
  either registers, the other's registrations are lost.
- **gogpu/ui's desktop/manual OnDraw flow would overwrite the engine's
  `Renderer.OnDraw`** (or vice versa, depending on ordering).

### 2. Existing engine → gogpu touchpoints

- App creation: `renderer_gogpu_runtime.go:84`.
- Render loop: `Renderer.Run()` → `r.app.Run()`; WASM path differs
  (`StepWasmFrame`, `wasm_blit.go`).
- Draw injection: `Renderer.OnDraw` → wraps callback in `DrawContext` and
  passes `renderer.RenderContext` (the 2D interface). The game registers
  `g.Renderer.OnDraw(func(dc renderer.RenderContext){ ... drawRuntimeFrame
  ... })` at `game_runtime_frame.go:176`.
- Input: `gogpuimpl.NewInputBackend(r.app, sys)` via
  `Renderer.InputBackendForSystem`; engine polls/registers as above.
- Misc: `Renderer.SurfaceView()`, `DrawTextureEx`, full `gogpu.Context`
  available to renderer internals (world/entities/overlay all draw through
  it).

### 3. Viable integration architectures for the UI rewrite

**Architecture A — Engine-owns-window, gogpu/ui rendered into engine canvas
(manual embedding path).**
- Engine keeps `gogpu.App`, swapchain, `OnDraw`, and input backend untouched.
- Per frame (inside the existing draw callback / after `flush2DOverlay`):
  1. `uiApp.Frame()` (advance signals, layout, draw into the gg canvas)
  2. `uiApp.Window().DrawTo(widgetCanvas)` where `widgetCanvas` is
     `render.NewCanvas(cc, w, h)` over a gg canvas the engine creates on the
     same `gogpu.Context` (pattern from `docs/GETTING_STARTED.md:74-112`).
  3. Composite the gg canvas output onto the engine surface (engine's own
     texture upload + draw, or gg's `RenderDirect` with the surface view).
- Input: engine's `input.System` remains authoritative; engine forwards
  translated key/mouse events into the ui tree via `uiApp.HandleEvent(e)` or
  by bridging gpucontext events (see Architecture C).
- **Pros:** no change to renderer ownership; world/entities/overlay pipeline
  untouched; input latching tests keep passing; engine's snapshot/screenshot
  paths keep working.
- **Cons:** two retained-mode systems on one frame; the ui widget tree draws
  into gg canvas then engine blits (extra copy vs GPUView); coordinate/DIP
  scaling must be bridged; the gg canvas needs device context wiring (new
  dependency `gg` + `ggcanvas`/`render`).

**Architecture B — desktop.Run with GPUView (gogpu/ui owns the window loop).**
- `desktop.Run(gogpuApp, uiApp)`; engine's world/entities rendered into a
  `gpuview.Widget` texture via `OnRender(func(view gpucontext.TextureView){...
  engine commands with view as color attachment ...})` with
  `gpuview.Continuous(true)`; UI overlays as widgets on top.
- **Pros:** full layer-tree compositing; GPUView keeps world rendering
  off-screen (preserve-content alpha compositing); matches the #468 BYO-kit
  composition exactly; per-boundary damage/redraw efficiency.
- **Cons:** the engine's current `RenderFrame` draws world directly to the
  surface — would need to re-target the scene-target pipeline (world +
  entities + waterwarp + polyblend into the gpuview texture, which is
  roughly the existing `sceneTargetActive` machinery) — significant renderer
  surgery; input single-slot conflict must be solved (engine backend vs ui
  bridge); screenshot/parity/host-speeds paths and software fallback need
  rework; `desktop.Run` blocks (must adapt to engine's run loop).

**Architecture C — Engine owns input, forwards to ui tree (cross-cutting).**
- Keep `input.System`/KeyDest router. When a gogpu/ui surface is active:
  - Keyboard: forward `event.KeyEvent`s into `uiApp.Window().HandleEvent`
    (or via a synthesized `gpucontext.EventSource`), and consume engine-side
    only if the ui returns unhandled.
  - Mouse: forward absolute positions (menu needs absolute, gameplay needs
    deltas — engine already computes both).
- This lets existing KeyDest semantics (game vs console vs menu) subset the
  ui tree (e.g. console input field focuses only when KeyConsole).

**Architecture D — uiApp owns EventSource, engine input backend bridged.**
- Give `app.WithEventSource(gogpuApp.EventSource())` to uiApp; route
  engine-required events (mouse delta accumulation for look, backtick key,
  etc.) via the ui app's own bridge or by re-registering the engine backend
  as a second hop. Requires the ui app to expose unconsumed events — not a
  current API (Window.HandleEvent returns void; no "was it consumed" out
  path).
- **Cons:** no consumed-propagation API today; risk to input regression
  tests. Least preferred.

### 4. Canvas/coordinate bridging

- gogpu/ui widgets work in logical DIP pixels at window size with `Scale()`.
  The engine's 320x200 canvas model (`CanvasMenu`, `CanvasSbar`) is a
  transform on top of the 2D interface. Under gogpu/ui the engine would
  instead size/position widgets in a 320x200 "menu viewport" layout (a Box
  with fixed aspect scaled to `scr_menuscale`), or translate canvas params
  into widget geometry. The engine's `internal/game/ui` helpers
  (`GUIDimensions`, `ConsoleDimensions`, `CanvasParams`, `StepConsoleSlide`)
  remain the source of truth for dims; only the draw target changes.
- Pixel aspect: `scr_pixelaspect` affects GUI dims — must be reflected in the
  widget viewport size.

### 5. What breaks / what must be preserved (blast radius)

- `Renderer.RenderFrame` phase 5 (overlay) gains or loses the UI draw
  depending on architecture (A: overlay draws engine HUD? or ui tree?;
  B: overlay becomes ui tree only).
- Software renderer (`renderer/software.go`) and screenshot path
  (`game_loop.go:591-604`) call `g.Menu.M_Draw(soft)` / console / HUD — any
  rewrite that removes those draw functions must keep a fallback path for
  headless/screenshot parity (or route these through `offscreen.Renderer`).
- Host-speeds timing, `Pass2DOverlay` toggle, walkthrough inspector pass
  toggles (`renderer_gogpu_frame.go` phase gates) reference the overlay
  callback — keep the hook shape.
- WASM: `wasm_blit.go` + `StepWasmFrame` path cannot use `desktop.Run`;
  Architecture A (engine-owned) is wasm-compatible; GPUView needs non-
  desktop blit in wasm (engine already has `SurfaceView`/upload paths).
- Parity screenshots (`mise run parity-*`) capture the overlay — must still
  render identical pixels under the experiment branch toggle.

## Recommended Resolution

- Default to **Architecture A** (engine-owns everything; gogpu/ui rendered
  into the engine frame via `Frame()` + `DrawTo(render.NewCanvas(...))`),
  with **Architecture B (GPUView)** as the stretch/goal architecture for the
  3D-viewport case once the block diagram is proven — decision left to the
  spec/ADR (there is real tradeoff; see Open Questions).
- Input follows **Architecture C** (engine KeyDest authoritative, forwards
  into ui tree) to preserve the latching/mouse-look regression suite.
- Preserve: `RenderFrame` phase 5 hook, software/screenshot fallback,
  `internal/game/ui` dims math, cvar surface; add a branch gate cvar
  (e.g. `ui_backend 0|1`) to flip between legacy and gogpu/ui paths for
  experimental comparison.

## Open Questions / Follow-ups

- Does `render.NewCanvas` + gg canvas work on the engine's existing
  `gogpu.Context` (device provider) without extra window management? Needs a
  spike (task in plan).
- GPUView under non-desktop: engine-owned blit of the `gpucontext.TextureView`
  — feasible with existing `ctx.DrawTextureEx`/surface machinery? Needs spike.
- `desktop.Run` block semantics vs engine's run loop (`Game.Run` owns loop;
  would need `desktop.Run` decomposed to per-frame `renderLoop.draw`) — spike.

## Source Index

- `/home/darkliquid/go/pkg/mod/github.com/gogpu/gogpu@v0.52.1/app.go`
  (OnDraw :182, OnUpdate :196, dispatch functions :880-970)
- `/home/darkliquid/go/pkg/mod/github.com/gogpu/gogpu@v0.52.1/event_source.go`
  (eventSourceAdapter :12-120)
- `/home/darkliquid/go/pkg/mod/github.com/gogpu/ui@v0.1.54/docs/GETTING_STARTED.md`
  (manual embedding :74-112)
- `/home/darkliquid/go/pkg/mod/github.com/gogpu/ui@v0.1.54/examples/gpuview/main.go`
- `/home/darkliquid/go/pkg/mod/github.com/gogpu/ui@v0.1.54/app/event_bridge.go`
  (attachEventBridge :68, keyboard :242, pointer :302)
- `/home/darkliquid/go/pkg/mod/github.com/gogpu/ui@v0.1.54/app/window.go`
  (HandleEvent :424, Frame :671, DrawTo :1060, HandlePointerEvent :1328)
- ironwail-go: `internal/renderer/renderer_gogpu_runtime.go:84,178,575`,
  `internal/renderer/input_backend_gogpu.go:10-12`,
  `internal/renderer/gogpu/input_backend.go:217-330`,
  `internal/game/game_runtime_frame.go:176,282-360`,
  `internal/game/game_loop.go:591-604`, `internal/renderer/software.go`
