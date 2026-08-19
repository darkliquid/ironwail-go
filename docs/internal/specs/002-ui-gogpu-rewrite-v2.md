# IRONWAIL-SPEC-002: gogpu/ui UI Rewrite v2 — Visual Fidelity + Desktop/GPUView + Isolation

**Component Identifier:** IRONWAIL-SPEC-002
**Status:** DRAFT (Stage 2) — pending Review 1
**Date:** 2026-08-19
**Branch:** `experiment/ui-rewrite-v2`
**Supersedes:** IRONWAIL-SPEC-001 (abandoned: TTF text, engine-owned in-loop bridge, game wiring leaks)
**Language/Runtime:** Go 1.26, `CGO_ENABLED=0`, gogpu/ui v0.1.54 / gogpu v0.53.0 / gg v0.52.3
**Primary dependencies:** `github.com/gogpu/ui`, `github.com/gogpu/gg`, `github.com/gogpu/gogpu`
**Target location:** `internal/quakui/` (self-contained) + narrow `internal/game` adapter
**Related research:** docs/internal/research/0001-current-ui-implementation.md,
0002-gogpu-ui-package.md, 0004-gogpu-ui-integration-points.md,
0006-desktop-run-integration.md (spike), 0007-desktop-input-model.md (spike)

---

## 1. Metadata & Overview

### 1.1 Purpose

Rewrite ironwail-go's menu, console, and HUD UI on gogpu/ui a second time,
correcting the three failures of SPEC-001:

1. **Visual fidelity** — render the actual Quake `gfx/*.lmp` images and the
   conchars bitmap text the original UI uses, instead of TTF substitution.
2. **Desktop/GPUView path** — use `desktop.Run(gogpuApp, uiApp)` with the
   world rendered into a `core/gpuview` texture, instead of the engine-owned
   in-loop gg-canvas bridge.
3. **Isolation** — a self-contained `internal/quakui` subsystem exposed to the
   engine through a narrow adapter, with no `internal/game` wiring leaks.

### 1.2 Scope (from Stage 1 elicit — user decisions)

| Subsystem | MVP status |
| --- | --- |
| Main menu + submenus | **IN** — first milestone (menu-first) |
| Dropdown console | **IN** — after menu |
| Player HUD (status bar, crosshair, centerprint) | **IN** — after console |
| CSQC-driven HUD bridge | **OUT (defer)** — CSQC falls back to legacy HUD |
| Demo playback controls | **OUT (defer)** — stays legacy |

**Milestone order (user): menu → console → HUD.** Each is a vertical slice
landed and verified before the next.

### 1.3 Branch & gating strategy

- New branch `experiment/ui-rewrite-v2` (user decision G9). The v1 `quakeui`
  code is **archived**: tagged `v1-ui-rewrite` on the current
  `experiment/ui-rewrite` tip, and removed from the v2 branch. The following
  **carry over** to v2 (additive, path-0 safe): `ui_backend` cvar gate,
  `internal/quakeui/backend.go` reader (moved to `internal/quakui/backend.go`),
  and the `menu.Manager` accessor surface (M3.1 v1) which the legacy path
  ignores. The following are **removed**: `internal/quakeui/*` widgets, host,
  gateway, canvas bridge, and the `internal/game` wiring (syncUIHostRoot,
  gateway raw sinks, CSQC fallback, UIHost/root fields). Legacy
  `internal/menu|console|hud` and the legacy draw path are untouched.
- Keep the `ui_backend` cvar gate (0|1, default 0) (user decision G7): 0 =
  legacy path (unchanged parity oracle), 1 = gogpu/ui v2 path.
- The UI must **never clear the engine world** (hard constraint G10): it always
  composites over the top.

### 1.4 Definition of Done (MVP)

1. `ui_backend 1` shows the menu using the real `gfx/*.lmp` images (plaques,
   main menu artwork, cursor dots, submenu titles) and conchars bitmap text at
   the legacy 320x200 layout positions.
2. Structural parity with `ui_backend 0`: same screens, items, actions, key
   and mouse navigation semantics, save/load/mods behavior (AC1).
3. In-window captures at `ui_backend 1` visually match the legacy layout
   (AC5): same geometry, same images — behavioral parity plus visual fidelity.
4. Input fallthrough contract (G10): menu and console capture input; the HUD
   is non-interactive and never captures; when no active UI input element,
   input falls through to the engine.
