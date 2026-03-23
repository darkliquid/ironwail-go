# Responsibility

## Purpose

The `game` node documents the intended top-level game-state consolidation layer for the Go port. It describes a future boundary where executable runtime state and orchestration currently spread across `cmd/ironwailgo` would live behind a single `Game` object.

Primary evidence:
- `internal/game/doc.go`

## Owns

- The architectural intent to consolidate subsystem references such as host, server, client, renderer, audio, input, menu, HUD, and draw state under one runtime object.
- The conceptual responsibility for per-frame update orchestration, entity collection, audio synchronization, input routing, camera/view computation, command registration, and demo helpers, as described by the package documentation.

## Does not own

- Any implemented runtime logic yet. There is no material source implementation in this package beyond the doc comment.
- The currently active executable orchestration, which still lives in `cmd/ironwailgo`.

## Boundaries

Today this node is architectural intent rather than implemented behavior. It should be treated as a placeholder for a future module boundary, not as an active runtime subsystem.
