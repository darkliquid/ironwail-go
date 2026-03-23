# Internals

## Logic

This subpackage is a deliberately thin wrapper around Go's `encoding/binary`, `math`, and `io` primitives. It exists to give the rest of the engine consistent names and return types for the little-endian and big-endian scalar conversions that Quake file formats and some protocol surfaces require.

## Constraints

- Slice-based helpers do not add bounds checks beyond what the underlying indexing requires.
- The package intentionally stops at scalar conversions rather than growing into a full binary-structure layer.

## Decisions

### Thin compatibility wrapper over standard-library binary primitives

Observed decision:
- The port exposes Quake-flavored endian helper names while delegating the real work to Go standard-library facilities.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Callers get familiar helper names with minimal maintenance cost, and binary semantics remain easy to audit.
