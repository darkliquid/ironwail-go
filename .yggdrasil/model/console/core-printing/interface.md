# Interface

## Main consumers

- host and engine subsystems that print diagnostic or gameplay text.
- runtime code that initializes, clears, dumps, or shuts down the console.
- draw paths that read line data and notify timestamps.

## Main surface

- `NewConsole`, `Init`, `Clear`, `Dump`, `Close`
- printing and log APIs
- line/scrollback state queries such as `GetLine`, `CurrentLine`, `LineWidth`, `TotalLines`, `NotifyTimes`, `ClearNotify`
- package-level singleton wrappers for the same core behavior

## Contracts

- The scrollback ring buffer is space-padded and line-width-dependent.
- Printing updates both text storage and notify state.
- Debug-log output is optional and should not block normal console use when disabled.
