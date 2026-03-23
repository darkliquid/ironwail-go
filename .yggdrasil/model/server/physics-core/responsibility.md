# Responsibility

## Purpose

`server/physics-core` owns per-frame authoritative simulation ordering and the shared physics/callback machinery that advances non-player and player gameplay state.

## Owns

- Frame sequencing for active server simulation.
- QC `StartFrame`, `think`, `touch`, and related callback dispatch.
- `PlayerPreThink`/`PlayerPostThink` QC wrapping for all client-slot entities, regardless of movetype (e.g. `MoveTypeNone` during intermission).
- Generic movetype physics helpers (gravity, clipping, fly/step logic).
- Rule enforcement that runs after simulation.
- Shared telemetry hooks around physics/touch execution.

## Does not own

- Player input interpretation and movement specifics.
- World tracing implementation details.
- Client signon/session management.