5. Full suite green on both paths; no regressions in `internal/menu`,
   `internal/console`, `internal/hud`, `internal/game` input latching tests
   (path 0 unchanged).
6. No `//go:build` tags; `CGO_ENABLED=0` preserved.
7. `internal/quakui` is self-contained; `internal/game` touches only a narrow
   adapter (no host construction, no input raw sinks, no CSQC fallback logic).

## 2. High-Level AI-DLC / Agent Prompt

Rewrite the menu, console, and HUD on gogpu/ui behind `ui_backend 0|1`.
Follow IRONWAIL-SPEC-002, ADRs 0006-0009, research 0006-0007. Constraints:

1. Use `desktop.Run(gogpuApp, uiApp)` (Architecture B). Render the engine
   world into a `core/gpuview` texture via `OnRender(view)`, and let the
   desktop compositor blit it under the UI widgets. The UI composite must
   never clear the world (LoadOpLoad / preserve content).
2. Render the real `gfx/*.lmp` menu images (plaques, titles, cursors) via the
   pic bridge (`QPicToImage`) and `canvas.DrawImage`; text via the conchars
   bitmap atlas (image.RGBA.SubImage per glyph). No TTF for menu text.
3. `internal/quakui` is self-contained. Expose one narrow adapter function
   the engine calls to boot the UI (e.g. `quakui.Run(host adapter)`); the
   engine does NOT wire input raw sinks, host construction, or CSQC fallback
   inside it.
4. Input: menu/console capture UI input; the HUD is non-interactive; input
   falls through to the engine when no active UI input element. Preserve the
   engine's KeyDest router and latching tests. Backtick/binding capture kept.
5. No //go:build tags, CGO off, Go 1.26.
6. Milestones: menu → console → HUD. Each is a red/green vertical slice.
7. TDD: run
   `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/quakui/...` and
   `mise run test` before closing a task.

## 3. Information Architecture / Topology

### 3.1 Package layout

```
internal/quakui/              SELF-CONTAINED gogpu/ui subsystem (v2)
  run.go                      quakui.Run(host) — desktop.Run + gpuview world
  adapter.go                  Host adapter interface implemented by internal/game
  worldtexture.go             gpuview widget wired to engine world render
  theme.go                    Quake theme tokens from palette.lmp
  pics.go                     gfx/*.lmp QPic -> image.Image bridge
  conchars.go                 conchars bitmap atlas (per-glyph SubImage)
  menu/                       menu widget tree (real LMP pics + conchars text)
  console/                    console widget tree
  hud/                        HUD widget tree (status bar, crosshair, centerprint)
internal/game/                WIRING ONLY: implements quakui.Host; calls quakui.Run on ui_backend 1
```

**Isolation boundary (AC7):** `internal/quakui` imports only gogpu/ui, the
existing `internal/menu|console|hud` state machines (read-only via accessors),
and `internal/image`. It NEVER imports `internal/game`, `internal/renderer`,
or `internal/renderer/*`. The world texture and cvar reads are abstracted by a
`quakui.Host` adapter (interface defined in `internal/quakui`) whose types are
gogpu/gpucontext types (a `gpucontext.TextureView` the engine renders into, and
plain value reads), not engine types. `internal/game` implements the adapter
and calls `quakui.Run(host)`. The engine's renderer exposes one method
(`RenderIntoWorldTexture(view)`) implemented in `internal/renderer` and routed
through `internal/game`'s adapter — never imported by `internal/quakui`.

### 3.2 Widget tree per surface

```
Root (window-sized; host-managed via desktop.Run)
├── GPUView (world texture, base layer — engine renders into it)
├── [hud surface] HUDWidget (non-interactive overlay)
├── [console surface] ConsoleWidget (captures input when active)
└── [menu surface] MenuRoot (captures input when active; real LMP pics)
```

The engine world renders into the gpuview texture (base). UI widgets
composite above. The UI never clears the world texture (base is LoadOpLoad).

### 3.3 Layering in the frame

`desktop.Run` owns the swapchain/render loop. The engine's world render is
retargeted from `OnDraw(surface)` to `gpuview.OnRender(view)` where the world
draws into the gpuview's offscreen `gpucontext.TextureView`. The desktop
compositor blits that texture as the base layer, then the UI widgets on top.
No gg-canvas bridge, no `render.NewCanvas` over the engine surface, no
`chrome canvas composite inside RenderFrame`.

## 4. Data Models

