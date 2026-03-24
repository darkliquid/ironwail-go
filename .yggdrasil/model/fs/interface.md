# Interface

## Main consumers

- startup/runtime code that constructs the canonical asset source for the engine.
- host/audio/world-loading paths that need raw bytes or fallback asset selection.
- tooling/tests that need mod discovery, file existence checks, or broad file enumeration.

## Main surface

- `NewFileSystem`
- `Init`, `AddGameDirectory`, `Close`
- `SearchPathEntries`
- `FindFile`, `LoadFile`, `OpenFile`, `FileExists`
- `FindFirstAvailable`, `LoadFirstAvailable`
- `LoadMapBSPAndLit`
- `ListMods`, `ListFiles`
- `GetGameDir`, `GetBaseDir`
- `SkipPath`, `StripExtension`, `GetExtension`, `AddExtension`, `DefaultExtension`, `FileBase`, `CreatePath`

## Contracts

- `Init` mounts `id1`, then optional `ironwail.pak`, then an optional non-`id1` game directory, and is a no-op once already initialized.
- `SearchPathEntries` snapshots the current lookup stack in the same front-to-back order used for file resolution, reporting pack file counts so debug commands can mirror Quake's `path` output without reaching into private fields.
- Search resolution is priority-driven: later-added game directories override earlier ones, and within one directory higher-numbered PAKs outrank lower-numbered PAKs.
- `FindFirstAvailable` prefers higher-priority search paths before candidate-name order.
- `LoadMapBSPAndLit` only accepts a `.lit` sidecar when it comes from the same or higher-priority source than the chosen BSP.
- `OpenFile` returns a read/seek handle and byte length for the resolved VFS file: loose files are opened as OS files, and pack files are exposed via section readers constrained to the entry byte range.
