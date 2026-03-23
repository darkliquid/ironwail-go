# Internals

## Logic

This node combines canonical Quake view calculations with runtime camera policy. `viewcalc.go` holds reusable state and helpers for stair smoothing, bob, kick, FOV/viewsize policy, and other persistent view effects. `game_camera.go` applies those helpers to the current runtime state, choosing a base origin, smoothing it, composing view angles, and deriving camera/viewmodel positions. `chase.go` adds the chase-camera variant. A key policy detail is that runtime player origin selection is authoritative-first: authoritative entity/server origin is returned when available, while predicted XY evaluation is latched and emitted for telemetry/policy decisions rather than blindly replacing the returned base origin.

## Constraints

- Shared `globalViewCalc` state means camera/view calculations are not purely functional.
- Teleport, stair smoothing, and prediction gates all affect whether cached or smoothed state can be reused.
- Camera results are consumed downstream by viewmodel, render, and sometimes audio logic, so changes here have wide blast radius.

## Decisions

### Prefer authoritative origin and treat prediction as a constrained policy aid

Observed decision:
- Runtime camera origin selection prioritizes authoritative origin and only uses predicted XY as a gated/latching decision with telemetry.

Rationale:
- **unknown — inferred from code and tests, not confirmed by a developer**

Observed effect:
- The command package keeps camera behavior conservative and observable during parity debugging, but the policy is subtle enough that it needs explicit graph documentation.
