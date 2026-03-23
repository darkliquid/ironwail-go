# Responsibility

## Purpose

`console/core-printing` owns the main console state model: scrollback storage, resize behavior, printing, notify timestamps, dumping, and debug-log lifecycle.

## Owns

- `Console` core fields and initialization.
- Scrollback ring buffer layout and resize/clear/dump behavior.
- `Printf`/`DPrintf`/`Warning`/`SafePrintf`/`CenterPrintf` output paths.
- Notify timestamp tracking and debug-log file management.
- Package-global singleton accessors for the console core.

## Does not own

- Input editing/history logic.
- Completion provider/session logic.
- Rendering implementation details.
