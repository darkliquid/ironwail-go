# Interface

## Main consumers

- `internal/host`, which drives demo commands and playback state.
- `client/runtime`, which stores and exposes the active `DemoState`.

## Main API

Observed surfaces:
- `NewDemoState()`
- start/stop recording
- write demo frame helpers
- playback source management (`StartDemoPlayback`, `StartDemoPlaybackFromData`, `StartDemoPlaybackFromSource`) and timedemo bookkeeping
- timedemo summary helpers (`PrintTimeDemoSummary`, `StopPlaybackWithSummary`) used by host/runtime stop paths

## Contracts

- Recording and playback are mutually exclusive.
- Recording rejects filenames containing `..` to match C path-safety behavior for `record`.
- Demo file format includes CD track header plus per-frame message size, view angles, and message payload.
- `StartDemoPlaybackFromSource` accepts an already-open seekable stream plus optional closer so callers can keep demo playback on top of VFS-owned handles instead of materializing a separate byte slice first.
