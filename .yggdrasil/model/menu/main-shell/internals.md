# Internals

## Logic

This node owns the shell around the menu system. The main menu routes into single-player, multiplayer, options, help, mods, or quit. Help cycles through six pages, the setup screen stages hostname/name/color edits before applying them, and quit confirmation is implemented as a generic three-line prompt with optional callbacks. The main-menu draw path also supports inserting a Mods row into a pre-baked graphic by splitting `mainmenu.lmp` into cached top and bottom sub-pics once, then drawing a mod label/picture in the gap. Single Player -> New Game now reuses the confirmation prompt path in two cases: when the runtime-provided session-state gate says a game is already active, accepting starts the standard new-game command pipeline and declining returns to the single-player menu; otherwise, when runtime wiring reports that `autosave/start` exists, the prompt offers resuming the canonical autosave on confirm and falling through to a fresh start on decline.

## Constraints

- Help pages wrap modulo `helpPages`.
- Setup hostname/name fields are length-limited and only accept printable ASCII, even though deletion is rune-safe.
- Setup shirt/pants colors wrap in `[0, setupColorMax]`.
- Confirmation prompts store only three lines and generic confirmation without an explicit confirm callback falls back to `quit
`.
- The mods browser always appends a synthetic `BACK` row after the discovered mods list.

## Decisions

### Preserve Quake menu command flow by queueing actions instead of invoking subsystems directly

Observed decision:
- Page actions like starting a new game, switching mods, or confirming quit are expressed as queued console commands and cvar writes rather than direct host/session API calls.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Menu code stays thin and engine-facing integration remains mostly command-driven, but some workflows are only understandable if you know the downstream console commands they enqueue.
