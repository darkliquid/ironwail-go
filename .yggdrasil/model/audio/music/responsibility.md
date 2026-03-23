# Responsibility

## Purpose

`audio/music` owns music-track selection, file resolution, codec decode, tracker rendering, looping policy, and feeding decoded music into the runtime raw-sample buffer.

## Owns

- CD-style track selection and loop-track policy.
- Explicit filename-based music playback.
- Candidate filename resolution across supported music extensions.
- Decode for OGG, Opus, MP3, FLAC, and tracker-module playback.
- Music state such as active track, paused state, loop state, and position.

## Does not own

- Final DMA mixing of raw samples into the output buffer.
- Hardware backend output.
- General SFX decode and channel spatialization.
