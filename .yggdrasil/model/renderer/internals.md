# Internals

## Structure

The package is split into focused concerns:
- `renderer/runtime`
- `renderer/canvas`
- `renderer/input`
- `renderer/opengl-core`
- `renderer/opengl-world`
- `renderer/opengl-entities`
- `renderer/gogpu-core`
- `renderer/gogpu-world`
- `renderer/shared-world`
- `renderer/effects`
- `renderer/assets`

## Decisions

### Multi-backend renderer under one package surface

Observed decision:
- The Go port keeps a single renderer package surface while splitting implementation across backend-specific and shared files.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- runtime callers can depend on one renderer package while backend-specific complexity stays internal
