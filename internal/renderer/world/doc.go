// Package world provides shared world-rendering types, cvar accessors,
// fog computation, entity lump parsing, lightmap page management, and
// render-pass classification helpers used by the GoGPU/WebGPU renderer.
//
// # Purpose
//
// This package sits between the BSP data layer (internal/bsp) and the
// GPU renderer (internal/renderer root package). It defines shared
// types (WorldVertex, WorldFace, WorldGeometry, WorldLightmapPage)
// and provides utility functions for fog density conversion, liquid
// alpha settings, and render-pass face classification (sky, turb,
// fence, opaque).
//
// # Original C lineage
//
// Types and helpers here derive from r_local.h, gl_rmain.c,
// gl_rsurf.c, and gl_warp.c. The C engine used global state and
// immediate-mode OpenGL; the Go port packages the shared types
// and logic into a reusable layer.
//
// # Role in the engine
//
// The renderer root package (internal/renderer) imports this package
// for WorldVertex, WorldFace, WorldGeometry, and the various helper
// functions. The world/gogpu sub-package extends these types with
// GoGPU-specific brush entity building logic.
//
// # Key types
//
// - WorldVertex: GPU vertex format (position, texcoord, lightmap UV,
//   normal, lightmap layer, material ID) — 48 bytes, matches WGSL
//   VertexInput struct
// - WorldFace: per-face render metadata (first index, num indices,
//   texture index, lightmap index, flags)
// - WorldGeometry: aggregated BSP geometry (vertices, indices, faces,
//   tree, lightmap pages)
// - WorldLightmapPage: lightmap atlas page with surfaces and RGBA cache
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/renderer/world -count=1
package world
