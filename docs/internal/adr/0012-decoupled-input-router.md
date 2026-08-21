# ADR-0012: Decoupled Input Router — Engine Polls app.Input(), UI Owns EventSource

**Status:** Accepted (2026-08-21)
**Deciders:** darkliquid (v4 Stage 1: engine-polling), lifecycle driver
**Date:** 2026-08-21
**Supersedes:** ADR-0007 (v2/v3 KeyForwarder shim), ADR-0003 (v1 gateway)
**Related:** IRONWAIL-SPEC-004 §1.1/§4.2/§5.2/AC9; deep-research report §2.7;
gap log rows 5, 8, 12, 13

## Context and Problem Statement

Input routing between the engine and the gogpu/ui widget tree has been the
source of repeated bugs across all three branches:

- v1 built a full `gpucontext.EventSource` gateway shim (ADR-0003) that
  registered on the engine's EventSource and forwarded by KeyDest.
- v2/v3 used a `KeyForwarder` shim (ADR-0007) that called `app.HandleEvent`
  from the engine's key handlers.
- v3 additionally had to rewrite the engine's input backend because the old
  "any callback disables polling" heuristic double-delivered keys under the
  new path (split into three separate callback-seen gates).

The root cause: gogpu feeds every platform event to THREE sinks simultaneously
— window callbacks, the EventSource, AND the `Input()` polling state
(`dispatchKeyEvent` → `dispatchKeyToWindow` + `dispatchKeyToEventSource` +
`dispatchKeyToInputState`). Any design where two of these sinks are both
active for the same event risks double-delivery.

## Decision Drivers

- Preserve the engine's KeyDest router and latching/mouse-look semantics.
- Eliminate the double-delivery class of bugs (v3 input backend rewrite).
- The UI must receive menu/console input; the engine must receive gameplay
  input; HUD is non-interactive.
- Backtick/binding capture preserved.

## Considered Options

1. **Engine callback backend + UI via app.HandleEvent (v2/v3 KeyForwarder)** —
   rejected: the engine's callback backend and the ui both want the EventSource
   or the engine forwards manually; the two-arms design caused the v3
   double-delivery rewrite.
2. **Engine polls app.Input(); UI owns EventSource** — chosen. gogpu already
   feeds `app.Input()` from every platform event, so the engine gameplay path
   can poll it (Ebiten-style) with zero conflict. The UI owns the EventSource
   via `app.WithEventSource(gogpuApp.EventSource())`. A single router
   (`input_router.go`) is the policy point: per KeyDest, route to engine | UI.
   Pros: clean exclusive split, no double-delivery, gogpu-native. Cons:
   the engine's latching tests must migrate to the polling path (de-risked by
   an M0 spike).
3. **Full gpucontext.EventSource gateway shim (v1)** — rejected: the shim maps
   two event systems and must track gogpu's event API; the polling path is
   simpler and already fed.

## Decision Outcome

Decoupled input router. The engine gameplay path polls `gogpuApp.Input()`
(`Keyboard().Pressed/JustPressed`, `Mouse().Delta/Position/Scroll`) each frame.
The UI owns the EventSource via `app.WithEventSource`. `input_router.go` is the
single policy point with **exclusive key routing** (R1.2): KeyGame/HUD-only →
engine; KeyConsole/KeyMenu → UI only; KeyMessage → engine; backtick/binding
capture → engine pre-route. The engine does NOT also process console/menu keys
on path 1 (guards the v3 double-dispatch bug).

- **Positive:** exclusive split eliminates double-delivery; gogpu-native
  (polling state already fed); KeyDest router unchanged; HUD structurally
  non-interactive.
- **Negative:** engine latching tests must migrate to polling (M0 spike
  de-risks); the polling path loses per-event callback timing (edge semantics
  must be verified equal).

**EventSource ownership follows the startup path (A1, amended by G11):**
`ui_backend` is startup-only. When 0, the engine's input backend owns the
gogpu EventSource (as today) for the whole session; when 1, the UI owns it via
`app.WithEventSource`. The engine gameplay polling path (`app.Input()`) is
independent of EventSource ownership — gogpu feeds it from every platform
event regardless. No mid-session re-registration occurs (G11).

## Links

- IRONWAIL-SPEC-004 §1.1, §4.2, §5.2, AC9
- Deep-research report §2.7; gogpu `app.go` dispatchKeyEvent
- Gap log rows 5, 8, 12, 13

## Review Log

### Stage 5 — Review 2 (2026-08-21)

Verdict: APPROVED WITH ONE FIX. Supersedes ADR-0007/0003 honestly. The
exclusive-split design and the M0 spike de-risking are specific. Consistent
with SPEC-004 §4.2/§5.2 and AC9. Fix A1 applied: EventSource ownership follows
`ui_backend` (UI when 1, engine backend when 0); the polling path is
independent; the toggle re-registration is test-covered.
