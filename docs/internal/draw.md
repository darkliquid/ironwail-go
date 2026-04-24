# Package `draw`

## Purpose
The `draw` package is the 2D asset manager for the engine. It is responsible for loading, parsing, and caching 2D graphics such as HUD icons, menu backgrounds, status bar elements, and the console font. It bridges the gap between the filesystem (WAD/PAK files) and the high-level UI rendering code.

## Key Types & Interfaces

- **`Manager`**: The central struct that owns the loaded `gfx.wad` archive, a cache of decoded `QPic` objects, and the game's color palette.
- **`image.QPic`**: (Defined in `internal/image`) A struct representing a Quake-style 2D picture, containing width, height, and raw palette-indexed pixel data.
- **`image.Wad`**: (Defined in `internal/image`) Represents a WAD3 archive file, which is Quake's standard container for UI graphics.

## Core Workflow

1.  **Initialization**: The `Manager` is initialized with `Init`, which loads `gfx.wad` and `palette.lmp` from the virtual filesystem.
2.  **Asset Request**: Higher-level code calls `GetPic(name)`.
3.  **Cascading Lookup**: The manager searches for the asset in this order:
    1.  In-memory `pics` cache.
    2.  `gfx.wad` as a full path (e.g., `gfx/pause.lmp`).
    3.  `gfx.wad` as a bare name (e.g., `pause`).
    4.  The virtual filesystem (PAK files) as a standalone `.lmp` file.
    5.  The OS filesystem (if initialized from a directory).
4.  **Caching**: Once found and parsed, the `QPic` is stored in a map to ensure O(1) access for subsequent frames.

## Integration

- **`internal/fs`**: Uses the `FileSystem` to load the WAD and standalone lump files.
- **`internal/image`**: Relies on this package for the low-level parsing of WAD archives and QPic binary data.
- **`internal/console`**: Provides the raw pixel data for the `conchars` font used by the console and menu systems.
- **UI/HUD/Menu**: These high-level systems use `GetPic` to obtain the pixel data they need to buildtextured quads for rendering.

## Learning Tips

- **Palette-Indexed Graphics**: Note that this package deals almost exclusively with 8-bit palette indices, not RGBA pixels. The conversion to color happens later in the rendering pipeline using the `Palette()` provided by this manager.
- **WAD vs. PAK**: Understand the distinction between assets inside `gfx.wad` and standalone `.lmp` files in a PAK. The `Manager` abstracts this away, providing a unified interface for both.
- **Thread Safety**: Like the console and cvar systems, the `Manager` uses a `sync.RWMutex` to allow safe concurrent access from both the main engine thread and the rendering thread.
- **Conchars**: Look at `GetConcharsData`. This is a special 128x128 bitmap font that doesn't follow the normal `QPic` header format and is handled as a raw "lump".
