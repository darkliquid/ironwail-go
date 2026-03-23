# Interface

## Consumers

Primary inbound consumer:
- `cmd/ironwailgo`, which constructs `host.Host`, assembles `host.Subsystems`, and drives `Host.Init`, `Host.Frame`, and `Host.Shutdown`.

Secondary consumers:
- Engine commands and runtime callbacks that interact with host-owned state through registered command handlers and callback interfaces.

## Core types

### `Host`

Stateful orchestrator for startup, per-frame sequencing, and shutdown.

Important methods and contracts:
- `NewHost() *Host`
  - Returns a host with conservative defaults: `maxFPS=250`, `netInterval=1/72`, `maxClients=1`, demo loop disabled until explicitly started, and a compatibility RNG.
- `Init(params *InitParams, subs *Subsystems) error`
  - Initializes subsystems in order.
  - Clamps `maxClients` into valid scoreboard range.
  - Resolves `userDir`, creates it if needed, initializes filesystem/commands/console/server/client/audio/renderer, then executes startup config.
  - Fails fast on filesystem, console, server, client, or renderer init errors.
  - Audio init is non-fatal and emits a warning through the console when available.
- `Frame(dt float64, cb FrameCallbacks) error`
  - Advances time, processes console commands, optionally isolates client/server simulation to the configured network interval, updates screen/audio, and increments frame count.
  - Returns nil when the host is already aborted; `FrameLoop` later materializes the abort as `HostError`.
- `FrameLoop(targetFPS float64, cb FrameCallbacks, shouldQuit func() bool) error`
  - Runs the main loop until quit or abort.
- `Shutdown(subs *Subsystems)`
  - Shuts down subsystems in a fixed order and clears initialized state.

State accessors:
- `SetSignOns(count int)` ends the loading plaque when the signon threshold reaches the active client constant.
- `SetMaxFPS(fps float64)` also recomputes whether network simulation stays isolated at 72 Hz or runs per-frame.
- `Abort(reason string)` / `ClearAbort()` / `IsAborted()` expose host-fatal state.

### `Subsystems`

Runtime dependency container owned by `Host`.

Observed responsibilities:
- Carries the active filesystem, command system, console, server, client, renderer, audio, input, and menu integrations.
- Lets host sequence startup/shutdown without hard-coding concrete types into global state.
- Some host features require concrete implementations behind the interfaces, especially autosave/save/load paths that type-assert to `*server.Server` and `*fs.FileSystem`.

### `FrameCallbacks`

Per-frame bridge implemented by the executable layer.

Required callbacks:
- `GetEvents()`
- `ProcessConsoleCommands()`
- `ProcessServer()`
- `ProcessClient()`
- `UpdateScreen()`
- `UpdateAudio(origin, forward, right, up [3]float32)`

Contract:
- Host depends on the caller to map these callbacks onto the correct concrete client/server/render/audio operations.
- Host may invoke `ProcessClient()` twice per frame in different phases (`send`, then `read`) when connected.

## Command-facing surface

Host registers console commands that expose session and lifecycle behavior. Observed command groups include:
- map/session management
- connect/disconnect/reconnect
- save/load/autosave support
- demo playback/record/timedemo
- system/config execution
- audio-related controls

These command handlers are part of the host interface because they are how user input and scripts reach host-owned policy.

## Failure modes

- Startup returns errors for fatal subsystem initialization failures.
- Remote network operations return explicit transport or parse errors.
- Save/load commands reject invalid save names, unsafe paths, missing files, wrong active game directories, and unsupported formats.
- Abort state short-circuits frame progression and later surfaces as `HostError`.
