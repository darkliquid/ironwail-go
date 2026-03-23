# Responsibility

## Purpose

`draw` owns the loading and caching of Quake's 2D UI assets: `gfx.wad` pictures, standalone `.lmp` pics, the shared palette, and raw `conchars` font data.

## Owns

- `gfx.wad` loading and parsed WAD lifetime.
- Palette acquisition and exposure.
- Lazy QPic lookup/caching across WAD, pak/filesystem, and loose-file fallback paths.
- Raw `conchars` data retrieval.
- Asset-manager lifecycle across init and shutdown.

## Does not own

- Actual drawing/compositing or GPU texture upload.
- HUD/menu/console layout logic.
- Player-color translation or generated fallback pics beyond loading raw source assets.
