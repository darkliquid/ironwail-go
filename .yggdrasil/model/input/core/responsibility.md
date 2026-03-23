# Responsibility

## Purpose

`input/core` owns the stable engine-facing input model: Quake-compatible key codes, key-name conversion, bindings, key-destination routing, frame input state, and the backend interface used by the rest of the engine.

## Owns

- Key constants and engine key vocabulary.
- `System`, `Backend`, `InputState`, `ModifierState`, `GamepadState`, and related enums.
- Binding storage and bind-name conversion (`KeyToString`, `StringToKey`).
- Key and character event routing, callback dispatch, and destination-aware text-mode switching.
- Delegation points for mouse grab, cursor mode, text mode, and gamepad state.

## Does not own

- SDL3 event polling details, gyro processing, or gamepad-device bookkeeping.
- The meaning of individual bindings or callbacks once they reach gameplay/menu/console consumers.
