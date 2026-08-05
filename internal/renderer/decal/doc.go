// Package decal implements the CPU-side projected-mark (bullet hole, scorch
// chip, ring, swirl) helpers for the GoGPU/WebGPU renderer: mark lifetime
// management, deterministic atlas seeding, far-to-near draw sorting, and
// quad basis/geometry construction.
//
// # Purpose
//
// This package is the single home for the shared decal math previously
// scattered between the root renderer (decal_shared.go, mark_system.go) and
// internal/renderer/world/gogpu. Plan 16b Step 16.2 unified them here; the
// root package keeps thin shims (mark_system.go, decal_shared.go wrappers)
// and the world/gogpu subpackage keeps only GPU vertex packing
// (DecalVertex, PrepareDecalDraw) which is not duplicated here.
//
// MarkEntity is a tiny interface so the package has no dependency on the
// root renderer's DecalMarkEntity type.
//
// # Original C lineage
//
// C Ironwail has no decal pipeline (OFFSET_DECAL is only a polygon-offset
// constant for SPR_ORIENTED sprites). The Go decal system is a superset kept
// dormant for parity; its quad/basis math follows standard tangent/bitangent
// projection conventions.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/renderer/decal -count=1
package decal
