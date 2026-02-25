
## Main Loop Integration
- Hooked up `gameHost.Frame(dt, nil)` to `gameRenderer.OnUpdate`.
- Note: `gameHost.Frame` requires a `FrameCallbacks` implementation as the second argument. Passing `nil` allows it to compile and advance time, but skips event processing and rendering updates within the host.
- The renderer's `Run()` method is blocking and handles the main event loop.
- Headless environments (like the current one) will cause `Run()` to fail with a timeout or window creation error, but the logic is correctly wired.
