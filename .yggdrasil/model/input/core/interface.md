# Interface

## Main consumers

- runtime code that constructs `System`, wires callbacks, sets key destinations, polls events, and consumes per-frame mouse/text state.
- bind/rebind code that serializes or parses Quake key names.
- backend implementations that satisfy the `Backend` interface.

## Main surface

- `NewSystem`, `Init`, `Shutdown`, `PollEvents`, `GetState`, `ClearState`
- `SetKeyDest`, `GetKeyDest`, `SetBackend`, `Backend`
- `SetBinding`, `GetBinding`, `IsKeyDown`, `AnyKeyDown`, `ClearKeyStates`
- `HandleKeyEvent`, `HandleCharEvent`
- `GetModifierState`, `SetCursorMode`, `ShowKeyboard`, `GetGamepadState`, `IsGamepadConnected`, `SetMouseGrab`
- `KeyToString`, `StringToKey`

## Contracts

- Repeated key-down events are filtered only in `KeyGame`; menus still see repeats.
- Stray key-up events for keys that are not marked down are ignored.
- Menu key routing invokes `OnMenuKey` first and only forwards to `OnKey` if the destination remains `KeyMenu`.
- `SetKeyDest` also updates backend text mode according to whether the engine expects textual input.
