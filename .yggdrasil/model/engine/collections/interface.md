# Interface

## Main consumers

- Callers that need simple membership checks without repeating raw map boilerplate.
- Single-threaded owners that need FIFO buffering for bursty command/event style workloads.

## Main surface

- `NewSet`, `Add`, `Has`, `Remove`, `Len`, `Slice`, `Range`
- `NewQueue`, `Push`, `Pop`, `Peek`, `Len`, `Clear`

## Contracts

- `Set` membership is O(1) average-case and iteration order is arbitrary.
- `Queue` preserves FIFO order and grows instead of dropping entries when full.
- `Queue` is explicitly not thread-safe.
