# Responsibility

## Purpose

`audio/runtime` owns the host-facing sound system, including startup/shutdown, listener state, active channel state, top-level sound update flow, and the adapter used by the host/runtime layer.

Primary evidence:
- `internal/audio/adapter.go`
- `internal/audio/sound.go`
- `internal/audio/types.go`

## Owns

- `System` as the primary runtime state holder.
- `AudioAdapter` as the bridge from host-facing calls to the sound system.
- Channel allocation, start/stop rules, static and ambient channel management.
- Listener state and view-entity state.
- The main `Update` loop that combines listener refresh, sound-time advancement, music fill, and final mixing.
- Audio-related cvar registration and volume control entry points.

## Does not own

- Detailed sample decode rules, mixing math, or codec-specific music parsing beyond delegating to sibling implementation files.
- Concrete backend implementation details beyond backend selection and lifecycle calls.
