
## Main Loop Integration
- Hooked up `gameHost.Frame(dt, nil)` to `gameRenderer.OnUpdate`.
- Note: `gameHost.Frame` requires a `FrameCallbacks` implementation as the second argument. Passing `nil` allows it to compile and advance time, but skips event processing and rendering updates within the host.
- The renderer's `Run()` method is blocking and handles the main event loop.
- Headless environments (like the current one) will cause `Run()` to fail with a timeout or window creation error, but the logic is correctly wired.

## Entity/Builtin Wiring
- `internal/qc` has no direct dependency path to `internal/server`, so builtin-to-server integration works best via injectable hook functions owned by `qc` and configured externally.
- Default builtin behavior can still update core edict fields (`origin`, `mins`, `maxs`, `size`, `absmin`, `absmax`, `model`, `modelindex`) in VM memory for deterministic unit tests.
- `ED_ParseEdict` can be made resilient by matching keys to `EntVars` via normalized field names and parsing by reflected field kind (float, int32, vec3).
## Console Commands Implementation
- Implemented , , , , , , , , , .
- Expanded  interface in  to support these commands.
- Added helper methods to .
- Added comprehensive tests in .
- Fixed  to actually call .
- Resolved circular dependency issues by carefully managing imports and interface definitions.
## Console Commands Implementation
- Implemented changelevel, restart, kill, god, noclip, notarget, give, name, color, ping.
- Expanded Server interface in internal/host/init.go to support these commands.
- Added helper methods to internal/server/server.go.
- Added comprehensive tests in internal/host/commands_test.go.
- Fixed CmdMap to actually call SpawnServer.
- Resolved circular dependency issues by carefully managing imports and interface definitions.
