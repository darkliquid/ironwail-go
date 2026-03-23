# Interface

## Primary consumer

- `internal/host`, which treats `AudioAdapter` as the runtime audio service.

## Main API

### `AudioAdapter`

Observed host-facing methods include:
- `Init() error`
- `Update(...)`
- `Shutdown()`
- sound precache/start/stop helpers
- listener/view-entity setters
- static/ambient sound helpers
- music control methods
- diagnostics (`SoundInfo`, `SoundList`)

### `System`

Observed runtime methods include:
- `Init(backend Backend, sampleRate int, load8Bit bool) error`
- `Startup() error`
- `Shutdown()`
- `PrecacheSound(...)`
- `StartSound(...)`, `StopSound(...)`
- `StartStaticSound(...)`, `ClearStaticSounds()`
- `SetListener(...)`, `SetViewEntity(...)`
- `UpdateFromCVars()`
- `Update(...)`

## Contracts

- `Init` must negotiate a backend before runtime playback can start.
- `Startup` clears runtime playback state and enables sound processing.
- `UpdateFromCVars` clamps cvar-backed mixer state before applying it, including `volume` in the `[0,1]` range and `snd_filterquality` in the `[1,5]` range.
- `Update` is a no-op when the system is not started or is blocked.
- The runtime assumes callers provide loaders for SFX and music assets instead of reaching into the filesystem directly.
