# Interface

## Main consumers

- `internal/host`, which drives client init, frame/send/read phases, and signon state.
- `internal/renderer`, `internal/hud`, and `internal/audio`, which consume client-maintained state and transient events.

## Main exposed shape

The package exposes:
- persistent client state (`Client`)
- protocol parsing (`Parser`)
- input/usercmd generation
- prediction helpers
- entity relink/interpolation helpers
- demo playback/recording state

Detailed contracts live in the child nodes where those responsibilities are implemented.
