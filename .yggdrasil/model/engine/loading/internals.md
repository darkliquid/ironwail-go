# Internals

## Logic

`ParallelLoad` preallocates one result slot per input key, bounds concurrency with a semaphore channel, and writes each goroutine's result back to the original input index so callers get deterministic batch ordering even though work ran concurrently. `LoadPipeline` instead owns long-lived worker goroutines reading from an input channel and pushing `LoadResult` values onto an output channel; a `WaitGroup` plus a `sync.Once`-guarded `Close` manage worker shutdown and output-channel closure.

## Constraints

- `LoadPipeline.Send` after `Close` is invalid and will panic because the input channel is closed.
- Callers must keep draining `Results()` until it closes; otherwise workers can block on the output channel and stall `Close()`.
- `LoadPipeline` preserves completion order, not submission order.
- Empty `ParallelLoad` input returns `nil` rather than an empty allocated slice.

## Decisions

### Expose both batch and streaming load patterns instead of one abstraction

Observed decision:
- The package offers a deterministic batch helper and a channel-based streaming pipeline rather than forcing one loading style onto all callers.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Callers can choose between stable ordered batch results and incremental streaming consumption, but they must also understand the different ordering and shutdown semantics of each API.
