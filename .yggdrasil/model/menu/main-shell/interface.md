# Interface

## Main consumers

- `menu/state-machine`, which dispatches to these handlers/drawers when the current state is one of the shell screens.

## Main surface

- page key handlers such as `mainKey`, `singlePlayerKey`, `multiPlayerKey`, `helpKey`, `quitKey`, `setupKey`, `modsKey`
- setup editing helpers
- corresponding draw helpers (`drawMain`, `drawSinglePlayer`, `drawMultiPlayer`, `drawHelp`, `drawQuit`, `drawSetup`, `drawMods`)

## Contracts

- Main-menu navigation conditionally skips the Mods slot when no mods are available.
- Setup text entry accepts printable ASCII only and commits through `name`, `color`, and `hostname` updates.
- Generic confirmations reuse the quit screen state and confirmation callback machinery.
- Single Player -> New Game can be guarded by a confirmation prompt when the state-machine provider indicates an active in-game session; confirm queues the usual new-game command sequence and cancel returns to the single-player menu.
