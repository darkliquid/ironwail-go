# Interface

## Main consumers

- host/runtime orchestration through registered frame callbacks.
- the app shell during loop selection and shutdown.

## Main surface

- `gameCallbacks`
- runtime loop helpers such as `headlessGameLoop`, `dedicatedGameLoop`, and `runRuntimeFrame`
- shutdown helpers

## Contracts

- Frame ordering is deliberate and affects later consumers like camera, entity, render, and audio updates.
- Demo playback follows a special path distinct from live networking.
- Shutdown is responsible for releasing initialized subsystems and persisting config/state where appropriate.
- Screenshot capture first attempts the active renderer export path and now falls back to software capture when renderer export is unavailable or errors, preserving screenshot command/flag behavior across backend states.
