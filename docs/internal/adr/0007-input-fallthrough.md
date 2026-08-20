# ADR-0007: Engine-Owned EventSource Shim Routing by KeyDest (Fallthrough)

**Status:** Accepted (2026-08-19)
**Deciders:** darkliquid (v2 Stage 1: fallthrough contract), lifecycle driver
**Date:** 2026-08-19
**Supersedes:** ADR-0003 (v1 gateway — abandoned with the engine-owned bridge)
**Related:** IRONWAIL-SPEC-002 §5.2/§7/AC9; research 0007; gap log rows 6, 10

## Context and Problem Statement

Under `desktop.Run` the ui app owns the EventSource. The engine's KeyDest
router (KeyGame/KeyConsole/KeyMenu) must decide what the ui consumes vs what
falls through to the game. The fallthrough contract (G10): menu and console
capture UI input; the HUD is non-interactive and input falls through to the
engine. The ui `Window.HandleEvent` returns void — there is no "consumed"
signal (research 0007 §2), so the engine must decide by KeyDest up front.

## Decision Drivers

- Preserve the engine's KeyDest router and latching/mouse-look tests
  (`game_movement_input_test.go:145-329`).
- The HUD must never capture input (structural guarantee).
- Backtick/binding capture preserved.
- Single-slot EventSource conflict: the engine's input backend and the ui app
  both want the gogpu EventSource (research 0007 §1).

## Considered Options

1. **Ui owns the EventSource and reports consumption** — rejected: there is
   no consumed-signal API (`Window.HandleEvent` is void); the engine cannot
   learn whether a widget swallowed an event (research 0007 §2).
2. **Engine-owned EventSource shim routing by KeyDest** — chosen. The engine
   keeps one authoritative EventSource registration (the shim in
   `internal/quakeui`). It routes per KeyDest:
   - `KeyDestGame`: route to engine input only (`g.handleGameKeyEvent`);
     gameplay latches run verbatim. No ui dispatch.
   - `KeyDestMenu`: route to `uiInput.ForwardKey` (M1.5). If unhandled,
     forward to engine (`g.handleMenuKeyEvent`).
   - `KeyDestConsole`: route to `uiInput.ForwardKey` (M1.5).

3. **EventSource Shim (M1.5)**:
   An engine-owned EventSource shim in `internal/quakeui` (the sole EventSource
   registration under `desktop.Run`) routes input by KeyDest: menu/console →
   ui; HUD-only/game → game (fallthrough). The HUD widget tree is draw-only
   (structural non-interactivity). The engine's KeyDest router and latching state
   machine are unchanged; the shim routes, it does not re-implement.

   Pros: KeyDest semantics intact; HUD structurally non-interactive; latching
   tests untouched; single authoritative input path. Cons: the shim is new
   code mapping two event systems; must track gogpu's event API.
4. **Hybrid (ui owns EventSource, engine polls unconsumed)** — rejected: no
   poll API exists; double-delivery risk; breaks latching.

## Decision Outcome

An engine-owned EventSource shim in `internal/quakui` (the sole EventSource
registration under `desktop.Run`) routes input by KeyDest: menu/console →
ui; HUD-only/game → game (fallthrough). The HUD widget tree is draw-only
(structural non-interactivity). The engine's KeyDest router and latching state
machine are unchanged; the shim routes, it does not re-implement.

- **Positive:** fallthrough contract satisfied; HUD never captures; latching
  tests intact; single authoritative input path.
- **Negative:** new mapping code; must stay in sync with gogpu's event API.

## Links

- IRONWAIL-SPEC-002 §5.2/§7/AC9; research 0007; ADR-0006 (desktop/GPUView),
  ADR-0009 (isolation); gap log 6, 10

## Review Log

### Stage 5 — Review 2 (2026-08-19)

Verdict: APPROVED. Supersedes ADR-0003 honestly. The no-consumed-signal
finding (research 0007 §2) is the decisive driver and is correctly stated.
Fallthrough is structural (HUD draw-only), not per-event. Consistent with
SPEC-002 §5.2 and AC9. No further findings.
