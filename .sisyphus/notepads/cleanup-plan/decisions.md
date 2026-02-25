
## Main Loop Implementation
- Decided to pass `nil` as the second argument to `gameHost.Frame(dt, nil)` to satisfy the compiler while following the instruction to call `gameHost.Frame(dt)` as closely as possible.
- Added error checking to `gameRenderer.Run()` to ensure failures are logged.

## Entity/Builtin Integration
- Added `SetServerBuiltinHooks` in `internal/qc` as the bridge point so `spawn/remove/setorigin/setsize/setmodel` can delegate to server behavior when server wiring is available.
- Kept a VM-only fallback path in those builtins so unit tests remain isolated and deterministic without requiring full server initialization.
- Implemented `SV_UnlinkEdict` inside `EntityManager` and called it from free/clear paths to avoid stale area linkage pointers during entity lifecycle transitions.
