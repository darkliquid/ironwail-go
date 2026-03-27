# Internals

## Logic

This node is the last command-package bridge before subsystem presentation. It takes the current runtime state assembled by earlier nodes and applies it to the renderer and audio systems: world/viewmodel submission, overlays, UI, and sound-listener/update behavior. The exact mechanics live in the underlying subsystems, but these files own the composition decisions about what gets submitted each frame and with which state. That includes flattening client intermission counters into `hud.State`.
Renderer visual helpers now take role-specific interfaces at their call boundaries (`gameRendererLights`, `gameRendererAssets`) while call sites pass `g.Renderer` as the concrete provider. This keeps behavior unchanged but makes seam-local dependencies explicit and shrinkable over time.

Intermission parity note:
- Overlay visibility is no longer derived from input key-destination state in this bridge. The intermission/finale HUD overlay is always passed through when the client is in intermission, matching C Ironwail where intermission drawing is not suppressed by `key_dest`.

## Constraints

- Presentation depends on correctly ordered upstream runtime state from bootstrap, loop, camera, entity, and input nodes.
- These helpers are orchestration glue and therefore should stay narrow; they should not re-own entity or camera logic.

## Decisions

### Keep final presentation wiring in the command package instead of pushing it entirely into renderer/audio subsystems

Observed decision:
- The executable retains a thin layer that decides what current runtime state should be handed to render/audio systems each frame.

Rationale:
- **unknown — inferred from code structure, not confirmed by a developer**

Observed effect:
- Presentation decisions remain close to the composition root, but graph documentation must describe that this node is a final wiring layer rather than a subsystem implementation.
