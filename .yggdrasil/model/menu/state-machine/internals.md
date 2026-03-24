# Internals

## Logic

`Manager` is the package's state hub. It stores one current `MenuState`, per-screen cursors and editing buffers, provider handles, browser/save-slot caches, and draw/input dependencies. `NewManager` seeds defaults such as `joinAddress`, host-game values, and the default command sink (`cmdsys.AddText`). `ToggleMenu`, `ShowMenu`, `ShowConfirmationPrompt`, and `HideMenu` manage activation and key-destination switching. `M_Key` and `M_Char` are dispatchers: they zero/reset some shared mouse state, then route events to the correct screen handler. Mouse handling translates wheel-like relative movement and absolute hover positions into row selection on the active page. The manager now accepts runtime policy callbacks for both Single Player -> New Game confirmation (`SetNewGameConfirmationProvider`) and Single Player -> Save entry gating (`SetSaveEntryAllowedProvider`) so shell logic can enforce host/session conditions without hard-coupling menu code to host internals.

## Constraints

- `ToggleMenu`'s close path is not identical to `HideMenu`; prompt cleanup and `ignoreMouseFrame` reset semantics differ.
- The first absolute mouse sample after showing a menu is deliberately ignored to avoid jumpy initial selection.
- `ForcedUnderwater` depends on both menu activity and the current video-menu cursor position, so menu state affects renderer preview behavior.
- `menuCursorForPoint` is row-oriented and largely ignores the X coordinate.

## Decisions

### Use providers and callbacks only at selected integration boundaries

Observed decision:
- The manager injects save-slot, mod-list, and sound providers, but still owns some concrete integrations such as the default command sink and built-in server browser.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Some menu dependencies are easy to fake in tests, while others remain more tightly coupled to engine defaults and same-package test overrides.
