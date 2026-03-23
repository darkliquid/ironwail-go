# Interface

## Main consumers

- `host`, which advances the compatibility stream each frame for parity.
- `server`, especially movement/chase logic that depends on Quake-compatible random branches.
- `qc`, which derives builtin `random()` values from the same stream.

## Main surface

- `New` / `NewSeed`
- `(*RNG).Seed`
- `(*RNG).Int31` / `(*RNG).Int`
- package-global `Shared`, `ResetShared`, `Int31`, and `Int`

## Contracts

- Seed `0` must behave like seed `1`.
- The generated sequence must stay compatible with the expected libc/glibc-style stream used for gameplay parity.
- Extra or missing draws from the shared authoritative stream can change gameplay behavior.
