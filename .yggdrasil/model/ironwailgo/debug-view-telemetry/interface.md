# Interface

## Main consumers

- view/camera/entity helpers in the command package that emit targeted diagnostics.
- tests verifying emitted telemetry behavior.

## Main surface

- debug cvar registration and enabled-level helpers
- frame-begin/reset helpers
- telemetry log helpers such as origin-select, relink, and entity collection emitters

## Contracts

- Telemetry is level-gated by `cl_debug_view`.
- Repeated identical log lines are coalesced rather than emitted every time.
- Telemetry state is frame-aware and remembers prior observations to report deltas and suppress duplicates.
