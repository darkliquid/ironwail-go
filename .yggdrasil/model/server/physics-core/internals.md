# Internals

## Logic

The core frame loop gates on `Active`/`Paused`, clears the shared datagram, ingests new clients, runs client commands, advances physics, enforces multiplayer rules, and then emits client messages. Within physics, helper paths clamp velocity/origin sanity, run scheduled QC think callbacks, dispatch impacts/touches, apply gravity/water transitions, and maintain callback/telemetry context.

## Constraints

- `StartFrame` and per-entity callback ordering are parity-sensitive with original Quake behavior.
- Duplicate touch/impact suppression and `suppressTouchQC` handling must not hide legitimate gameplay callbacks.
- Rule enforcement must observe post-simulation state, not pre-simulation intent.

## Decisions

### Explicit frame pipeline instead of implicit global sequencing

Observed decision:
- The Go port expresses the server frame as a direct ordered pipeline on `Server`.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Simulation ordering is easier to audit and test, while still following Quake's authoritative server-frame semantics.
