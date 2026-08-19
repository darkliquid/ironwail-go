# Implementation Plan 30: gogpu/ui UI Rewrite v2 (IRONWAIL-SPEC-002)

**Priority**: Experiment (branch `experiment/ui-rewrite-v2`); the reference
BYO-kit validation case for gogpu discussion #468.
**Status**: PLANNED (2026-08-19) — pending Review 3.
**Prerequisite**: stable baseline (`mise run test` green on
`experiment/ui-rewrite` at the v1 archive tip).
**Spec**: `docs/internal/specs/002-ui-gogpu-rewrite-v2.md`
**ADRs**: `docs/internal/adr/0006..0009`
**Research**: `docs/internal/research/0006..0007`
**Gaps**: `.ai-dlc/ui-rewrite-v2/gaps.md` (rows 3, 6, 12-14 resolved →
resolved by tasks D0/D1 below)
**Estimated effort**: 6-10 focused sessions (phases M0-M3)

---

## 1. Executive Summary

Rewrite the menu, console, and HUD on gogpu/ui a second time, correcting the
three SPEC-001 failures: (1) visual fidelity with the real `gfx/*.lmp` menu
images and conchars bitmap text; (2) the `desktop.Run` + `core/gpuview` path
(world renders into a gpuview texture; the desktop compositor blits it as the
base, never clearing the world); (3) a self-contained `internal/quakui`
subsystem with a narrow `quakui.Host` adapter and no `internal/game` wiring
leaks. `ui_backend 0|1` gate kept; wasm/software/screenshot stay legacy.
Milestone order: menu → console → HUD. All work on `experiment/ui-rewrite-v2`;
no commit/push unless asked (conservative policy).

## 2. Milestones & Phases

| Milestone | Theme | Deliverable |
| --- | --- | --- |
| M0 | Archive v1 + baseline | tag `v1-ui-rewrite`; remove `internal/quakeui` + game wiring; carry `ui_backend` gate + `menu.Manager` accessors; full suite green |
| M1 | quakui core | `internal/quakui` host (adapter, run.go, worldtexture, EventSource shim), pics + conchars bridges, theme; desktop.Run boots path 1 windowed with world in gpuview |
| M2 | Menu on gogpu/ui | menu widget tree with real LMP pics + conchars text at 320x200 CANVAS_MENU transform; structural + capture parity |
| M3 | Console + HUD on gogpu/ui | console widget (scrollback/input/notify/completion); HUD status bar/crosshair/centerprint; input fallthrough |

Sequencing rationale: M0 first (archive risk, fail fast); M1 builds the
host everything hangs off (desktop.Run + gpuview is the integration spike);
M2 menu is the visual-fidelity proof; M3 console+HUD are independent slices
on the M1/M2 infra.

## 3. Tasks (Red-Green TDD, file paths, test commands)

