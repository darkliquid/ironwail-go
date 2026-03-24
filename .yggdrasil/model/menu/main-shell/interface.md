# Interface

## Main consumers

- `menu/state-machine`, which dispatches to these handlers/drawers when the current state is one of the shell screens.

## Main surface

- page key handlers such as `mainKey`, `singlePlayerKey`, `skillKey`, `multiPlayerKey`, `helpKey`, `quitKey`, `setupKey`, `modsKey`
- setup editing helpers
- corresponding draw helpers (`drawMain`, `drawSinglePlayer`, `drawSkill`, `drawMultiPlayer`, `drawHelp`, `drawQuit`, `drawSetup`, `drawMods`)

## Contracts

- Main-menu navigation conditionally skips the Mods slot when no mods are available.
- Setup text entry accepts printable ASCII only and commits through `name`, `color`, and `hostname` updates.
- Generic confirmations reuse the quit screen state and confirmation callback machinery.
- Single Player -> New Game now enters a dedicated skill/resume submenu (`MenuSkill`) once any active-session confirmation gate is satisfied.
- In `MenuSkill`, selecting a skill row (0–3) queues the fresh-start command sequence with `skill N`, while selecting the optional Resume row queues `load "autosave/start"`.
- `SetResumeGameAvailableProvider` controls whether the Resume row is visible and whether the initial selection lands on Resume (when available) or on the current `skill` cvar value (when not).
- Single Player -> Save now consults the state-machine save-entry provider before transitioning to `MenuSave`; disallowed attempts remain on `MenuSinglePlayer` and play cancel feedback.
