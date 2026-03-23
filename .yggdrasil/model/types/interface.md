# Interface

## Main consumers

- renderer code for view/projection matrices, transforms, and GPU buffer packing.
- client/server/QC code for vector math, angle math, and Quake-compatible scalar helpers.
- any public/importing package that needs shared engine math without depending on `internal/`.

## Main surface

- vector types and helpers: `Vec3`, `Vec3Add`, `Vec3Sub`, `Vec3Scale`, `Vec3Dot`, `Vec3Cross`, `Vec3Len`, `Vec3Normalize`, `Vec3MA`, `Vec3Lerp`, `NewVec3`, plus method forms
- scalar/angle helpers: `Clamp`, `ClampInt`, `AngleMod`, `AngleByte`, `ByteToAngle`, `Lerp`, `NormalizeAngle`, `AngleDifference`, `LerpAngle`, `QRint`, `QLog2`, `QNextPow2`
- angle conventions/constants: `Pitch`, `Yaw`, `Roll`, `VectorAngles`, `AngleVectors`
- matrix types/helpers: `Mat4`, `Vec4`, `Color`, `IdentityMatrix`, `RotationMatrix`, `TranslationMatrix`, `Mat4Multiply`, `Mat4.Mul`, `FrustumMatrix`, `Mat4MulVec4`, `ViewMatrix`, `Mat4.Determinant`, `Mat4ToBytes`

## Contracts

- The package encodes Quake's right-handed coordinate system: X=forward, Y=left, Z=up.
- Euler angles are stored as `{pitch, yaw, roll}`.
- `Mat4` is column-major and `Mat4ToBytes` preserves that layout in little-endian float32 form.
- `FrustumMatrix` expects radians; `ViewMatrix` expects degrees.
