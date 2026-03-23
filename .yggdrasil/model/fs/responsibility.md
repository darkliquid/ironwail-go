# Responsibility

## Purpose

`fs` owns the Quake virtual filesystem used to mount base data, optional engine assets, and mod directories, then resolve virtual asset paths across loose files and `.pak` archives.

## Owns

- `FileSystem` lifecycle (`NewFileSystem`, `Init`, `Close`).
- Search-path mounting and override precedence.
- Numbered `pakN.pak` discovery and PAK directory parsing.
- Raw file lookup/loading for loose files and PAK-resident assets.
- Mod discovery, glob-style listing, and Quake-path helper utilities.
- Path sanitization and root-escape protection for VFS lookups.

## Does not own

- Parsing the contents of BSP/WAD/QC/model/sound/image files after the bytes are loaded.
- Renderer/server/host domain policy beyond filesystem search semantics.
- Rich archive validation, caching, or structured logging.
