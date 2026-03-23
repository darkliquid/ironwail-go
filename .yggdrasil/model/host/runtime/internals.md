# Internals

## Logic

### Startup sequence

`Host.Init` applies parameters, clamps host-wide limits, resolves the user directory, initializes subsystems in a fixed order, then runs the startup config chain. The sequence is policy, not convenience: later steps assume earlier services already exist.

Server-browser network advertisement wiring (`updateServerBrowserNetworking`) now enables UDP listen before installing a `ServerInfoProvider`. If listen startup fails (accept socket cannot bind/open), host runtime clears provider state and keeps LAN advertisement disabled instead of exposing stale/partial server info.

### Frame scheduling

`Host.Frame` preserves the classic host order:
- gather events
- process console commands
- send client command
- advance server frame
- read from server
- update screen
- update audio

For loopback clients, send command construction now performs send-time one-shot input latching (attack/jump/impulse) through client runtime helpers, matching remote send semantics and C engine timing.

When `maxFPS` is above 72 or invalid, network simulation is isolated to a `1/72` interval. When `maxFPS` is at or below 72, simulation may run every frame.

### Shutdown

`Host.Shutdown` tears down client, server, console, commands, audio, input, renderer, then filesystem. This ordering mirrors dependency direction during runtime ownership.

## Constraints

- `maxClients` is clamped into `[1, MaxScoreboard]`.
- Dedicated mode and `deathmatch` policy are derived from init parameters.
- `userDir` must exist before host-managed config and save paths can be used.
- Abort state short-circuits the frame loop and later surfaces as a host-level error.

## Decisions

### Explicit host object and subsystem container

Observed decision:
- The Go port uses `Host` plus `Subsystems` instead of a broad global control block.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- runtime lifetime is explicit
- startup and shutdown order are centralized
- executable wiring is testable and replaceable
