# Responsibility

## Purpose

`menu/state-machine` owns the core `Manager` facade and the top-level menu state machine: activation, current page, screen-local cursors/buffers, injected providers, and dispatch into the correct screen logic.

## Owns

- `MenuState`, `Manager`, `SaveSlotInfo`, `ModInfo`, and `DrawManager`.
- Menu visibility transitions (`ShowMenu`, `HideMenu`, `ToggleMenu`).
- Input-destination switching between menu and gameplay.
- Screen-local persistent state such as cursors, text buffers, confirmation callbacks, cached split pics, and provider results.
- Top-level `M_Key`, `M_Char`, `M_Draw`, and mouse routing entrypoints.

## Does not own

- The detailed rendering/layout of each individual menu page.
- Screen-specific command/cvar behavior beyond dispatching into the correct helper.
