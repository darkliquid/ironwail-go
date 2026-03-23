# Interface

## Main consumers

- runtime/input code that edits or commits console input.
- host and engine subsystems that print messages or manage debug logging.
- renderer/runtime code that draws full-console and notify views.
- startup wiring that injects completion providers.

## Main surface

- console construction/init/clear/dump/close/print APIs
- input/history editing helpers
- completion-provider registration and completion/hint APIs
- draw entry points and notify-line count helpers

## Contracts

- The console is the textual UI/storage layer, not the command executor.
- Scrollback and notify behavior depend on the console ring buffer staying coherent across resize, print, and draw paths.
- Completion is provider-driven and intentionally decoupled from direct `cmdsys`/`cvar` imports.
