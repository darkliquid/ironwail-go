# Internals

## Logic

The app shell is the narrowest place that still sees the whole executable. `Game` centralizes mutable process state so the rest of the `package main` helpers can coordinate through one runtime bag. `main()` parses startup options, chooses headless/dedicated/runtime behavior, and hands off to the appropriate bootstrap and loop code. The large `main_test.go` suite exercises this shell from many angles, including startup, runtime ordering, input/view policy, and integration edges that span multiple files.

## Constraints

- `main_test.go` is intentionally broad and currently cannot be cleanly attributed to only one narrow runtime concern.
- The app shell depends on almost every other child node and is therefore an orchestration/documentation seam rather than an algorithmic module.
- Sprite-runtime regression tests in `main_test.go` verify that collected sprite entities keep parsed frame pixels reachable through both `SpriteEntity.SpriteData` and cached `model.Model.SpriteData`.

## Decisions

### Use one process-wide game state bag instead of passing a narrow context through every helper

Observed decision:
- The command package keeps a global/process-wide `Game` structure that child helpers read and mutate directly.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Wiring is straightforward for a large `package main`, but reasoning about behavior requires documenting which child nodes consume and update shared `Game` fields.
