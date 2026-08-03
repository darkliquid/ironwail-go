// Package gogpu (world/gogpu) provides GoGPU/WebGPU-specific brush entity
// building, translucent face sorting, sprite uniform packing, and shader
// source constants for the world rendering pipeline.
//
// # Purpose
//
// This sub-package extends the shared world types (internal/renderer/world)
// with GoGPU-specific logic: building brush entity vertex/index data from
// BSP submodels, sorting translucent liquid faces for correct depth
// ordering, packing sprite uniform bytes, and declaring WGSL shader
// source strings for the alias model, sprite, and decal pipelines.
//
// # Original C lineage
//
// The brush building logic mirrors R_DrawBrushModel and
// R_DrawBModelTriangles in gl_rsurf.c. The translucent sorting mirrors
// the qsort-based depth sort in R_DrawWaterSurfaces. Sprite uniforms
// and shaders derive from gl_sprite.c and gl_shaders.c.
//
// # Role in the engine
//
// The renderer root package imports this for:
//   - BrushEntityParams / BuildBrushEntityDraw (brush entity geometry)
//   - SpriteUniformBytes / SpriteDraw (sprite rendering data)
//   - AliasVertexShaderWGSL / AliasFragmentShaderWGSL (alias shaders)
//   - SpriteVertexShaderWGSL / SpriteFragmentShaderWGSL (sprite shaders)
//   - DecalVertexShaderWGSL / DecalFragmentShaderWGSL (decal shaders)
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/renderer/world/gogpu -count=1
package gogpu
