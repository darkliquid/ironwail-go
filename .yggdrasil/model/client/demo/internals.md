# Internals

## Logic

The demo path owns both recording and playback-related file/reader state. Recording writes a header and then frame payloads with embedded view angles. Playback tracks frame offsets, playback source ownership, and timedemo counters. Playback startup now exposes an explicit seekable-source seam so higher layers can pass through already-open VFS handles and let `DemoState` own the optional closer for the duration of playback.
Recording path creation now enforces a C-style safety guard by rejecting filenames containing `..` before joining under `demos/`.
Timedemo finalization is centralized in `PrintTimeDemoSummary` / `StopPlaybackWithSummary` so every playback stop path (EOF, explicit stopdemo, rewind/error teardown) can emit the same benchmark summary before state reset.

## Constraints

- Recording and playback modes must not overlap.
- Demo file IO depends on correct frame ordering and message serialization from the runtime/parsing path.

## Decisions

### Dedicated demo state object

Observed decision:
- Demo behavior lives in a separate `DemoState` object instead of being spread only across general client state fields.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- recording/playback IO concerns are separated from the core connection/protocol state
