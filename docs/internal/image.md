# Package image

## Purpose

The `image` package is responsible for parsing Quake-specific image container formats and palette-indexed picture data. It handles the low-level decoding of assets used in the UI, console, and world textures, providing a bridge between raw disk data and standard RGBA buffers suitable for GPU upload.

## Key Types & Interfaces

- **`Wad`**: Represents a loaded WAD2 archive, containing a map of lumps keyed by their normalized names.
- **`Lump`**: A single entry in a WAD file, consisting of a name, a type (e.g., `TypQPic`, `TypMipTex`), and raw data bytes.
- **`QPic`**: A simple 2D picture format used for HUD and menu graphics, containing width, height, and palette-indexed pixels.
- **`MipTex`**: A more complex texture format that includes four mip-map levels, used for world geometry and some console assets.
- **`Palette`**: A 256-color lookup table that maps 8-bit indices to 32-bit RGBA colors.

## Core Workflow

1.  **Loading**: A WAD2 archive is loaded from an `io.ReaderAt` using `LoadWad`. The directory is read, and lumps are indexed in memory.
2.  **Parsing**: Specific assets are retrieved from the `Wad` and parsed into structured types like `QPic` (via `ParseQPic`) or `MipTex` (via `ParseMipTex`).
3.  **Conversion**: Since modern GPUs do not natively support paletted rendering, the `Palette.ToRGBA` method is used to expand 8-bit indices into full RGBA images.
4.  **Processing**: Optional fixes like `AlphaEdgeFix` are applied to prevent color bleeding artifacts on transparent edges before the texture is sent to the renderer.

## Integration

The `image` package serves as a low-level asset decoding layer. It is primarily consumed by:
- **`internal/draw`**: For UI and HUD element rendering.
- **`internal/renderer`**: For uploading world textures and sprites to the GPU.
- **`internal/bsp`**: For processing textures embedded in map files.

## Learning Tips

- **Palette Transparency**: In Quake, index 255 is traditionally treated as transparent. Note how `ToRGBA` handles the `transparent` flag.
- **Lump Name Normalization**: Look at `CleanupName` to see how Quake's 16-byte fixed-length names are converted to canonical Go strings.
- **Alpha Bleeding**: Examine `AlphaEdgeFix` to understand how the engine prevents "dark halos" around transparent textures through neighborhood averaging.

## Tests

### `image_test.go`

**`TestWad`** *(requires pak0.pak)* — Loads `gfx.wad` from the real game pak, verifies it has lumps, finds `conchars`, and parses a QPic lump. The WAD reader is the foundation for all 2D UI graphics; a parse error would corrupt menus and the HUD. Uses `testutil.SkipIfNoPak0`, initializes `fs.FileSystem`, then exercises `LoadWad` and `ParseQPic`.

**`TestPalette`** *(requires pak0.pak)* — Loads `gfx/palette.lmp` and verifies all entries have `alpha=255`. The Quake palette is the translation table for all indexed-colour textures; a corrupt load would miscolor the entire game. Calls `LoadPalette`, checks `pal[0].A`.

**`TestPNG`** — Encodes a 1×1 RGBA image in memory, then decodes it with `LoadPNG`, asserting 1×1 bounds. Screenshots and external texture overlays use PNG; a decode that breaks on trivial input is immediately visible. Uses `image/png` in-process; no file I/O needed.

**`TestAlphaEdgeFix`** — Places eight red opaque pixels in a 3×3 grid (leaving the center transparent), runs `AlphaEdgeFix`, then checks the center was filled with red. Transparent edges in atlas textures cause black fringing on bilinear filtering; the fix must propagate the nearest opaque colour. Directly manipulates `image.RGBA.Pix`, calls `AlphaEdgeFix`, reads back the center pixel.

**`TestDecodeTGA_UncompressedTrueColor`** — Builds a synthetic 18-byte TGA header for a 2×1 24-bit image, writes two BGR pixels (red, green), and verifies `DecodeTGA` returns the correct RGBA values. Many Quake texture replacements ship as TGA; incorrect BGR→RGB conversion or row-order mismatch would show swapped colours. Uses `bytes.Buffer` + `binary.LittleEndian` to build the header in-process.

**`TestDecodeTGA_RLETrueColor`** — Same as above but with a type-10 (RLE-compressed) TGA containing a run of two identical blue pixels. RLE-TGA decompression is a separate code path; bugs there would show garbage colours or a crash on the first RLE tile. Writes `0x81, 255, 0, 0` (run of 2 blue BGR pixels) after the header.

**`TestWritePNG_OrientationAndBPP`** — Tests that `WritePNG` flips rows when `upsidedown=false` and passes them through when `true`; also tests 24-bpp output. OpenGL framebuffers are stored bottom-up; screenshots written without flipping would appear mirrored vertically. Creates a 1×2 pixel buffer with red on top and blue below, writes with each flag, decodes back, checks top/bottom colours.

**`TestWritePNG_RejectsInvalidBufferOrBPP`** — Ensures `WritePNG` returns an error for unsupported bit depths (16-bpp) and for a buffer shorter than `w*h*(bpp/8)`. Silent truncation of a screenshot would produce a corrupted file without any indication. Two calls with deliberately bad arguments, checks `err != nil`.

**`TestWriteTGA_BPPOrientationAndChannelOrder`** — Validates `WriteTGA` byte order (BGRA for 32-bit, BGR for 24-bit), the top-origin descriptor byte, and the header `bpp` field. TGA files have BGR channel order; writing RGBA directly would corrupt colour on load. Compares raw bytes of the written TGA output against hard-coded expected byte sequences.

**`TestWriteTGA_RejectsInvalidBufferOrBPP`** — Same error-rejection pattern for unsupported 8-bpp and short buffer cases. Same reason — silent corruption of a TGA screenshot. Two bad calls, checks `err != nil`.

### `wad_test.go`

**`TestSubPic`** — Extracts a 2×2 sub-image from a 4×4 QPic at offset (1,1) and verifies the correct 4 pixels are returned. HUD and menu elements are sub-regions of atlas images; off-by-one in `SubPic` would display the wrong graphic slice. Fills a 4×4 QPic with sequential values 0–15, calls `SubPic(1,1,2,2)`, checks pixels `{5,6,9,10}`.

**`TestSubPicClamp`** — Requests a 5×5 sub-image starting at (3,3) of a 4×4 source, expects the result clamped to 1×1. Menu code may request a region that extends past the source boundary; clamping prevents out-of-bounds reads. Checks `sub.Width==1 && sub.Height==1`.

**`TestSubPicNegativeOrigin`** — Requests a sub-image with origin (−1,−1), expects a 2×2 result (negative part clamped to 0). Defensive against negative offsets that could arise from position arithmetic in the UI. Checks `sub.Width==2 && sub.Height==2`.

**`TestSubPicEmpty`** — Requests a sub-image with origin exactly at the border (4,4), expects 0×0. A completely out-of-bounds request should return an empty image, not crash. Checks `sub.Width==0 && sub.Height==0`.
