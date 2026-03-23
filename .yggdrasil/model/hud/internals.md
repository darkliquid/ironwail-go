# Internals

## Logic

The package centers on a top-level `HUD` coordinator that updates renderer canvas parameters, chooses one status presentation style, then draws crosshair and centerprint overlays on their respective canvases. Internally the package splits into orchestration/state management, classic/QuakeWorld status-bar logic, and transient overlay helpers such as centerprint, crosshair, and the compact HUD.

## Constraints

- Viewsize thresholds are behavior-critical and decide whether the classic bar, compact overlay, inventory strip, or crosshair are visible.
- The package expects renderer canvas transforms to provide coordinate-space centering/scaling rather than recomputing everything in raw screen pixels.
- Tests cover a mix of status, crosshair, and centerprint behavior, so the package-level node has to preserve the relationships between child components.

## Decisions

### Keep overlay logic separate from the broader renderer/screen implementation

Observed decision:
- The Go port isolates HUD behavior in its own package instead of spreading overlay rules across screen and renderer code.

Rationale:
- **unknown — inferred from code and comments, not confirmed by a developer**

Observed effect:
- HUD behavior can evolve and be tested against gameplay-state contracts without mixing core overlay rules into low-level rendering backends.
