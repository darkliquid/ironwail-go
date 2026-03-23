# Responsibility

## Purpose

`server/foundation-qc-bridge` owns the core server-side state model and the boundary between typed Go gameplay state and the QuakeC VM memory model.

## Owns

- `Server`, `ServerStatic`, `Client`, `Edict`, and related protocol/type definitions.
- Edict allocation/free semantics and reserved-player-slot behavior.
- Go→QC and QC→Go synchronization helpers.
- QC builtin registration for the authoritative server runtime.

## Does not own

- Map bootstrap sequencing.
- Collision tracing and world linkage details.
- Per-frame movement/physics policy.
- Save/load serialization format.
