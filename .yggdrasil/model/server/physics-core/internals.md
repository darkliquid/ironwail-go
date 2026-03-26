# Internals

## Logic

The core frame loop gates on `Active`/`Paused`, increments a server dev-stats frame counter, clears the shared datagram, ingests new clients, runs client commands, advances physics, enforces multiplayer rules, and then emits client messages. Within physics, helper paths clamp velocity/origin sanity, refresh the QC VM snapshot before `StartFrame`, run scheduled QC think callbacks, dispatch impacts/touches, apply gravity/water transitions, and maintain callback/telemetry context. Refreshing QC state before `StartFrame` is required so QuakeC frame logic (including intermission exit checks) sees the latest button and player fields written during `RunClients`.

Within `Physics()`, client-slot entities (edict indices 1..maxclients) receive `PlayerPreThink`/`PlayerPostThink` QC wrapping regardless of movetype — mirroring C `SV_Physics_Client`. For `MoveTypeWalk` clients, `PhysicsWalk` owns the Pre/PostThink calls; for all other movetypes (especially `MoveTypeNone` during intermission), `Physics()` wraps Pre/PostThink directly around the movetype dispatch. This is the mechanism by which `IntermissionThink` (called from `PlayerPreThink`) can detect button presses and advance the level: without it, `MoveTypeNone` clients never run `PlayerPreThink` and the attack-to-advance path in QC is never reached.

`AddGravity` follows the C `SV_AddGravity` pattern instead of reading only typed Go entity state. The server caches the QuakeC field offset for `gravity` when progs are loaded, then resolves that field from QC edict memory at runtime. A non-zero QC value scales `sv_gravity`; a missing field or zero value falls back to `1.0`. That preserves mod behavior that uses `.gravity` as an opt-in multiplier while leaving entities at default world gravity when the field is absent or unset.

`CheckWaterTransition` intentionally mirrors the C `SV_CheckWaterTransition` edge-case semantics. On spawn (`watertype == 0`) it seeds `watertype` from `PointContents` and sets `waterlevel = 1`. On transitions out of liquid it writes `watertype = CONTENTS_EMPTY` and stores the raw `PointContents` result in `waterlevel` (not zero), matching legacy behavior used by mods and parity checks.

FitzQuake `sendinterval` bookkeeping in `Physics()` intentionally matches the C `sv_phys.c` logic rather than a simplified approximation. After movetype dispatch it clears `SendInterval`, then only re-enables it when the entity is still live, `nextthink > sv.time`, and either the mover is `MOVETYPE_STEP`/`MOVETYPE_WALK` or its animation frame changed. The interval test uses the same rounded byte window as C — `Q_rint((nextthink-oldthinktime)*255)` with `25` and `26` suppressed as "close enough" to the client's assumed `0.1` cadence — leaving serialization to emit the remaining `nextthink - sv.time` payload only when that flag is set.

`SV_WalkMove` uses the same unstick trigger threshold as C (`0.03125`, `DIST_EPSILON`) for "no progress after step-up" detection. The Go implementation centralizes this in `walkMoveNeedsUnstick` and references `DistEpsilon` directly rather than a duplicate literal, preserving strict `< DIST_EPSILON` behavior on X/Y and keeping world/physics epsilon parity coupled to one constant.

`SV_WalkMove` step-down grounding intentionally mirrors the C condition that checks the mover's own solidity (`ent->v.solid == SOLID_BSP`) before setting `FL_ONGROUND` and `groundentity`, rather than checking the contacted `downtrace.ent` solidity. This preserves canonical behavior for players and other non-BSP movers traversing liquid-adjacent step geometry.

`PushMove` mirrors FitzQuake's `sv_gameplayfix_elevators` gate: when a rider remains blocked by the same pusher after the normal move, it only applies a `DistEpsilon` upward nudge if the cvar allows that edict class (`1` for client edicts `<= maxclients`, `2` for all entities). When disabled (`0`) or disallowed for non-clients at level `1`, the blocked move reverts exactly as in C.

`PushMove` blocked-callback execution now mirrors the QC synchronization pattern already used for touch/think callbacks: it snapshots pusher state, syncs pushers and the blocking entity to the QC VM before calling `blocked`, then applies mutated pushers and newly spawned edicts back into Go state when the callback succeeds. This closes a parity gap where blocked callbacks could mutate QC state that was not re-materialized into authoritative server edicts.

Deathmatch respawn progression now defers to QuakeC whenever `PlayerPreThink` is present. The server still blocks dead clients from running Go-side movement in `RunClients`, but it no longer bypasses QC by calling `PutClientInServer` directly from the Go-only deathmatch shortcut. That preserves QC `PlayerDeathThink` semantics for held-button release, button clearing before respawn, and any additional respawn side effects implemented by mods.


## Constraints

- `StartFrame` and per-entity callback ordering are parity-sensitive with original Quake behavior.
- Duplicate touch/impact suppression and `suppressTouchQC` handling must not hide legitimate gameplay callbacks.
- Rule enforcement must observe post-simulation state, not pre-simulation intent.
- `PlayerPreThink` must not be called for `MoveTypeWalk` clients from the outer loop — `PhysicsWalk` already calls it. Double-calling would apply QC per-think logic twice per frame, breaking weapon, movement, and stat updates.
- Gravity lookup must read the live QC edict slot for the specific entity index. Looking only at typed `EntVars` would miss extension fields like `.gravity` that are not part of the fixed Go struct.

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

### Cached QC gravity field offset rather than repeated name lookups

Observed decision:
- The server resolves the `gravity` field offset once during spawn/progs load and reuses that offset in `AddGravity`.

Rationale:
- This mirrors the existing cached-field approach already used for alpha/scale and avoids string lookup on every physics step while still preserving QuakeC extension-field semantics.

Observed effect:
- Per-entity gravity parity stays cheap enough for hot physics paths and behaves like C for both custom multipliers and zero-value fallback.

### Blocked callback sync parity in PushMove

Observed decision:
- `PushMove` blocked callbacks run through the same VM sync/apply discipline as other QC callback surfaces (`think`, `touch`).

Rationale:
- C `SV_PushMove` allows pusher blocked handlers to mutate gameplay state; using stale QC snapshots or skipping post-callback apply risks losing those mutations in Go authoritative state.

Observed effect:
- Mutations to pushers and spawned entities performed by blocked callbacks now persist immediately after the callback, matching expected Quake callback behavior and reducing callback-surface divergence.
