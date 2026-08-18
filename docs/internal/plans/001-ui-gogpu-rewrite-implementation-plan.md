# Implementation Plan 29: gogpu/ui UI Rewrite (IRONWAIL-SPEC-001)

**Priority**: Experiment (branch `experiment/ui-rewrite`); the reference
BYO-kit validation case for gogpu discussion #468.
**Status**: PLANNED (2026-08-18) — pending Review 3.
**Prerequisite**: stable baseline (`mise run test` green on
`experiment/ui-rewrite` at gogpu v0.52.1 before the dependency bump).
**Spec**: `docs/internal/specs/001-ui-gogpu-rewrite.md`
**ADRs**: `docs/internal/adr/0001..0005`
**Research**: `docs/internal/research/0001..0004`
**Gaps**: `.ai-dlc/ui-gogpu-rewrite/gaps.md` (rows 13-18 open → resolved by
tasks S1/T1/D0 below)
**Estimated effort**: 8-12 focused sessions (phases M0-M5)

---

## 1. Executive Summary

Rewrite menu, console, and HUD in ironwail-go on gogpu/ui behind a
`ui_backend 0|1` cvar (default 0 = legacy), per IRONWAIL-SPEC-001 and ADRs
0001-0005. Engine owns the gogpu.App, render loop, and input; gogpu/ui runs
inside the engine frame via `Frame()` + `Window().DrawTo(render.NewCanvas)`.
Text via TTF rasterized from conchars (spike T1 picks pixel-TTF vs
QuakeTextWidget atlas). CSQC and demo bar stay legacy in MVP. All work on
`experiment/ui-rewrite`; no commit/push unless asked (conservative policy).

## 2. Milestones & Phases

| Milestone | Theme | Deliverable |
| --- | --- | --- |
| M0 | Baseline guard + dependency bump | suite green pre-bump checkout; `ui_backend` cvar + gate skeleton; deps bumped, full suite still green |
| M1 | Spikes (text + integration) | T1 pixel-TTF decision; S1 GPUView stretch scoping (out of MVP, documented); escape-hatch atlas spike (R1.4) |
| M2 | core ui plumbing | `internal/quakeui` host (widget_host, backend, gateway_events, canvas_bridge) + Quake theme; first widget: QuakeText widget |
| M3 | Menu on gogpu/ui | menu.Manager accessor surface (G.13); menu widget pages; navigation/actions reuse; menu surface parity walkthrough |
| M4 | Console on gogpu/ui | console/console-widget; input line; completion bridge; slide anim |
| M5 | HUD on gogpu/ui | statusbar (classic/modern/QW), crosshair, centerprint, scoreboard; CSQC fallback gate |
| (S) | Stretch — GPUView 3D viewport | scoped but NOT in MVP (G.16) |

Sequencing rationale: M0 first (risk: dep bump breaks renderer — fail fast);
M1 spikes de-risk the two biggest unknowns (text pipeline, integration
canvas) before building widgets on them (TDD survivability); M2 builds the
host everything hangs off; M3-M5 are independent vertical slices ordered by
dependency (menu uses accessor surface from M3 first, console reuses its text
widget, HUD last as it consumes hud.State + the text/canvas infra).

## 3. Tasks (Red-Green TDD, file paths, test commands)

**Command key** (all with `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp
CGO_ENABLED=0`):
- `FULL = go test ./...`
- `MENU = go test ./internal/menu/...`
- `CONS = go test ./internal/console/...`
- `HUD = go test ./internal/hud/...`
- `GAME = go test ./internal/game/...`
- `QUI = go test ./internal/quakeui/...`
- `REND = go test ./internal/renderer/...`

### M0 — Baseline + dependency bump

**M0.1 Create `ui_backend` cvar + gate skeleton (RED first)**
- Files: `internal/game/game_init.go` (register cvar), new
  `internal/quakeui/backend.go` (`UIBackend() int` reader + `IsGogpuUIPath()`
  bool), `internal/game/game_runtime_frame.go` (branch at overlay call).
- Tests: `internal/quakeui/backend_test.go` — RED: no `ui_backend` cvar →
  read fails; GREEN: register + default 0; toggle test asserts path switch.
- Acceptance: AC1 (cvar surface), AC4 (toggle clean).

