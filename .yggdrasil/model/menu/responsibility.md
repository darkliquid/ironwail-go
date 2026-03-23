# Responsibility

## Purpose

`menu` owns the in-game front-end UI layer: menu activation, screen selection, cursor movement, menu-space drawing, and the translation of menu actions into engine-facing commands, cvar changes, and binding edits.

## Owns

- The package-level separation between state-machine orchestration, shared menu drawing helpers, and screen-specific flows.
- The menu system's dependence on Quake 320x200 virtual UI space and menu-specific input routing.
- Menu-specific integrations with draw assets, input destinations, cvars, and command queuing.

## Does not own

- Renderer/backend implementation details beyond `RenderContext` usage.
- Filesystem persistence for saves/mods; those arrive through providers.
- Gameplay, networking, or host execution beyond queueing commands and invoking callbacks.
