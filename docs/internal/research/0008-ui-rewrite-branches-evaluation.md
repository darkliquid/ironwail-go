# RESEARCH-0008: Three gogpu/ui Rewrite Branches — Evaluation & Upstream Feedback for Discussion #468

- **Status:** Draft (for external posting to https://github.com/orgs/gogpu/discussions/468)
- **Owner:** darkliquid (ironwail-go)
- **Date:** 2026-08-21
- **Related:** IRONWAILL-SPEC-001/002/003, ADRs 0001-0010, research 0001-0007,
  gogpu discussion #468 (https://github.com/orgs/gogpu/discussions/468)

---

## 1. Context

ironwail-go (a pure-Go, `CGO_ENABLED=0` port of the Ironwail Quake engine on
gogpu/WebGPU) ran **three successive experimental branches** rewriting its
hand-rolled UI (menu, dropdown console, HUD) on top of gogpu/ui. Each branch
took a different integration architecture, each had its own bugs and
tradeoffs, and each was superseded by the next. This document is the
deep-dive evaluation of all three approaches, framed as the validation-case
feedback the gogpu org explicitly invited in discussion #468.

The three branches:

| Branch | Integration architecture |
| --- | --- |
| `experiment/ui-rewrite` (v1) | Engine-owned loop; gogpu/ui rendered into a bridged gg canvas inside the engine's `RenderFrame` |
| `experiment/ui-rewrite-v2` (v2) | `desktop.Run` owns the loop; the 3D world rendered into a `core/gpuview` texture the compositor blits under the UI |
| `experiment/ui-rewrite-v3` (v3, current) | Engine-owned loop; widgets drawn into a CPU `gg.Context`, then blitted to the swapchain via a custom WGSL overlay-composite pass with `LoadOpLoad` |

---

## 2. The three approaches at a glance

| | **v1** | **v2** | **v3** (current) |
| --- | --- | --- | --- |
| **Core idea** | Engine-owned loop; gogpu/ui into a bridged gg canvas inside `RenderFrame` | `desktop.Run` owns the loop; world rendered into a `core/gpuview` texture | Engine-owned loop; widgets into a CPU `gg.Context`, blitted via a custom HAL composite pass |
| **Text** | TTF (later QuakeTextWidget atlas) | Conchars bitmap atlas (real retro glyphs) | Conchars bitmap atlas (same as v2) |
| **Menu art** | Text-only rows (no LMP pics) | Real `gfx/*.lmp` pics | Real `gfx/*.lmp` pics |
| **Isolation** | Leaked into `internal/game` (UIHost, gateway, CSQC fallback) | Clean `Host` adapter | Clean `Host` adapter |
| **Input** | Full `gpucontext.EventSource` gateway shim | `app.HandleEvent` KeyForwarder | `app.HandleEvent` KeyForwarder |
| **WASM** | Works | Gated off (`desktop.Run` blocks) | Works |
| **Headless** | Works | Needs specialized mocks | Works |

---

## 3. What each branch got right and wrong

### v1 — the "in-loop bridge" (Architecture A+C)

**Strengths that carried forward:** the engine-owned loop (WASM/headless
compatible), the `ui_backend` cvar gate for A/B comparison, the legacy
`menu.Manager` accessor surface, and the layered `Stack` surface model. The
input gateway (a full `gpucontext.EventSource` shim with pointer/scroll/IME)
was the most complete input solution of the three.

**Failures that killed it:**

1. **TTF text produced visuals that "look nothing like" Quake.** The pixel-TTF
   spike found `GlobalFontRegistry` lives in an *internal* package and is
   unimportable; the public `plugin.AssetLoader.LoadFont` registerer cannot
   reach it either. **gogpu/ui has no public bitmap-font path**, so a retro
   game either hand-rolls per-glyph draws or accepts non-native text.
2. **The gg-canvas bridge fought the framework.** The bridge had to manually
   manage `ggcanvas.Canvas` lifecycle (`BeginAcceleratorFrame`,
   `BeginGPUFrame`, `ResetFrameDamage`, `MarkDirty`, `Render`) and resize —
   exactly the plumbing that breaks on the first framework change. The v2
   ADR records it "failed: canvas sizing fought the framework, the composite
   could clear the world, and input invalidation was fragile."
3. **Wiring leaked into the game package** — host construction, root
   syncing, input raw sinks, and CSQC fallback all lived in game code.

### v2 — the "desktop.Run + GPUView" (Architecture B)

**Strengths:** real visual fidelity (conchars + LMP pics), clean isolation via
a `Host` adapter (world texture exposed as a `gpucontext.TextureView`), and
the C-lineage 320x200 menu transform math. This was the #468-endorsed BYO-kit
composition path exactly as the maintainer proposed it.

**Failures that killed it:**

1. **`desktop.Run` inverted application control.** The engine lost ownership
   of the frame loop. The world had to render into an offscreen `gpuview`
   texture, which required retargeting the entire scene-target pipeline
   (world/entities/waterwarp/polyblend).
2. **Texture lifecycle churn:** continuous texture recreation/destruction
   during window resize and widget invalidation.
3. **Vulkan command buffers referenced retired/destroyed textures → swapchain
   presentation failures.** The deepest technical failure of the three — a
   GPU-lifetime bug, not a design preference.
4. **WASM was impossible** (`desktop.Run` blocks; the `requestAnimationFrame`
   path is incompatible), so v2 was native-desktop-only.
5. **Headless needed specialized mocks.**

### v3 — the "engine-owned overlay + HAL composite" (current)

**The synthesis:** it keeps v1's engine-owned loop (WASM/headless work again)
and v2's visual fidelity and clean adapter. The novel part: widgets draw into
a reusable CPU `gg.Context` → `render.NewCanvas` → `dc.Image()`, and the
renderer's `DrawRGBA` uploads that as one texture and composites it with a
hand-written WGSL overlay-composite pipeline using `LoadOpLoad` on the current
swapchain view.

**Difficulties it hit (the most fix-commits of any branch):**

1. **The "single-pass GPU flush" in the spec is aspirational, not what's
   implemented.** The spec/ADR claim `FlushGPUWithViewPreserveContent` is the
   mechanism, but the code never calls it — it does a CPU `gg.Context` raster
   + texture upload + custom WGSL blit instead. The doc/code drift is itself
   a finding.
2. **Making the world survive the composite required deep renderer surgery:**
   a helper uses **reflection + `unsafe` to poke gogpu's internal
   `frameCleared`/`hasPendingClear` fields** to force `LoadOpLoad`. That is a
   maintenance landmine — it breaks on any gogpu internal rename.
3. **UV orientation bugs:** WebGPU texture Y-origin vs clip-space Y required
   shader UV flipping.
4. **Alpha-mode bugs:** the CPU `gg.Context` produces straight alpha but the
   composite pipeline's blend state had to be tuned (premultiplied vs
   straight) to match.
