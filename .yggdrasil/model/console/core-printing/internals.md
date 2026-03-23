# Internals

## Logic

The console stores text in one flat byte slice sized as a ring of fixed-width lines. Printing writes bytes directly into that ring, line-feeding and wrapping as needed, stamps notify times for newly started lines, and mirrors output into optional callbacks/log files. Resize snapshots the old ring, rebuilds the buffer at a new width, and remaps recent notify lines so the on-screen overlay remains coherent after resolution changes.

## Constraints

- The ring buffer is coupled to `lineWidth`, so resize correctness is essential.
- High-bit color encoding and `[skipnotify]` handling are parity-sensitive console behaviors.
- `SafePrintf` currently behaves like `Printf`, which is a deliberate API-preservation choice but not full C parity.

## Decisions

### Fixed-width ring buffer for console text storage

Observed decision:
- Console text is stored as a fixed-width circular buffer rather than an append-only slice of strings.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Drawing and scrollback access remain cheap and Quake-like, but resize and wrapping semantics become a core maintenance concern.
