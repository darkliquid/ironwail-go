# Responsibility

## Purpose

`client/runtime` owns the persistent `Client` state and the shared runtime contracts that the host, parser, renderer, HUD, and audio layers consume.

## Owns

- Connection state, signon state, protocol flags, timing fields, view state, stats, precaches, entity maps, and transient event storage.
- The core `Client` type and its runtime-facing helpers.
- Shared error and telemetry types used to inspect client behavior.

## Does not own

- Detailed message decode mechanics.
- Input/button state evolution and prediction replay algorithms.
- Demo file IO details beyond the state fields exposed on `Client`.
