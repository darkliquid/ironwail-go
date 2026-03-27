# Internals

## Organization

Mapped files:

- `bucket.go`
- `draw.go`
- `pass.go`
- `probe.go`
- `sprite.go`

## Logic

This node groups the extracted helpers that turn already-prepared world data into pass-local draw work. The parent node still decides which passes run and with what renderer state; this node owns the repeated mechanics that were previously duplicated inside those root methods.

## Constraints

- draw ordering and probe outputs are parity-sensitive
- helpers here should remain receiver-free or DTO-driven so the parent can keep renderer-state ownership
