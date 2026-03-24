# Internals

## Logic

The app shell is the narrowest place that still sees the whole executable. `Game` centralizes mutable process state so the rest of the `package main` helpers can coordinate through one runtime bag. `main()` parses startup options, chooses headless/dedicated/runtime behavior, and hands off to the appropriate bootstrap and loop code. The large `main_test.go` suite exercises this shell from many angles, including startup, runtime ordering, input/view policy, and integration edges that span multiple files.

Intermission parity regression coverage in `main_test.go` now asserts that runtime HUD state keeps intermission overlays visible even when key destination is console/message/menu. This aligns shell behavior with C Ironwail's top-level screen path, where intermission draw is keyed off intermission state rather than focus-mode suppression.

Effect-source regression coverage in `main_test.go` now includes a rocket-model case (`model.EFRocket` with `state.Effects == 0`) to ensure command-layer collection keeps rocket light sources that renderer effect lights consume.

Control-cvar regression coverage in `main_test.go` now asserts that changing `cl_nolerp` updates `g.Client.NoLerp` through the control-cvar callback/sync path, preventing interpolation-policy drift between cvar state and client lerp logic.
Control-cvar regression coverage in `main_test.go` now also asserts `v_centermove`/`v_centerspeed` sync into `g.Client.CenterMove`/`g.Client.CenterSpeed`, including replacement-client resync through `syncHostClientState`.
Frame/interpolation policy regression coverage in `main_test.go` now also asserts that `syncHostClientState` mirrors host-derived local fast-server policy to `g.Client.LocalServerFast` when `host_maxfps` crosses the 72 FPS net-interval threshold.
Console-completion regression coverage in `main_test.go` now also asserts that startup wiring exposes VFS-backed map completion (`map e1` -> `map e1m1`) and that the broad command-completion smoke test uses a unique `toggleconsole` prefix so full-package command registration does not make the assertion order-dependent.
Command-surface regression coverage in `main_test.go` now also asserts the new `entities` debug command prints indexed `EMPTY` gaps plus live client entity lines from the process-wide `g.Client` state, and stays silent while the client is disconnected.
Command-surface regression coverage in `main_test.go` now also asserts `centerview` restarts pitch drift through the executable command-binding surface by flipping `g.Client.NoDrift` off and reseeding `PitchVel`.
Runtime telemetry regression coverage in `main_test.go` now also asserts `scr_showfps=2` uses milliseconds text (`xx.xx ms`) in the runtime FPS overlay path, matching legacy Ironwail semantics where nonzero showfps modes include an ms-style display mode in addition to FPS text.
Render-frame regression coverage in `main_test.go` now also asserts that `buildRuntimeRenderFrameState` picks up worldspawn fog defaults from the loaded BSP entity lump before sampling client fog for renderer submission.
Input regression coverage in `main_test.go` now also asserts that gameplay look-path processing consumes right-stick gamepad look deltas and (optionally) gyro deltas through cvar gates, so stick look can ship independently while gyro remains explicitly opt-in.
Screenshot regression coverage in `main_test.go` now asserts capture behavior for both renderer-present and renderer-absent flows, including dimension expectations for renderer-sized fallback output and the default 1280x720 software fallback.

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
