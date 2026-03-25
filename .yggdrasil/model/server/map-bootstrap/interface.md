# Interface

## Main consumers

- `host/runtime` and command paths that start or change maps
- `server/savegame`, which relies on bootstrap semantics before restore paths

## Main surface

- server initialization/reset entry points
- `SpawnServer`
- helper paths that parse map entities and prepare world/model state
- `SpawnServer` accepts map names with optional `maps/` prefix and optional `.bsp` suffix and normalizes to canonical basename before loading `maps/<name>.bsp`.

## Contracts

- World model/precache ordering is part of both gameplay and network protocol behavior.
- Entity spawn filtering must respect skill/deathmatch/co-op semantics before QC spawn logic runs.
- Touch QC is intentionally suppressed during entity loading and initial settle frames.
