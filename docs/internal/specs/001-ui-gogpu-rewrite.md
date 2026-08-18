# IRONWAIL-SPEC-001: gogpu/ui UI Rewrite — Experimental Branch `experiment/ui-rewrite`

**Component Identifier:** IRONWAIL-SPEC-001
**Status:** DRAFT (Stage 2) — pending Review 1
**Date:** 2026-08-18
**Branch:** `experiment/ui-rewrite`
**Language/Runtime:** Go 1.26, `CGO_ENABLED=0`, gogpu/ui v0.1.54 / gogpu v0.53.0 / gg v0.52.3
**Primary dependencies:** `github.com/gogpu/ui`, `github.com/gogpu/gg`, `github.com/gogpu/gogpu`
**Target location:** `internal/` new `ui`-adjacent packages + existing `internal/menu|console|hud|game|renderer`
**Related research:** docs/internal/research/0001-current-ui-implementation.md,
0002-gogpu-ui-package.md, 0003-gogpu-discussion-468-vee.md,
0004-gogpu-ui-integration-points.md

---

## 1. Metadata & Overview

### 1.1 Purpose

Rewrite ironwail-go's four hand-rolled UI subsystems (main menu + submenus,
dropdown console, player HUD, and the CSQC-HUD fallback bridge) on top of the
gogpu/ui widget toolkit, as an **experiment** on the existing branch
`experiment/ui-rewrite`. This is the reference BYO-kit validation case the
gogpu org explicitly invited in discussion #468 (see research 0003).

### 1.2 Scope (from Stage 1 elicit — user decisions)

| Subsystem | MVP status |
| --- | --- |
| Main menu + all submenus (single/multi/options/controls/video/audio/save/load/join/host/mods/help/quit/setup) | **IN** — full rebuild |
| Dropdown console (open/close slide, scrollback, input line, notify lines) | **IN** — full rebuild |
| Player HUD (classic/modern/QuakeWorld styles, status bar, crosshair, centerprint, scoreboard) | **IN** — full rebuild |
| CSQC-driven HUD bridge | **OUT (defer)** — when CSQC active, legacy HUD path renders (mod compat); `ui_backend=1` ignores CSQC |
| Demo playback controls (char progress bar) | **OUT (defer)** — stays legacy in MVP; existing bd `ironwail-go-cuy` continues to own interactive scrubbing |

**Out of scope (explicit non-goals for the experiment):** demo timeline
interactivity, CSQC rewiring, a second "modern" design system beyond the
Quake-styled one, networking/multiplayer UI changes, HUD re-theming away from
cvar-driven appearance, performance work beyond "not obviously worse",
packaging/distribution (no L1 module publishing — code stays in-repo).

### 1.3 Branch & gating strategy

- All work happens on `experiment/ui-rewrite` (already checked out; clean).
- A new cvar **`ui_backend`** (0|1, default 0) selects the render path:
  - `0` = legacy path (current code, untouched behavior; parity oracle).
  - `1` = gogpu/ui path (new widget tree per active surface).
- Each subsystem ships with its own `ui_backend` scope: the cvar switches the
  whole UI layer (menu+console+HUD together) so A/B is a single flip. (Per-
  subsystem scoping was considered and rejected for complexity; see ADR-0002.)
