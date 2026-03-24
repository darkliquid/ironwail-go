# Internals

## Logic

The bootstrap flow clears prior runtime state, loads BSP and `.lit` data, rebuilds the world model, reserves the world edict, precaches submodels, parses the entity lump, runs QC spawn functions for accepted entities, and then settles the map through initial frame processing before declaring the server active. It also prepares the signon/static-world state that later clients consume. Server initialization now resets dev-stats accumulators (`devStats`, `devPeak`) so map/session transitions begin with clean current/peak counters.

## Constraints

- Bootstrap must leave the world/link state coherent before any client begins signon.
- Spawn ordering and precache indices are parity-sensitive with original Quake behavior.
- Bootstrap errors must fail map startup cleanly rather than leaving partially active state behind.

## Decisions

### Two-phase map load with explicit world-model and entity bootstrap

Observed decision:
- The Go port separates BSP/world-model setup from later entity/QC bootstrap work.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Collision/world data is established before entity spawn logic depends on it, which reduces implicit ordering hazards.
