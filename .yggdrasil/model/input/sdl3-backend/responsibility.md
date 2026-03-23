# Responsibility

## Purpose

`input/sdl3-backend` owns the optional SDL3 implementation of the input backend contract, translating SDL keyboard/mouse/controller events into the engine's input model and exposing SDL-specific controller/gyro capabilities.

## Owns

- `NewSDL3Backend` and the build-tag-selected stub behavior.
- SDL event polling and translation into engine key/text/mouse events.
- SDL keyboard scancode/keycode mapping.
- Gamepad button/axis processing, deadzones, trigger thresholds, alt-layer transformation, and connection tracking.
- Gyro filtering, calibration, mode handling, and command/cvar integration.

## Does not own

- The backend-neutral routing semantics in `System`.
- Menu/gameplay meaning of translated key events.
- Non-SDL renderer-owned input adapters elsewhere in the repo.
