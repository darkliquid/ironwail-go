# Internals

## Logic

The package centers on a mutex-protected map of lowercase cvar names to `*CVar`. Registration is idempotent, mutation updates the canonical string plus cached numeric views, and typed read helpers expose those cached conversions without repeated parsing. `Set` enforces flag policy, creates unknown vars implicitly when needed, unlocks before firing callbacks, and optionally triggers the global auto-cvar hook for QC synchronization. A package-global singleton preserves the classic flat cvar API used across the engine.

## Constraints

- String, float, and int views must remain synchronized on every successful mutation.
- Name canonicalization is part of the lookup contract.
- The current `FlagLatched` behavior is only a partial latch model: it suppresses callbacks but still mutates the live value immediately.
- Implicit creation on `Set` can diverge from later intended registration metadata (flags/default/description).

## Decisions

### Global cvar facade over a mutex-protected registry map

Observed decision:
- The Go port keeps a global cvar surface for easy engine-wide access while implementing the real registry as a small synchronized map-backed system.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Call sites stay concise and familiar to Quake-style code, while the underlying implementation is safer and easier to test than the original C global-list approach.
