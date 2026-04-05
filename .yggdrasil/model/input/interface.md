# Interface

## Main consumers

- runtime wiring in `cmd/ironwailgo`, which creates the input system, assigns callbacks, and installs a concrete backend when available.
- menu and command code that serializes bindings and reacts to key-destination changes.
- renderer-side input adapters that implement or cooperate with the package's backend contract.

## Main surface

- `System`, `Backend`, `InputState`, `KeyEvent`, `ModifierState`, `GamepadState`
- key/destination/text/cursor enums and constants
- binding helpers and key-name conversion helpers

## Contracts

- Core input routing and key numbering stay stable regardless of which backend is active.
- Backend-specific features are hidden behind the `Backend` interface and build tags.
- Platform event ingestion is separate from gameplay meaning; consumers attach callbacks and bindings on top.
