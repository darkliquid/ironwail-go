# Interface

## Main consumers

- `menu/state-machine`, which dispatches here for `MenuLoad`, `MenuSave`, `MenuJoinGame`, and `MenuHostGame`.

## Main surface

- `loadKey`, `saveKey`, `joinGameKey`, `hostGameKey`
- `joinGameChar`, `hostGameChar`
- entry helpers like `enterLoadMenu`, `enterSaveMenu`, `enterModsMenu`, `syncHostGameValues`, `startServerSearch`
- draw helpers for load/save/join/host screens

## Contracts

- Load/save use fixed slot indices `s0` through `s11`.
- Join Game defaults an empty address to `local`.
- Host Game edits live settings locally/cvar-by-cvar and issues a fixed command sequence when starting.
- Only the first `joinGameVisibleResults` server entries are displayed/selectable.
