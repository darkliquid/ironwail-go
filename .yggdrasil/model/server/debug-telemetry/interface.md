# Interface

## Main consumers

- `server/physics-core`, which emits touch/think/physics/frame events
- `server/client-session` and other server paths that want opt-in QC/debug visibility
- developers debugging parity and implementation issues

## Main surface

- debug telemetry cvar registration
- telemetry config and filtering helpers
- frame lifecycle hooks for batching/summarizing output
- QC trace helpers

## Contracts

- Telemetry is opt-in and should not change gameplay semantics when disabled.
- Filters by event kind, classname, and entity number are part of the operator-facing debug surface.
- Output batching/coalescing is intentional to keep high-volume traces usable.
