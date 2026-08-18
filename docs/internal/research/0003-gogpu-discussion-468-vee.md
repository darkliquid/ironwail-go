# RESEARCH-0003: gogpu Discussion #468 — Verified Extension Ecosystem (VEE) RFC

- **Status:** Integrated
- **Owner:** research (fetched via gh api graphql, 2026-08-18)
- **Date:** 2026-08-18
- **Blocks:** gap log #3 (ai-dlc/ui-gogpu-rewrite)

---

## Research Question

What does the gogpu organization's discussion #468 ("RFC: Verified Extension
Ecosystem (VEE)") propose, what linked documentation does it reference, and
what does it mean for ironwail-go's plan to rewrite its UI on gogpu/ui?

## Background & Constraints

- Discussion URL: https://github.com/orgs/gogpu/discussions/468
- Author: lkmavi (Collaborator), 2026-08-17; status open. Participants:
  lkmavi, kolkov (Maintainer), AgentNemo00, darkliquid (the repo owner).
- This discussion directly spawned the request: "This is all part of an
  experiment spawned by this discussion... reading up on this discussion and
  all linked documentation is also extremely important."

## Investigation Findings

### 1. The RFC proposal (body, verbatim extraction)

**Status:** open for feedback — ecosystem architecture (not ui-repo-only).
**Builds on:** ADR-034 (Painter/behavior), ADR-036 (extensibility), SceneCache
(ui PR #210), ADR-057 (Widget Authoring SDK, proposed). **Moved from:**
gogpu/ui discussion #229 (body + catalog clarification merged here).

Proposal: `gogpu/ui` = lean kernel + contracts; community widgets = separate
Go modules (`go get`); `gogpu/widgets` = curated index + CI gates (not a
source warehouse).

**Three layers:**
- **L0 KERNEL** — `github.com/gogpu/ui`: primitives + stable core widgets,
  layout/theme/registry/plugin/cdk/uitest/a11y, `sdk/` façade. Rule: no
  community widgets accepted.
- **L1 EXTENSION MODULES** — independent Go modules (`go get` + import).
- **L2 CATALOG** — `github.com/gogpu/widgets`: `index.yaml`/`entries/*.yaml`
  + docs only (no widget sources). CI checks compat × ui versions ×
  platforms; issues "verified" badges.

**Core inclusion criterion:** a widget lands in `ui` only if ≥70% of apps need
it AND org supports it forever AND it is design-system basics. Core: Button,
TextField, Checkbox, Radio, Slider, List, Dialog, Menu, Tabs. Outside core:
Charts, Markdown, CodeEditor, Calendar, Maps, RichText, CRM/domain.

**BYO-kit (Bring Your Own Kit):** stock widgets are optional — savings from
not importing `core/*` (Go dead-code elimination), not a kill switch.
Supported composition path: `app` + `widget` + layout/theme contracts + your
L1 modules, no `core/*`, no reference ThemeBundle. Analogy: Angular CDK
without Material.

**Extension contracts (lock before v1.0):** `sdk/` façade (success metric:
themeable + tested + registered widget in <1h), registry metadata fields
(`Author`, `Module`, `License`, `Tags`, `Homepage`, `Themes`), `widget.yaml`
(out-of-band catalog metadata), ThemeBundle (`app.WithThemeBundle(nordicui.
NewTheme())`), explicit API preferred over blank import.

**Catalog trust tiers:** community (CI green on submit) → verified (nightly
re-check) → org (under `gogpu/*` or MoU). "Verified ≠ security audit."

**Answers to open questions:** Q1 extract `uicore`? Not required now. Q2
naming: `uicore`, `sdk/` inside kernel, `widgets` = catalog only, `uiplugin`
for WASM/RPC. Q3 WASM plugins? Yes for IDE/editor, no as primary enterprise
path. Q4 separate module `gogpu/uiplugin`. Q5 priority: P0 sdk/ + Tier-2
example + docs; P0/P1 index-only widgets + submit CI; P1 ThemeBundle +
registry metadata; P2 nightly re-verify + binary budget; P3 optional uicore,
WASM/RPC runtime.

**Out of scope (phases A–C):** `ui/contrib` community dump, `widgets`
monorepo, ratings marketplace, binary hosting, stock-UI kill switch.

**Phased plan:** A) sdk/ + examples/custom_widget/ + docs/sdk/; B) widgets
catalog + widget.yaml + submit-gate CI; C) ThemeBundle + extended WidgetInfo +
compat badges; D) nightly re-verify + binary budget + community process;
E) WASM/RPC contracts → uiplugin runtime.

