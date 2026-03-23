# Internals

## Logic

The SDL3 backend is the platform-facing implementation of the input contract. It initializes SDL joystick support, opens and tracks controllers, polls SDL events, maps keyboard scancodes/keycodes into engine keys, accumulates mouse deltas, emits text input, and turns mouse buttons/wheel and controller buttons into engine key events. It also polls gamepad axes directly when callers request `GamepadState`, applying deadzones/curves for sticks, trigger deadzones for analog triggers, and a transform layer that can shift certain controller buttons into `*_ALT` keys when the backend's alt modifier is engaged. Gyro support accumulates yaw/pitch deltas from raw sensor data after calibration, noise filtering, axis selection, sensitivity scaling, and mode gating.

## Constraints

- SDL3 support is build-tagged; the stub file must preserve compile-time availability when SDL3 is disabled.
- Trigger digital events are threshold-based, while analog trigger values remain separately available through `GamepadState`.
- Text mode, cursor mode, and on-screen keyboard hooks exist in the backend contract, but SDL3 currently implements some of them as no-ops.
- Device connection/removal bookkeeping is best-effort and tied to SDL's controller events rather than an engine-owned device model.

## Decisions

### Extend the SDL backend with modern controller and gyro features beyond classic Quake input

Observed decision:
- The SDL3 implementation goes beyond keyboard/mouse translation to include controller deadzones, alternate gamepad bindings, gyro modes, rumble, and LED commands.

Rationale:
- **unknown — inferred from code and tests, not confirmed by a developer**

Observed effect:
- The backend can support modern controller-centric play styles without changing the core input contract, but SDL-specific cvar/cmd coupling lives in this implementation rather than the backend-neutral core.
