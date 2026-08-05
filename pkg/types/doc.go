// Package types provides fundamental 3D vector, angle, matrix, and primitive types for Quake.
//
// # Purpose
//
// Shared primitive types (Vec3, Angles, Quat, BoundingBox) used across engine subsystems.
//
// # Original C lineage
//
// Mirrored from original Quake / Ironwail C sources:
//   - quakedef.h: Core engine typedefs.
//   - mathlib.c: Vector and matrix arithmetic.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./pkg/types -count=1
package types
