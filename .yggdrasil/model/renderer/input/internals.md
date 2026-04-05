# Internals

## Logic

This layer isolates backend/window-system input differences from the rest of the renderer package.
For CGO-free GoGPU builds, the concrete backend lives in `internal/renderer/gogpu`. It registers `gpucontext.EventSource` callbacks when available, but it also keeps a polling fallback for X11/plain-GoGPU sessions. That fallback must not call `gogpu`'s per-frame `Update()` a second time because the app already advances input state before `OnUpdate`; double-advancing would erase `JustPressed`/delta edges before the engine consumes them. The fallback preserves the engine-facing key vocabulary needed for console input, navigation, function keys, and numpad behavior, and it polls mouse buttons/scroll/position so degraded callback delivery still leaves the engine playable.

## Constraints

- Input behavior can diverge by backend because the event sources differ.

## Decisions

### Backend-specific input adapters

Observed decision:
- Input handling that depends on the graphics/window backend is kept alongside renderer backends instead of being forced into the generic input package.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

### GoGPU fallback preserves the established engine-facing key vocabulary

Observed decision:
- The CGO-free GoGPU input adapter should preserve the same engine-facing key vocabulary even when the window system forces a polling fallback.

Rationale:
- The engine reserves certain physical keys such as console grave/backquote and expects navigation/function/numpad keys to bind identically across backends. A narrower GoGPU map makes input look dead even when the library is still producing usable key state.
