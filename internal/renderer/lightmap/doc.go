// Package lightmap implements the CPU-side lightmap atlas helpers for the
// GoGPU/WebGPU renderer: page compositing, dirty-region tracking, region
// extraction, and the vertical page-stacking layout that works around the
// gogpu Vulkan WriteTexture BaseArrayLayer bug.
//
// # Purpose
//
// The renderer root package (internal/renderer) used to own these helpers in
// world_lightmap_gogpu.go. They are pure functions over
// world.WorldLightmapPage / world.WorldLightmapSurface plus a [256]float32
// lightstyle table, so they moved here with no receiver changes (Plan 16b
// Step 16.1). The root keeps thin wrappers (uploadWorldLightmapArray,
// updateUploadedLightmapsLocked) that own the wgpu texture/bind-group work.
//
// # Original C lineage
//
// Compositing mirrors GL_PackLitSurfaces / GL_FillSurfaceLightmap in
// gl_lightmap.c and gl_rsurf.c (Ironwail). The vertical page stacking is a
// GoGPU-specific layout workaround with no C counterpart.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/renderer/lightmap -count=1
package lightmap
