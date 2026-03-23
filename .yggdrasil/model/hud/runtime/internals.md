# Internals

## Logic

`HUD` is primarily a coordinator. It owns references to `StatusBar`, `CompactHUD`, `Crosshair`, and `Centerprint`, stores the latest `State`, and updates renderer canvas transform parameters from screen size plus cvars such as `scr_sbarscale`, `scr_menuscale`, and `scr_crosshairscale`. `Draw` first picks the appropriate HUD style (`classic`, `compact`, or `QuakeWorld`) when not in intermission, then always switches to the crosshair canvas and finally the default/menu-driven centerprint flow.

## Constraints

- Compact HUD only draws when `viewsize < 120`; classic inventory hides at `viewsize >= 110`; crosshair hides at `viewsize >= 130`.
- `hud_style` is registered in `NewHUD`, so repeated HUD construction assumes duplicate cvar registration is benign in this codebase.
- `IsActive` currently reflects only manual centerprint state, not every possible HUD visual.

## Decisions

### Feed the HUD a flattened state snapshot instead of live subsystem objects

Observed decision:
- The package defines a single `State` struct carrying all overlay inputs rather than depending directly on the full client runtime.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The HUD layer stays easier to test and decouple, but integration code must continuously translate live engine state into this flattened struct.