**Governance:** core freeze (RFC + ≥70% criterion), DCO/CLA on catalog
entries, demote + advisory on vulns, `status: broken` on failed nightly,
SBOM via `go list -m all`.

**Why enterprise-grade:** `database/sql` driver model, Angular CDK, VS Code
extension marketplace without store UI, Go module proxy as sole artifact
store, Flutter package `sdk` constraints (`ui_compat` in widget.yaml), SLSA-
inspired attestations.

### 2. Maintainer reply (kolkov) — the analysis that matters for us

- Studied: ADR-034 (Painter/behavior, partially implemented), ADR-036
  (SceneCache done; module extraction + plugin interfaces open), 3 research
  reports (15 frameworks, 4 WASM runtimes), VEE.
- **Open topics:** single module vs extraction (conflicting evidence);
  where kernel lives; sdk/ scope; ThemeBundle (interface exists, 4 built-in
  implementations M3/DevTools/Fluent/Cupertino, `app.WithThemeBundle()`
  wiring needed per ADR-034 Phase 3); catalog; WASM/RPC (plugin types purely
  additive in new module `uiplugin` — zero modifications to existing
  interfaces).
- **Clear consensus:** Painter pattern core; compile-time Go packages primary
  model; ThemeBundle interface correct; SceneCache solved compile surface; `go
  plugin` ruled out; WASM/RPC in separate module; no community code in core.
- **Next steps:** build `examples/custom_widget/`; complete ThemeBundle
  implementations (ADR-034 Phase 3); update `docs/EXTENSIONS.md` (outdated —
  v0.4.x, March 2026).

### 3. The game-UI thread — the direct spawn of this task

kolkov pinged game devs (kivutar/goro, besmpl/hearth, BrentFarris/kaiju,
darkliquid/ironwail-go) about game UI:

**Engine pattern survey (key table):**

| Engine | Logic/Layout | Visual Override | Escape Hatch |
|--------|-------------|----------------|--------------|
| Unreal Slate | Widget behavior | FSlateBrush → Material | FSlateDrawElement |
| Godot | Control nodes | StyleBox + theme cascade | CanvasItem._draw() with RenderingServer |
| Dear ImGui | Immediate mode | ImGuiStyle (55 colors, 25 vars) | ImDrawList::AddCallback |
| Qt | QWidget | QStyle::drawControl | QPainter |
| Minecraft Forge | Screen | render() override | GuiGraphics raw buffer |
| Source Engine | vgui::Panel | Paint() override | surface() draw calls |

**What gogpu/ui provides today:** Painter per widget (26 widgets / 61
painters) = "custom draw routine" tier; LayoutMetrics (7 widgets) control
spatial metrics (Godot theme_constant equivalent); ThemeScope = subtree
override, scoped theming; GPUView widget = offscreen GPU texture composited in
the Layer Tree (3D viewport + UI overlay).

**The Canvas gap analysis (critical for Quake):**
`widget.Canvas` has 18 drawing methods + 9 optional interfaces but is missing
capabilities games need:

| What's missing | Why games need it | gg already supports it |
|----------------|-------------------|----------------------|
| Gradients (linear/radial) | Health bars, cooldowns, glowing borders | LinearGradientBrush, RadialGradientBrush |
| Arbitrary paths (bezier, arc) | Shield icons, radar sweeps, custom shapes | MoveTo/LineTo/CubicTo/ClosePath/Fill |
| Image regions (sprite atlas, 9-slice) | Item icons, character portraits from atlas | DrawImageEx with SrcRect |
| Rotation/scale transforms | Tilted UI, compass, minimap | Rotate(), Scale(), RotateAbout() |
| Blend modes | Glow, multiply, screen effects | 30+ blend modes |

**Planned approach:** Tier-1 granular interfaces (discoverable, mockable,
documented): `GradientFiller`, `PathDrawer`, `ImageRegionDrawer`. Tier-2 single
escape hatch:
```go
if adv, ok := canvas.(widget.AdvancedCanvas); ok {
    dc := adv.GGContext() // full gg 2D API
    dc.Push(); defer dc.Pop()
    // gradients, paths, transforms, blend modes, GPU textures, shaders...
}
```
Same pattern as Dear ImGui AddCallback, Godot _draw(), Minecraft GuiGraphics.

