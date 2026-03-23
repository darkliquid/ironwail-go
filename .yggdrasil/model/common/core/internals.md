# Internals

## Logic

The `core` node mixes several categories of low-level helpers because they all survive from the same compatibility-oriented subset of the old Quake common layer. `SizeBuf` tracks current size and read position over a single backing slice, `COM_Parse` and friends preserve the original text-tokenizing style with global token state, and path/hash helpers provide small reusable utilities without introducing new package dependencies. `Link` and `BitArray` remain tiny allocation-free helpers for subsystems that want intrusive lists or packed-bit state.

## Constraints

- `ComToken` and argv globals create deliberate shared mutable state.
- `SizeBuf.WriteAngle` and `WriteAngle16` encode angles with simple truncation, which is a parity-sensitive detail callers must understand.
- Some helpers appear lightly used or future-facing, so their long-term boundary rationale is not fully proven by current call sites.

## Decisions

### Preserve C-style helpers where compatibility outweighs idiomatic redesign

Observed decision:
- The port keeps several global/stateful utility surfaces (`ComToken`, argv checks, intrusive links) instead of rewriting everything into stateless abstractions.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Existing Quake-style parsing and utility call patterns remain easy to port, but the package retains some non-idiomatic shared state and mixed concerns.
