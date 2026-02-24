
## Test Utilities
- Created `internal/testutil` to centralize asset discovery and comparison logic.
- Decided to use `reflect.DeepEqual` for general comparison but added specialized hex dump output for byte slices to aid in debugging binary format issues.
- Chose to return the path from `SkipIfNoPak0` to allow tests to use it directly if they don't skip.

## Porting wad.c and gl_texmgr.c texture parsing
- Decided to use io.ReaderAt for LoadWad to allow flexible reading from memory or files.
- Decided to implement AlphaEdgeFix to maintain visual parity with the original engine.
- Decided to use standard image.RGBA for converted textures to integrate well with Go's image ecosystem.
