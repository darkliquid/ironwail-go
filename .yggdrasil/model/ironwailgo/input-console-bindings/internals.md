# Internals

## Logic

This node coordinates how the executable interprets runtime input. It keeps menu, gameplay, console, and message/chat destinations coherent with one another and with mouse-grab state. When gameplay focus is lost, the command package deliberately releases held gameplay buttons so movement/attack state does not stick. It also forwards mouse movement into menu-space coordinates, applies gameplay mouse-look/strafe/forward rules from cvars and client state, and now applies a narrow gamepad look path: right-stick yaw/pitch is consumed during gameplay via `input.System.GetGamepadState(0)` when `joy_look` is enabled, with optional gyro contribution gated behind `joy_gyro_look` and independent yaw/pitch scales. Gameplay mouse-look is additionally suppressed while the client is in a fixed-angle intermission/cutscene so the forced camera cannot be rotated by local mouse deltas. That same lightweight command surface now includes `sizeup`/`sizedown`, matching C's screen-size keybinding commands by mutating the mirrored `scr_viewsize` alias in ±10 increments so both canonical and legacy view-size cvars stay synchronized, while clamping to `30..110` to keep normal runtime gameplay out of the HUD-hidden `>=120` range unless a user intentionally sets larger values directly via cvars; it also includes the `entities` debug dump that reuses runtime entity-model resolution helpers to print current client entity slots in C-style indexed form, `centerview`, which simply re-enters the existing pitch-drift logic by calling `Client.StartPitchDrift()`, and manual pprof capture commands that write CPU/heap/alloc profiles into the current mod directory.

The same command-registration surface also performs console-completion wiring. At startup it injects command/cvar/alias providers unconditionally, then opportunistically exposes `*fs.FileSystem.ListFiles` as the file provider when the executable owns the concrete VFS implementation. This keeps the console package decoupled from filesystem imports while still allowing command-aware file argument completion for map/config/demo commands.

The profiling commands are intentionally file-based and manual rather than an always-on HTTP endpoint. `profile_cpu_start`/`profile_cpu_stop` guard a single active CPU profile across the process, while `profile_dump_heap` and `profile_dump_allocs` force a GC before writing the one-shot runtime profiles so captured data reflects the just-played scenario as closely as the runtime allows. Relative paths resolve through the same `<basedir>/<moddir>` convention used by screenshots, keeping ad-hoc profiling output scoped to the active game directory without introducing extra startup flags.

## Constraints

- Input destination, mouse grab, and held-button release are a coupled policy decision.
- Chat and console editing are frame-time aware because held backspace is repeated locally.
- Command-package input handling assumes the underlying `input.System` already routes keys/chars according to the current destination.

## Decisions

### Couple input-destination changes to mouse-grab and button-release policy

Observed decision:
- The command package treats leaving gameplay input as a mode transition that must also clear mouse state and release gameplay buttons.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- UI transitions are less likely to leave stuck movement/attack state behind, but the input policy spans multiple concerns and belongs in explicit graph documentation.