**Command key** (all with `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp
CGO_ENABLED=0``):
- `FULL = go test ./...`
- `MENU = go test ./internal/menu/...`
- `CONS = go test ./internal/console/...`
- `HUD = go test ./internal/hud/...`
- `GAME = go test ./internal/game/...`
- `QUAKUI = go test ./internal/quakui/...`
- `REND = go test ./internal/renderer/...`

### M0 — Archive v1 + baseline

**M0.1 Archive v1 quakeui (G9)**
- Mechanics (pinned): on `experiment/ui-rewrite`, `git tag v1-ui-rewrite` at
  the current tip; `git checkout -b experiment/ui-rewrite-v2` from that tag;
  then remove the v1 code on the v2 branch.
- Remove `internal/quakeui/*` (widgets, host, gateway, canvas bridge, stack,
  theme, notes) and the `internal/game` wiring (UIHost/root fields,
  syncUIHostRoot, gateway raw sinks, CSQC fallback, quakeUIText).
- Carry over: `ui_backend` cvar gate (game_init.go), `backend.go` reader
  (moved to `internal/quakui/backend.go`), `menu.Manager` accessor surface
  (accessors.go — additive, path-0 safe).
- Tests: `FULL` — RED: v1 tests referencing removed packages fail;
  GREEN: full suite green on the archive tip with legacy path untouched.
- Acceptance: AC2 (path 0 untouched), G9.

**M0.2 Baseline guard**
- Run `FULL` on the archive tip before any v2 code; report any pre-existing
  failures before proceeding (AGENTS.md stable-baseline rule).
- Acceptance: baseline green.

### M1 — quakui core

**M1.1 `internal/quakui` adapter + backend (ADR-0009)**
- Files: `internal/quakui/adapter.go` (`Host` interface: world texture as a
  `gpucontext.TextureView`, `RenderIntoWorldTexture(view)`, cvar reads as
  values, command text/sound sinks as `func(string)`, input keydest as a
  plain enum), `internal/quakui/backend.go` (`ui_backend` reader, moved from
  v1).
- Tests: `QUAKUI` — RED: adapter interface missing; GREEN: implements. The
  import-graph assertion is a test that runs `go list -deps` (or parses the
  package's imports) and fails if `internal/game` or `internal/renderer`
  appear in `internal/quakui`'s import closure.
- Acceptance: AC7, ADR-0009.

**M1.2 pics + conchars bridges (ADR-0008)**
- Files: `internal/quakui/pics.go` (`QPicToImage`: palette-indexed WAD pic →
  RGBA, index 255 transparent), `internal/quakui/conchars.go` (128x128 atlas →
  per-glyph `image.RGBA.SubImage`).
- Tests: `QUAKUI` — RED: QPicToImage converts with 255 transparent; conchars
  glyph sub-images correct; GREEN.
- Acceptance: ADR-0008, AC5 (visual fidelity primitives).

**M1.3 theme (palette tokens)**
- Files: `internal/quakui/theme.go` — Quake palette tokens (palette[0]
  background, palette[4] surface, bright row) as a `theme.Theme` +
  extension.
- Tests: `QUAKUI` — RED: token mapping; GREEN.
- Acceptance: spec §4.3.

**M1.4a renderer world-into-view (ADR-0006, research 0006)**
- Files: `internal/renderer` — expose `RenderIntoWorldTexture(view
  gpucontext.TextureView, state)` that retargets the world/entities/polyblend
  render into `view`, reusing the waterwarp scene-target machinery.
- Tests: `REND` — RED: method missing; GREEN: world renders into an offscreen
  view without clearing it. No UI code yet.
- Acceptance: AC9 (world never cleared), ADR-0006.

**M1.4b run.go + desktop.Run + game adapter (ADR-0006, research 0006)**
- Files: `internal/quakui/worldtexture.go` (gpuview widget wired to
  `Host.RenderIntoWorldTexture`), `internal/quakui/run.go`
  (`quakui.Run(host)`: build uiApp with gpuview base, `desktop.Run(gogpuApp,
  uiApp)`); `internal/game` implements `quakui.Host` and calls `quakui.Run`
  on `ui_backend=1` (wasm/software gated off).
- Tests: `QUAKUI` + `GAME` — RED: run.go missing; GREEN: path 1 boots
  windowed, world visible in gpuview, UI composites over, world never cleared
  (capture assert).
- Acceptance: AC3 (windowed), AC9, ADR-0006.

**M1.5 EventSource shim (ADR-0007, research 0007)**
- Files: `internal/quakui/input.go` — engine-owned EventSource shim; routes
  by KeyDest: KeyMenu/KeyConsole → ui, KeyGame/HUD-only → game (fallthrough).
  Backtick/binding capture preserved.
- Mechanism (pinned): use `uiApp.HandleEvent(e event.Event)` for keys/chars
  (sufficient for menu/console); defer a full `gpucontext.EventSource` shim
  only if mouse/scroll/IME must reach the ui (research 0007 open question).
- Tests: `QUAKUI` + `GAME` — RED: KeyConsole → ui, KeyGame → game, HUD-only
  never captures; GREEN. Latching tests still pass on path 0.
- Acceptance: AC9 (fallthrough), ADR-0007.

### M2 — Menu on gogpu/ui

**M2.1a menu root + core pages (real LMP pics + conchars)**
- Files: `internal/quakui/menu/{menu.go,main.go,options.go}` — menu root
  widget + main/singleplayer/options pages; real `gfx/*.lmp` pics via
  `QPicToImage` + `canvas.DrawImage`; conchars text; cursor via
  `gfx/menudot1..6.lmp`; actions via `menu.Manager.M_Key/M_Char`.
- Tests: `QUAKUI menu_test.go` — RED: canned `menu.Manager` state renders
  expected pics/labels/cursor at legacy 320x200 positions; GREEN. Cross-check
  with legacy `M_Draw` layout constants.
- Acceptance: AC1, AC5, ADR-0008.

**M2.1b remaining menu pages**
- Files: `internal/quakui/menu/{game.go,setup.go,joinhost.go,mods.go}` —
  load/save/multi/join/host/setup/mods/controls/video/audio/help/quit pages.
- Tests: `QUAKUI menu_test.go` — RED: each page renders expected pics/rows
  from canned state; GREEN.
- Acceptance: AC1, AC5, ADR-0008.

**M2.2 menu scaling (spec §4.4, G11)**
- Files: `internal/quakui/menu/scale.go` — CANVAS_MENU transform:
  `s=min(guiwidth/320, guiheight/200)` clamped by `scr_menuscale`, centered.
- Tests: `QUAKUI` — RED: transform math for 1280x720 / 1892x1072; GREEN.
- Acceptance: AC5 (capture matches legacy layout), spec §4.4.

### M3 — Console + HUD on gogpu/ui

**M3.1a console widget (scrollback + input + notify)**
- Files: `internal/quakui/console/console.go` — reads `console.Console` via
  accessors; conchars text; `]` prompt + blink cursor; notify fade.
- Tests: `QUAKUI console_test.go` — RED: N lines/input/scroll/notify-alpha;
  GREEN. `CONS` still green (console pkg untouched).
- Acceptance: AC1, ADR-0008.

**M3.1b completion bridge**
- Files: `internal/quakui/console/completion.go` — hooks
  `console.CompleteInput`; renders match list as conchars text.
- Tests: `QUAKUI console_test.go` — RED: Tab → matches → list; cycle; GREEN.
- Acceptance: AC1.

**M3.2 HUD widgets (status bar + crosshair + centerprint)**
- Files: `internal/quakui/hud/{statusbar.go,crosshair.go,centerprint.go}` —
  from `hud.State`/`HUDStyle`; status bar pics + nums; crosshair glyph;
  centerprint typewriter + bg modes.
- Tests: `QUAKUI hud_test.go` — RED: canned `hud.State` renders per style;
  GREEN. `HUD` still green.
- Acceptance: AC1, AC5, ADR-0008.

**M3.3 input fallthrough integration (ADR-0007)**
- Files: `internal/game` adapter wiring — KeyDest routes menu/console → ui,
  HUD-only → game; HUD widget tree is draw-only (structural non-capture).
- Tests: `GAME` — RED: menu/console capture, HUD-only falls through, latching
  tests pass; GREEN.
- Acceptance: AC9, ADR-0007.

## 4. Traceability Matrix

| Task | Spec § | ADR | AC | Gaps |
| --- | --- | --- | --- | --- |
| M0.1 | §1.3 | — | AC2 | 9 |
| M0.2 | — | — | AC2 | — |
| M1.1 | §3.1 | 0009 | AC7 | 5 |
| M1.2 | §4.3 | 0008 | AC5 | 4 |
| M1.3 | §4.3 | — | AC5 | — |
| M1.4a | §3.3 | 0006 | AC9 | 3, 10 |
| M1.4b | §3.3/§5.1 | 0006 | AC3, AC9 | 3, 10 |
| M1.5 | §5.2 | 0007 | AC9 | 6, 10 |
| M2.1a | §3.2/§4.3 | 0008 | AC1, AC5 | 4 |
| M2.1b | §3.2/§4.3 | 0008 | AC1, AC5 | 4 |
| M2.2 | §4.4 | — | AC5 | 11 |
| M3.1a | §5.3 | 0008 | AC1 | — |
| M3.1b | §5.3 | — | AC1 | — |
| M3.2 | §5.4 | 0008 | AC1, AC5 | — |
| M3.3 | §5.2 | 0007 | AC9 | 6, 10 |

## 5. Risks & Mitigations

| Risk | Mitigation | Gap |
| --- | --- | --- |
| desktop.Run world retarget surgery (waterwarp/polyblend/scene-composite) | M1.4 reuses the existing waterwarp scene-target machinery; REND suite; fallback: only the world pass retargeted, rest surface-bound | 3 |
| desktop.Run blocks / loop ownership conflict | M1.4 is the integration spike; engine does NOT register its own OnDraw on path 1; wasm/software gated off | 3, 14 |
| Menu.Manager accessor drift | M0.1 carries the v1 accessor surface; M2.1 RED first against canned state | 9 |
| Visual fidelity (LMP pics + conchars) | M1.2/M2.1 use the proven QPicToImage + DrawImage path; capture compare vs legacy | 4 |
| Input fallthrough hard under desktop.Run | M1.5/M3.3 KeyDest shim; no consumed-signal API so KeyDest decides; latching tests gate | 6 |
| UI clears world | M1.4 gpuview base via compositor blit (no LoadOpClear); capture assert | 10 |
| wasm/software path | ui_backend=1 gated off on GOOS=js; legacy path untouched | 14 |

## 6. Post-MVP follow-ups (bd candidates)

- CSQC bridge onto the ui canvas (spec §1.2 defer).
- Demo bar UI + interactive scrubbing (bd ironwail-go-cuy).
- Full-window unscaled UI option (spec §4.4 note: future TTF/full-window).
- PARITY.md UI deviations section for v2; docs/internal/{menu,console,hud}.md
  rewrite for quakui packages.

## Review Log

### Stage 8 — Review 3 (2026-08-19)

Verdict: APPROVED WITH FIXES. Six findings resolved in-spec:
- R3.1 pinned archive mechanics (tag on `experiment/ui-rewrite`, branch v2
  from the tag, remove there).
- R3.2 split M1.4 into M1.4a (renderer world-into-view) + M1.4b (run.go +
  desktop.Run + game adapter).
- R3.3 pinned the input mechanism (uiApp.HandleEvent for keys/chars; defer
  full EventSource shim).
- R3.4 split M2.1 into M2.1a (root + core pages) + M2.1b (remaining pages).
- R3.5 split M3.1 into M3.1a (console widget) + M3.1b (completion bridge).
- R3.6 pinned the import-graph assertion (go list -deps / import parse).

Self-check: each task ≤ ~2 days; TDD ordering survivable (M0 → M1 → M2 →
M3, no task needs later code); test commands are the repo's real commands;
file paths exist or are new under `internal/quakui`; acceptance criteria
staged per task; traceability matrix covers gaps 3-14.
