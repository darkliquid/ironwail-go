# Interface

## Main consumers

- runtime code that constructs the menu manager, injects draw/input dependencies, and calls it during input/draw phases.
- host/runtime integration that provides sound, save-slot, and mod-list callbacks.

## Main surface

- `Manager`, `MenuState`, `SaveSlotInfo`, `ModInfo`, `DrawManager`
- manager lifecycle and visibility methods
- menu input entrypoints (`M_Key`, `M_Char`, mouse movement handlers)
- `M_Draw`
- provider/callback registration helpers
- menu state/query helpers such as `ForcedUnderwater` and `WaitingForKeyBinding`

## Contracts

- The manager is the sole owner of menu activation state and page-specific cursors/buffers.
- Menu actions affect the rest of the engine through callbacks, cvar writes, and queued console commands rather than direct subsystem calls.
- Mouse/key input is interpreted in menu space, not gameplay space.
