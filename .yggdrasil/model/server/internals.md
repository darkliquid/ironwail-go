# Internals

## Logic

The package is structured around one authoritative `Server` instance per active map. That instance owns edicts, client slots, signon/datagram buffers, collision state, and the QC VM bridge. Map bootstrap loads BSP data, rebuilds world state, instantiates entities through QC spawn functions, settles initial physics, and prepares signon state for clients. Each active frame then follows the Quake ordering of client command ingestion, physics, rules, and message emission.

## Constraints

- Edict numbering is semantic: edict `0` is worldspawn and player slots occupy the reserved range immediately after it.
- `EntVars`/QC field layout is parity-critical with `progs.dat` and must not drift.
- Touch/impact/think callbacks must preserve QC execution context and synchronize spawned/mutated edicts back into Go state.
- Save/load portability depends on converting QC string handles to text and allocating fresh handles on restore.

## Decisions

### Split server persistence by gameplay concern

Observed decision:
- The package is best represented as focused child nodes rather than one giant module artifact.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- QC bridging, map bootstrap, world collision, physics, networking, save/load, and debug tooling each have distinct contracts and failure modes that are easier to track independently.
