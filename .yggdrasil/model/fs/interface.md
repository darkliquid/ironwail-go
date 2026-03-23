# Interface

## Main consumers

- startup/runtime code that constructs the canonical asset source for the engine.
- host/audio/world-loading paths that need raw bytes or fallback asset selection.
- tooling/tests that need mod discovery, file existence checks, or broad file enumeration.

## Main surface

- `NewFileSystem`
- `Init`, `AddGameDirectory`, `Close`
- `FindFile`, `LoadFile`, `FileExists`
- `FindFirstAvailable`, `LoadFirstAvailable`
- `LoadMapBSPAndLit`
- `ListMods`, `ListFiles`
- `GetGameDir`, `GetBaseDir`
- `SkipPath`, `StripExtension`, `GetExtension`, `AddExtension`, `DefaultExtension`, `FileBase`, `CreatePath`

## Contracts

- `Init` mounts `id1`, then optional `ironwail.pak`, then an optional non-`id1` game directory, and is a no-op once already initialized.
- Search resolution is priority-driven: later-added game directories override earlier ones, and within one directory higher-numbered PAKs outrank lower-numbered PAKs.
- `FindFirstAvailable` prefers higher-priority search paths before candidate-name order.
- `LoadMapBSPAndLit` only accepts a `.lit` sidecar when it comes from the same or higher-priority source than the chosen BSP.
