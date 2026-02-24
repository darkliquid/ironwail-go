
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