- The dependency bump (gogpu v0.52.1 → v0.53.0, + gg v0.52.3) is accepted on
  this branch only (gap log #11, user-accepted). `go.mod`/`go.sum` changes
  stay on the branch and are rolled back if the experiment is abandoned.

### 1.4 Definition of Done (MVP) — user decision: behavioral parity + ui_backend gate

1. `ui_backend 1` produces the same **cvar/command surface and menu structure**
   as `ui_backend 0`: same screens, same items, same actions, same key and
   mouse navigation semantics, same save/load/mods provider behavior.
2. The **entire existing test suite passes on both paths** where the legacy
   path is unchanged: `mise run test` green; no regressions in
   `internal/menu`, `internal/console`, `internal/hud`, `internal/game`
   (input latching/mouse-look tests in `game_movement_input_test.go:145-329`
   must keep passing on path 0).
3. `ui_backend 1` boots headless and in-window, opens the main menu, opens
   submenus, operates the console (scrollback, input, tab completion),
   renders the HUD in classic/modern/QW styles while in-game, with no panics
   and no obvious GPU leaks (`.Release()` discipline per webgpu-gogpu skill).
4. Toggling `ui_backend` mid-session switches the active UI layer cleanly
   without state corruption (menu cursor, console scroll, HUD values),
   matching cvar-gate A/B expectations.
5. In-window screenshots/captures at `ui_backend 1` visually match legacy
   layout (same 320x200 menu geometry, same status-bar arrangement) —
   **behavioral, not pixel-exact** (TTF text decision; see ADR-0004).
   The `-screenshot` command / software renderer path stays legacy-only as a
   parity oracle (R1.6); path 1 is compared via in-window captures.
6. No `//go:build` tags introduced; `CGO_ENABLED=0` preserved
   (AGENTS.md gotcha #1/#4).

## 2. High-Level AI-DLC / Agent Prompt

Hand this block to an implementer agent:

```
Rewrite ironwail-go's menu, console, and HUD UI on gogpu/ui in the
experiment/ui-rewrite branch, behind a ui_backend 0|1 cvar (default 0 =
legacy untouched, 1 = new gogpu/ui tree). Follow IRONWAIL-SPEC-001,
ADRs 0001-0005, and the implementation plan NN-ui-gogpu-rewrite
(docs/internal/plans/). Constraints:

1. Pure Go, CGO_ENABLED=0, no //go:build tags. Go 1.26.
2. Engine stays the owner of the gogpu.App, the render loop, and the
   input pipeline (RESEARCH-0004 Architecture A+C). gogpu/ui app runs
   inside the engine frame: uiApp.Frame(); Window().DrawTo(render.
   NewCanvas(cc, w, h)); composite onto the engine surface. A gateway
   EventSource shim feeds uiApp; the engine KeyDest router decides
   which events reach the ui tree and which stay in the game.
3. Config/MV/cvar surface is IDENTICAL on both paths: scr_menuscale,
   scr_menubgalpha, scr_conwidth/conscale/conspeed, con_notify*,
   scr_sbarscale/alpha, scr_crosshairscale, hud_style, crosshair, and
   every menu command/binding. No cvar renames, no new required cvars.
4. Text is TTF: rasterize Quake conchars into a pixel font with a
   Private-Use-Area rune mapping (ADR-0004). Colors come from the Quake
   palette converted to theme/widget tokens.
5. Custom Quake widgets (conchars text, palette fills, WAD pics, 9-patch
   boxes, menu plaques) are built on widget.WidgetBase + registry,
   no stock core/* widgets needed (BYO-kit). Escape hatch to gg is
   Context() *gg.Context for sprite/precise drawing.
6. CSQC is NOT rewired: when a mod's CSQC_DrawHud draws, the HUD
   falls back to the legacy path even with ui_backend 1.
7. TDD red/green per task in the plan; run
   TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/menu/... (etc)
   and full mise run test before closing a task.
8. Do not commit unless told. Keep this experiment branch isolated.
   Widget root viewport sizing: derive DIP dims from internal/game/ui dims
   math (GUI/GL + pixel aspect); apply scr_* scales as widget transforms
   (spec §3.3 R1.5). WAD pics draw via canvas.DrawImage, never
   primitives.Image (placeholder — R1.3).
```

## 3. Information Architecture / Topology

### 3.1 New package layout (proposed)

```
internal/quakeui/            NEW — gogpu/ui integration host + gate
  widget_host.go             uiApp lifecycle (app.New, SetRoot, Frame, DrawTo),
                             RenderModeHostManaged, gateway EventSource shim
  backend.go                 ui_backend cvar read + path selection hooks
  gateway_events.go          KeyDest-aware input forwarding into the ui tree
  canvas_bridge.go           render.NewCanvas wiring + composite onto engine surface
internal/quakeui/theme/      NEW — Quake theme
  theme.go                   theme.Theme from DefaultDark() + Quake palette tokens
  extension.go               quakeui ThemeExtension (conchars colors, bg, prompt, etc.)
  font.go                    conchars → pixel TTF registration (GlobalFontRegistry)
  pics.go                    WAD QPic → image.Image bridge for DrawImage/atlas
internal/quakeui/widgets/    NEW — custom Quake widgets (BYO-kit)
  text.go                    conchars-equivalent TTF text widget + prompt/cursor
  pic.go                     QPic + 9-patch box + plaque widgets; WAD pics via
                             canvas.DrawImage directly (NOT primitives.Image,
                             which is a placeholder — research 0002 §3, R1.3)
  sliders.go                 option rows (volume/brightness/sliders via drag gesture)
  list.go                    virtualized list (save slots, server browser, mods)
  dialog.go                  quit/confirm prompt (OverlayManager or custom)
  bar.go                     (deferred) demo progress bar — NOT in MVP
internal/quakeui/console/    NEW — console widget
  console_widget.go          scrollback + input line + notify lines as widgets
  completion_bridge.go       hooks existing console.TabCompleter
internal/quakeui/hud/        NEW — HUD widgets
  statusbar.go               classic/modern/QW status bars (nums, faces, items)
  crosshair.go               crosshair widget (conchars glyph via TTF char)
  centerprint.go             centerprint widget (typewriter, bg modes 1/2/3)
  scoreboard.go              scoreboard widget
internal/quakeui/menu/       NEW — menu widgets
  menu.go                    menu root widget + per-state pages
  main.go, options.go, game.go, setup.go, joinhost.go, mods.go  (port of
                             internal/menu screen logic to widget pages)
  keys.go                    KeyDest/menu key mapping reuse (normalizeMenuKey)
internal/menu/               LEGACY — kept untouched (path 0), consulted as
                             the behavior spec (commands/actions/cursors)
internal/console/            LEGACY — untouched; console data (ring buffer,
                             input line, completion) is RETAINED and consumed
                             by quakeui/console widget
internal/hud/                LEGACY — untouched; hud.State + scoreboard +
                             StatusBar pics are RETAINED/REUSED by quakeui/hud
internal/game/               WIRING ONLY — game_init registers ui_backend &
                             constructs quakeui host; game_runtime_frame/overlay/
                             ui switch between path 0 and path 1
```

Design principle: **legacy logic, new presentation.** The engine's
menu *state machine* (`internal/menu.Manager` command/action side), console
*data* (ring buffer/input/completion), and HUD *state aggregation* remain the
source of truth; gogpu/ui widgets present them. This minimizes behavioral
drift and reuses the parity-tested logic. (Menu *navigation/draw* gets
re-implemented as widget state — the draw-specific parts of `internal/menu`
are the part being replaced.)

### 3.2 Widget tree per active surface (path 1)

```
Root (window-sized; ThemeScope(quakeTheme))
├── [menu surface]  MenuRoot (320x200 layout viewport, scr_menuscale)
│     ├── backdrop (DrawFillAlpha scr_menubgalpha over frozen scene)
│     └── page widget per state: MainPage | SinglePlayerPage | LoadPage |
│         SavePage | MultiPage | JoinPage | HostPage | OptionsPage |
│         ControlsPage | VideoPage | AudioPage | HelpPage | QuitPage |
│         SetupPage | ModsPage
│           └── cursor, plaque/title pics, rows (text/slider/textfield)
│                 └── text input rows → console re-use or textfield-style
├── [console surface]  ConsoleWidget (slide-in, scr_conwidth/conscale,
│     scr_conspeed-driven offset; ThemeScope for conback texture)
│     ├── background (conback pic scaled) / black fill
│     ├── scrollback lines (virtualized), ^ indicator, title
│     ├── notify lines (fade alpha, con_notify*)
│     └── input line (prompt ']' + TTF text + blink cursor) +
│         tab-completion hint
└── [hud surface]  HUDWidget (CanvasDefault layer + per-style transforms)
      ├── StatusBarWidget (classic 320x48 / QW / modern 400x225 via
      │     hud_style, scr_sbarscale/alpha)
      ├── CrosshairWidget (CanvasCrosshair center, crosshair cvar)
      └── CenterprintWidget (scr_centerprintbg modes, typewriter)
```

The three surfaces are separate roots (only one active typically), all under
one `uiApp`; gating by `KeyDest`/menu-active/HUD-visible state.

### 3.3 Layering of the new UI in the engine frame (path 1)

Replaces the legacy overlay phase call with a **UI-host hook**:

```
RenderFrame phase 5 (Draw2DOverlay):
  if ui_backend==0: legacy drawRuntimeOverlayFrame (current code)
  if ui_backend==1: game.drawRuntimeOverlayFrameGogpuUI(overlay)
      ├─ HUD layer  (HUDWidget.Draw → into gg canvas)
      ├─ console layer (slide fraction from ConsoleSlideFraction)
      ├─ demo bar: NOT drawn (deferred; legacy drawRuntimeDemoControls skipped on path 1)
      └─ menu layer (Backdrop + MenuRoot)
      then uiApp.Window().DrawTo(widgetCanvas); composite canvas onto surface
```

The legacy `RenderFrame` phases 1-4 (clear/world/entities/polyblend) and the
`MenuActive` preserve-colored-scene behavior are unchanged on path 1.

## 4. Data Models

### 4.1 Retained legacy sources of truth (no schema changes)

| Source | Consumed by | Kept as-is |
| --- | --- | --- |
| `menu.Manager` state fields (state, cursors, text buffers, host settings, mods) | quakeui/menu pages | Yes (state + actions); draw/nav methods re-implemented in widgets |
| `menu.Manager` command/action side (`*Key` handlers' side effects) | quakeui/menu keys | Yes — reused: keep the action methods, replace the draw |
| `console.Console` (ring buffer, input line, history, backScroll, completion) | quakeui/console widget | Yes — widget reads via existing accessors (`Line`, `Scroll`, `CommitInput`, `CompleteInput`, `SetInputLine`) |
| `hud.State`, `ScoreEntry`, `HUDStyle`, StatusBar pic set, Centerprint text | quakeui/hud widgets | Yes — `updateHUDFromServer` keeps producing `hud.State` |
| `renderer.CanvasTransformParams` + `internal/game/ui` dims math | quakeui layout viewport dims | Yes (translated to widget geometry) |

No new DB/schema; no new binary formats. New Go types only (widgets).

### 4.2 New cvar

| cvar | type | default | meaning |
| --- | --- | --- | --- |
| `ui_backend` | int | 0 | 0 = legacy UI path, 1 = gogpu/ui path. Read per-frame; toggling switches the whole UI layer. Registered in `game_init.go`. |

### 4.3 Quake theme tokens (from palette.lmp)

`quakeui/theme` converts the 768-byte Quake palette + conchars conventions
into `theme.Theme` tokens: `Background` (index 0), `OnSurface` (bright text
row / high-bit glyphs), `Surface` (palette 4 sbar), `Primary` etc. mapped for
widget defaults, plus a `quakeui` ThemeExtension carrying
`{PromptGlyph, ScrollHintGlyph, BrightRow bool, MenuBgAlpha, SbarAlpha,
Palette []byte, PuaBase}`. Documentation of exact mappings lives in
`quakeui/theme/theme.go` and is derived from research 0001 §3-5.

## 5. State Machines / Flows

### 5.1 ui_backend gate flow (per frame)

```
frame: RenderFrame(state, draw2DOverlay)
  ├─ path = ui_backend==1 ? gogpu-ui : legacy
  ├─ path==legacy → existing drawRuntimeOverlayFrame (untouched)
  └─ path==gogpu-ui → uiHost.Frame():   // quakeui/backend.go
       ├─ keyDest→ activeSurface(config, console, menu, hud, none)
       ├─ ensure uiApp root matches activeSurface (SetRoot per surface or
       │     stacked overlays; see ADR-0002)
       ├─ uiApp.Frame()
       ├─ gateway events already delivered this frame (input pump)
       ├─ Window().DrawTo(widgetCanvas) → engine surface composite
       └─ (demo bar skipped; CSQC HUD→legacy override)
```

### 5.2 Menu state machine (path 1)

Reuses `menu.Manager` transitions verbatim:
- open/toggle: `ToggleMenu()`/`ShowMenu()`/`ShowState()` → sets
  `KeyDest(KeyGame|KeyMenu)` (engine side unchanged).
- page select: `mainSelect()`, per-page `*Key`/`M_Char` action methods reused;
  the widget layer mirrors `state` to the matching page widget.
- cursor: `moveCursorDown/Up`, absolute mouse hit-test
  (`menuCursorForPoint`) reused to set widget focus/selection.
- Escape/backspace stack: unchanged (flat state machine).

The **draw** side (`M_Draw` + `draw*` functions) is replaced by the page
widgets; the **action** side is shared.

### 5.3 Console flow (path 1)

- `toggleconsole` command / backtick → engine KeyDest flip (unchanged).
- On KeyConsole: input events → (via gateway) consoleWidget input field;
  `CommitInput` → `g.Host.Cmd.ExecuteText(line)` (unchanged).
- Tab → `CompleteInput`; the completion **match-list print** is rendered by
  the widget (conchars→TTF text) instead of raw console lines.
- Slide animation: `ConsoleSlideFraction`/`StepConsoleSlide`
  (internal/game/ui) drives the console widget's vertical offset (retained
  engine-side math, applied as a widget transform).

### 5.4 HUD flow (path 1)

- `updateHUDFromServer` (game_visual.go) → `hud.State` (unchanged) →
  HUDWidget renders styles per `hud_style`, `scr_sbarscale/alpha`,
  `scr_crosshairscale`, `viewsize`.
- Intermission/finale/centerprint: `hud.State.Intermission` +
  `CenterPrint` fields → CenterprintWidget (typewriter via
  `scr_printspeed`, bg modes via `scr_centerprintbg`).
- Scoreboard: `ShowScores` + `ScoreEntry` slice → ScoreboardWidget
  (columns, colors via `colorForMap`).
- CSQC override: if `CSQC.DrawHud` drew this frame (legacy hook), skip
  HUDWidget entirely and let the legacy CSQC canvas draw (deferred scope).

### 5.5 Branch lifecycle

```
experiment/ui-rewrite (exists, clean)
  forbid pushing to main; all PRs/commits stay local to the branch
  (user's conservative git policy; no commits/pushes unless asked)
```

## 6. Security Model

No new attack surface: UI renderer changes are local input → draw. Notes:
- Input forwarding: the gateway EventSource shim must not re-enter the
  engine's input backend (single-slot conflict, research 0004 §1) — one
  authoritative registration; double delivery corrupts menu cursor/mouse look
  (existing regression tests police this on path 0).
- Text rendering: TTF rasterization from local `palette.lmp`/`conchars` is
  offline asset data, no untrusted bytes executed. Font registry input is
  engine-controlled.
- No network, no auth, no secrets. A11y roles added by gogpu/ui widgets are
  inert in the engine (no a11y backend on desktop target).
- CSP/eBPF/sandbox N/A (native desktop + wasm target unchanged).

## 7. Edge Cases & Failure Handling

| Edge case | Behavior |
| --- | --- |
| gogpu/ui init failure (uiApp.New error) | log, keep `ui_backend` pinned 0, legacy path unchanged (fail-open, parity preserved) |
| `ui_backend 1` + WASM | engine-owned path is wasm-compatible (Architecture A); if gg canvas unavailable on wasm, gate path 1 off with log (env `GOOS=js` check) |
| `ui_backend 1` + software renderer (screenshot/headless) | legacy path always (software backend has no gogpu/ui canvas); screenshots stay comparable |
| CSQC mod active | HUD falls back to legacy path (see 1.2) |
| demo playback active | demo bar renders on legacy path only; on path 1 it is skipped (deferred) — no crash, documented in PARITY.md |
| text entry focus (menu setup/join/host) | widget input field owns keyboard while KeyDest==KeyMenu && page has entry; Esc cancels (reuse M_Char) |
| window resize / DIP scale change | `uiApp` resize bridge (`OnResize` on the gateway) re-layouts all surfaces; canvas params recomputed from `internal/game/ui` dims |
| frame timing / VSync | uiApp.Frame() driven by engine frame (no separate loop); `RenderModeHostManaged` (no double clear) |
| menu over frozen gameplay | preserved scene behind menu: engine's `!state.MenuActive` clear logic unchanged; ui path draws backdrop over it |
| pixel-aspect (`scr_pixelaspect`) | viewport dims from GUIDimensions incl. aspect; widget layout uses those dims |
| fullscreen vs windowed | vid_width/height source unchanged; widget tree re-laid-out on resize |
| GPU resource lifetime | every widget-owned texture/view released on Unmount/surface switch (webgpu-gogpu skill: `.Release()` discipline, no per-frame leaks) |
| backtick during menu/binding capture | normalizeMenuKey/pending-binding logic reused; async char delivery preserved |
| ui_backend toggle mid-session | surfaces re-created (roots keyed by surface, not monolithic): cursors/scroll re-read from legacy sources (state persists) |

## 8. Acceptance Criteria (each mapped to implementer-verifiable checks)

| # | Criterion (from 1.4) | Verifiable by |
| --- | --- | --- |
| AC1 | cvar/command surface identical on both paths | `grep` diff of registered cvars/commands; functional menu walkthrough on both paths (manual checklist) |
| AC2 | full `mise run test` green; menu/console/hud/game tests pass on path 0 | `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./...` |
| AC3 | path 1 boots headless + windowed, opens menu/submenus/console/HUD without panic, no GPU leak growth | `mise run smoke-*` + `-ui_backend 1`; renderer resource counters / `host_speeds`; manual GPU metrics |
| AC4 | ui_backend toggle mid-session is clean (no state corruption) | integration test toggling cvar between frames + menu cursor/console scroll assertions |
| AC5 | in-window captures at `ui_backend 1` visually match legacy layout (not pixel-exact) | in-window capture at `ui_backend 1` vs 0; `-screenshot` command stays legacy oracle (R1.6) |
| AC6 | no `//go:build` tags; `CGO_ENABLED=0`; pure Go | `grep -r "//go:build" internal/quakeui` empty; `go build` with CGO off |
| AC7 | CSQC mod compat preserved | run a mod with `csprogs.dat` at `ui_backend 1`: HUD uses legacy CSQC path |
| AC8 | no stock `core/*` widgets imported by quakeui widgets (BYO-kit validated) | import graph check: `internal/quakeui` imports `app`,`widget`, `render`, `primitives`(selective), `core/gpuview`? (not in MVP) — no `core/button` etc. |
| AC9 | dependency bump contained to branch | `git diff main...experiment/ui-rewrite -- go.mod go.sum` shows only the bump + new gg |

## 9. Cross-references

- In-scope research: 0001, 0002, 0003, 0004 (all docs/internal/research).
- ADRs (Stage 4, to be written): 0001 branch/gate strategy, 0002 architecture
  A+C + per-surface roots, 0003 input gateway model, 0004 TTF text, 0005
  dependency bump.
- Related bd: `ironwail-go-cuy` (demo scrubbing — deferred, out of MVP),
  `ironwail-go-1hg` (CSQC authoring in QuakeGo — feeds deferred CSQC scope).
- Docs to update: `docs/PARITY.md` (UI deviations), `docs/internal/{menu,
  console,hud}.md` (new package docs), `docs/LEARNING_GUIDE.md` (package map).

## Review Log

### Stage 3 — Review 1 (spec defense), 2026-08-18

**Verdict: APPROVED WITH FINDINGS** (7 raised, 2 fixed in-spec, 5 spun to
research tasks for the plan; none blocks Stage 4).

| # | Finding | Seve. | Resolution |
| --- | --- | --- | --- |
| R1.1 | "legal logic, new presentation" split is real: menu action side is `*Key`/`*Char`/`*Select` methods (verified `mainKey`, `mainSelect`, `optionsKey`, `controlsKey`, `setupChar`, `setControlBinding`, ...); presentation is `draw*` methods (verified `drawMain`, `drawOptions`, `drawVideo`, `drawControls`, `drawAudio`, ... `manager.go:624-731`). Design stands. | info | no change |
| R1.2 | **Unexported Manager state.** `state`, `mainCursor`, `optionsCursor` etc. are lowercase (`manager.go:270-334`) — `internal/quakeui/menu` cannot read them without either exporting accessors on `menu.Manager` or the plan adding an accessor package. This contradicts "reuse the state machine" (§3.1/§4.1). | **high** | new gap G.13 → research/task: define a small exported accessor surface on `menu.Manager` (cursors, state, text buffers) that the widget pages consume; do NOT export the fields themselves |
| R1.3 | **primitives.Image is a placeholder** (verified `primitives/image.go:165-200` draws a gray box + cross; comment says full rendering "requires a Canvas implementation that supports DrawImage"). The actual `internal/render/canvas.go` DOES implement `DrawImage` (`:588`) — but the spec (3.1, widgets/pic.go) says "primitives.Image"; using it would draw gray boxes for every WAD pic. | **high** | fixed in-spec: widgets/pic.go must use `canvas.DrawImage` directly (or a custom `PicWidget` on `widget.WidgetBase`); remove reliance on `primitives.Image` in §3.1 |
| R1.4 | **Image atlas/source-rect gap** (research 0002 §3): no `DrawImageSrcRect` / `ImageRegionDrawer` in v0.1.54. Conchars → TTF (ADR-0004) partially avoids this, but WAD pics, 9-patch boxes, and player-skin pics are whole-image draws (fine). Sprite-atlas sub-rects (status bar faces/weapons) need `image.RGBA.SubImage` slicing or the gg escape hatch. | med | research task in plan (spike: SubImage path perf vs escape hatch) |
| R1.5 | **DIP scaling / `Scale()`.** gogpu/ui widgets are logical DIP at window scale; engine canvas params (GUIWidth/GLWidth, pixel aspect, scr_menuscale/conscale) are framebuffer px. Widget geometry must be derived from dims math (internal/game/ui) per §3.3 — spec implies but does not pin the mapping function. | med | spec fix: §3.3 add "widget viewport size = framebuffer dims / (scale * pixelaspect); scr_menuscale etc. applied as widget transform" |
| R1.6 | **AC5 vs screenshots path.** `-screenshot` uses the software renderer (`game_loop.go:591-604`) which has no gogpu/ui canvas; AC5 "parity screenshot harness at ui_backend 1" is therefore only valid for in-window captures, not the screenshot command. | med | fixed in-spec: AC5 must say "in-window capture at ui_backend 1" (screenshot command stays legacy); screenshot parity gate remains an oracle for path 0 only, and path 1 compares via in-window captures |
| R1.7 | **Overlapping surfaces** (menu+console+HUD can coexist; console forced-up over menu at boot). §3.2 roots "only one active typically" is wrong for boot (console forced up while menu shows) and game-over (menu over frozen world + HUD). gogpu/ui overlays (`OverlayManager`) or a stacked root handles stacking. | med | new gap G.14 → ADR/plan: surface stacking model (stacked widget layers vs single root with visibility toggles); pinned in ADR-0002 |

Spec §3.3/§9/AC5 updated accordingly. Gap log rows G.13, G.14 added.

## Change Log

| Date | What | Why |
| --- | --- | --- |
| 2026-08-18 | Initial spec draft (Stage 2) | After Stage 1 elicit (7 user decisions recorded in gap log) |
| 2026-08-18 | Stage 6 revision: folded Review 1 (R1.3 pic widget, R1.5 scaling, R1.6 AC5) + Review 2 ADR fixes (ADR-0003 HandleEvent fallback, ADR-0004 plugin.LoadFont, ADR-0002 GPUView stretch scope) | Reviews 1-2 findings; gap rows 13-18 |
| 2026-08-18 | ADRs 0001-0005 accepted (Review 2 verdicts in each ADR Review Log) | Stage 5 |
