# Internals

## Logic

`Manager` is the package's state hub. It stores one current `MenuState`, per-screen cursors and editing buffers, provider handles, browser/save-slot caches, and draw/input dependencies. `NewManager` seeds defaults such as `joinAddress`, host-game values, and the default command sink (`cmdsys.AddText`). `ToggleMenu`, `ShowMenu`, `ShowConfirmationPrompt`, and `HideMenu` manage activation and key-destination switching. `M_Key` and `M_Char` are dispatchers: they zero/reset some shared mouse state, then route events to the correct screen handler. Mouse handling translates wheel-like relative movement and absolute hover positions into row selection on the active page. The manager now accepts runtime policy callbacks for Single Player -> New Game confirmation (`SetNewGameConfirmationProvider`), Single Player -> resume-autosave availability (`SetResumeGameAvailableProvider`), and Single Player -> Save entry gating (`SetSaveEntryAllowedProvider`) so shell logic can enforce host/session conditions without hard-coupling menu code to host internals. The Single Player flow now includes a dedicated `MenuSkill` state that owns cursoring between four skill rows plus an optional Resume row and shares the same mouse/key cursor plumbing as other row-based pages.

## Constraints

- `ToggleMenu`'s close path is not identical to `HideMenu`; prompt cleanup and `ignoreMouseFrame` reset semantics differ.
- The first absolute mouse sample after showing a menu is deliberately ignored to avoid jumpy initial selection.
- `ForcedUnderwater` depends on both menu activity and the current video-menu cursor position, so menu state affects renderer preview behavior.
- `menuCursorForPoint` is row-oriented and largely ignores the X coordinate.
- `MenuSkill` row hit-testing uses a non-uniform table (`56,72,88,104[,128]`) rather than the 20px stride used by `MenuSinglePlayer`.
- Controls-menu cursor math and absolute-row hit testing are coupled to `controlsItems` and `controlRowY`; when the bind matrix expands these values must stay aligned or keyboard and mouse navigation diverge.
- While Controls rebinding is active, absolute mouse hover updates are intentionally ignored so the selected bind row cannot drift before the capture key is chosen or cancelled.

## Recent update: Controls key-matrix parity expansion

- The Controls menu now exposes a broader legacy movement/look key matrix by extending binding rows from 12 actions to 21 actions.
- Newly surfaced commands are: `+speed`, `+strafe`, `+lookup`, `+lookdown`, `centerview`, `+mlook`, `+klook`, `+moveup`, `+movedown`.
- The existing semantics are preserved: settings rows still live-toggle cvars, binding rows still use Enter/Right to rebind, and Left/Backspace to clear.
- `controlsItems` increased to keep wrap-around navigation and absolute row hit testing in sync with the larger binding surface.

## Decisions

### Use providers and callbacks only at selected integration boundaries

Observed decision:
- The manager injects save-slot, mod-list, and sound providers, but still owns some concrete integrations such as the default command sink and built-in server browser.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Some menu dependencies are easy to fake in tests, while others remain more tightly coupled to engine defaults and same-package test overrides.