**BYO-kit for ironwail-go (verbatim composition path offered):**
```go
import (
    "github.com/gogpu/ui/app"
    "github.com/gogpu/ui/widget"
    "github.com/gogpu/ui/core/gpuview" // 3D viewport widget
    // your game-specific painters — no stock themes needed
)
```

**The explicit invitation:** "If you're considering swapping ironwail-go's UI
to gogpu/ui, that would be an ideal validation case: which Canvas methods are
sufficient vs which need the escape hatch? Is the Painter pattern flexible
enough for Quake's visual style? Does the GPUView widget work for the 3D
viewport overlay pattern? Real game UI feedback before v1.0 would directly
shape these decisions."

### 4. darkliquid's reply (the repo owner's stated position)

"The case of Ironwail-Go is different because it's porting over an _already
implemented in C_ UI system, so for feature parity there wasn't really a need
to explore beyond that. However, I have been considering swapping out the UI
systems for ones built on top of gogpu's offerings as an experimental branch,
to get a feeling for what it would be like. Generally though, games tend to
have very bespoke UIs that require a lot of control over what and how they
render things, and may also need to be themeable to accommodate mods, DLCs,
different game modes, etc so if using an existing toolkit, that toolkit needs
to provide options for radical theming/graphical changes, whilst handling the
generic UI logic and layouting so those don't need to be reimplemented."

### 5. Reply thread (others)

- AgentNemo00: lifecycle/abandonment questions → kolkov: module proxy caches
  versions forever; v1.x backward compatible; nightly re-verify → `broken`
  status; sumdb prevents silent modification; MVS prevents silent upgrades;
  catalog never removes entries. AgentNemo00 top suggestions: add to core
  "whenever community widgets depend on it"; first reference extensions from
  M3.
- kolkov final: proposed **DatePicker + ColorPicker** as the two reference L1
  extensions — DatePicker validates the standard widget path (overlay,
  gesture, signals, a11y ARIA grid, LayoutMetrics, i18n); **ColorPicker
  validates the custom rendering path (Canvas gradients, drag gesture,
  real-time preview, hex/RGB input — exactly the game UI pattern darkliquid
  raised)**.

## Recommended Resolution

- This discussion is the **mandate and upstream context** for the
  experiment/ui-rewrite branch. The rewrite should be framed (in the spec)
  as the reference BYO-kit validation case the org explicitly requested.
- The plan must watch three upstream deliverables: (1) `sdk/` + custom-widget
  docs (Phase A), (2) `ImageRegionDrawer`/`GradientFiller`/`PathDrawer` +
  `AdvancedCanvas.GGContext()` escape hatch, (3) `ThemeBundle` in `app`.
  Until they land (none exist in v0.1.54), the engine relies on
  `Context() *gg.Context` (which exists today) and custom widgets on
  `widget.WidgetBase`.
- The engine's radical-theming requirement (darkliquid's own words) maps to:
  painters (per-widget), ThemeScope (per-scene), ThemeExtension (component
  tokens), LayoutMetrics (geometry), and eventually ThemeBundle — all
  available or planned.
- Linked docs to reference in the spec/ADRs: ADR-034 (Painter/behavior),
  ADR-036 (extensibility), ADR-057 (proposed SDK), ui discussions #65/#229,
  ui PR #210 (SceneCache), gogpu PR #65, RFCs #377/#328, docs/EXTENSIONS.md
  (outdated in v0.1.54).

## Open Questions / Follow-ups

- Track upstream: new gogpu/ui releases (v0.1.5x+) that add sdk/,
  ImageRegionDrawer, AdvancedCanvas, ThemeBundle — pin and upgrade strategy in
  the plan.
- Whether to contribute findings upstream (Canvas gap feedback) as part of
  this experiment — optional, out of MVP scope.

## Source Index

- https://github.com/orgs/gogpu/discussions/468 (full body + all comments via
  `gh api graphql` repository(owner:gogpu, name:gogpu) discussion(number:468),
  fetched 2026-08-18)
- https://github.com/gogpu/ui (kernel repo)
- https://github.com/gogpu/widgets (catalog repo)
- https://github.com/gogpu/ui/discussions/65, /229
- https://github.com/gogpu/ui/pull/210 (SceneCache)
- https://github.com/gogpu/gogpu/pull/65 (pointer events)
- https://github.com/gogpu/gogpu/discussions/377 (naming RFC), /328 (neon RFC)
- https://m3.material.io/components/date-pickers/overview
- Repos: goro, hearth, kaiju, ironwail-go (pings)
