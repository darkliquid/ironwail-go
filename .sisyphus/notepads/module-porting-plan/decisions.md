
## Test Utilities
- Created `internal/testutil` to centralize asset discovery and comparison logic.
- Decided to use `reflect.DeepEqual` for general comparison but added specialized hex dump output for byte slices to aid in debugging binary format issues.
- Chose to return the path from `SkipIfNoPak0` to allow tests to use it directly if they don't skip.

## Porting wad.c and gl_texmgr.c texture parsing
- Decided to use io.ReaderAt for LoadWad to allow flexible reading from memory or files.
- Decided to implement AlphaEdgeFix to maintain visual parity with the original engine.
- Decided to use standard image.RGBA for converted textures to integrate well with Go's image ecosystem.

## BSP tree loading
- Added a dedicated `LoadTree` path in `internal/bsp/tree.go` instead of extending the existing generic loader, to keep a focused port of `Mod_Load*` BSP tree routines and enable strict lump-level validation.
- Normalized version-specific disk formats (`ds*`, `dl1*`, `dl2*`) into shared in-memory `Tree*` structs so tests can assert geometry invariants uniformly across BSP variants.

## Alias model loading
- Added `LoadAliasModel(io.ReadSeeker)` returning `*Model` so alias loading now produces both `AliasHeader` metadata and Quake-style model bounds (`mins/maxs`, `ymins/ymaxs`, `rmins/rmaxs`) in one pass.
- Kept existing `mdl.go` API intact and introduced the `alias.go` loader as the `Mod_LoadAliasModel`-style path to avoid breaking existing callers while enabling model-level tests.

## Server/world port scope
- Added `Server.Init`, `Server.SpawnServer`, and `Server.Shutdown` in `internal/server/sv_main.go` as the minimum headless lifecycle needed for `map <name>` startup tests.
- Used `bsp.LoadTree` to build a minimal `*model.Model` world representation sufficient for `SV_ClearWorld`/`SV_LinkEdict` map-start flow, deferring full BSP clip hull conversion to later server physics tasks.

## Server physics port scope
- Kept frame iteration in `physics_loop.go` and moved/expanded `sv_phys.c` behavior into `physics.go` helpers and per-movetype handlers (`PhysicsToss`, `PhysicsStep`, `PhysicsPusher`, `PushMove`, `PushEntity`, `FlyMove`).
- Added `physics_test.go` with deterministic unit coverage for core physics handlers plus a pak-aware integration smoke test (`SkipIfNoPak0`) that runs one server physics frame on `start`.

## Server movement port scope
- Added `internal/server/movement.go` with direct ports of `SV_CheckBottom`, `SV_movestep`, `SV_StepDirection`, `SV_NewChaseDir`, `SV_CloseEnough`, and `SV_MoveToGoal`, while keeping existing world collision primitives in `world.go` as the authoritative tracing backend.
- Introduced explicit `SV_Move`, `SV_HullForEntity`, and `SV_TestEntityPosition` wrapper methods to satisfy sv_move.c naming parity without duplicating collision implementation.
- Added `internal/server/movement_test.go` with unit checks plus a pak-aware spawned-map movement smoke test that dynamically samples a walkable point to avoid brittle map-coordinate fixtures.

## Server user port scope
- Replaced `internal/server/sv_user.go` with `internal/server/user.go` and centered the implementation around C-parity entry points (`SV_ClientThink`, `SV_ReadClientMessage`, `SV_ExecuteUserCommand`) plus compatibility wrappers used by existing server flow.
- Added `internal/server/user_test.go` with command whitelist checks, move-message decoding checks, noclip think behavior, and a pak-aware spawned-map `RunClients` smoke test using `testutil.SkipIfNoPak0`.
- Decided to use interfaces for subsystems in the Host struct to facilitate mocking and testing.
- Decided to register host commands in a separate method called during initialization.
- Decided to implement most host commands as stubs for now, as they require deeper integration with server/client internals that are still being ported.

## Network Porting Decisions
- Renamed loopback functions in `loopback.go` to have a `Loopback` suffix to avoid name collisions with the high-level API.
- Used `stdnet "net"` as an alias for the standard library `net` package to avoid conflict with our `net` package.
- Implemented a simplified `Connect` and `CheckNewConnections` that handles the basic Quake connection handshake (CCREQ_CONNECT / CCREP_ACCEPT).

## Client port scope decisions
- Replaced the previous minimal `internal/client/client.go` implementation with a structured split across `client.go`, `parse.go`, `input.go`, and `demo.go` to mirror the source module boundaries (`cl_main`, `cl_parse`, `cl_input`, `cl_demo`).
- Implemented a focused `Parser` that covers sign-on path commands first (`svc_serverinfo`, `svc_signonnum`, `svc_setview`, `svc_cdtrack`) and returns explicit errors on unsupported message opcodes to keep the port fail-fast while incomplete.
- Added `LerpPoint` and `kbutton` impulse-driven movement assembly (`AdjustAngles`, `BaseMove`, `AccumulateCmd`) as the initial prediction/input baseline rather than deferring all client movement behavior.

## Audio Porting Decisions
- Moved spatialization logic to a separate file `spatial.go` as requested, but kept it as a method on `System` for easier access to state.
- Added `viewEntity` to `System` to handle full-volume sounds from the player.
- Implemented the software mixer with support for 8-bit and 16-bit sounds, and 16-bit stereo output.

## Renderer core init scope decisions
- Added a dedicated `Core` type in `internal/renderer/core.go` for headless WebGPU initialization instead of changing the existing windowed `Renderer` API in `renderer.go`.
- Implemented backend support as pure-Go only (`BackendGo/BackendAuto`) for this stage, returning a deterministic error for Rust backend selection.
- Added `renderer_test.go` coverage that validates unsupported-backend behavior and runs headless init with environment-based skip fallback when no GPU stack is available.

## Renderer surface/model scope decisions
- Implemented `internal/renderer/surface.go` as CPU-side algorithm parity helpers (texture animation, chart/lightmap allocation, and lightmap sample packing) without binding to GPU upload state yet.
- Implemented `internal/renderer/model.go` as alias interpolation + batching primitives with explicit Quake-style lerp flags, keeping rendering backend concerns decoupled from state setup logic.
- Added focused renderer tests in `internal/renderer/surface_test.go` (surface + model behavior) to keep verification within the renderer package while this module is still headless-first.

## Renderer particle scope decisions
- Implemented `internal/renderer/particle.go` as CPU-side parity helpers for particle allocation, simulation (`CL_RunParticles`), effect generation (`R_RunParticleEffect`, `R_RocketTrail`), and draw-prep utilities (draw-pass filtering, projection scaling, vertex color packing).
- Kept rendering backend wiring out of scope for this task and exposed deterministic helpers/tests in `internal/renderer/particle_test.go` to validate particle behavior independently from GPU availability.


## Renderer screen scope decisions
- Implemented `internal/renderer/screen.go` as headless, testable screen-state helpers instead of wiring directly into live draw calls, matching the existing renderer port style for incremental parity.
- Added `screen_test.go` coverage for `SCR_UpdateZoom`, `AdaptFovx`/`CalcFovy`, `SCR_CalcRefdef`-style layout math, and `SCR_TileClear` rectangle generation/skip logic.
