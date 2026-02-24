
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
