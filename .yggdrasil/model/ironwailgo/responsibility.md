# Responsibility

## Purpose

`ironwailgo` owns the executable composition root and runtime shell for the engine. It is where subsystems are wired together, startup options are applied, the host frame loop is driven, and client/server/render/input/audio state is translated into a running game process.

## Owns

- The `cmd/ironwailgo` package split between bootstrapping, runtime orchestration, camera/view policy, input routing, debug view telemetry, entity collection, and presentation helpers.
- Process-wide `Game` state as the mutable application shell that ties otherwise-separate subsystems together.
- The top-level executable concerns that do not belong to reusable `internal/` packages.

## Does not own

- The internal subsystem implementations themselves (`host`, `client`, `server`, `renderer`, `menu`, `hud`, etc.).
- Reusable engine/library contracts that already live under `internal/` or `pkg/`.
