# Internals

## Logic

This node is a thin command-registration layer over the `cvar` package. It installs a small family of convenience verbs, validates argument shape where needed, and translates those verbs into `cvar.Get`, `cvar.Set`, `cvar.All`, and default-value operations.

## Constraints

- The commands are only as correct as the underlying `cvar` defaults/flags they delegate to.
- Reset behavior must distinguish between all cvars and archived-only cvars.
- Registration is opt-in; the package defines the commands but does not itself prove where they are installed.

## Decisions

### Keep cvar convenience verbs separate from the core command substrate

Observed decision:
- The package isolates cvar-manipulation commands in a dedicated file/API instead of baking them into the command runtime itself.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The core command engine stays focused on parsing/routing semantics, while cvar helper commands remain an optional registration layer.
