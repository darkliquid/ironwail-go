# ADR-0015: Demo Bar in Scope — Display-Only DemoBarRoot

**Status:** Accepted (2026-08-21)
**Deciders:** darkliquid (v4 Stage 1: all-plus-demo), lifecycle driver
**Date:** 2026-08-21
**Related:** IRONWAIL-SPEC-004 §1.2/§5.4/AC11; research 0001 §7; gap log rows 9, 15

## Context and Problem Statement

The demo playback progress bar was deferred in all three prior branches
(stays legacy; bd `ironwail-go-cuy` owns interactive scrubbing). The user's
v4 scope decision (G6: all-plus-demo) brings it into MVP. The legacy demo bar
is **display-only** — research 0001 §7 confirms: "Not clickable/draggable; no
mouse interaction with the demo bar."

## Decision Drivers

- All four UI surfaces in v4 MVP (menu, console, HUD, demo bar).
- The demo bar must render identically to the legacy one on path 1.
- Interactive scrubbing stays in bd `ironwail-go-cuy` (out of scope).

## Considered Options

1. **Display-only DemoBarRoot** — chosen. Renders the playback progress bar
   (38-char track, cursor, status glyph, speed label, demo name, M:SS readout)
   mirroring legacy `drawRuntimeDemoControls` (research 0001 §7). No mouse
   interaction.
2. **Interactive scrubbing in MVP** — rejected: pre-existing bd work item
   (`ironwail-go-cuy`) owns it; scope creep.
3. **Defer (stay legacy)** — rejected against the user's G6 decision.

## Decision Outcome

`DemoBarRoot` (display-only) is in v4 MVP and renders on path 1. Interactive
scrubbing stays in bd `ironwail-go-cuy`.

- **Positive:** all four surfaces in MVP; legacy parity for the bar.
- **Negative:** the bar's animation (timebar cursor) needs the per-frame
  redraw model — already accepted per G4.

## Links

- IRONWAIL-SPEC-004 §1.2, §5.4, AC11
- Research 0001 §7 (demo bar display-only); gap log rows 9, 15

## Review Log

### Stage 5 — Review 2 (2026-08-21)

Verdict: APPROVED. Display-only scope pinned against research 0001 §7;
scrubbing stays in bd cuy. Consistent with SPEC-004 AC11. No further findings.
