# Internals

## Logic

### Cache and decode

The cache layer finds or creates named SFX entries, lazily loads decoded sample data, and resamples it to the negotiated DMA rate.

### Spatialization

The spatialization path preserves Quake-style stereo placement:
- full-volume shortcut for the active view entity
- left/right stereo balance from listener-right dot product
- attenuation scaled by source distance

### Painting

The mixer accumulates channel samples plus raw/music samples ahead of playback time, then transfers clamped results into the DMA buffer.

## Constraints

- The runtime effect path is effectively centered on mono WAV decode for SFX.
- Mixing and clamp behavior are tightly coupled to the DMA format and playback cursor semantics.
- Pitch/Doppler state exists in the channel model, but runtime behavior currently favors conservative parity over broad dynamic pitch behavior.

## Decisions

### Explicit paint pipeline instead of opaque backend mixing

Observed decision:
- The package maintains a Quake-style software mix pipeline that fills a DMA-style buffer before backend output.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- backend implementations stay relatively dumb
- most parity-sensitive sound behavior remains testable in Go code