### 4.1 Retained legacy sources of truth

- `menu.Manager` state/actions (accessors from M3.1 v1) — consumed read-only.
- `console.Console` ring buffer/input/completion.
- `hud.State` + scoreboard.
- `renderer.CanvasTransformParams` / `internal/game/ui` dims math → translated
  to widget geometry; `scr_menuscale`/`conscale`/`sbarscale` as transforms.

### 4.2 cvar

- `ui_backend` 0|1 default 0 (kept from v1).

### 4.3 Visual tokens

- `palette.lmp` (768 bytes) → RGBA table; conchars 128x128 → per-glyph
  SubImage atlas (index 0 transparent).
- `gfx/*.lmp` menu art (ttl_main, mainmenu, sp_menu, mp_menu, p_option, etc.)
  drawn via `QPicToImage` + `canvas.DrawImage` at legacy 320x200 positions.

### 4.4 Menu scaling (C-lineage exact, G11)

The menu virtual viewport is **320x200** (matches C Ironwail `CANVAS_MENU`,
`gl_draw.c:1214`: `Draw_Transform(320, 200, s, CENTERX, CENTERY)`; the user's
recollection of 320x240 applies to the QuakeWorld status bar / sbar2, not the
menu). The widget layer reproduces the transform exactly:

```
s = min(guiwidth/320, guiheight/200)
s = clamp(1.0, scr_menuscale, s)          // scr_menuscale clamps the min scale
offset = ((guiwidth - 320*s)/2, (guiheight - 200*s)/2)   // centered
```

`guiwidth/guiheight` are the engine's GUI dims from `internal/game/ui` dims
math (pixel aspect applied). All menu LMP pics and conchars glyphs are drawn
inside this 320x200 viewport at their legacy coordinates; the transform maps
them to the screen. `scr_menuscale` is applied as a widget transform (not a
per-glyph scale hack).

## 5. State Machines / Flows

### 5.1 Boot / gating

```
init: registered ui_backend (0 legacy / 1 gogpu/ui)
Run frame:
  path = ui_backend==1 ? quakui : legacy
  path==legacy → existing host/screen/menu/console/HUD (untouched)
  path==gogpu-ui → engine calls quakui.Run(host) which:
        builds uiApp with core/gpuview widget as the base of the root
        (research 0006 §2-3)
        wires gpuview.OnRender(view) → host.RenderIntoWorldTexture(view)
        (engine retargets world into the gpuview texture; research 0006 §4)
        calls desktop.Run(gogpuApp, uiApp)  // sole OnDraw owner (research 0006 §1)
        compositor blits gpuview texture as base; UI widgets on top;
        world never cleared (research 0006 §3)
```

The engine's EventSource shim (ADR-0007, research 0007) is the sole input
registration; KeyDest routes menu/console → ui, HUD-only/game → engine
(fallthrough).

### 5.2 Input

- **Menu active** → UI captures keyboard/mouse (KeyDest KeyMenu).
- **Console active** → UI captures input (KeyDest KeyConsole).
- **HUD only** → HUD is non-interactive; input falls through to engine.
- Fallthrough contract (G10): when no active UI input element, engine receives
  input. Backtick/binding capture preserved. Exact wiring deferred to the
  input spike (research 0007).

### 5.3 CSQC

- CSQC active → HUD falls back to legacy (mod compat); not rewired in v2.

## 6. Security Model

