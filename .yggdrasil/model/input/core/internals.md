# Internals

## Logic

`System` is the core state machine for input. It stores the active backend, current key destination, per-key down state, frame-accumulated characters, current mouse deltas, bindings, and callback hooks. `PollEvents` delegates to the backend, `GetState` pulls fresh mouse deltas from the backend, and `ClearState` clears one-frame character/mouse accumulators. `HandleKeyEvent` enforces Quake-style repeat suppression and stray-up filtering before updating key state, modifier flags, and destination-aware callbacks. `HandleCharEvent` appends text to the frame buffer and routes characters to menu/general callbacks according to destination.

## Constraints

- Key codes are used as direct array indices, so numbering stability matters.
- `StringToKey` returns `0` for unknown names; that sentinel is part of bind-command validation behavior.
- `ClearKeyStates` is the dedicated stuck-key recovery path for focus/mode transitions.
- Some system-level fields (such as tracked modifiers and `InputState.Gamepads`) are broader than what every current consumer uses, so callers rely on only part of the stored state today.

## Decisions

### Preserve Quake-style key routing while separating it from backend polling

Observed decision:
- The system layer keeps Quake-like key destination and binding semantics independent of whichever platform backend is currently providing raw events.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Game, menu, console, and chat routing behavior remains consistent even as backends change, but backend implementations must faithfully translate their raw events into the package's key vocabulary and callback model.
