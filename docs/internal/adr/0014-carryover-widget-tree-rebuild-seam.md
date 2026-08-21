# ADR-0014: Carryover of v2/v3 Widget Tree + Rebuild of Integration Seam

**Status:** Accepted (2026-08-21)
**Deciders:** darkliquid (v4 Stage 1: keep-widgets), lifecycle driver
**Date:** 2026-08-21
**Related:** IRONWAIL-SPEC-004 §1.2/§2#5/§3.1; research 0008 §4; gap log rows 3, 9, 15

## Context and Problem Statement

The three prior branches share a near-identical ~5,000-line widget tree
(menu/console/HUD roots, conchars atlas, LMP pic bridge, `ComputeMenuTransform`,
`Stack` container). The churn in all three was **entirely in the integration
seam** — how the widget tree is hosted, composited, and fed input. v4 should
not re-derive the widgets, and should not carry forward the failed seams.

## Decision Drivers

- Reuse the proven, tested widget presentation (visual fidelity, layout math).
- Rebuild only the fragile integration seam (Scenario A composite + decoupled
  input router) and add the demo bar.
- Discard the failed seams: v1 CPU canvas bridge, v2 desktop.Run+GPUView,
  v3 CPU readback + custom WGSL + reflection.

## Considered Options

1. **Keep widgets + Host adapter; rebuild integration** — chosen. Carry over:
   the v2/v3 widget tree (menu/console/HUD roots, conchars atlas, QPicToImage,
   `ComputeMenuTransform`), the `Host` adapter, the `ui_backend` gate, the
   `Stack` surface model, and the `menu.Manager` accessor surface. Rebuild:
   the overlay/composite path (ADR-0011) and the input path (ADR-0012); add
   `DemoBarRoot`.
2. **Keep only accessors + gate; rebuild widgets** — rejected: the widgets are
   proven, tested, and carry visual fidelity; rebuilding them is wasted effort.
3. **Port the v3 seam wholesale** — rejected: the v3 seam is precisely the
   thing that failed — CPU readback of the UI, a hand-written WGSL blit, and
   reflection into gogpu internals. Carrying it forward would enshrine the
   failure.
4. **Full rewrite (widgets too)** — rejected: no new requirements justify
   discarding the tested presentation layer; the widgets are proven, tested,
   and carry visual fidelity.

## Decision Outcome

Keep widgets + Host adapter; rebuild the integration seam. The v2/v3 widget
tree carries over; the seam is reconstructed per ADR-0011 (Scenario A) and
ADR-0012 (decoupled input router); `DemoBarRoot` is added (display-only,
R1.5).

- **Positive:** proven widgets reused; effort concentrated on the seam that
  actually failed three times; visual fidelity preserved.
- **Negative:** the carried widgets carry v2/v3 baggage (full-redraw model —
  accepted per G4; demo bar now needs a v4 host).

## Links

- IRONWAIL-SPEC-004 §1.2, §2#5, §3.1
- Research 0008 §4 (all three branches share the widget layer)
- Gap log rows 3, 9, 15

## Review Log

### Stage 5 — Review 2 (2026-08-21)

Verdict: APPROVED. The carryover scope is specific (widgets + adapter + gate +
Stack + accessors; rebuild seam; add DemoBarRoot). Consistent with SPEC-004
and research 0008 §4. No further findings.
