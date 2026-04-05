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
The sprite helper in this node applies poster-only OpenGL depth bias during submission. Shared sprite geometry no longer nudges `SPR_ORIENTED` quads forward; this node only owns the final `glPolygonOffset` draw-state toggle while root-level sprite shading and texture policy remain outside the helper.

## Constraints

- draw ordering and probe outputs are parity-sensitive
- helpers here should remain receiver-free or DTO-driven so the parent can keep renderer-state ownership
