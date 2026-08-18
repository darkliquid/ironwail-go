# ADR-0003: Engine-Owned Input with a Gateway EventSource into the ui Tree

**Status:** Accepted (2026-08-18)
**Deciders:** darkliquid (Stage 1: A+C), lifecycle driver
**Date:** 2026-08-18
**Related:** IRONWAIL-SPEC-001 §5.2-5.4, §7; research 0004 §1, §3-C;
gap log row 7; ADR-0002

## Context and Problem Statement

gogpu/ui's `app.New` wants `WithEventSource(gogpuApp.EventSource())`, but
gogpu's `eventSourceAdapter` holds a **single** callback per event kind and
gogpu's `App` has single-slot `OnDraw`/`OnUpdate` (research 0004 §1).
ironwail-go's renderer already owns those slots (renderer OnDraw +
`gogpuimpl.NewInputBackend`). Two registrations overwrite each other. The
engine also routes input through its own `KeyDest` router (KeyGame /
KeyConsole / KeyMenu) with carefully tested latching semantics.

## Decision Drivers

- Preserve the engine's key/mouse latching and mouse-look behavior
  (`game_movement_input_test.go:145-329`).
- Avoid double-delivery (the existing input backend already guards against
  the callback path double-feeding the polling path).
- Meet gogpu/ui's `EventSource` contract without stealing the engine's slots.

## Considered Options

1. **Give real EventSource to uiApp** (Architecture D) — rejected: overwrites
   engine backend; no consumed-out API; breaks latching tests.
2. **Engine forwards translated events into the ui tree manually
   (`uiApp.HandleEvent`)** — does not satisfy `app.WithEventSource`;
   requires mapping engine `iinput.KeyEvent` → `event.KeyEvent` and mouse →
   `gesture.PointerEvent`; feasible but bypasses the app's bridge (IME,
   text input, modifiers) — more mapping to maintain.
3. **Gateway EventSource shim** — chosen. The engine implements
   `gpucontext.EventSource` (plus `PointerEventSource`/`ScrollEventSource`
   where beneficial) backed by its own input pipeline, and passes it to
   `app.New(app.WithEventSource(gateway))`. The shim is the **only**
   registration on the underlying gogpu adapter (the engine's existing
   backend keeps those slots for legacy path 0); on path 1 the shim forwards
   events from the engine's authoritative `input.System` into the ui tree
   when a ui surface is active, and reflects unconsumed events back to the
   game (KeyDest still decides surface priority: console > menu > game).

## Decision Outcome

A gateway `gpucontext.EventSource` implementation in `internal/quakeui`
(`gateway_events.go`) that:
- is registered exactly once (the engine keeps its own backend registration
  for legacy path 0; the gateway replaces it when `ui_backend 1` is active —
  i.e. the *engine* still owns the physical adapter and feeds both paths);
- translates engine `iinput.KeyEvent` / char events / mouse moves into the
  `gpucontext` callback shapes the ui app bridge expects;
- routes per `KeyDest`: KeyConsole → console widget input; KeyMenu → menu
  pages; KeyGame → gameplay only (ui HUD listens for nothing interactive);
  CSQC_InputEvent still runs first for mods (unchanged);
- preserves modifiers/IME via the existing engine-side modifier tracking.

- **Positive:** KeyDest semantics intact; single authoritative input path;
  regression tests on path 0 unaffected; ui tree gets a standard EventSource.
- **Negative:** the shim is new code (~200-300 LOC) mapping two event
  systems; must be kept in sync with gogpu's gpucontext event API as it
  evolves.

## Links

- research 0004 §1, §3-C; ADR-0002; IRONWAIL-SPEC-001 §5.2-5.4, §7;
  gap log 7

## Review Log

### Stage 5 — Review 2 (2026-08-18)

Verdict: APPROVED WITH ADDITIONAL OPTION. Review found `App.HandleEvent(e
event.Event)` (app/app.go:191, documented "alternative to using an
EventSource") — engine could map iinput → ui event.Event directly and skip
implementing gpucontext.EventSource. Gateway shim retained (option 3) because
it uses gogpu/ui's own event_bridge (pointer/scroll/IME standardization,
W3C pointer events, touch-ready) instead of hand-mapping ui event types;
note in "negative" cost is that both worlds exist until path 0 dies. The
HandleEvent path stays a fallback if the gpucontext.EventSource contract
drifts. Consistent with spec §5.2-5.4; ADR-0002.