**M0.2 Dependency bump (ADR-0005)**
- Commands: `go get github.com/gogpu/gogpu@v0.53.0 github.com/gogpu/gg@
  v0.52.3 && go get github.com/gogpu/ui@v0.1.54`, then `FULL` + `REND`.
- Files: `go.mod`, `go.sum` only. No code changes unless renderer breaks.
- Acceptance: AC9 (bump confined); full suite green on new deps.

### M1 — Spikes

**M1.1 (T1) Pixel-TTF vs QuakeTextWidget (ADR-0004 decision)**
- Spike branch-local file `internal/quakeui/notes/text-spike.md` — read-only
  experiment: build a 16x16-grid pixel TTF from conchars (PUA mapping) OR
  implement the atlas widget with `image.RGBA.SubImage`; measure
  `MeasureStyledText` width correctness + draw cost.
- Test: spike harness `QUI`; decision recorded (ADR-0004 outcome updated).
- Gaps: 17 → RESOLVED.

**M1.2 (S1) GPUView stretch scoping (G.16)**
- Read-only spike: confirm feasibility of rendering engine world into a
  `core/gpuview` texture (engine already has offscreen scene-target
  machinery `sceneTargetActive` in `renderer_gogpu_frame.go`). Output:
  `internal/quakeui/notes/gpuview-spike.md`; marks stretch milestone (S) —
  explicitly NOT in MVP.
- No code. Gaps: 16 → RESOLVED (kept as stretch).

**M1.3 (R1.4) sprite-atlas sub-rect path**
- Spike: status bar faces/weapons drawn via `image.RGBA.SubImage` slices vs
  gg escape hatch (`Context().DrawImageEx` or `DrawImage` sub-rects).
- Decision note in `notes/sprite-atlas-spike.md`. Gaps: 15 → RESOLVED.

### M2 — core ui plumbing

**M2.1 `internal/quakeui/theme` (theme.go, extension.go, pics.go)**
- Files: `internal/quakeui/theme/theme.go` (`QuakeTheme()` from
  `theme.DefaultDark()` + palette tokens, spec §4.3), `extension.go`
  (`quakeui` ThemeExtension: PromptGlyph, ScrollHintGlyph, BrightRow,
  MenuBgAlpha, SbarAlpha, Palette, PuaBase), `pics.go` (`QPicToImage`
  bridge for `canvas.DrawImage`).
- Tests: `theme_test.go` — RED asserts token mapping (palette[0]=Background,
  palette[4]=Surface, bright-row flag), GREEN implements. `QUI`.
