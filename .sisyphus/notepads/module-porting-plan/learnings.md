
## internal/testutil
- Established `internal/testutil` for shared test helpers.
- `LocatePak0` checks `QUAKE_PAK0_PATH` environment variable and common relative paths (e.g., `id1/pak0.pak`).
- `SkipIfNoPak0` allows tests to gracefully skip if assets are missing, which is essential for CI environments where the full game data might not be available.
- `CompareStructs` provides a unified way to compare complex objects in tests, with hex dump support for byte slices.
Ported foundational math and string utilities from common.c and mathlib.c to internal/common and pkg/types.
Implemented COM_Parse, COM_CheckParm, path/extension utilities, and FNV-1a hash in internal/common.
Implemented Lerp, NormalizeAngle, AngleDifference, LerpAngle, VectorAngles, AngleVectors, and other math utilities in pkg/types.

## Porting wad.c and gl_texmgr.c texture parsing
- WAD2 files use a simple header and lump table.
- QPic format is basically width, height, and indexed pixels.
- MipTex format includes 4 mip levels and is used for world textures.
- Quake palette is 768 bytes (256 RGB entries).
- Index 255 is often used for transparency in Quake UI/HUD graphics.
- AlphaEdgeFix is used to prevent color bleeding from transparent pixels when using linear filtering.
- Go's image/png is a direct replacement for lodepng.

## BSP tree loading (gl_model.c -> internal/bsp/tree.go)
- The on-disk struct sizes in `bspfile.h` are critical: `dplane_t=20`, `dsnode_t=24`, `dl2node_t=44`, `dl2leaf_t=44`, and `dmodel_t=64`.
- Parsing BSP children must preserve Quake semantics: standard BSP uses `uint16` reinterpretation (`leaf = 65535 - child`), BSP2 uses bitwise complement of negative child indices.
- Loading order matters for validation parity with C path: faces -> marksurfaces -> leafs -> nodes lets node/leaf references be validated during load.

## Sprite loading (gl_model.c -> internal/model/sprite.go)
- `dspriteframetype_t` dispatch needs strict validation parity with C (`SPR_SINGLE`, `SPR_GROUP`, `SPR_ANGLED`; angled groups require exactly 8 frames).
- Group intervals must be strictly positive during decode (`interval > 0`) to match `Mod_LoadSpriteGroup` behavior.
- A robust integration test path is `progs/*.spr` from `pak0.pak` using `internal/fs` plus `testutil.SkipIfNoPak0`.

## Alias model loading (gl_model.c -> internal/model/alias.go)
- `Mod_LoadAliasModel` parsing order matters: skins first, then `stvert_t`, then `dtriangle_t`, then per-frame payloads.
- Alias frame groups contain repeated `daliasframe_t` blocks before each pose vertex block; preserving that layout is required for correct frame-group traversal.
- Quake computes alias bounds from decoded pose vertices (`scale` + `scale_origin`) and derives yaw-rotated and fully-rotated bounds from max squared radii.

## Server/world port (sv_main.c + world.c)
- A practical headless `SpawnServer` path can be validated without full QuakeC execution by loading `maps/<name>.bsp` through `internal/fs` and parsing it with `bsp.LoadTree`.
- `SV_LinkEdict` trigger behavior needs a two-pass approach (collect then execute) to avoid list mutation issues while touch callbacks run.
- Initializing brush hulls to invalid clipnode ranges (`FirstClipNode=-1`, `LastClipNode=-1`) is a safe fallback for map-load verification before full clipnode/hull conversion is implemented.

## Server physics port (sv_phys.c -> internal/server/physics.go)
- `SV_Physics_Toss` parity requires early return on `FL_ONGROUND`, angular velocity integration, bounce overbounce (`1.5`), and ground stop behavior (`normal.z > 0.7` with low z-velocity stop).
- `SV_Physics_Pusher` should use `ltime` + partial-frame movement (`movetime`) and run think only when `nextthink` crosses the new `ltime`.
- `SV_FlyMove` style sliding needs iterative clipping across multiple planes (`MAX_CLIP_PLANES`) to avoid getting stuck or tunneling through corners.
