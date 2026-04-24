# Package: fs

## Purpose
The `fs` package implements the Quake Virtual File System (VFS). It abstracts the underlying storage (loose files on disk and `.pak` archives) into a unified, layered search-path system. This allows the engine to load assets like maps, textures, and sounds using virtual paths while supporting the complex override mechanisms required for Quake mods.

## Key Types & Interfaces
- **`FileSystem`**: The central manager for the VFS. It maintains the stack of search paths and handles file resolution.
- **`Pack`**: Represents an open `.pak` archive, containing a file handle and a list of internal file metadata.
- **`PackFile`**: Metadata for a single file within a `.pak`, including its name, offset, and length.
- **`SearchResult`**: A structure describing where a file was found (either on disk or in a `.pak`), providing the necessary information to read it.
- **`SearchPathEntry`**: A snapshot of a mounted search path, used for debugging and console commands (like the `path` command).

## Core Workflow
1. **Initialization**: The `FileSystem` is initialized with a base directory (usually `id1`). It then mounts any additional game directories (mods) or engine-specific `.pak` files.
2. **Mounting**: When a directory is added, the package automatically finds and mounts numbered `.pak` files (e.g., `pak0.pak`, `pak1.pak`) in ascending order.
3. **Resolution**: When a file is requested via `FindFile`, the system walks the search path stack from newest to oldest ("last-added-wins").
4. **Reading**: Files are read using `LoadFile` (for full reads) or `OpenFile` (for streaming). The package handles the transparency of whether the file is a loose disk file or a segment of a `.pak`.

## Integration
- **`host`**: Uses `fs` to initialize the game environment and locate the base game data.
- **`renderer`, `audio`, `qc`**: All asset-loading logic in the engine goes through the `FileSystem` to ensure that mod overrides are respected.
- **`draw`**: Loads UI images and WAD data using the VFS.

## Learning Tips
- **Override Logic**: Look at `AddGameDirectory` and how it prepends to `lookupPaths` to implement Quake's specific override rules.
- **Case Sensitivity**: Notice how the package handles case-insensitive lookups on case-sensitive filesystems (like Linux) by maintaining a "canonical" lookup name.
- **Path Sanitization**: Check `sanitizePath` and `isWithinRoot` for how the engine protects against directory traversal attacks, a critical security feature when handling external mod data.
- **Section Reading**: Examine `openSearchResult` to see how `io.NewSectionReader` is used to provide a standard `io.Reader` interface over a specific portion of a large `.pak` file.
