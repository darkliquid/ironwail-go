# Internals

## Logic

This node reads debug cvars, derives an effective filter/config, and collects event/QC trace lines during authoritative server execution. It batches and coalesces repeated lines, tracks per-frame counts, and emits summaries or detailed event logs depending on the active configuration.

## Constraints

- Instrumentation must not mutate gameplay semantics.
- Event filtering must remain cheap enough to leave enabled during focused debugging.
- QC trace output depends on preserving accurate execution context from the core server/QC bridge.

## Decisions

### Dedicated diagnostic layer inside the server package

Observed decision:
- Debug telemetry and QC tracing live as first-class server support code rather than ad hoc logging scattered through gameplay paths.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Trigger, touch, think, physics, and QC execution can be inspected systematically without permanently raising noise in normal runtime paths.
