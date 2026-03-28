# Internals

## Logic

### SDL3 backend

Uses an audio callback path and updates the playback cursor while consuming the circular DMA buffer.

### oto backend

Uses a goroutine/ticker-driven output loop that reads from the DMA buffer and writes into the oto player/pipe.

### miniaudio backend

Uses a pure-Go runtime-loaded miniaudio device. The playback callback reads from the existing circular DMA byte buffer, decodes little-endian `int16` samples into callback frame slices, and advances the shared sample cursor in lockstep with the runtime mixer.

### Null backend

Advances a fake playback cursor without hardware output so the rest of the sound system can run in tests or unsupported environments.

## Constraints

- Cursor and buffer synchronization are backend responsibilities.
- Backend timing behavior affects how `soundTime` advances and therefore affects mix scheduling.

## Decisions

### Multiple backend implementations behind one DMA contract

Observed decision:
- The Go port supports SDL3, oto, and null output backends behind a common contract.
- The Go port supports SDL3, oto, miniaudio, and null output backends behind a common contract.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- runtime sound logic is backend-agnostic
- platform/audio availability issues can be handled by fallback rather than by build failure
