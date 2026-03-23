# Responsibility

## Purpose

`server/map-bootstrap` owns the transition from an initialized-but-empty server into an active map simulation.

## Owns

- BSP/LIT loading for the active map.
- World model construction and collision-model preparation.
- Worldspawn/bootstrap edict initialization.
- Entity-lump parsing, spawn filtering, and QC spawn execution.
- Precache population and initial signon/static-world preparation.

## Does not own

- General edict/QC bridge primitives.
- Runtime collision queries after bootstrap.
- Ongoing per-frame movement/physics.
