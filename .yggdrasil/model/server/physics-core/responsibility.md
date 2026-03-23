# Responsibility

## Purpose

`server/physics-core` owns per-frame authoritative simulation ordering and the shared physics/callback machinery that advances non-player gameplay state.

## Owns

- frame sequencing for active server simulation
- QC `StartFrame`, `think`, `touch`, and related callback dispatch
- generic movetype physics helpers (gravity, clipping, fly/step logic)
- rule enforcement that runs after simulation
- shared telemetry hooks around physics/touch execution

## Does not own

- Player input interpretation and movement specifics.
- World tracing implementation details.
- Client signon/session management.
