# Interface

## Main consumers

- runtime setup that selects SDL3 input when built with the `sdl3` tag.
- `input/core`, which talks to this implementation only through the `Backend` interface.

## Main surface

- `NewSDL3Backend`
- the `Backend` interface methods implemented by `sdl3Backend`
- build-tag stub fallback returning `nil` when `sdl3` is not enabled

## Contracts

- With `sdl3` disabled, `NewSDL3Backend` returns `nil` and the rest of the engine must tolerate that.
- SDL polling translates wheel motion into synthetic press/release key events and exposes mouse motion as accumulated per-frame deltas.
- Gamepad and gyro behavior is cvar-sensitive and includes modern extensions such as alternate button layers and rumble/LED commands.
