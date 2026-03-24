# Internals

## Logic

This node centralizes the dual-representation problem: every authoritative entity exists both as typed Go state and as flat QC VM memory. The sync helpers copy data between those representations at key boundaries so QC logic, physics, and networking all observe consistent values. It also caches field offsets and protocol-related knobs used throughout the rest of the package.

The server also exposes narrow bridge helpers that project QC VM internals to host policies without leaking VM ownership; `QCProfileResults(top)` is one such bridge and only returns counters when the server and QC VM are active.

## Constraints

- `EntVars` layout and QC field offsets are parity-critical.
- Freed edicts must clear QC-visible model/state surfaces so stale entities do not leak into later frames.
- QC execution context (`self`, `other`, time globals, spawned edict sync) must be preserved across builtin and callback paths.
- Server-level collaborators that introduce nondeterminism (such as network accept polling) should be injectable so protocol/session parity behaviors remain unit-testable.

## Decisions

### Typed Go wrappers around QC-backed server state

Observed decision:
- The Go port models server state with ordinary structs while keeping QC VM synchronization explicit.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Server-side gameplay code is easier to read and test than raw pointer arithmetic, but still remains constrained by the QC memory layout contract.
