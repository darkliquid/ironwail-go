# Interface

## Primary consumers

- `cmd/ironwailgo`, which constructs `Host`, assembles `Subsystems`, and drives `Init`, `Frame`, and `Shutdown`.

## Main API

### `Host`

Key methods:
- `NewHost() *Host`
- `Init(params *InitParams, subs *Subsystems) error`
- `Frame(dt float64, cb FrameCallbacks) error`
- `FrameLoop(targetFPS float64, cb FrameCallbacks, shouldQuit func() bool) error`
- `Shutdown(subs *Subsystems)`
- accessors/mutators for signons, max FPS, abort state, and runtime flags
- `NetInterval() float64` and `LocalServerFast() bool` expose host frame/network policy so runtime client interpolation policy can mirror C `sv.active && !host_netinterval` semantics

Contracts:
- `Init` fails on fatal filesystem/console/server/client/renderer init errors.
- Audio init warnings do not abort startup.
- `Init` always registers host commands and cvar helper commands in the global `cmdsys`, even when `Subsystems.Commands` is nil.
- `Init` synchronizes runtime host player-slot policy into cvars by setting `maxplayers` from the clamped host `maxClients` value.
- `Init` registers the `devstats` cvar (default `0`) as part of baseline host/runtime cvar setup.
- `Frame` expects the caller to supply callbacks that map onto concrete client/server/render/audio work.
- `Shutdown` may use `Host.Subs` when the caller passes `nil`.

### `Subsystems`

Dependency container for the live runtime integrations. The host relies on it for sequencing but not for concrete implementations.

### `FrameCallbacks`

Bridge from host orchestration to executable/runtime code:
- event collection
- console command processing
- server processing
- client processing
- screen update
- audio update
