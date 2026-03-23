# Responsibility

## Purpose

`console/completion` owns tab completion and completion-hint behavior for console input.

## Owns

- `TabCompleter` state and lifecycle
- injected providers for commands, cvars, aliases, and files
- match building, sorting, deduplication, cycling, and hint generation
- package-global completion helpers

## Does not own

- The actual command/cvar/alias registries.
- Prompt editing state outside the string passed in by callers.
- Drawing of completion popups or hint text.
