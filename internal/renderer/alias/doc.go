// Package alias implements MDL (alias) model vertex interpolation,
// frame animation, and yaw/angle rotation math for the GoGPU/WebGPU
// renderer's CPU-side vertex transform path.
//
// # Purpose
//
// This package handles the CPU-side computation for alias model
// rendering: decoding compressed TriVertX vertices from MDL files,
// interpolating between keyframes (pose blending), and applying
// yaw/pitch/roll rotations. The result is a set of world-space
// WorldVertex structs uploaded to the GPU each frame.
//
// # Original C lineage
//
// Mirrors r_alias.c (R_AliasSetupFrame, R_AliasDrawModel,
// R_AliasBlendLight, GL_DrawAliasFrame). The C version used
// immediate-mode OpenGL with glBegin/glEnd per triangle; the Go
// version batches all vertices into a single buffer upload.
//
// # Role in the engine
//
// Called by the renderer (internal/renderer) each frame for each
// visible alias entity (monsters, weapons, items, gibs). The
// renderer's DrawContext calls BuildVerticesInterpolatedInto to
// produce world-space vertices, which are then packed into the
// alias scratch buffer and uploaded via queue.WriteBuffer.
//
// # Key types
//
// - Mesh: holds pose data and a MeshRef accessor
// - MeshRef: per-vertex index + texcoord pair
// - AliasEntity: per-entity animation state (frame, lerp, origin, angles)
// - InterpolationData: pose1/pose2 indices and blend factor
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/renderer/alias -count=1
package alias
