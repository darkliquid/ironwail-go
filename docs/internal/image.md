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
