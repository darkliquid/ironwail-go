# Interface

## Main consumers

- `internal/host`, which drives demo commands and playback state.
- `client/runtime`, which stores and exposes the active `DemoState`.

## Main API

Observed surfaces:
- `NewDemoState()`
- start/stop recording
- write demo frame helpers
- playback source management and timedemo bookkeeping

## Contracts

- Recording and playback are mutually exclusive.
- Demo file format includes CD track header plus per-frame message size, view angles, and message payload.
