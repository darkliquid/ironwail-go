# Internals

## Logic

The bootstrap flow clears prior runtime state, loads BSP and `.lit` data, rebuilds the world model, reserves the world edict, precaches submodels, parses the entity lump, runs QC spawn functions for accepted entities, and then settles the map through initial frame processing before declaring the server active. It also prepares the signon/static-world state that later clients consume. Server initialization now resets dev-stats accumulators (`devStats`, `devPeak`) so map/session transitions begin with clean current/peak counters. A minimal map-check parity slice now lives in `sv_main`: `SV_MapCheckThresh` gates map-check diagnostics via `map_checks`/`developer`, while `SV_PrintMapCheck` and `SV_PrintMapChecklist` intentionally remain thin reporting helpers (telemetry-backed when debug telemetry is enabled, otherwise warning/local-client print fallback) until full C checklist semantics are ported.

Model bounds caching in `cacheModelInfo` now uses `FileSystem.OpenFile` streaming handles instead of eager `LoadFile` buffers. This keeps parser input on `io.Reader`/`io.ReadSeeker` streams for `.mdl`, `.spr`, and `.bsp`, and always closes the handle via defer even when parsing fails. Node-owned tests in `sv_main_test.go` pin both parsing parity against the legacy buffered path and close-on-error/close-on-success handle semantics.

## Constraints

- Bootstrap must leave the world/link state coherent before any client begins signon.
- Spawn ordering and precache indices are parity-sensitive with original Quake behavior.
- Bootstrap errors must fail map startup cleanly rather than leaving partially active state behind.
- Map-check helpers are intentionally stub-level for now; behavior-focused tests lock current no-op/reporting semantics to avoid accidental expansion before parity work.

## Decisions

### Two-phase map load with explicit world-model and entity bootstrap

Observed decision:
- The Go port separates BSP/world-model setup from later entity/QC bootstrap work.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Collision/world data is established before entity spawn logic depends on it, which reduces implicit ordering hazards.
