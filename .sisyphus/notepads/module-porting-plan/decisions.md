
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
