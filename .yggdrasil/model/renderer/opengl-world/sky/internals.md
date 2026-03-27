# Internals

## Organization

Mapped files:

- `sky.go`
- `sky_pass.go`
- `sky_upload.go`

## Logic

This node groups the extracted OpenGL sky helper implementations. It is intentionally narrower than the parent: it owns sky-specific helper code, while the parent keeps the lifecycle and renderer-state edges that still need `*Renderer`.

The helpers here already consume shared-world sky policy for embedded-sky splitting, fast-sky flat-color behavior, procedural-sky gating, and cvar-backed sky speeds.

## Constraints

- fast-sky and procedural-sky behavior must remain deterministic
- external skybox helpers must not interfere with embedded-sky fallback behavior
