# ADR-0001: ui_backend Cvar Gate for the gogpu/ui Rewrite

**Status:** Accepted (2026-08-18)
**Deciders:** darkliquid (via Stage 1 elicit), lifecycle driver
**Date:** 2026-08-18
**Related:** IRONWAIL-SPEC-001 §1.3/§4.2/§5.1; research 0004

## Context and Problem Statement

The gogpu/ui rewrite is experimental and must not regress the parity-proven
legacy UI. We need a way to run both UI stacks side by side on the
`experiment/ui-rewrite` branch, compare them, and roll back cheaply if the
experiment fails.

## Decision Drivers

- Parity oracle: legacy path must stay byte-identical and testable.
- Cheap A/B: one switch, no rebuild.
- No build tags (AGENTS.md gotcha #1 forbids `//go:build` everywhere).
- Fail-open: if gogpu/ui init fails, the engine must still boot.

## Considered Options

1. **Build tags (`ui_gogpu`)** — rejected: AGENTS.md gotcha #1 is explicit
   ("Zero `.go` files contain `//go:build` directives"); two full builds to
   compare; CI matrix explosion.
2. **Feature branch only (no gate)** — rejected: losing the legacy path
   entirely on the branch means no in-branch A/B and every merge conflict
   with ongoing main work.
3. **`ui_backend` cvar 0|1, read per-frame** — chosen. Runtime switch, single
   binary, no build tags, fail-open (default 0).

## Decision Outcome

New cvar `ui_backend` (int, default 0) registered in `game_init.go`. Per
frame, `game.drawRuntimeOverlayFrame*` selects path: 0 = legacy (current
code), 1 = gogpu/ui host. Toggling mid-session re-creates the active
surfaces; legacy sources of truth (menu state, console buffer, hud.State)
persist, so no state corruption. Widget roots are keyed per surface so a
toggle rebuilds the view, not the model.

- **Positive:** single-binary A/B, zero build-tag debt, parity path always
  present, easy rollback (set cvar 0).
- **Negative:** dead legacy code stays in the binary (size), two draw paths
  must be maintained until the experiment concludes; per-frame cvar read has
  negligible cost but adds a branch in the hot overlay path.

## Links

- IRONWAIL-SPEC-001 §1.3, §5.1 (gate flow)
- Gap log rows 5, 12

## Review Log

### Stage 5 — Review 2 (2026-08-18)

Verdict: APPROVED. Options realistic (build tags rejected for AGENTS.md #1;
feature-branch-only rejected for A/B loss). Consistent with spec §1.3/§5.1.
Negative consequences specific (dead-code size, two paths). No changes.
