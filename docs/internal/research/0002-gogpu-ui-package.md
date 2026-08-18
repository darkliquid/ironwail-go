# RESEARCH-0002: gogpu/ui Package — Architecture, Widgets, Theming, and Game-UI Fit

- **Status:** Integrated
- **Owner:** research (source deep-dive of `github.com/gogpu/ui@v0.1.54`,
  plus discussion #468), 2026-08-18
- **Date:** 2026-08-18
- **Blocks:** gap log #2 (ai-dlc/ui-gogpu-rewrite)

---

## Research Question

What is gogpu/ui, what widgets does it provide, how does its theme/painter
system work (especially how rendering is themed/overridden for custom UIs),
what does the discussion #468 ecosystem RFC mean for ironwail-go, and how would
a Quake engine embed it (3D viewport + UI overlay, custom drawn widgets, custom
fonts/colors)?

## Background & Constraints

- ironwail-go pins `gogpu v0.52.1`, `gpucontext v0.28.0`, `wgpu v0.31.4`
  (`go.mod`). gogpu/ui v0.1.54 requires `gogpu v0.53.0`, `gg v0.52.3`,
  `gpucontext v0.28.0`, `wgpu v0.31.4` (its own go.mod) — **a dependency bump
  of `gogpu` v0.52.1 → v0.53.0 and a new `gg` v0.52.3 dependency** would be
  required (gg is not currently in ironwail-go's go.sum).
- The engine already consumes gogpu via `internal/renderer` (`gogpu.NewApp`,
  `OnDraw`, `EventSource`, platform input mapping in
  `internal/renderer/gogpu/input_backend.go`).
- Discussion #468 (VEE RFC) is at
  https://github.com/orgs/gogpu/discussions/468 — darkliquid participated; the
  maintainer explicitly invited ironwail-go as the **validation case** for the
  BYO-kit path and game-UI Painter/Canvas-gap work.

## Investigation Findings

### 1. What gogpu/ui is

- "Enterprise-grade GUI toolkit for Go", zero CGO, WebGPU rendering (via gg +
  wgpu), Flutter-style retained-mode widget tree. Targets IDEs, design tools,
  CAD, dashboards (README).
- Version v0.1.54 (2026-08-13), Go 1.25+. Module structure:
  `app/`, `widget/`, `event/`, `geometry/`, `gesture/`, `state/` (signals),
  `layout/`, `theme/` (core generic theme + `material3|devtools|fluent|cupertino`
  design systems), `core/` (27 widgets, one package each), `primitives/`
  (Box/Text/Image/ThemeScope/RepaintBoundary), `registry/`, `cdk/`, `plugin/`,
  `overlay/`, `focus/`, `a11y/`, `icon/`, `i18n/`, `animation/`, `transition/`,
  `render/`, `offscreen/`, `desktop/`, `compositor/`, `dnd/`, `uitest/`.
- **No `sdk/` directory exists in v0.1.54** — the VEE RFC proposes it
  (Phase A). Custom-widget authoring today is: implement `widget.Widget` +
  embed `widget.WidgetBase`, register via `registry.RegisterWidget` (docs
  `EXTENSIONS.md:39-107`).

### 2. The Widget contract

`widget.Widget` interface (`widget/widget.go:22-98`):
```go
type Widget interface {
    Layout(ctx Context, constraints geometry.Constraints) geometry.Size
    Draw(ctx Context, canvas Canvas)
    Event(ctx Context, e event.Event) bool
    Children() []Widget
}
```
- `widget.WidgetBase` (`widget/base.go:54-102`) — embeddable struct: bounds,
  visibility, enabled, focus, children tree, screen origin, compositor clip,
  **RepaintBoundary** + layout-cache flags, bindings/effects.
- Optional interfaces: `RepaintBoundaryMarker`, `LayoutFunc/DrawFunc/EventFunc`
  (`widget/widget.go:109-125`), `Lifecycle` (Mount/Unmount),
  `KeyGrabber`, `Accessible`.
- Rendering is **retained-mode**: `SetNeedsRedraw(true)` propagates dirty
  upward to nearest RepaintBoundary (`base.go:533,567` — Flutter
  markNeedsPaint, ADR-007/024). Boundary content recorded as `SceneCache`
  display lists (`widget/boundary_draw.go:71`), layer tree of
  per-boundary GPU textures (ADR-007 Phase 7; `desktop/desktop.go` renderLoop).
- Layout: Flutter box-constraint model; `geometry.Constraints`
  (`geometry/constraints.go:23`); `LayoutChild` caching (`layout_child.go:58`)
  + `MarkNeedsLayout` upward invalidation (`layout_cache.go:45`, ADR-032).
- `widget.Context` (`widget/context.go:36-113`): focus, time, Invalidate/
  InvalidateRect, cursor, `Scale() float32`, **`ThemeProvider() ThemeProvider`**,
  `OverlayManager` (PushOverlay/PopOverlay for popups — used by dialog,
  dropdown, context menu), `WindowSize`, `Scheduler`. Optional capability
  interfaces via type assertion: `PointerCapturer`, `GPUTextureProvider`
  (used by GPUView), `AnimationScheduler`, `ImageCacheProvider`,
  `DirtyTrackerProvider`.

### 3. The canvas and draw methods

`widget.Canvas` interface (`widget/canvas.go:79-203`):
`Clear`, `DrawRect`, `FillRectDirect`, `StrokeRect`, `DrawRoundRect`,
`StrokeRoundRect`, `DrawCircle`, `StrokeCircle`, `StrokeArc`, `DrawLine`,
`DrawText`, `MeasureText`, `DrawImage`, `PushClip/PopClip`,
`PushClipRoundRect`, `PushTransform/PopTransform`, `ReplayScene`.
Optional interfaces (type assert): `DamageController`, `BoundaryRecorder`,
`ArcStroker`, `SVGFiller`, `SVGRenderer`, `DeviceScaler`,
`TextModeController` (Auto/MSDF/Vector/Bitmap/GlyphMask — gg SDF text),
`StyledTextDrawer` (custom fonts: `DrawStyledText(text, bounds, TextStyle)`
with `FontFamily/FontSize/Bold/Italic/Color/Align`).

Concrete implementation `internal/render/canvas.go` sits on **gg**:
- `Context() *gg.Context` (`internal/render/canvas.go:76`) — **full escape
  hatch to the gg 2D API** (gradients, paths, transforms, blend modes, GPU
  textures). Public documented as "advanced use, bypasses Canvas state".
- `DrawGPUTexture(view gpucontext.TextureView, x, y, w, h)` (:619) for
  compositing GPU textures.
- Fonts: embedded Inter (regular/bold); `GlobalFontRegistry`
  (`internal/render/fontregistry.go`) maps family+weight+style → TTF data;
  custom fonts registered via `FontRegistry.Register(family, weight, style,
  data)`. `widget.Canvas.DrawText` uses Inter; `DrawStyledText` resolves via
  registry with CSS weight matching.

**Canvas gap analysis from discussion #468 (kolkov):** the standard Canvas has
18 methods + 9 optional interfaces but is missing for game UIs: gradients
(linear/radial), arbitrary paths (bezier/arc), **image regions (sprite atlas,
9-slice)** — proposed `ImageRegionDrawer` — rotation/scale transforms, and
blend modes. Planned Tier-1 granular interfaces: `GradientFiller`,
`PathDrawer`, `ImageRegionDrawer`; Tier-2 single escape hatch:
`widget.AdvancedCanvas{ GGContext() *gg.Context }`. **None of these exist in
v0.1.54** (verified by search). The engine can today reach the same power via
`Context() *gg.Context` on the concrete canvas, or via `DrawImage` with
`image.RGBA.SubImage` slices for sprite atlases (missing source-rect API).

Note: `ImageRegionDrawer` does **not** exist anywhere in gogpu/ui (main
branch or releases) — it is an aspirational interface proposed in #468.

### 4. Theme system (the critical part for Quake)

**Two theme tiers:**

(a) **Generic `theme.Theme`** (`theme/theme.go:23-200`) — the one passed to
`app.WithTheme` / `App.SetTheme`:
```go
type Theme struct {
    Name string
    Mode ThemeMode          // Light/Dark/SystemFollower
    Colors ColorPalette     // Primary, PrimaryLight/Dark, Secondary*,
                            // Background, Surface, SurfaceVariant,
                            // Error, Warning, Success, Info,
                            // OnPrimary, OnSecondary, OnBackground,
                            // OnSurface, OnError,
                            // Divider, Outline, Shadow
    Typography Typography   // FontFamily, body/label/title scales
    Spacing SpacingScale
    Shadows ShadowStyles
    Radii RadiusScale
    Extensions map[string]any       // custom data (Flutter ThemeExtension pattern)
    typedExts *typedExtensions
}
```
- Construction: `theme.New(name, mode)`, `theme.DefaultLight()` /
  `DefaultDark()` (presets.go), builder methods `Clone/WithName/WithMode/
  WithColors/WithTypography/WithSpacing/WithShadows/WithRadii/
  ScaleTypography/ScaleSpacing/Compact/Comfortable` (immutable by
  convention). **Fluent overrides**: `t.Colors.Primary = widget.Hex(...);`
  then pass the same `*theme.Theme` — documented pattern
  (`EXTENSIONS.md:163-179`).
- `ThemeExtension` interface (`theme/extension.go:63+`): `Name()`,
  `Merge(other)`, `Lerp(other, t)`, `CopyWith(overrides)` for component-
  specific custom tokens with animation; `theme.RegisterExtension`,
  `ExtensionAs[T]`.
- `ThemeScope` (`primitives/themescope.go:44`): **subtree theme override** —
  `primitives.ThemeScope(darkTheme, children...)`; priority chain: widget
  override > nearest ThemeScope > app theme > default. This is how the engine
  would give the console/menu scene a different visual theme from the in-game
  HUD.

(b) **Design-system painters + Bundle** (`theme/bundle.go:26`):
```go
type Bundle interface {
    Name() string
    BaseTheme() *Theme
    Painter(widget string) any   // "button", "checkbox", ..., concrete painter
    Painters() map[string]any
}
```
Built-ins: `material3` (New(seedColor)/NewDark, 21 painters), `devtools`,
`fluent`, `cupertino` (4 design systems, 61 painters per AGENTS.md).

**The Painter pattern (ADR-034) — the primary theming/override mechanism:**

Every core widget delegates visuals to a `Painter` interface:
```go
// core/button/painter.go
type Painter interface { PaintButton(canvas widget.Canvas, state PaintState) }
type LayoutMetrics interface {
    ButtonHeight(size Size) float32
    ButtonPadding(size Size) (paddingX, paddingY float32)
    ButtonFontSize(size Size) float32
    ButtonRadius() float32
}
```
- `PaintState` carries `Bounds`, hovered/pressed/focused/disabled flags, and a
  `ColorScheme` struct (per-widget `ButtonColorScheme`,
  `TextFieldColorScheme`, ...) with theme-derived colors.
- **LayoutMetrics lets painters control spatial metrics — not just colors**
  (Godot `theme_constant` equivalent). 7 widgets expose it.
- Setting: `button.New(button.PainterOpt(myPainter))` or the Bundle map:
  ```go
  b := material3.NewBundle(t)
  for name, painter := range b.Painters() { registry.SetPainter(name, painter) }
  ```
- Widgets without a painter use `DefaultPainter` (minimal gray).
- Per discussion #468, the Painter pattern "maps to the custom draw routine
  tier" — game engines' logic/visual separation (Unreal FSlateBrush→Material,
  Godot StyleBox, ImGui ImGuiStyle, Qt QStyle::drawControl). A game mod author
  implements `ButtonPainter`/`TextFieldPainter` etc with Quake-style visuals
  and behavior stays unchanged.

**ThemeBundle in `app`: not present in v0.1.54.** The VEE RFC proposes
`app.New(app.WithThemeBundle(nordicui.NewTheme()))` as Phase C. Today the only
app theme option is `app.WithTheme(*theme.Theme)`.

### 5. Embedding gogpu/ui in an existing gogpu app (ironwail-go path)

Pattern (from `examples/gpuview/main.go` and `docs/GETTING_STARTED.md`):
```go
gogpuApp := gogpu.NewApp(cfg)                    // ironwail already creates this
uiApp := app.New(
    app.WithWindowProvider(gogpuApp),            // gogpu.App satisfies all three
    app.WithPlatformProvider(gogpuApp),
    app.WithEventSource(gogpuApp.EventSource()),
    app.WithTheme(m3.AsTheme()),                 // or custom theme
)
uiApp.SetRoot(widgetTree)
```
Two run modes:
1. **`desktop.Run(gogpuApp, uiApp)`** (`desktop/desktop.go:58`) — owns the
   whole window and per-boundary GPU render loop; forces
   `RenderModeHostManaged`. **Path (b) below is the fit here.**
2. **Manual embedding** (`GETTING_STARTED.md:74-112`): engine keeps its own
   OnDraw; per frame: `uiApp.Frame()` → wrap the existing gg canvas as
   `widgetCanvas := render.NewCanvas(cc, w, h)` → `uiApp.Window().DrawTo(widgetCanvas)`.

**Conflict to resolve: single-slot callbacks on gogpu.App.** `gogpu.App.OnDraw`,
`OnUpdate`, `OnClose`, `OnResize` are single-slot setters (`app.go:182`);
the `EventSource` adapter holds **single** callbacks per event
(`event_source.go:45-120`: `onKeyPress`, `onPointer`, ... — a second
registration overwrites the first). ironwail's `Renderer.OnDraw` already holds
that single OnDraw slot, and `gogpuimpl.NewInputBackend` already registers the
single EventSource slots (`internal/renderer/gogpu/input_backend.go:
initCallbacks`). **Therefore gogpu/ui must either (a) interpose on the
EventSource/OnDraw single slots and fan out internally, or (b) be owned by the
renderer's existing draw path via `RenderModeHostManaged` + `DrawTo`.**
The VEE/maintainer comment confirms the intended BYO-kit composition path for a
game:
```go
import (
    "github.com/gogpu/ui/app"
    "github.com/gogpu/ui/widget"
    "github.com/gogpu/ui/core/gpuview"   // 3D viewport widget
    // your game-specific painters — no stock themes needed
)
```
Go dead-code elimination keeps unimported `core/*` out of the binary; `app` +
`widget` have zero deps on `core/`.

### 6. GPUView — the 3D viewport bridge

`core/gpuview.Widget` (`core/gpuview/widget.go:20`):
```go
func New(opts ...Option) *Widget
func Size(w, h int) Option
func OnRender(fn func(view gpucontext.TextureView)) Option   // render 3D into it
func Continuous(v bool) Option                               // per-frame 3D
// + Invalidate(), Texture(), ViewportSize(), SetContinuous(), Release()
```
- Owns an offscreen `gpucontext.TextureView`; texture creation deferred to
  first `Draw` via `GPUTextureProvider.CreateGPUTexture` (`widget/context.go:308`),
  wired by desktop to gg's `CreateOffscreenTexture` (`desktop/desktop.go:1056`).
- `OnRender` receives the texture view; the external renderer (the Quake
  engine, via `(*wgpu.TextureView)(tv.Pointer())`) issues GPU commands into
  it. `Continuous(true)` = per-frame re-render + `ScheduleAnimationFrame`.
- Layer tree: the widget is detected via unexported
  `externalTextureWidget` (`app/layer_tree.go:19-26`) → an
  `ExternalTextureLayer` sibling of the PictureLayer (`app/layer_tree.go:329`),
  blitted by the desktop compositor via `cc.DrawGPUTexture`
  (`desktop/desktop.go:876-883`). UI widgets composited after it draw on top.
- Example: `examples/gpuview/main.go` (no-op OnRender in demo).
- gg side preserves external content for alpha-compositing overlays
  (`FlushGPUWithViewPreserveContent`, gg v0.50.9+, per CHANGELOG) — exactly
  the "3D viewport with UI overlay" pattern. **Caveat:** GPUView is only
  composited by the `desktop` render loop, not by the standalone
  `compositor.Compositor` (which handles PictureOwner layers only) and not by
  `offscreen` (no GPUTextureProvider). If ironwail keeps its own renderer loop
  rather than `desktop.Run`, GPUView's texture must be blitted by the engine
  itself (engine already has `SurfaceView`, `DrawTextureEx`, and full
  `gpucontext` access).

### 7. Widget inventory (core/)

27 widgets, each its own package, constructors use functional options +
fluent setters + Painter + gestures:
`button` (4 variants/3 sizes), `textfield` (selection, clipboard, validation),
`checkbox`, `radio` (Group), `slider` (drag, click-to-position),
`scrollview`, `listview` (virtualized), `gridview`, `datatable` (sortable,
virtualized), `treeview`, `dropdown`, `menu` (MenuBar + ContextMenu),
`dialog` (modal overlay), `tabview`, `popover`, `tooltip`, `chip`, `badge`,
`collapsible`, `splitview`, `docking`, `toolbar`, `titlebar`, `stripe`,
`progress`, `progressbar`, `linechart`, `gpuview`.
`primitives/`: `Box/HBox/VBox`, `Text/TextFn`, `Image` (placeholder draw!),
`Expanded`, `RepaintBoundary`, `ThemeScope`.

Relevant to Quake:
- **textfield** → console input line (cursor, selection, scroll, validation),
  menu text fields (hostname/name/address). `TextFieldColorScheme` +
  `LayoutMetrics` for conchars look.
- **menu (Bar/ContextMenu)** + **dropdown** → menu popups; but Quake's menus
  are bespoke 320x200 layouts — likely **custom widgets** instead, with
  `OverlayManager` for popups.
- **listview** → load/save game slots, server browser, mods list.
- **slider** → volume/options sliders (or custom drawn rows).
- **dialog** → quit prompt / confirmation (or reuse menu's own prompt).
- **gpuview** → 3D viewport; **ThemeScope** → per-scene theming; **registry/**
  → registering Quake widget factories; **a11y** (35+ roles) free.

### 8. Input, focus, gestures, state

- App event bridge (`app/event_bridge.go:68`): `EventSource` → `event.KeyEvent`
  / pointer events → `Window.HandleEvent` (`app/window.go:424`): overlays
  first → focus (Tab/shift-tab + shortcuts) → pointer capture (ADR-031) →
  hover → root dispatch.
- Gestures (`gesture/`, ADR-049): arena-based; `ClickRecognizer`,
  `DragRecognizer`, `LongPressRecognizer`, `TapAndDragRecognizer`; widgets
  expose via `GestureAware.GestureHitTest(pos)`; `PointerCapturer` for drags.
- State (`state/state`): `Signal[T]` reactive values (coregx/signals),
  `Scheduler` — widgets bind signals for reactive text/values.
- Sound: `widget.RegisterSoundPlayer` + `sound.SetEnabled(true)` triggers
  platform sounds on interactive widgets (ADR-037). Engine already has
  `playMenuSound` infra; could map `SoundClick→misc/menu1.wav` etc.
- Keyboard focus: `FocusManager`; `KeyGrabber` for Tab trapping.

### 9. Input-mode conflict with the engine's KeyDest router

The engine routes all keyboard to its own `input.System` (`KeyGame/KeyConsole/
KeyMenu`). gogpu/ui expects to own the EventSource and dispatch events down its
widget tree. Two viable models:
- **UI-owns-input**: when a gogpu/ui surface is active (console open, menu
  open, HUD overlay), route OS events to the widget tree first; engine
  gameplay input only when the ui surface reports unconsumed events.
- **Engine-owns-input**: engine keeps KeyDest router, forwards a translated
  event stream into the widget tree (e.g. call `uiApp.HandleEvent`), and the
  UI returns "consumed" flags. This preserves existing latching/mouse-look
  regression tests (`game_movement_input_test.go:145-329`).

### 10. Discussion #468 (VEE RFC) — what it means for ironwail-go

- **darkliquid's reply:** "games tend to have very bespoke UIs that require a
  lot of control over what and how they render things, and may also need to be
  themeable to accommodate mods, DLCs, game modes... the toolkit needs to
  provide options for radical theming/graphical changes, whilst handling the
  generic UI logic and layouting."
- **kolkov's reply (the actionable part):**
  - The Painter pattern (ADR-034) + LayoutMetrics (7 widgets) + ThemeScope +
    GPUView are the intended game-UI mechanisms; a mod author implements
    `ButtonPainter` etc with game visuals, packaged as a Go module.
  - **Canvas gap table** (gradients, arbitrary paths, image regions/sprite
    atlas, rotation/scale, blend modes) — all supported by gg already, not yet
    exposed on `widget.Canvas`; planned `GradientFiller`, `PathDrawer`,
    `ImageRegionDrawer` (Tier 1) + `AdvancedCanvas.GGContext()` (Tier 2
    escape hatch, Dear-ImGui AddCallback analogue).
  - **BYO-kit composition for ironwail-go** (imports shown above): no stock
    themes needed; `app`+`widget` have zero deps on `core/`.
  - Explicit call: "If you're considering swapping ironwail-go's UI to
    gogpu/ui, that would be an ideal validation case: which Canvas methods are
    sufficient vs which need the escape hatch; is the Painter pattern flexible
    enough for Quake's visual style; does GPUView work for the 3D viewport
    overlay pattern?"
- Ecosystem timeline (VEE): Phase A `sdk/` + `examples/custom_widget/` +
  `docs/sdk/`; B catalog `gogpu/widgets` (index-only) + `widget.yaml` + CI;
  C ThemeBundle + registry metadata; D nightly re-verify + binary budget; E
  WASM/RPC (`gogpu/uiplugin`). Kernel boundary: ≥70% criterion; "add it
  whenever community widgets depend on it".
- Relevance: the ironwail-go rewrite is the **reference L1 extension / BYO-kit
  validation case** the org wants before v1.0; `ImageRegionDrawer` (or the
  `AdvancedCanvas` escape hatch) would likely land as a result.

## Recommended Resolution

- Target `ui@v0.1.54` semantics (Go 1.25+ compatible; ironwail-go is Go
  1.26). Accept the dependency bump `gogpu v0.52.1 → v0.53.0` + new `gg
  v0.52.3` (verify with the project's existing `gogpu` integration; gg is pure
  Go, zero CGO, so fits the "no CGO" rule).
- **Prefer Path (b) manual embedding**: engine keeps its own window/swapchain
  and render loop; per frame `uiApp.Frame()` then
  `Window().DrawTo(render.NewCanvas(cc, w, h))` where `cc` is the engine's gg
  canvas bridged from its renderer — OR use `desktop.Run` with the engine's
  world rendered into a `gpuview` texture via `OnRender`. Decision to be made
  in the spec (tradeoffs: engine renderer currently draws world+entities+overlay
  in one `RenderFrame`; GPUView requires world in offscreen texture).
- **Custom Quake widgets over stock widgets**: implement Quake-painter
  widgets (conchars text, palette fills, WAD pics) with `widget.WidgetBase`;
  use `primitives.ThemeScope` for console/menu scene theming; register via
  `registry`. Use `core/textfield` only for text input or write a conchars
  input widget.
- **Escape hatch**: rely on `Context() *gg.Context` (public on
  `internal/render/canvas.go:76`) for sprite atlas / gradients / paths until
  `ImageRegionDrawer` / `AdvancedCanvas` land upstream; track upstream in the
  plan.
- **Fonts**: register Quake's `conchars` as a bitmap font is not directly
  supported by the TTF-based font system; either (a) per-char `DrawImage`
  widgets from the conchars atlas (preserves 8x8 retro look), or (b) switch
  text to a TTF bitmap font renderer (loses retro glyphs). Spec decision.
- **Input**: resolve single-slot EventSource conflict (engine owns input,
  forwards into ui tree; or ui owns input while a ui surface is active).
- **Theming**: build `theme.Theme` from `theme.DefaultDark()` with Quake
  palette tokens; `ThemeScope` per scene; `ThemeExtension` (e.g.
  `quakeui/ConcharTheme{AscBright, palette, bg, promptGlyph, ...}`) for
  component tokens.
- Watch upstream for: `sdk/`, `ThemeBundle` in `app`, `ImageRegionDrawer`,
  `AdvancedCanvas` — the plan should pin the module and note upgrade points.

## Open Questions / Follow-ups

- Does `desktop.Run` work with the engine's existing input backend (both fight
  over the single-slot EventSource)? Needs a spike (spec research gap).
- Is gg's text rasterizer (SDF/MSDF/vector modes) good enough for 8x8 conchars
  or must we keep per-glyph image draws? (spike)
- GPUView vs engine-rendered-overlay: measure the cost of world-into-texture
  (currently world renders directly to surface) before choosing.

## Source Index

- `github.com/gogpu/ui@v0.1.54` module cache:
  `/home/darkliquid/go/pkg/mod/github.com/gogpu/ui@v0.1.54/` — file:line refs
  inline above (widget.go, base.go, context.go, canvas.go, theme/theme.go,
  theme/bundle.go, theme/extension.go, primitives/themescope.go,
  core/button/painter.go, core/gpuview/widget.go, core/textfield/widget.go,
  app/app.go, app/event_bridge.go, app/layer_tree.go, desktop/desktop.go,
  internal/render/canvas.go, internal/render/fontregistry.go,
  docs/{ARCHITECTURE,EXTENSIONS,GETTING_STARTED,RENDER-PIPELINE,VERSIONING}.md,
  CHANGELOG.md, AGENTS.md, README.md).
- Discussion #468: https://github.com/orgs/gogpu/discussions/468 (full body +
  comment thread fetched via gh api graphql, 2026-08-18).
- Related links from #468: gogpu/ui discussions #65, #229; ui PR #210
  (SceneCache); gogpu PR #65; RFCs #377 (naming), #328 (neon); ADR-034,
  ADR-036, ADR-057; docs/EXTENSIONS.md; goro, hearth, kaiju.
- GitHub code search confirmed `ImageRegionDrawer` absent in gogpu/ui (main).
