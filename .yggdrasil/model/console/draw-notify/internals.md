# Internals

## Logic

The draw layer recalculates console width from the current screen width, asks the console core to resize if needed, and then renders either the full drop-down overlay or just recent notify lines. Full-console rendering snapshots visible lines plus the current prompt, clips long prompts from the left, and draws a blinking cursor. Notify rendering filters recent lines by timestamp/fade state, then applies either left-aligned or centered text output depending on cvars.

## Constraints

- Resize is triggered from draw, so rendering and buffer layout are tightly coupled.
- Notify fading is visually approximated through deterministic stipple-like character omission.
- Background scaling and prompt clipping must preserve readability across varied resolutions.

## Decisions

### Renderer-agnostic console drawing behind a tiny draw interface

Observed decision:
- The package draws through a small abstract `DrawContext` instead of importing renderer backends directly.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Console rendering can be reused across backends and tested more easily, while still exposing the exact primitives the package needs.
