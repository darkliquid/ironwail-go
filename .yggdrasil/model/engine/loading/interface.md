# Interface

## Main consumers

- Asset/bootstrap code that wants concurrency without rewriting worker scaffolding.
- Callers that need either ordered batch completion (`ParallelLoad`) or streaming completion (`LoadPipeline`).

## Main surface

- `LoadFunc`
- `LoadResult`
- `ParallelLoad`
- `NewLoadPipeline`
- `Send`
- `Results`
- `Close`

## Contracts

- Non-positive worker counts are coerced to 1.
- `ParallelLoad` returns one result per input key in input order.
- `LoadPipeline` emits results in completion order and must be drained until closed after `Close`.