5. **Input double-delivery:** the engine's input backend had to be rewritten
   to split one "any callback disables polling" gate into three separate
   gates (key / mouse-button / mouse-move) because the old heuristic
   double-delivered keys under the new path. A regression in the engine's
   core input backend caused by the UI experiment.
6. **Key debounce bounce on menu toggle** — the key-forwarder now maps ASCII
   to `event.Key`, which changed menu-toggle behavior.
7. **The isolation boundary is not actually enforced in v3:** the UI package
   imports `internal/renderer` (its `DrawOverlay` takes a renderer type), and
   the import-closure test only forbids `internal/game`, silently dropping the
   `internal/renderer` check that v2's test enforced.

---

## 4. The pattern across all three branches

Every branch converged on the **same core widget layer** (the menu/console/HUD
widgets are near-identical between v2 and v3 — v3 is v2's widget tree with
`desktop.Run`/`WorldTexture` deleted and an `OverlayRenderer` added). The
churn was **entirely in the integration seam**, and each iteration was a
reaction to the previous seam's failure:

```
v1: gg-canvas bridge in engine loop   →  "fought the framework, could clear world"
v2: desktop.Run + gpuview texture      →  "texture churn + Vulkan presentation crashes + no WASM"
v3: CPU gg.Context + custom WGSL blit →  "reflection/unsafe on gogpu internals + alpha/UV/input fixes"
```

**The root cause:** gogpu/ui's only first-class embedding path is
`desktop.Run`, which is a whole-application ownership model. There is **no
supported "render my widget tree into a texture I already own, on my own
loop" path**. Every branch had to hand-build that seam, and each hand-build
touched gogpu internals (reflection on internal frame state, the
`externalTextureWidget`/`DrawGPUTexture` compositor path, the single-slot
`OnDraw`/`EventSource`).

---

## 5. What this means for discussion #468

### 5.1 Feedback the experiment validates (evidence-backed)

1. **The BYO-kit path works.** All three branches used `app` + `widget` +
   custom `widget.WidgetBase` widgets with **zero `core/*` imports**; Go
   dead-code elimination kept the binary lean. The org's "full custom kit"
   scenario is real and viable.

2. **The Painter pattern is not what a game actually uses.** All three
   branches implemented **custom widgets on `widget.WidgetBase`**, not
   painters over stock widgets. Quake's UI is bespoke enough that there is no
   stock widget to paint — the "custom look + behavior" row of the VEE table
   is the one that matters, and it needs a **good custom-widget authoring path
   (sdk/)**, not more painters.

3. **The Canvas gap table was validated precisely.** The `image.RGBA.SubImage`
   workaround for sprite atlases (status bar faces/weapons, 9-patch boxes)
   works exactly as predicted but forces callers to manage atlas lifecycle.
   **`ImageRegionDrawer` (or `DrawImageSrcRect`) is confirmed the single most
   valuable Canvas addition for game UIs.** The conchars atlas is 128x128
   with 256 sub-rects; the status bar alone does dozens of sub-rect draws per
   frame.

