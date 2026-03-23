# Interface

## Main consumers

- `audio/runtime`, which relies on these helpers to populate channel caches, compute per-channel left/right volume, and paint the next slice of audio.

## Main surfaces

- SFX cache creation/loading helpers
- WAV decoding helpers
- spatialization helpers
- mixer pipeline methods that paint channel data and raw/music samples into the output buffer

## Contracts

- SFX cache loading expects valid mono WAV data for effect playback.
- Mixing writes into the DMA buffer format negotiated at runtime.
- Spatialization uses listener vectors and per-channel attenuation to derive final left/right volume.