- Acceptance: AC8 (no stock core/*), spec §4.3.

**M2.2 `internal/quakeui/widgets/text.go` — QuakeText widget**
- Files: `internal/quakeui/widgets/text.go` (per T1 outcome: DrawStyledText
  with `FontFamily:"quake-conchars"` + PUA mapping, or atlas-backed
  QuakeTextWidget with 8px advance; prompt `]`, blink cursor, bright row).
- Tests: `text_test.go` — RED: measure/width, PUA high-bit mapping, cursor
  blink timer, prompt rendering; GREEN. `QUI`.
- Acceptance: ADR-0004; spec §2 #4.

**M2.3 `internal/quakeui/widget_host.go` — uiApp lifecycle + canvas bridge**
- Files: `internal/quakeui/widget_host.go` (`app.New(WithWindowProvider/
  WithPlatformProvider/WithEventSource(gateway)/WithTheme(quake)),
  SetRoot-by-surface, Frame()+DrawTo(render.NewCanvas(cc,w,h))`,
  `RenderModeHostManaged`), `canvas_bridge.go` (gg canvas from engine
  `gogpu.Context` — the M0/M2 integration spike), resize bridge.
- Tests: `widget_host_test.go` — RED: host constructs headless, root swap
  per surface, DrawTo no panic; GREEN. `QUI`.
- Acceptance: AC3 (boots), spec §3.1, ADR-0002.

**M2.4 `internal/quakeui/gateway_events.go` — input gateway (ADR-0003)**
- Files: `internal/quakeui/gateway_events.go` (engine `iinput.KeyEvent`/char/
  mouse → `gpucontext.EventSource` shapes; KeyDest routing; unconsumed →
  game; backtick/binding capture preserved).
- Tests: `gateway_events_test.go` — RED: KeyConsole → console widget gets
  keys; KeyMenu → menu gets keys; game keys not delivered to ui when KeyGame;
  mouse pos/delta mapping; GREEN. `QUI` + `GAME` (latching tests still pass
  on path 0).
- Acceptance: AC2 (no regression), spec §5.2-5.4, 7 (input edge cases).

### M3 — Menu

**M3.1 menu.Manager accessor surface (G.13)**
- Files: `internal/menu/manager.go` (+ `accessors_test.go`): add exported
  read accessors — `State()`, `CursorFor(state)`, `TextBuffer(name)`,
  `HostSettings()`, `Mods()`, `SaveSlots()`, plus action passthroughs
  already public (`M_Key`, `M_Char`, `ToggleMenu`, `ShowState`, ...).
- Tests: `MENU` — RED: accessors missing; GREEN: implemented, zero behavior
  change (all existing menu tests pass).
- Acceptance: R1.2; spec §4.1 (state reused, fields stay unexported).

**M3.2 menu widget root + page widgets**
- Files: `internal/quakeui/menu/{menu.go, main.go, options.go, game.go,
  setup.go, joinhost.go, mods.go, keys.go}` — page widgets for all 15 states;
  cursor/focus from `State()`/`CursorFor`; actions via existing M_Key/M_Char;
  navigation (arrows/enter/esc/mouse hit-test) mapped to widget event/gesture.
- Tests: `QUI menu_test.go` per page — RED: page renders expected labels/
  cursor position from a canned Manager state; GREEN. Cross-check with legacy
  `M_Draw` layout constants (draw rows from research 0001 §3).
- Acceptance: AC1, AC5 (in-window capture compare).

**M3.3 menu overlay stacking (G.14)**
- Files: quakeui host root layering (menu above console above HUD; console
  forced-up at boot), per ADR-0002.
- Tests: `widget_host_test.go` stacking order + visibility matrix
  (boot: console+menu; game: hud only; game+menu: hud+menu; console over
  menu). `QUI`.
- Acceptance: spec §7 edge cases, ADR-0002.

### M4 — Console

**M4.1 console widget (scrollback + notify + input)**
- Files: `internal/quakeui/console/console_widget.go` — reads
  `console.Console` via existing accessors (`Line`, `Scroll`, `BackScroll`,
  `NotifyTimes`); virtualized rows; `]` prompt + input line + blink cursor;
  `con_notify*` fade/dither → theme alpha; `conback.lmp` bg via pics bridge;
  slide fraction from `StepConsoleSlide` as widget transform.
- Tests: `QUI console_test.go` — RED: widget renders N lines from buffer,
  input line, scroll indicator, notify alpha; GREEN. `CONS` still green
  (unmodified console pkg; widget is additive).
- Acceptance: AC1 (console cvars surface preserved), spec §5.3.

**M4.2 completion bridge**
- Files: `internal/quakeui/console/completion_bridge.go` — hooks
  `console.TabCompleter`/`GlobalTabCompleter`; renders match list as widget
  text (TTF, `con_maxcols` columns).
- Tests: `QUI completion_test.go` — RED: Tab yields matches → widget list;
  cycle fwd/back; GREEN. `CONS`.

### M5 — HUD

**M5.1 status bar widget (classic/modern/QW)**
- Files: `internal/quakeui/hud/statusbar.go` — from `hud.State`,
  `HUDStyle`/`hud_style`, `scr_sbarscale/alpha`; big/small nums via
  QuakeText + num pics; faces/items/weapons via pics bridge (SubImage
  per M1.3); scoreboard.
- Tests: `QUI hud_test.go` — RED: canned `hud.State` renders expected
  numbers/positions per style; GREEN. `HUD` still green (unchanged pkg).
- Acceptance: AC1, AC5; spec §5.4.

**M5.2 crosshair + centerprint widgets**
- Files: `internal/quakeui/hud/crosshair.go`, `centerprint.go` — crosshair
  glyph via TTF char; centerprint typewriter (`scr_printspeed`), bg modes
  (`scr_centerprintbg` 1/2/3), intermission/finale.
- Tests: `QUI hud_test.go` — typewriter reveal timing, bg mode draws,
  crosshair hidden conditions (viewsize≥130 / intermission / char 0).
- Acceptance: AC1; spec §5.4.

**M5.3 CSQC fallback + demo-bar skip on path 1**
- Files: `internal/game/game_runtime_csqc.go` guard, `game_runtime_frame.go`
  (path 1: skip demo bar), spec §1.2/§7.
- Tests: `GAME` — RED: with a stub CSQC DrawHud active + ui_backend 1, HUD
  falls back to legacy (widget not drawn); path 1 no demo-bar panic when
  demo active; GREEN.
- Acceptance: AC7, spec §7 demo/CSQC rows.

## 4. Traceability Matrix

| Task | Spec § | ADR | AC | Gaps |
| --- | --- | --- | --- | --- |
| M0.1 | §1.3/§4.2/§5.1 | 0001 | AC1, AC4 | 5,12 |
| M0.2 | §1.3/§7 | 0005 | AC9 | 11, 18 |
| M1.1 | §2#4/§4.3 | 0004 | AC8 | 8, 17 |
| M1.2 | §3.1 (GPUView stretch) | 0002 | — | 16 |
| M1.3 | §3.1 pic/atlas | — | AC8 | 15 |
| M2.1 | §4.3 | 0001 | AC8 | — |
| M2.2 | §2#4 | 0004 | AC8 | 8, 17 |
| M2.3 | §3.1-3.3 | 0002 | AC3 | 6, 14 |
| M2.4 | §5.2-5.4/§7 | 0003 | AC2 | 7 |
| M3.1 | §4.1 | — | AC1 | 13 |
| M3.2 | §3.2/§5.2 | 0001 | AC1, AC5 | — |
| M3.3 | §7 | 0002 | AC4 | 14 |
| M4.1 | §5.3 | — | AC1 | — |
| M4.2 | §5.3 | — | AC1 | — |
| M5.1 | §5.4 | — | AC1, AC5 | — |
| M5.2 | §5.4 | — | AC1 | — |
| M5.3 | §1.2/§7 | — | AC7 | 10 |

## 5. Risks & Mitigations

| Risk | Mitigation | Gap |
| --- | --- | --- |
| gogpu v0.53.0 breaks renderer | M0.2 fails fast, REND suite, revert bump (ADR-0005 rollback) | 18 |
| Pixel-TTF build too costly | T1 fallback = atlas QuakeTextWidget (ADR-0004 variant b) | 17 |
| gg canvas bridge won't mount on engine surface | M2.3 is the integration spike; fallback = App.HandleEvent + manual composite (ADR-0003) or reassess A vs B | 6, 7 |
| Menu.Manager accessor surface drifts behavior | M3.1 RED first against all existing MENU tests (zero behavior change contract) | 13 |
| Surface stacking (boot console+menu) hard in ui | M3.3 explicit visibility-matrix tests; OverlayManager if needed | 14 |
| Sprite atlas sub-rects slow | M1.3 measures SubImage vs escape hatch; escape hatch (gg) is always available | 15 |
| input double-delivery on path 1 | gateway tests + existing latching tests; single authoritative registration | 7 |
| CSQC mod + path 1 | M5.3 explicit fallback-to-legacy test | 10 |
| Scope creep to GPUView/scrubbing | (S) milestone is explicitly out of MVP; demo bar/scrub stays in bd cuy | 16, 9 |

## 6. Post-MVP follow-ups (bd candidates)

- CSQC bridge onto ui canvas (spec §1.2 defer; feeds bd ironwail-go-1hg).
- Demo bar UI + interactive scrubbing (absorb/coordinate with bd
  ironwail-go-cuy).
- GPUView 3D-viewport stretch (S).
- PARITY.md UI deviations section; docs/internal/{menu,console,hud}.md
  rewrite for quakeui packages.
- Feedback upstream (#468): Canvas gap report + custom-widget authoring
  experience notes.

## Review Log

### Stage 8 — Review 3 (2026-08-18)

Verdict: awaiting sign-off (lifecycle driver must stop for human approval).
Draft self-check: tasks ≤ ~2 days each; TDD ordering survivable (M0 → M1
spikes → M2 infra → M3-M5 slices; no task needs later code); test commands
are the repo's real commands (`TMPDIR=.../.tmp CGO_ENABLED=0 go test
./internal/...`); file paths exist or are new under `internal/quakeui`;
acceptance criteria staged per task (each task maps to spec ACs). Open
review points to fix if raised: M1.1/M1.2 are spikes (RED not applicable —
defined as decision-output tasks); traceability matrix covers all gaps 5-18.