4. **The `AdvancedCanvas.GGContext()` escape hatch is real and used.** The
   whole overlay draw relies on the concrete canvas's `Context()` — but it's
   the *only* escape hatch, and it bypasses Canvas state. A first-class
   `AdvancedCanvas` would be cleaner.

5. **The bitmap-font gap is the biggest unaddressed one.** `GlobalFontRegistry`
   is unimportable (internal package) and the public `plugin.AssetLoader`
   registerer can't reach it. **A public font-registration path (or a
   bitmap-strike API) is a concrete, blocking ask** — it's why v1 failed on
   text and why v2/v3 had to hand-roll per-glyph atlas draws instead of using
   the text system.

6. **`desktop.Run` ownership is the sharpest edge.** The single-slot
   `OnDraw`/`EventSource` conflict forced every branch into a bespoke seam.
   **The org should document/standardize the standalone-engine path** (render
   the widget tree into a caller-owned texture on the caller's loop) — this is
   what games and embedded apps hit first, and it's currently undocumented.

7. **GPUView needs a non-desktop blit path.** The `gpuview` widget is only
   composited by `desktop.Run`'s layer tree; a standalone engine must blit
   the texture itself. v2 hit this directly.

### 5.2 The honest negative findings the org should hear

- **The retained-mode invalidation model fights continuous 3D rendering.**
  Every branch ended up scheduling an animation frame + forcing a redraw
  **every frame** for the menu (animated cursor) and HUD (per-frame state) —
  i.e. the damage/repaint optimization is bypassed entirely for a game. The
  per-boundary `SceneCache`/PictureLayer machinery (a #468 selling point) is
  dead weight for this workload.
- **The theme system is overkill for palette-indexed art.** The
  `ThemeExtension` `Merge/Lerp/CopyWith` contract was implemented but is
  essentially unused by the widgets — Quake colors are palette indices read
  directly, not semantic theme tokens. The semantic `ColorScheme` assumption
  in the Painter pattern is a poor fit for palette-indexed games.
- **The custom-WGSL-composite approach in v3 is a warning sign.** That a game
  had to write its own shader + pipeline + bind-group to blit a UI texture
  onto the swapchain (because the framework's own 2D path couldn't preserve
  the world reliably) suggests the **2D-over-3D composite path needs
  first-class support**, not a per-app workaround.

### 5.3 Concrete recommendations for the org

1. **P0: Public font registration / bitmap-strike API.** Blocking for any
   retro or custom-font game. Without it, text is the first thing that
   breaks.
2. **P0: `ImageRegionDrawer` / `DrawImageSrcRect`.** Highest-value Canvas
   addition; removes the SubImage boilerplate all three branches carried.
3. **P1: Document the standalone-engine embedding path** (widget tree →
   caller-owned texture, caller loop) and the GPUView non-desktop blit. This
   is the #468 "kernel boundary" question from a game's perspective — the
   answer is "the seam needs to be a supported contract, not app code."
4. **P1: `AdvancedCanvas.GGContext()`** as a documented Tier-2 escape hatch
   (already proposed in #468) — confirmed needed.
5. **P2: A raw-palette color escape** for palette-indexed game art, since the
   semantic `ColorScheme` assumption doesn't map.
6. **P2: Consider a `Continuous`/`AlwaysDirty` widget mode** so games don't
   have to fight the retained-mode invalidation to get per-frame redraws.

---

## 6. Where the branches stand now

- **v1** is archived (tagged) and superseded.
- **v2** is superseded by v3 — its `desktop.Run`/`gpuview` architecture is
  abandoned.
- **v3** is the current working branch. It is the most complete (all three
  surfaces render with real art, tests green, WASM works), but it carries
  real debt: the reflection/unsafe poke at gogpu internals, the renderer
  import in the UI package (isolation boundary silently weakened), the
  input-backend regression risk, and the doc/code drift on the "single-pass
  GPU flush."

**The single most useful thing the three-branch saga proves for #468:** the
widget toolkit itself is fine (BYO-kit, custom widgets, atlas draws all work),
but the **embedding seam** — how a non-`desktop.Run` host renders a widget
tree over its own 3D content — is the unsupported, undocumented,
repeatedly-reinvented part. That's where the org should invest before v1.0: a
supported standalone-render contract, a public font path, and the source-rect
Canvas API.

---

## Source Index

- Branch `experiment/ui-rewrite` (v1): `internal/quakeui/*`, specs/plans/ADRs
  001/0001-0005, research 0001-0004, spike notes.
- Branch `experiment/ui-rewrite-v2` (v2): `internal/quakui/*`, specs/plans/ADRs
  002/0006-0009, research 0006-0007.
- Branch `experiment/ui-rewrite-v3` (v3): `internal/quakeui/*`, SPEC-003,
  ADR-0010, plan 003.
- gogpu discussion #468 (fetched 2026-08-21): body + kolkov's maintainer
  reply; note the game-UI thread and darkliquid's reply referenced in earlier
  research live in the moved-from gogpu/ui discussion #229, not on #468.
- gogpu/ui v0.1.54, gogpu v0.53.0, gg v0.52.3 module sources.
