# Internals

## Logic

The package centers on a `Manager` that owns a parsed WAD pointer, optional virtual-filesystem or base-directory fallback source, a cache of already-parsed `QPic` assets, and shared palette/font data. Initialization loads `gfx.wad`, extracts palette data, and marks the manager ready. Asset lookup first checks the cache, then resolves by WAD full name, WAD bare normalized name, pak/filesystem lookup, and finally direct directory lookup, caching successful `QPic` parses so repeated HUD/menu/console draws avoid reparsing binary data.

## Constraints

- Palette data must be at least 768 bytes.
- `GetPic` only accepts WAD lumps of `TypQPic` or `TypConsolePic`; `conchars` follows a separate raw-data path.
- Returned palette and `conchars` slices alias manager-owned backing data.
- Cache keys are the requested names, so multiple equivalent lookup names can occupy distinct cache entries.
- `IsPicCached` is cache-state only and must not trigger image loading.

## Decisions

### Separate 2D asset loading from rendering backends

Observed decision:
- The Go port factors Quake 2D asset loading/caching into a dedicated package instead of keeping it inside renderer-specific draw code.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- HUD/menu/console code can share one asset-loading layer across render backends, while renderer packages only need the decoded palette/pic/font data.
