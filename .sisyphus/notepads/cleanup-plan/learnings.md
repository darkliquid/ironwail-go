
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

## Cleanup Task Learnings
- The original C source code was located in the `C/` directory and has been removed.
- The `internal/input/types.go` file had a bug where many key constants were assigned the same value (127) because they were in a `const` block without `iota` or explicit values. This was fixed by using `iota` properly.
- The project requires `CGO_ENABLED=0` to build successfully because of a dependency (`github.com/go-webgpu/goffi`) that has issues with CGO on this platform.
- Empty directories (`internal/mathlib`, `internal/zone`, `internal/qcvm`, `internal/render`) were removed.
- Build artifacts like the `ironwailgo` executable in the root were removed.
