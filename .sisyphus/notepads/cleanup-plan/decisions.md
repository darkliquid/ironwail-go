
## Main Loop Implementation
- Decided to pass `nil` as the second argument to `gameHost.Frame(dt, nil)` to satisfy the compiler while following the instruction to call `gameHost.Frame(dt)` as closely as possible.
- Added error checking to `gameRenderer.Run()` to ensure failures are logged.
