# Internals

## Logic

### oto backend

Uses a goroutine/ticker-driven output loop that reads from the DMA buffer and writes into the oto player/pipe.

### Null backend

Advances a fake playback cursor without hardware output so the rest of the sound system can run in tests or unsupported environments.

## Constraints

- Cursor and buffer synchronization are backend responsibilities.
- Backend timing behavior affects how `soundTime` advances and therefore affects mix scheduling.

## Decisions

### Canonical Oto backend behind one DMA contract

Observed decision:
- The Go port supports Oto and null output backends behind a common contract, with Oto as the canonical hardware path.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- runtime sound logic is backend-agnostic
- hardware audio remains available in default builds while platform/audio availability issues can still degrade to null output rather than build failure
