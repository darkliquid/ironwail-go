# Interface

## Main consumers

- host/client-facing runtime code that loads CSQC programs and invokes HUD or score draw entry points

## Main API

Observed surfaces:
- `NewCSQC()`
- `Load(...)`
- `IsLoaded()`
- `HasDrawScores()`
- `SyncGlobals(...)`
- CSQC entry-point execution paths through the wrapped VM

## Contracts

- `CSQC_DrawHud` is required for a load to succeed.
- CSQC uses a separate VM instance with its own function table, globals, and precache registries.
- Draw/client behavior depends on caller-supplied hook implementations.
- Near-term parity milestones do not currently require end-to-end host/client CSQC runtime integration; CSQC wrapper APIs are maintained as infrastructure for later scope expansion.
