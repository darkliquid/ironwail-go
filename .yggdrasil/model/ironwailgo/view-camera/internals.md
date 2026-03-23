# Internals

## Logic

This node combines canonical Quake view calculations with runtime camera policy. `viewcalc.go` holds reusable state and helpers for stair smoothing, bob, kick, FOV/viewsize policy, and other persistent view effects. `game_camera.go` applies those helpers to the current runtime state, choosing a base origin, smoothing it, composing view angles, and deriving camera/viewmodel positions. `chase.go` adds the chase-camera variant. A key policy detail is that runtime player origin selection is authoritative-first: authoritative entity/server origin is returned when available, while predicted XY evaluation is latched and emitted for telemetry/policy decisions rather than blindly replacing the returned base origin.

During intermission, `runtimeViewState()` uses `g.Client.Entities[g.Client.ViewEntity].Angles` (the view entity's server-authoritative orientation) as camera angles instead of `runtimeInterpolatedViewAngles()` (which returns `cl.ViewAngles`). This mirrors C `V_CalcIntermissionRefdef` which reads `ent->angles` directly and ignores `cl.viewangles`. Without this branch, keyboard left/right input during intermission modifies `cl.ViewAngles` and the camera visually rotates — correct for normal gameplay but incorrect for intermission where the camera position and orientation are server-authoritative.

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

### Use view entity angles during intermission, not cl.ViewAngles

Observed decision:
- `runtimeViewState()` reads `g.Client.Entities[viewEntity].Angles` when `g.Client.Intermission != 0`, bypassing `runtimeInterpolatedViewAngles()`.

Rationale:
- C `V_CalcIntermissionRefdef` (view.c) copies `ent->angles` to `r_refdef.viewangles` unconditionally. `cl.viewangles` is still mutated by `CL_AdjustAngles` (left/right keys) but is never used for the rendered camera during intermission. The Go client had no equivalent guard, causing intermission camera rotation on keyboard input.

Observed effect:
- Intermission camera stays fixed at the server-placed orientation. Left/right keys still update `cl.ViewAngles` (matching C) but do not visually rotate the camera during intermission.
