# Internals

## Logic

### Startup

`AudioAdapter.Init` selects a backend in priority order:
1. oto
2. null backend

It tries 44.1 kHz first, retries at 48 kHz on failure, and falls back to the null backend if hardware init still fails.
Backend availability and backend-selection traces in `AudioAdapter.Init` are diagnostic and now emit at `Debug`; successful hardware initialization remains visible at `Info`, while degraded/failed startup still uses `Warn`/`Error`.

### Runtime update flow

`System.Update` performs the high-level per-frame audio work:
- refresh listener state
- combine compatible static channels
- update sound time from the backend DMA cursor
- compute a mix horizon
- top up music/raw samples
- paint mixed audio into the DMA buffer

### Shutdown flow

`System.Shutdown` now quiesces playback before backend teardown: it forces mixer volume to zero, blocks backend output, clears active channels plus the DMA buffer, and only then closes the backend. This keeps exit-time teardown from replaying stale mixed samples through the live Oto player while the rest of the process is shutting down.

### Cvar application

`System.UpdateFromCVars` is the runtime bridge from console variables into mixer state. It clamps `volume` to the `[0,1]` range before delegating to `SetVolume`, and it clamps `snd_filterquality` to `[1,5]` before applying the value through the mixer's filter-quality setter.

### Channel policy

- ambient channels occupy the low fixed range
- dynamic channels follow
- static looping channels live after the dynamic range

Runtime channel ownership and replacement rules are therefore a core part of this node’s behavior.

## Constraints

- Runtime processing depends on `started`, `initialized`, and `blocked` state.
- View-entity sounds are spatialized at full volume.
- Backend locking only protects DMA/cursor-sensitive sections; the wider runtime state model assumes a mostly single-owner update path.

## Decisions

### Backend fallback instead of startup failure

Observed decision:
- Audio startup degrades to alternate sample rates and finally a null backend before failing.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- the engine can boot without hardware audio support
