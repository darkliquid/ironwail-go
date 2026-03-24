# Interface

## Main consumers

- runtime setup that constructs the manager and registers sound/save/mod callbacks.
- frame/input code that forwards menu-related key, char, and mouse events.
- render code that calls `M_Draw` and checks helper states like `ForcedUnderwater`.

## Main surface

- `NewManager`
- `SetSoundPlayer`, `SetSaveSlotProvider`, `SetModsProvider`, `SetCurrentMod`, `SetNewGameConfirmationProvider`, `SetResumeGameAvailableProvider`, `SetSaveEntryAllowedProvider`
- `ToggleMenu`, `ShowMenu`, `HideMenu`, `ShowConfirmationPrompt`, `ShowQuitPrompt`
- `M_Key`, `M_Char`, `M_Draw`, `M_Mousemove`, `M_MousemoveAbsolute`
- `IsActive`, `GetState`, `ForcedUnderwater`, `WaitingForKeyBinding`, `MainCursor`

## Contracts

- Opening the menu redirects input to `input.KeyMenu`; hiding it restores `input.KeyGame`.
- `MenuQuit` is the shared confirmation-screen state, not only the quit page.
- Provider-backed screens refresh on entry or draw according to manager-owned rules.
- `SetNewGameConfirmationProvider` controls whether selecting Single Player -> New Game goes directly to command queueing or first enters a confirmation prompt that returns to `MenuSinglePlayer` on cancel.
- `SetResumeGameAvailableProvider` controls whether selecting Single Player -> New Game offers a resume prompt that confirms into `load "autosave/start"` and declines into the normal fresh-start command sequence.
- `SetSaveEntryAllowedProvider` controls whether selecting Single Player -> Save transitions to `MenuSave`; when disallowed, selection stays on `MenuSinglePlayer` and emits cancel feedback.
