// Package surface provides lightmap atlas allocation and surface
// texture management for the GoGPU/WebGPU renderer.
//
// # Purpose
//
// This package implements the LightmapAllocator (a skyline bin-packer
// for lightmap sample blocks within fixed-size atlas pages) and the
// SurfaceTexture type (wrapping animated Quake texture chains with
// GoGPU-specific metadata like atlas bounds and layer indices).
//
// # Original C lineage
//
// The lightmap allocator mirrors the block allocator in gl_rsurf.c
// (LM_AllocBlock, LM_InitBlock). The C version used a simple
// greedy 2D bin-packer for lightmap blocks within 128×128 (or
// larger) texture pages. The Go version uses the same algorithm
// but with configurable page sizes.
//
// SurfaceTexture mirrors the texture_t struct in gl_model.h,
// extending it with animation chain tracking and GoGPU bind group
// references.
//
// # Role in the engine
//
// The renderer imports this package during world geometry building
// (world_geometry_gogpu.go) to allocate lightmap blocks for each
// BSP face, and during world upload (world_upload_gogpu.go) to
// manage texture animations.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/renderer/surface -count=1
package surface
