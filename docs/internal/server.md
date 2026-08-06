# Package `server`

## Purpose
The `server` package is the authoritative core of the Ironwail-Go engine. It manages the entire game world simulation, including entity physics, collision detection, game rules, and player state. Even in single-player mode, the engine runs a local server to maintain consistency between the simulation and the view.

## Layout: root facade + sub-packages

The server has been decomposed into focused sub-packages over successive refactor passes. The root package is now a facade/session orchestrator; each sub-package owns a portable leaf that is unit-tested in isolation against narrow interfaces from `internal/server/types`.

| Sub-package | Owns |
| :--- | :--- |
| `types` | `Edict`, `Client`, `MessageBuffer`, `EntityState`, protocol constants, and the seam interfaces (`PhysicsFacade`, `FrameDriver`, `ClientThinker`, `CollisionWorld`, `EntityStore`, `CVarReader`). Sub-packages import this, never the root. |
| `physics` | `physics.System` — per-entity leaf algorithms (`FlyMove`, `PushMove`, `PhysicsWalk/Toss/Step/None/NoClip`, `Impact`, `RunThink`) plus the frame loop (`StepFrame`, which dispatches directly to the System's own leafs) and `ClientMover`. |
| `collision` | BSP model builders, hulls, areanodes, trace queries (`CollisionSystem`). |
| `net` | Wire encoding: entity delta encoding, `WriteClientData`, spawn signon writers. |
| `edict` | Edict allocation, map/savegame parsing, field accessors. |
| `state` | Session/signon state manager. |
| `commands` | Server console command parse/dispatch. |
| `debug` | Telemetry + `svdbg` logging. |
| `qc` | Server-side QuakeC hook helpers. |
| `savegame` | Save/load serialization. |

## Key Types & Interfaces
- **`Server`** (`server.go`): The main state for a running map, containing edicts, world geometry, physics settings, and network buffers. Holds injected subsystems (`PhysicsSys`, `CollisionSys`, `NetManager`) built in the constructor.
- **`ServerStatic`**: State that persists across level changes, such as the list of connected clients and server-wide flags.
- **`Edict`**: Represents a game entity. Fields are read/written through VM-backed accessors (`ent.Origin()` / `ent.SetOrigin()` etc.) defined in `internal/server/types`.
- **`Client`**: Tracks the state of a connected player, including their input commands (`UserCmd`), network connection, and synchronization state. Defined in `types` so sub-packages can reference it.
- **`PhysicsFacade`** (`types`): The narrow interface the physics leafs consume (QC callbacks, telemetry, cvar reads, sounds, scratch buffers). `*Server` implements it; tests mock it.

## Core Workflow
1. **World Loading**: `SpawnServer` loads a BSP map, initializes the physics world, and spawns the initial set of entities from the map's entity lump.
2. **Physics Frame**: `Server.Physics()` (`server_physics_loop.go`) is a thin delegator to `PhysicsSys.StepFrame`. The frame loop:
   - Runs the QC `StartFrame` function.
   - Per-edict movetype dispatch — calling the System's own leaf methods directly (no bounce through the root).
   - Integrates velocity, resolves collisions, executes think/touch functions, `SendInterval` bookkeeping, and `force_retouch` decay.
3. **Client Updates**: The server processes `UserCmd` packets from clients to move player entities and then broadcasts messages to all clients to keep them in sync.
4. **QuakeC Sync**: The engine synchronizes data between Go edict state and the QuakeC VM's memory at key boundaries.

## Integration
- **Host**: Orchestrates the server's lifecycle and connects it to the client.
- **QC**: The server owns and drives a `qc.VM` instance to run the game's logic.
- **Net**: Uses the `net` package to communicate with remote or local clients.
- **BSP**: Relies on `internal/bsp` for world geometry and collision hulls (wrapped by `collision`).

## Learning Tips
- **Physics code**: The leaf algorithms live in `internal/server/physics/leafs.go` as `System` methods; the frame loop is `internal/server/physics/stepframe.go`. The root `server_physics*.go` files hold only thin delegators whose remaining call sites (QC builtins, tests) reference the root surface.
- **Seam interfaces**: Read `internal/server/types/physicsfacade.go` and `internal/server/types/stepframe.go` — they define what the sub-packages may call back into the root for.
- **Spatial Partitioning**: Read `internal/server/collision/areanode.go` to understand how the area-node tree accelerates entity queries.
- **QC Builtins**: Examine `internal/server/server.go` for the Go implementations of functions that QuakeC scripts rely on, such as `traceline` and `spawn`.
- **Delegation design**: The recent refactors explicitly removed the `Server → PhysicsSys` bounce in the frame dispatch; when changing delegation, keep root methods only where the root surface is genuinely consumed, and prefer moving tests into the sub-package next to the code they test.
