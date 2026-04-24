# Package `server`

## Purpose
The `server` package is the authoritative core of the Ironwail-Go engine. It manages the entire game world simulation, including entity physics, collision detection, game rules, and player state. Even in single-player mode, the engine runs a local server to maintain consistency between the simulation and the view.

## Key Types & Interfaces
- **`Server`**: The main state for a running map, containing edicts, world geometry, physics settings, and network buffers.
- **`ServerStatic`**: State that persists across level changes, such as the list of connected clients and server-wide flags.
- **`Edict`**: Represents a game entity. It contains both engine-side state (like collision hulls) and a pointer to QuakeC-accessible `EntVars`.
- **`Client`**: Tracks the state of a connected player, including their input commands (`UserCmd`), network connection, and synchronization state.
- **`AreaNode`**: A node in the spatial partitioning tree used to quickly find entities in a specific area for collision or visibility checks.

## Core Workflow
1. **World Loading**: `SpawnServer` loads a BSP map, initializes the physics world, and spawns the initial set of entities from the map's entity lump.
2. **Physics Loop**: `SV_Physics` (in `physics.go`) is the heart of the server. Every frame, it:
   - Integrates velocity into position for all active entities.
   - Resolves collisions against the world BSP and other entities.
   - Executes QuakeC "think" functions on schedule.
3. **Client Updates**: The server processes `UserCmd` packets from clients to move player entities and then broadcasts `svc_update` messages to all clients to keep them in sync.
4. **QuakeC Sync**: The engine synchronizes data between the Go-native `Edict` structs and the QuakeC VM's memory at key boundaries.

## Integration
- **Host**: Orchestrates the server's lifecycle and connects it to the client.
- **QC**: The server owns and drives a `qc.VM` instance to run the game's logic.
- **Net**: Uses the `net` package to communicate with remote or local clients.
- **BSP**: Relies on `internal/bsp` for world geometry and collision hulls.

## Learning Tips
- **Physics Dispatch**: Check `internal/server/physics.go` to see how the engine dispatches to different physics handlers based on an entity's `MoveType`.
- **Spatial Partitioning**: Read `internal/server/world.go` to understand how the `AreaNode` tree accelerates entity queries.
- **QC Builtins**: Examine `internal/server/server.go` for the Go implementations of functions that QuakeC scripts rely on, such as `traceline` and `spawn`.
