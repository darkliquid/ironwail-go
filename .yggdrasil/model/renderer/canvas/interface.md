# Interface

## Main consumers

- backend implementations
- HUD/menu/draw systems that depend on consistent canvas semantics

## Main surface

- `RenderContext`
- `Backend`
- `Config`
- canvas and transform-related types

## Contracts

- canvas transforms define the logical coordinate spaces used by 2D drawing
- config and backend interfaces are the stable package contract for renderer creation and runtime control, including screenshot/export (`CaptureScreenshot(filename)`)
- renderer cvar-name constants exposed from this layer include sky controls consumed by backend world paths, including `r_fastsky`, `r_skyfog`, `r_skysolidspeed`, and `r_skyalphaspeed`
- renderer cvar-name constants also include `r_dynamic`, used by backend-neutral effect/light helpers as the runtime gate for dynamic-light spawning and contribution
