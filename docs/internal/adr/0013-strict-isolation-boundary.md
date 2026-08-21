# ADR-0013: Strict Isolation Boundary — Hard AC7 for internal/quakeui

**Status:** Accepted (2026-08-21)
**Deciders:** darkliquid (v4 Stage 1: strict), lifecycle driver
**Date:** 2026-08-21
**Amends:** ADR-0009 (v2/v3 isolation — boundary was silently weakened in v3)
**Related:** IRONWAIL-SPEC-004 §3.1/§4.2/AC7; gap log rows 6, 14

## Context and Problem Statement

v2's ADR-0009 defined `internal/quakui` as self-contained (imports only
gogpu/ui + legacy state + image; never `internal/game` or `internal/renderer`),
with an import-closure test forbidding both. In v3 the boundary was silently
weakened: `internal/quakeui/overlay.go` imports `internal/renderer` (its
`DrawOverlay(target renderer.RenderContext, ...)` signature leaks the renderer
type), `internal/quakeui/hud/hud.go` imports `internal/renderer`, and the v3
`TestNoEngineImports` test only forbids `internal/game` — dropping the
`internal/renderer` check entirely.

## Decision Drivers

- Clean package boundaries (the user's explicit v2 requirement, carried to v4).
- The UI must be testable in isolation.
- The engine touches one function (the adapter), not UI internals.
- The UI needs legacy state machines read-only + engine services (cvars,
  command text, sound) + the gogpu GPU provider.

## Considered Options

1. **Strict boundary (hard AC7)** — chosen. `internal/quakeui` imports only
   gogpu/ui (incl. gg, gpucontext — gogpu ecosystem types), the legacy state
   machines (read-only via accessors), and `internal/image`. It NEVER imports
   `internal/game` or `internal/renderer`. The `quakeui.Host` adapter (defined
   in `internal/quakeui`) abstracts engine services as plain values /
   `func(string)` sinks; gogpu/gg types (`gg.Context`,
   `gpucontext.TextureView`, `gpucontext.DeviceProvider`) may cross the seam
   (they are gogpu ecosystem types, not engine internals — R1.4). Enforced by
   an import-closure test forbidden on BOTH `internal/game` and
   `internal/renderer`.
2. **Pragmatic (allow renderer types in UI)** — rejected: it is the v3 state
   that silently weakened the boundary and leaked the renderer's
   `RenderContext` into the UI package.
3. **Partial (strict for game, allow renderer)** — rejected: inconsistent and
   reintroduces the leak.

## Decision Outcome

Strict boundary restored and hardened. `internal/quakeui` never imports
`internal/game` or `internal/renderer`. The `quakeui.Host` adapter and
`OverlayRenderer.DrawOverlay(cc *gg.Context, ...)` may use gogpu/gg types.
The import-closure test (go list -deps) forbids both engine packages as a hard
AC7.

- **Positive:** UI testable in isolation; engine touches one function; the
  boundary is verified by CI-style tests; gogpu/gg types keep the adapter
  honest (no engine types leak).
- **Negative:** the adapter must abstract enough of the engine surface
  (cvars, command text, sound, GPU provider) without leaking engine types;
  some plumbing to route the GPU provider.

## Links

- IRONWAIL-SPEC-004 §3.1, §4.2, AC7
- ADR-0009 (amended); gap log rows 6, 14

## Review Log

### Stage 5 — Review 2 (2026-08-21)

Verdict: APPROVED. Amends ADR-0009 honestly (v3 weakened the boundary).
The gogpu/gg-types-allowed clarification (R1.4) is explicit. Consistent with
SPEC-004 AC7. No further findings.
