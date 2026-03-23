# Interface

## Main consumers

- startup/runtime wiring that initializes draw and passes palette/font data onward.
- `hud`, `menu`, and other UI packages that request cached `QPic` assets by name.
- renderer setup that consumes palette and `conchars` bytes.

## Main surface

- `NewManager`
- `Init` / `InitFromDir`
- `GetPic`
- `GetConcharsData`
- `Palette`
- `Shutdown`

## Contracts

- `GetPic` returns cached or lazily loaded `QPic` data, or `nil` if the manager is uninitialized or the asset cannot be resolved.
- Palette data is expected to be the first 768 bytes of Quake palette content.
- `conchars` is exposed as raw 128×128 indexed pixel bytes, not as a `QPic`.
- Initialization is idempotent once successful; callers should `Shutdown` before switching asset bases.
