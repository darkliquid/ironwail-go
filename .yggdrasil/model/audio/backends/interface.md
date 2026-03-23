# Interface

## Main consumers

- `audio/runtime`, which asks backends to initialize DMA and expose cursor-safe playback output.

## Main API

Observed backend contract:
- initialize backend and DMA info
- lock/unlock around DMA access
- report/play back DMA buffer state
- shutdown cleanly

## Contracts

- The backend owns the output thread/callback mechanics.
- The DMA buffer and sample cursor must stay coherent with the runtime mix path.
- The null backend provides a timing/testing fallback rather than real audio output.
