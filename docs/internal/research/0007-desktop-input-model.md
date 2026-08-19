# RESEARCH-0007: Input Model under desktop.Run (KeyDest + Fallthrough)

- **Status:** Delivered
- **Owner:** ironwail-go-if4 (spike bead)
- **Date:** 2026-08-19
- **Blocks:** `.ai-dlc/ui-rewrite-v2/gaps.md #6` (R1.2)

---

## Research Question

Under `desktop.Run`, the ui app owns the EventSource. How does the engine's
KeyDest router (KeyGame/KeyConsole/KeyMenu) and the input fallthrough contract
(menu/console capture; HUD is non-interactive and input falls through to the
engine) map onto the ui tree?

## Background & Constraints

- SPEC-002 §5.2 / AC9 (G10): menu and console capture UI input; the HUD is
  non-interactive; when no active UI input element, input falls through to the
  engine. Backtick/binding capture preserved; existing latching tests intact.
- The engine's input backend (`internal/renderer/gogpu/input_backend.go`)
  registers on the gogpu `EventSource()` single-slot callbacks and feeds
  `sys.HandleKeyEvent` (the KeyDest router). The ui app also wants
  `app.WithEventSource(gogpuApp.EventSource())`.

## Investigation Findings

### 1. Single-slot EventSource conflict

`gogpu.App.EventSource()` returns an adapter holding **single** callbacks per
event kind (event_source.go:45-120); a second registration overwrites the
first. The engine's `InputBackend.initCallbacks` already holds those slots

(input_backend.go:177). If the ui app also calls
`app.WithEventSource(gogpuApp.EventSource())`, the two registrations fight.
Under `desktop.Run`, the ui must receive input; the engine must still route
gameplay input. A single authoritative shim is required.

### 2. No "consumed" signal from the ui tree

`Window.HandleEvent(e)` (app/window.go:424) returns void; the event bridge
callbacks (OnKeyPress etc.) fire into it with no return value. There is no
public API to learn whether a widget consumed an event. So the engine cannot
"ask" the ui whether a key was swallowed — it must decide up front, by KeyDest,
what the ui sees.

### 3. The workable model: engine-owned EventSource shim routing by KeyDest

The engine keeps one authoritative EventSource registration (the shim). It
routes per KeyDest:
- **KeyMenu / KeyConsole** → forward the translated event to the ui app
  (`uiApp.HandleEvent(e)` or via the ui's EventSource bridge). The ui widget
  tree (menu/console) consumes it. The game does NOT also process it.
- **KeyGame / HUD-only** → do NOT forward to the ui (the HUD is
  non-interactive); the event goes only to the game's KeyDest router. This
  satisfies fallthrough: when no active UI input element, input stays in the
  game.
- **Backtick / binding capture** → the engine's existing `normalizeMenuKey`
  and pending-binding logic run first; the shim preserves them.

This is exactly the v1 gateway model (ADR-0003), but inverted: v1 fed the ui
from the engine's raw sinks; v2 makes the ui the primary consumer under
`desktop.Run` while the engine's shim decides what the ui sees by KeyDest.
The shim lives in `internal/quakui` (self-contained), exposing a
`ForwardKeyToUI` / `ForwardCharToUI` pair the engine's KeyDest router calls.

### 4. HUD is non-interactive by construction

The HUD widget tree contains no focusable/interactive widgets (status bar,
crosshair, centerprint are draw-only). Because the shim never forwards input
to the ui during HUD-only, the HUD cannot capture input. This is a structural
guarantee, not a per-event check.

### 5. Latching / mouse-look tests intact

The engine's `sys.HandleKeyEvent` dedup/latching logic is unchanged; the shim
only changes WHERE events are forwarded (ui vs game), not the engine's
internal state machine. The existing
`game_movement_input_test.go:145-329` latching tests remain on path 0
unchanged; path 1 adds the shim routing.

## Recommended Resolution

- `internal/quakui` owns a `gpucontext.EventSource` shim (or a direct
  `ForwardKeyToUI`/`ForwardCharToUI` pair into `uiApp.HandleEvent`) that is
  the sole EventSource registration under `desktop.Run`.
- The engine's KeyDest router calls the shim: KeyMenu/KeyConsole → ui;
  KeyGame/HUD-only → game only (fallthrough).
- The HUD widget tree is draw-only (no interactive widgets), structurally
  guaranteeing it never captures input.
- Backtick/binding capture and the engine's latching state machine are
  preserved; the shim routes, it does not re-implement.

## Open Questions / Follow-ups

- Whether to use `uiApp.HandleEvent(e event.Event)` directly (bypasses the
  EventSource bridge's pointer/scroll/IME standardization) or implement the
  full `gpucontext.EventSource` shim (v1 gateway shape). The HandleEvent path
  is simpler and sufficient for menu/console keys; the full shim is needed if
  mouse/scroll/IME must reach the ui. Plan decision.

## Source Index

- gogpu/ui v0.1.54: app/window.go:424 (HandleEvent void); app/app.go:62
  (WithEventSource); app/event_bridge.go (bridge callbacks).
- gogpu v0.53.0: event_source.go:45-120 (single-slot callbacks).
- ironwail-go: internal/renderer/gogpu/input_backend.go:177;
  internal/input/types_binding.go:185 (KeyDest router);
  internal/game/game_movement_input_test.go:145-329 (latching).
