# Internals

## Logic

### Startup

`AudioAdapter.Init` selects a backend in priority order:
1. SDL3
2. oto
3. null backend

It tries 44.1 kHz first, retries at 48 kHz on failure, and falls back to the null backend if hardware init still fails.

### Runtime update flow

`System.Update` performs the high-level per-frame audio work:
- refresh listener state
- combine compatible static channels
- update sound time from the backend DMA cursor
- compute a mix horizon
- top up music/raw samples
- paint mixed audio into the DMA buffer

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
