# Internals

## Logic

### Track selection

The music path supports two modes:
- CD-style track numbers with optional loop-track fallback
- explicit filename playback resolved against supported music extensions

### Decode model

The package decodes supported formats into PCM-like music-track data, then feeds that data into the raw sample buffer used by the runtime mixer.

Tracker modules are rendered through a tracker playback library and converted into PCM sample streams before playback.

### Runtime refill

`updateMusic(endTime)` advances the active track and appends enough decoded frames to keep `RawSamplesBuffer` ahead of the mix horizon.

## Constraints

- Music decode is eager enough that large assets can create load-time or memory costs.
- `JumpMusic` is currently stubbed and does not implement order-jump behavior.
- Runtime music volume behavior depends on how raw samples are added to the mix path.

## Decisions

### Unified music path over multiple codec loaders

Observed decision:
- The package normalizes several codec and tracker formats into one music-track abstraction.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- runtime mixing does not need format-specific logic once a track is resolved
