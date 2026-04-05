# Internals

## Logic

The app shell is the narrowest place that still sees the whole executable. `Game` centralizes mutable process state so the rest of the `package main` helpers can coordinate through one runtime bag. `main()` parses startup options, chooses headless/dedicated/runtime behavior, and hands off to the appropriate bootstrap and loop code. The large `main_test.go` suite exercises this shell from many angles, including startup, runtime ordering, input/view policy, and integration edges that span multiple files.
Process-wide logging policy now also lives in this node: `main()` parses `-loglvl`, installs the default slog logger, and applies a subsystem-aware handler that derives subsystem names from caller source paths. The handler keeps existing `slog.*` call sites untouched while supporting a global baseline plus per-subsystem overrides such as `INFO,renderer=WARN,input=DEBUG`. Override matching is prefix-based (`renderer` applies to `renderer.gogpu` unless a more specific override exists), and the handler tags emitted records with a `subsystem` attribute so mixed logs stay readable.
`Game.Renderer` is now stored behind a command-layer `gameRenderer` interface so app-shell code no longer requires a concrete `*renderer.Renderer` field type for routine runtime calls. Startup subsystem wiring now consumes this interface directly when constructing `renderer.NewRendererAdapter`, removing the temporary concrete bridge.
The app-shell renderer contract is decomposed into embedded role interfaces (`frame loop`, `assets`, `world`, `lights`, and `input`) so future decoupling can reduce surface area by seam without rewriting call sites all at once.
Runtime visual helpers in `game_visual.go` now consume role-specific renderer interfaces (`lights` and `assets`) rather than closing over the full renderer composite, with `main.go` passing `g.Renderer` as the provider.
The app shell now serializes runtime reload mutations against render-thread frame work via a shared runtime-state mutex: the GoGPU `OnDraw` callback holds this lock while reading/drawing shared `Game` state, and game-dir reload callbacks take the same lock before tearing down/rebuilding mod-scoped runtime state. This prevents mid-frame cross-thread state replacement during mod switches.
Runtime mod-reload asset refresh now stages palette/conchars bytes in a pending queue and applies them from `OnDraw` on the dedicated render thread, rather than mutating renderer assets directly from the update-thread reload callback.

CSQC draw hooks now route pic lookups through a parity cache bridge that matches C `DrawQC_CachePic` behavior: `iscachedpic` remains a pure cache query, `precache_pic` only fails under BLOCK on missing assets, and draw/getsize/subpic use a shared AUTO cache path. Regression coverage keeps this split explicit by asserting that `iscachedpic` only flips true after an AUTO-loading call (`GetImageSize`/draw path), not before.
Frame-state assembly also now publishes CSQC extglobals from runtime host/client state, including realtime-backed `cltime`, intermission timing, local player numbers/entities, and client command frame.
When CSQC is loaded, the runtime loop now treats CSQC HUD rendering as successful when `CallDrawHud` returns nil and either (a) QuakeC reports `drewHUD=true` via `OFSReturn` or (b) CSQC actually issued draw builtins during that call. If neither signal occurs, the shell immediately falls back to native HUD rendering for that frame. This matches C Ironwail behavior where `CSQC_DrawHud` ownership is call-presence based rather than strictly return-value based, and it fixes runtime cases where CSQC draws HUD primitives but leaves `OFSReturn` unset.

The runtime console-forced-up gate now uses a dedicated helper that considers `client.State == StateActive` authoritative and only falls back to signon-count checks while not active. This mirrors Quake's effective runtime readiness better during demo playback/live transitions where signon counters can temporarily lag state changes, and it prevents HUD/overlay suppression in OpenGL frames that were previously misclassified as forced-console mode.

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
Loading-plaque regression coverage in `main_test.go` now also asserts `drawLoadingPlaque` is a no-op when render context is nil, matching pause-overlay safety and preventing nil-context panics in focused overlay draw tests.
Menu-close regression coverage in `main_test.go` now follows the dedicated `MenuSkill` submenu: the shell only restores gameplay input/mouse grab after a second confirm from the skill screen, rather than directly from `MenuSinglePlayer`.
Command-surface regression coverage in `main_test.go` now also asserts HUD-safe `sizeup`/`sizedown` clamping: command-driven viewsize changes remain bounded to `30..110`, preserving mirrored `viewsize`/`scr_viewsize` sync while preventing accidental runtime HUD hide states from crossing into `>=120`.

## Constraints

- `main_test.go` is intentionally broad and currently cannot be cleanly attributed to only one narrow runtime concern.
- The app shell depends on almost every other child node and is therefore an orchestration/documentation seam rather than an algorithmic module.
- Sprite-runtime regression tests in `main_test.go` verify that collected sprite entities keep parsed frame pixels reachable through both `SpriteEntity.SpriteData` and cached `model.Model.SpriteData`.
- Render-frame regression tests in `main_test.go` also preserve `model.Model.SpriteData` when a sprite entity carries no explicit `SpriteEntity.SpriteData`, guarding renderer fallback expectations.

## Decisions

### Use one process-wide game state bag instead of passing a narrow context through every helper

Observed decision:
- The command package keeps a global/process-wide `Game` structure that child helpers read and mutate directly.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Wiring is straightforward for a large `package main`, but reasoning about behavior requires documenting which child nodes consume and update shared `Game` fields.
