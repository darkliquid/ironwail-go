# Internals

## Logic

The telemetry node is a targeted debugging instrument rather than a general logger. It registers a cvar, tracks per-frame debug state, coalesces repeated messages, and emits structured lines describing origin selection, relink phases, entity collection decisions, and related viewmodel/debug context. It intentionally remembers previous values so it can report meaningful deltas and suppress spam.

## Constraints

- Telemetry is only meaningful when the relevant runtime state (`g.Client`, current frame, prior entity/view values) is populated.
- Coalescing behavior is part of the usability contract for this debug surface.
- This node observes several other child nodes without owning their logic.

## Decisions

### Build a dedicated view-debug telemetry surface instead of relying on ad hoc logs

Observed decision:
- The command package uses a purpose-built `cl_debug_view` telemetry path with structured state and coalescing.

Rationale:
- **unknown — inferred from code and recent debugging work, not confirmed by a developer**

Observed effect:
- View/debug investigations can be much more targeted and less noisy, but the behavior and gating rules need explicit documentation to stay coherent across future debugging tasks.
