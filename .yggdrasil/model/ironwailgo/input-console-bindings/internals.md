# Internals

## Logic

This node coordinates how the executable interprets runtime input. It keeps menu, gameplay, console, and message/chat destinations coherent with one another and with mouse-grab state. When gameplay focus is lost, the command package deliberately releases held gameplay buttons so movement/attack state does not stick. It also forwards mouse movement into menu-space coordinates, applies gameplay mouse-look/strafe/forward rules from cvars and client state, and registers package-local gameplay commands/binds that bridge executable policy into subsystem behavior. Gameplay mouse-look is additionally suppressed while the client is in a fixed-angle intermission/cutscene so the forced camera cannot be rotated by local mouse deltas. That same lightweight command surface now includes `sizeup`/`sizedown`, matching C's screen-size keybinding commands by mutating the mirrored `scr_viewsize` alias in ±10 increments so both canonical and legacy view-size cvars stay synchronized, the `entities` debug dump that reuses runtime entity-model resolution helpers to print current client entity slots in C-style indexed form, and `centerview`, which simply re-enters the existing pitch-drift logic by calling `Client.StartPitchDrift()`.

The same command-registration surface also performs console-completion wiring. At startup it injects command/cvar/alias providers unconditionally, then opportunistically exposes `*fs.FileSystem.ListFiles` as the file provider when the executable owns the concrete VFS implementation. This keeps the console package decoupled from filesystem imports while still allowing command-aware file argument completion for map/config/demo commands.

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
