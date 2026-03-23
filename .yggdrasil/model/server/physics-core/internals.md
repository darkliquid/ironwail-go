# Internals

## Logic

The core frame loop gates on `Active`/`Paused`, clears the shared datagram, ingests new clients, runs client commands, advances physics, enforces multiplayer rules, and then emits client messages. Within physics, helper paths clamp velocity/origin sanity, refresh the QC VM snapshot before `StartFrame`, run scheduled QC think callbacks, dispatch impacts/touches, apply gravity/water transitions, and maintain callback/telemetry context. Refreshing QC state before `StartFrame` is required so QuakeC frame logic (including intermission exit checks) sees the latest button and player fields written during `RunClients`.

Within `Physics()`, client-slot entities (edict indices 1..maxclients) receive `PlayerPreThink`/`PlayerPostThink` QC wrapping regardless of movetype — mirroring C `SV_Physics_Client`. For `MoveTypeWalk` clients, `PhysicsWalk` owns the Pre/PostThink calls; for all other movetypes (especially `MoveTypeNone` during intermission), `Physics()` wraps Pre/PostThink directly around the movetype dispatch. This is the mechanism by which `IntermissionThink` (called from `PlayerPreThink`) can detect button presses and advance the level: without it, `MoveTypeNone` clients never run `PlayerPreThink` and the attack-to-advance path in QC is never reached.

## Constraints

- `StartFrame` and per-entity callback ordering are parity-sensitive with original Quake behavior.
- Duplicate touch/impact suppression and `suppressTouchQC` handling must not hide legitimate gameplay callbacks.
- Rule enforcement must observe post-simulation state, not pre-simulation intent.
- `PlayerPreThink` must not be called for `MoveTypeWalk` clients from the outer loop — `PhysicsWalk` already calls it. Double-calling would apply QC per-think logic twice per frame, breaking weapon, movement, and stat updates.

## Decisions

### Explicit frame pipeline instead of implicit global sequencing

Observed decision:
- The Go port expresses the server frame as a direct ordered pipeline on `Server`.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Simulation ordering is easier to audit and test, while still following Quake's authoritative server-frame semantics.

### PlayerPreThink/PostThink wrapping for all client movetypes

Observed decision:
- `Physics()` calls `PlayerPreThink`/`PlayerPostThink` for non-Walk client entities before/after the movetype switch, delegating Walk clients to `PhysicsWalk`.

Rationale:
- C `SV_Physics_Client` wraps Pre/PostThink unconditionally around movetype dispatch. During intermission, players have `MoveTypeNone`, so without this wrapping `IntermissionThink` never fires from `PlayerPreThink` and button presses cannot advance the level.

Observed effect:
- Intermission attack-to-advance now works. `IntermissionThink` is called once per frame from `PlayerPreThink` (and once per 0.1s from the player's scheduled RunThink), matching C behavior.
