# Responsibility

## Purpose

`types` owns the public math vocabulary and helper routines shared across the engine: Quake coordinate-system vectors, angle conversion/interpolation helpers, and 4x4 matrix math used for view/projection and GPU uploads.

## Owns

- `Vec3`, `Vec4`, `Mat4`, and `Color`.
- Core vector/scalar helpers such as dot/cross/normalize, angle wrapping/interpolation, and Quake-style rounding/power-of-two helpers.
- Matrix construction and multiplication helpers used by renderer/view code.
- Little-endian matrix serialization for GPU upload.
- Tests that lock in Quake conventions and matrix/projection semantics.

## Does not own

- Renderer-specific pipeline logic beyond the math/serialization contract.
- Physics, collision, or networking logic that merely consumes these types/functions.
