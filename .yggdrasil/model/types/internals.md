# Internals

## Logic

The package is a single cohesive public math library. `types.go` defines the engine-wide scalar/vector vocabulary and preserves Quake-specific conventions around axes, angle encoding, interpolation, and rounding. `mat4.go` extends that same contract into renderer-facing 4x4 matrix math while deliberately preserving C Ironwail's column-major storage and the split where the view matrix stays in Quake world space and the projection matrix performs the Quake-to-clip-space remap.

## Constraints

- Quake coordinate-system semantics are the core contract: many helpers only make sense if callers respect X=forward, Y=left, Z=up and `{pitch, yaw, roll}` angle ordering.
- `AngleMod` intentionally preserves Quake's 16-bit quantized C behavior, not just a mathematically convenient modulo.
- `RotationMatrix` uses raw axis ids `0/1/2` without extra validation.
- `QNextPow2` returns `1` for non-positive inputs and `Vec3Normalize` leaves the zero vector unchanged.
- `Mat4ToBytes` is a GPU-facing ABI detail: 16 float32 values, little-endian, column-major, 64 bytes total.

## Decisions

### Keep vector/angle helpers and matrix helpers in one public package

Observed decision:
- The repo exposes one public `pkg/types` package that combines core Quake vector/angle math with renderer-facing matrix math.

Rationale:
- **unknown — inferred from code and package role, not confirmed by a developer**

Observed effect:
- Importers get one shared math vocabulary across subsystems, and the most important invariants span both files, so splitting the graph further would hide the single Quake-coordinate-system contract rather than clarifying it.

### Bake Quake-to-clip-space remapping into projection, not view

Observed decision:
- `ViewMatrix` stays in Quake world-space semantics, while `FrustumMatrix` performs the Quake-to-clip remap.

Rationale:
- **unknown — inferred from code comments and implementation, not confirmed by a developer**

Observed effect:
- The projection/view contract differs from textbook OpenGL expectations, so this behavior must be documented explicitly for future renderer or math work.