No new attack surface: all input is local; text/images come from local Quake
assets (palette.lmp, conchars, gfx/*.lmp). The gpuview texture is engine-mutated
GPU content; the UI composites only. No network/auth/secrets. A11y roles inert
on desktop. CSP/eBPF N/A.

## 7. Edge Cases & Failure Handling

| Edge case | Behavior |
| --- | --- |
| UI init failure (desktop.Run/app.New error) | log, keep ui_backend pinned 0, legacy unchanged (fail-open) |
| `ui_backend 1` + software/screenshot renderer | legacy path always (no gpuview); screenshots stay comparable |
| CSQC mod active | HUD falls back to legacy path |
| Demo playback active | demo bar legacy-only; skipped on path 1 (deferred) |
| UI never clears world | gpuview base uses LoadOpLoad; assert in capture/parity |
| window resize / DPI | desktop.Run layouts widgets; gpuview resized |
| GPU resource lifetime | gpuview widget releases texture on Unmount; per-frame no leaks |
| input fallthrough | menu/console capture; HUD non-interactive; engine gets unconsumed |
| ui_backend toggle mid-session | sa stable; surfaces re-created, state re-read from legacy |

## 8. Acceptance Criteria

| # | Criterion | Verifiable by |
| --- | --- | --- |
| AC1 | cvar/command surface identical on both paths; same menu structure | grep diff of registered cvars/commands; functional menu walkthrough |
| AC2 | full `mise run test` green; menu/console/hud/game tests pass on path 0 | `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./...` |
| AC3 | path 1 boots windowed, opens menu/submenus without panic, no GPU leak growth. Headless boot stays `ui_backend=0` legacy (desktop.Run needs a window; no gpuview canvas headless) | `mise run smoke-*` + `+ui_backend 1` (windowed); headless smoke on path 0 |
| AC4 | ui_backend toggle mid-session clean (no state corruption) | integration test toggling + menu cursor/console scroll assertions |
| AC5 | in-window captures at ui_backend 1 visually match legacy layout (real LMP pics, 320x200 geometry) | in-window capture at 1 vs 0; `-screenshot` stays legacy oracle |
| AC6 | no //go:build tags; CGO off; pure Go | grep empty; `go build` with CGO off |
| AC7 | internal/quakui self-contained; internal/game only via narrow adapter | import graph check: `internal/quakui` imports only legacy state + image + gogpu/ui; no `internal/game` in quakui |
| AC8 | no stock core/* widgets used by quakui widgets (BYO-kit) | import graph check |
| AC9 | UI never clears the world; input falls through when no active UI element | capture/parity assert world visible under UI; input tests for fallthrough |

## 9. Cross-references

- In-scope research: 0001, 0002, 0004 (done), 0006, 0007 (spikes).
- ADRs: 0006 (desktop/GPUView integration), 0007 (input/fallthrough),
  0008 (visual fidelity: LMP pics + conchars), 0009 (isolation boundary).
- Prior (superseded): SPEC-001, ADRs 0001-0005. Gap log `.ai-dlc/ui-rewrite-v2/gaps.md`.

## Review Log

### Stage 3 — Review 1 (spec defense), 2026-08-19

**Verdict: APPROVED WITH FINDINGS** (6 raised; 2 fixed in-spec, 2 blocked on
research, 2 noted).

| # | Finding | Seve. | Resolution |
| --- | --- | --- | --- |
| R1.1 | AC7 boundary: `internal/quakui` must not import `internal/renderer`, yet the world texture source is a renderer concern. The `quakui.Host` adapter must abstract the world texture as a `gpucontext.TextureView` and cvar reads as plain values; the engine renderer exposes `RenderIntoWorldTexture(view)` routed via `internal/game`. | high | fixed in-spec (§3.1 isolation boundary pinned) |
| R1.2 | AC9 (input fallthrough) is not verifiable until the input spike lands: under `desktop.Run` the ui owns the EventSource; how unconsumed events reach the engine during HUD-only is unknown. | high | blocked on research 0007 (G6); AC9 gated on spike |
| R1.3 | §3.3 desktop.Run integration understates the renderer surgery: retargeting world/waterwarp/polyblend/scene-composite from the surface to a gpuview texture. | high | blocked on research 0006 (G3); integration architecture gated on spike |
| R1.4 | Archive mechanics unspecified (which files move, tag name, what carries). | med | fixed in-spec (§1.3 archive pinned: tag `v1-ui-rewrite`, backend.go + accessors carry, widgets/wiring removed) |
| R1.5 | AC2 path-0 requires legacy untouched; archive must not break legacy menu/console/hud. | med | noted — v1 accessors are additive and path-0 safe; archive step runs full suite before removing |
| R1.6 | AC7 (isolation) is MVP-wide but milestone order is menu-first; AC7 verified at milestone 3. | info | noted — AC7 is a global constraint, not milestone-1 gate |

## Change Log
| Date | What | Why |
| --- | --- | --- |
| 2026-08-19 | Initial v2 spec draft (Stage 2) | After Stage 1 elicit (user decisions G1-G10) |
| 2026-08-19 | Stage 3 revision: pinned adapter contract (R1.1) + archive mechanics (R1.4) | Review 1 findings |
| 2026-08-19 | Stage 4-6: ADRs 0006-0009 derived; research 0006/0007 delivered and folded into §3.3/§5.1; AC3 headless caveat (R2.2); G4 clarified (R2.1) | Reviews 1-2 findings + spikes |