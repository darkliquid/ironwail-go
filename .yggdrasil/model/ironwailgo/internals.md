# Internals

## Logic

`cmd/ironwailgo` is the application shell above the engine subsystems. Startup/bootstrap code configures the process, initializes files/QC/server/renderer/input/audio, and assembles `host.Subsystems`. Runtime-loop code drives the host frame, demo handling, client read/send ordering, and orderly shutdown. Input/command code owns key destinations, mouse grab, binds, and console/chat routing. View/camera code owns first-person/chase camera policy and shared view smoothing state. Entity and presentation helpers translate client/server state into renderer-facing models, particles, dynamic lights, HUD/menu overlays, and audio updates.

## Constraints

- This package is intentionally cross-cutting and therefore only understandable as a set of cooperating child nodes rather than one leaf node.
- Runtime ordering matters: camera, entity, render, and audio helpers consume state produced earlier in the host/client/server frame.
- Tests are broad and often assert policy or wiring decisions rather than isolated algorithms.

## Decisions

### Keep the composition root in `cmd/ironwailgo` and push reusable logic downward into `internal/`

Observed decision:
- The executable keeps the last-mile wiring and runtime orchestration in `cmd/ironwailgo` while reusable subsystem logic lives under `internal/`.

Rationale:
- **unknown — inferred from code and package role, not confirmed by a developer**

Observed effect:
- The command package becomes large and cross-cutting, but subsystem boundaries remain explicit and the graph can capture orchestration policies without polluting reusable packages.
