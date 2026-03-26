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
- `CallDrawHud(...)` (returns whether CSQC reports that HUD draw occurred)
- CSQC entry-point execution paths through the wrapped VM

## Contracts

- `CSQC_DrawHud` is required for a load to succeed.
- CSQC uses a separate VM instance with its own function table, globals, and precache registries.
- Draw/client behavior depends on caller-supplied hook implementations.
- Runtime integration supports bootstrap-time CSQC load (`csprogs.dat` fallback `progs.dat`) plus optional `CSQC_Init` execution with Ironwail engine metadata.
- Global sync contract mirrors C timing split: `cltime` uses realtime while legacy `time` global remains client simulation time.
- `CallDrawHud` clears stale `OFSReturn` before invoking `CSQC_DrawHud` and treats the post-call return value (`OFSReturn != 0`) as the source of truth for whether CSQC actually drew HUD content.
