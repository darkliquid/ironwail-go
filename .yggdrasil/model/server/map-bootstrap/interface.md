# Interface

## Main consumers

- `host/runtime` and command paths that start or change maps
- `server/savegame`, which relies on bootstrap semantics before restore paths

## Main surface

- server initialization/reset entry points
- `SpawnServer`
- helper paths that parse map entities and prepare world/model state
- `SpawnServer` accepts map names with optional `maps/` prefix and optional `.bsp` suffix and normalizes to canonical basename before loading `maps/<name>.bsp`.
- entity-load spawn dispatch, including per-entity signon reservation before QC spawn calls

## Contracts

- World model/precache ordering is part of both gameplay and network protocol behavior.
- QC globals such as `serverflags` must be seeded before `worldspawn`/entity spawn runs, because mods read them during bootstrap.
- Entity spawn filtering must respect skill/deathmatch/co-op semantics before QC spawn logic runs.
- Each spawned map entity reserves 512 bytes of signon space before its QC spawn function, matching the C bootstrap guard against mid-spawn signon fragmentation.
- Touch QC is intentionally suppressed during entity loading and initial settle frames.
- Entity 0 (`worldspawn`) is a hard bootstrap invariant: if worldspawn parsing, classname resolution, spawn-function lookup, or spawn execution fails, map bootstrap must return an error immediately and must not free/reuse entity 0 for subsequent entities.
