# Internals

## Logic

This node is a thin command-registration layer over the `cvar` package. It installs a small family of convenience verbs, validates argument shape where needed, and translates those verbs into `cvar.Get`, `cvar.Set`, `cvar.All`, and default-value operations. `cycle`/`cycleback` first resolve an existing cvar and then mirror Ironwail's mixed numeric/string matching semantics when walking the candidate list; `inc` also resolves an existing cvar before computing the new numeric value. `cvarlist` snapshots and sorts cvars by name before logging, which keeps output stable across map/hash iteration order.

## Constraints

- The commands are only as correct as the underlying `cvar` defaults/flags they delegate to.
- Helper commands that mutate values must preserve Ironwail's refusal to create unknown cvars through these convenience paths.
- Reset behavior must distinguish between all cvars and archived-only cvars.
- Registration remains explicit via `RegisterCvarCommands`, while host runtime now calls the package-level registration helper during startup so helper commands are present in normal runs.

## Decisions

### Keep cvar convenience verbs separate from the core command substrate

Observed decision:
- The package isolates cvar-manipulation commands in a dedicated file/API instead of baking them into the command runtime itself.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The core command engine stays focused on parsing/routing semantics, while cvar helper commands remain an optional registration layer.
