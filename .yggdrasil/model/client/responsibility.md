# Responsibility

## Purpose

The `client` node is the umbrella for the client-side Quake subsystem. It covers persistent client state, server-message decoding, interpolation and temp effects, local input/prediction, and demo playback/recording.

## Owns

- The top-level decomposition of client concerns into runtime state, protocol ingestion, input/prediction, and demo playback.
- Package-level responsibility for maintaining what the player knows about the running server and transforming that into render/audio/HUD-facing state.

## Does not own

- Host lifecycle policy beyond the client-facing contracts the host calls.
- Server simulation or authoritative world state.
- Renderer- or audio-specific drawing/playback behavior beyond exposing transient events and client state.
