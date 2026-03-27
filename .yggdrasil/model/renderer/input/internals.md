# Internals

## Logic

This layer isolates backend/window-system input differences from the rest of the renderer package.
For OpenGL builds, key/mouse mapping helpers are now treated as canonical in `internal/renderer/opengl`; renderer-root code only wires `InputBackendForSystem` to that implementation, and GLFW keymap regression tests call the opengl mapping helpers directly.

## Constraints

- Input behavior can diverge by backend because the event sources differ.

## Decisions

### Backend-specific input adapters

Observed decision:
- Input handling that depends on the graphics/window backend is kept alongside renderer backends instead of being forced into the generic input package.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
