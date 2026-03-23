# Responsibility

## Purpose

`engine/collections` owns the lightweight generic collection helpers in `internal/engine`: a set for membership checks and a FIFO queue for single-owner buffered work.

## Owns

- `Set[T]` membership semantics.
- `Queue[T]` FIFO storage, wraparound, growth, and clearing behavior.
- The minimal collection helpers that replace repetitive `map[T]struct{}` and slice-shifting boilerplate.

## Does not own

- Thread-safe queueing or blocking channel semantics.
- Ordering guarantees for map-backed set iteration.
- Priority queues, bounded-drop policies, or richer collection families.
