# Responsibility

## Purpose

`engine/loading` owns the generic loader helpers in `internal/engine`: one API for bulk parallel loads that preserve input order and another for streaming work through a worker pipeline.

## Owns

- `LoadFunc[T]` and `LoadResult[T]` contracts.
- `ParallelLoad` bounded fan-out behavior.
- `LoadPipeline[T]` worker lifecycle, input/output channels, and idempotent close semantics.

## Does not own

- Asset format parsing logic itself.
- Cancellation, retry, timeout, or backoff policy.
- Deduplication, caching, or batching beyond caller-provided key lists.
