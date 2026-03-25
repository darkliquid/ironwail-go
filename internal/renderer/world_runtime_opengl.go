//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"encoding/binary"
	"fmt"
	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/model"
	"log/slog"
	"sort"
	"strings"
)

type glWorldMesh struct {
	vao           uint32
	vbo           uint32
	ebo           uint32
	indexCount    int32
	hasLitWater   bool
	faces         []WorldFace
	lightmaps     []uint32
	lightmapPages []WorldLightmapPage
}

// flattenWorldVertices converts WorldVertex structs to a flat float32 array for GL buffer upload. Layout: 3 position + 2 texcoord + 2 lightmap UV + 3 normal = 10 floats per vertex.
func flattenWorldVertices(vertices []WorldVertex) []float32 {
	data := make([]float32, 0, len(vertices)*10)
	for _, v := range vertices {
		data = append(data,
			v.Position[0], v.Position[1], v.Position[2],
			v.TexCoord[0], v.TexCoord[1],
			v.LightmapCoord[0], v.LightmapCoord[1],
			v.Normal[0], v.Normal[1], v.Normal[2],
		)
	}
	return data
}

// uploadWorldMesh uploads BSP geometry (vertices + indices) to GPU buffers, creating a VAO with the standard world vertex layout. Returns a glWorldMesh handle for later binding during draw calls.
func uploadWorldMesh(vertices []WorldVertex, indices []uint32) *glWorldMesh {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil
	}

	vertexData := flattenWorldVertices(vertices)
	mesh := &glWorldMesh{indexCount: int32(len(indices))}

	gl.GenVertexArrays(1, &mesh.vao)
	gl.GenBuffers(1, &mesh.vbo)
	gl.GenBuffers(1, &mesh.ebo)

	gl.BindVertexArray(mesh.vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, mesh.vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.STATIC_DRAW)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, mesh.ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	const stride = 10 * 4
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(1, 2, gl.FLOAT, false, stride, 3*4)
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(2, 2, gl.FLOAT, false, stride, 5*4)
	gl.EnableVertexAttribArray(3)
	gl.VertexAttribPointerWithOffset(3, 3, gl.FLOAT, false, stride, 7*4)

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, 0)

	return mesh
}

// destroy releases the GL resources (VAO, VBO, EBO) for a world mesh.
func (mesh *glWorldMesh) destroy() {
	if mesh == nil {
		return
	}
	if mesh.vao != 0 {
		gl.DeleteVertexArrays(1, &mesh.vao)
		mesh.vao = 0
	}
	if mesh.vbo != 0 {
		gl.DeleteBuffers(1, &mesh.vbo)
		mesh.vbo = 0
	}
	if mesh.ebo != 0 {
		gl.DeleteBuffers(1, &mesh.ebo)
		mesh.ebo = 0
	}
	for i, tex := range mesh.lightmaps {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
			mesh.lightmaps[i] = 0
		}
	}
}

// ensureBrushModelLocked lazily builds and uploads GPU geometry for a BSP submodel (doors, platforms, lifts). Each brush entity references a submodel by index.
func (r *Renderer) ensureBrushModelLocked(submodelIndex int) *glWorldMesh {
	if mesh, ok := r.brushModels[submodelIndex]; ok && mesh != nil {
		return mesh
	}
	tree := r.worldTree
	if tree == nil {
		return nil
	}
	renderData, err := buildModelRenderData(tree, submodelIndex)
	if err != nil {
		slog.Warn("OpenGL brush model build failed", "submodel", submodelIndex, "error", err)
		return nil
	}
	if renderData == nil || renderData.Geometry == nil || len(renderData.Geometry.Vertices) == 0 || len(renderData.Geometry.Indices) == 0 {
		return nil
	}
	mesh := uploadWorldMesh(renderData.Geometry.Vertices, renderData.Geometry.Indices)
	if mesh == nil {
		return nil
	}
	mesh.faces = append(mesh.faces, renderData.Geometry.Faces...)
	mesh.hasLitWater = renderData.HasLitWater
	mesh.lightmapPages = append(mesh.lightmapPages, renderData.Lightmaps...)
	mesh.lightmaps = uploadLightmapPages(renderData.Lightmaps, r.lightStyleValues)
	r.brushModels[submodelIndex] = mesh
	return mesh
}

// worldTextureFilters returns GL texture filter parameters: lightmaps use LINEAR for smooth interpolation; diffuse textures use NEAREST_MIPMAP_LINEAR for Quake's pixel-art look with distance mipmapping.
func worldTextureFilters(lightmap bool) (minFilter, magFilter int32) {
	if lightmap {
		return gl.LINEAR, gl.LINEAR
	}
	return parseGLTextureMode(cvar.StringValue(CvarGLTextureMode))
}

func parseGLTextureMode(mode string) (minFilter, magFilter int32) {
	switch strings.ToUpper(strings.TrimSpace(mode)) {
	case "GL_NEAREST":
		return gl.NEAREST, gl.NEAREST
	case "GL_LINEAR":
		return gl.LINEAR, gl.LINEAR
	case "GL_NEAREST_MIPMAP_NEAREST":
		return gl.NEAREST_MIPMAP_NEAREST, gl.NEAREST
	case "GL_LINEAR_MIPMAP_NEAREST":
		return gl.LINEAR_MIPMAP_NEAREST, gl.LINEAR
	case "GL_LINEAR_MIPMAP_LINEAR":
		return gl.LINEAR_MIPMAP_LINEAR, gl.LINEAR
	default:
		return gl.NEAREST_MIPMAP_LINEAR, gl.NEAREST
	}
}

func readTextureAnisotropy() float32 {
	raw := float32(cvar.FloatValue(CvarGLAnisotropy))
	if raw < 1 {
		return 1
	}
	return raw
}

func readTextureLodBias() float32 {
	return float32(cvar.FloatValue(CvarGLLodBias))
}

// uploadWorldTextureRGBAWithFilters creates a GL texture from RGBA data with specified min/mag filters and generates mipmaps to reduce aliasing at distance.
func uploadWorldTextureRGBAWithFilters(width, height int, rgba []byte, minFilter, magFilter int32) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, minFilter)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, magFilter)
	gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_LOD_BIAS, readTextureLodBias())
	gl.TexParameterf(gl.TEXTURE_2D, gl.TEXTURE_MAX_ANISOTROPY, readTextureAnisotropy())
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	return tex
}

// uploadWorldTextureRGBA uploads a world diffuse texture with NEAREST filtering for Quake's pixel-art aesthetic.
func uploadWorldTextureRGBA(width, height int, rgba []byte) uint32 {
	minFilter, magFilter := worldTextureFilters(false)
	return uploadWorldTextureRGBAWithFilters(width, height, rgba, minFilter, magFilter)
}

// uploadWorldLightmapTextureRGBA uploads a lightmap texture with LINEAR filtering for smooth lighting gradients.
func uploadWorldLightmapTextureRGBA(width, height int, rgba []byte) uint32 {
	minFilter, magFilter := worldTextureFilters(true)
	return uploadWorldTextureRGBAWithFilters(width, height, rgba, minFilter, magFilter)
}

// ensureWorldFallbackTextureLocked creates a 1x1 white fallback texture for faces missing their texture data, ensuring the shader always has a valid texture bound.
func (r *Renderer) ensureWorldFallbackTextureLocked() {
	if r.worldFallbackTexture != 0 {
		return
	}
	r.worldFallbackTexture = uploadWorldTextureRGBA(1, 1, []byte{200, 200, 200, 255})
}

// ensureLightmapFallbackTextureLocked creates a 1x1 white fallback lightmap so unlit faces render at full brightness.
func (r *Renderer) ensureLightmapFallbackTextureLocked() {
	if r.worldLightmapFallback != 0 {
		return
	}
	r.worldLightmapFallback = uploadWorldLightmapTextureRGBA(1, 1, []byte{255, 255, 255, 255})
}

// setLightStyleValues updates the 64-element lightstyle brightness array. Quake's lightstyle system animates lighting using 64 independent channels for effects like flickering torches and pulsing lights.
func (r *Renderer) setLightStyleValues(values [64]float32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Mark surfaces dirty where any referenced lightstyle changed.
	changed := lightStylesChanged(r.lightStyleValues, values)
	if r.worldData != nil {
		markDirtyLightmapPages(r.worldData.Lightmaps, changed)
	}
	for _, mesh := range r.brushModels {
		if mesh == nil {
			continue
		}
		markDirtyLightmapPages(mesh.lightmapPages, changed)
	}

	r.lightStyleValues = values
	r.updateUploadedLightmapsLocked()
}

// defaultLightStyleValues returns the default lightstyle array where style 0 has brightness 1.0 (normal) and all others are 0.
func defaultLightStyleValues() [64]float32 {
	var values [64]float32
	values[0] = 1
	return values
}

// uploadWorldTexturesLocked uploads all world textures to the GPU: converts from palette to RGBA, extracts fullbright masks (palette indices 224-254 glow in the dark), splits sky textures into layers, and uploads to GL textures. Called once per map load.
func (r *Renderer) uploadWorldTexturesLocked(tree *bsp.Tree) error {
	r.worldTextures = make(map[int32]uint32)
	r.worldFullbrightTextures = make(map[int32]uint32)
	r.worldSkySolidTextures = make(map[int32]uint32)
	r.worldSkyAlphaTextures = make(map[int32]uint32)
	r.worldSkyFlatTextures = make(map[int32]uint32)
	r.worldTextureAnimations = nil
	r.ensureWorldFallbackTextureLocked()
	r.ensureWorldSkyFallbackTexturesLocked()

	if tree == nil || len(tree.TextureData) < 4 {
		return nil
	}

	palette := append([]byte(nil), r.palette...)
	count := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if count <= 0 || len(tree.TextureData) < 4+count*4 {
		return nil
	}
	textureNames := make([]string, count)

	for i := 0; i < count; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			slog.Debug("OpenGL world texture parse failed", "index", i, "error", err)
			continue
		}
		textureNames[i] = miptex.Name
		pixels, width, height, err := miptex.MipLevel(0)
		if err != nil || width <= 0 || height <= 0 {
			continue
		}
		rgba := ConvertPaletteToRGBA(pixels, palette)
		tex := uploadWorldTextureRGBA(width, height, rgba)
		if tex != 0 {
			r.worldTextures[int32(i)] = tex
		}
		// Check for fullbright pixels and create fullbright texture
		fbRGBA, hasFB := ConvertPaletteToFullbrightRGBA(pixels, palette)
		if hasFB {
			fbTex := uploadWorldTextureRGBA(width, height, fbRGBA)
			if fbTex != 0 {
				r.worldFullbrightTextures[int32(i)] = fbTex
			}
		}
		if classifyWorldTextureName(miptex.Name) != model.TexTypeSky {
			continue
		}
		solidRGBA, alphaRGBA, layerWidth, layerHeight, ok := extractEmbeddedSkyLayers(
			pixels,
			width,
			height,
			palette,
			shouldSplitAsQuake64Sky(tree.Version, width, height),
		)
		if !ok {
			continue
		}
		solidTex := uploadWorldTextureRGBA(layerWidth, layerHeight, solidRGBA)
		alphaTex := uploadWorldTextureRGBA(layerWidth, layerHeight, alphaRGBA)
		if solidTex != 0 {
			r.worldSkySolidTextures[int32(i)] = solidTex
		}
		if alphaTex != 0 {
			r.worldSkyAlphaTextures[int32(i)] = alphaTex
		}
		flatRGBA := buildSkyFlatRGBA(alphaRGBA)
		flatTex := uploadWorldTextureRGBA(1, 1, flatRGBA[:])
		if flatTex != 0 {
			r.worldSkyFlatTextures[int32(i)] = flatTex
		}
	}

	animations, err := BuildTextureAnimations(textureNames)
	if err != nil {
		return fmt.Errorf("build world texture animations: %w", err)
	}
	r.worldTextureAnimations = animations
	return nil
}

var skyboxCubemapTargets = [...]uint32{
	gl.TEXTURE_CUBE_MAP_POSITIVE_X,
	gl.TEXTURE_CUBE_MAP_NEGATIVE_X,
	gl.TEXTURE_CUBE_MAP_POSITIVE_Y,
	gl.TEXTURE_CUBE_MAP_NEGATIVE_Y,
	gl.TEXTURE_CUBE_MAP_POSITIVE_Z,
	gl.TEXTURE_CUBE_MAP_NEGATIVE_Z,
}

// lightStylesChanged returns a 64-element bitmask indicating which lightstyle
// indices changed between the old and new value arrays.
func lightStylesChanged(old, new_ [64]float32) [64]bool {
	var changed [64]bool
	for i := range old {
		if old[i] != new_[i] {
			changed[i] = true
		}
	}
	return changed
}

// markDirtyLightmapPages sets the Dirty flag on surfaces and pages where
// any referenced lightstyle changed. This enables partial re-upload:
// only dirty pages need recompositing.
func markDirtyLightmapPages(pages []WorldLightmapPage, changed [64]bool) {
	for i := range pages {
		pageDirty := false
		for j := range pages[i].Surfaces {
			surf := &pages[i].Surfaces[j]
			for _, style := range surf.Styles {
				if style == 255 {
					break
				}
				if style < 64 && changed[style] {
					surf.Dirty = true
					pageDirty = true
					break
				}
			}
		}
		if pageDirty {
			pages[i].Dirty = true
		}
	}
}

// clearDirtyFlags resets Dirty flags on all surfaces and pages after
// their lightmaps have been recomposited and uploaded.
func clearDirtyFlags(pages []WorldLightmapPage) {
	for i := range pages {
		if !pages[i].Dirty {
			continue
		}
		for j := range pages[i].Surfaces {
			pages[i].Surfaces[j].Dirty = false
		}
		pages[i].Dirty = false
	}
}

// compositeSurfaceRGBA blends a single surface's lightmap samples into an
// existing RGBA atlas buffer. Each surface can reference up to 4 lightstyles
// whose brightness values are looked up from the values array.
func compositeSurfaceRGBA(rgba []byte, pageWidth int, surface WorldLightmapSurface, values [64]float32) {
	if surface.Width <= 0 || surface.Height <= 0 {
		return
	}
	styleCount := 0
	for _, style := range surface.Styles {
		if style == 255 {
			break
		}
		styleCount++
	}
	if styleCount == 0 {
		styleCount = 1
	}
	faceSize := surface.Width * surface.Height * 3
	if len(surface.Samples) < faceSize*styleCount {
		return
	}
	for y := 0; y < surface.Height; y++ {
		for x := 0; x < surface.Width; x++ {
			sampleIndex := (y*surface.Width + x) * 3
			var rSum, gSum, bSum float32
			for styleIndex := 0; styleIndex < styleCount; styleIndex++ {
				offset := styleIndex*faceSize + sampleIndex
				scale := lightstyleScale(values, surface.Styles[styleIndex])
				rSum += float32(surface.Samples[offset]) * scale
				gSum += float32(surface.Samples[offset+1]) * scale
				bSum += float32(surface.Samples[offset+2]) * scale
			}
			dst := ((surface.Y+y)*pageWidth + (surface.X + x)) * 4
			rgba[dst] = byte(clamp01(rSum/255.0) * 255)
			rgba[dst+1] = byte(clamp01(gSum/255.0) * 255)
			rgba[dst+2] = byte(clamp01(bSum/255.0) * 255)
		}
	}
}

// buildLightmapPageRGBA rasterizes a lightmap atlas page to RGBA by blending all surface lightmap samples with their lightstyle brightness values. This CPU-side compositing makes Quake's animated lighting work: each surface can reference up to 4 lightstyles. The result is cached in page.rgba for subsequent partial updates.
func buildLightmapPageRGBA(page *WorldLightmapPage, values [64]float32) []byte {
	if page.Width <= 0 || page.Height <= 0 {
		return nil
	}
	rgba := make([]byte, page.Width*page.Height*4)
	for i := 0; i < len(rgba); i += 4 {
		rgba[i] = 255
		rgba[i+1] = 255
		rgba[i+2] = 255
		rgba[i+3] = 255
	}

	for _, surface := range page.Surfaces {
		compositeSurfaceRGBA(rgba, page.Width, surface, values)
	}

	page.rgba = rgba
	return rgba
}

// recompositeDirtySurfaces updates only the dirty surface regions in an
// existing RGBA atlas buffer. Returns true if any surfaces were recomposited.
// This avoids rebuilding the entire page when only a few surfaces changed.
func recompositeDirtySurfaces(rgba []byte, page WorldLightmapPage, values [64]float32) bool {
	recomposited := false
	for _, surface := range page.Surfaces {
		if !surface.Dirty {
			continue
		}
		compositeSurfaceRGBA(rgba, page.Width, surface, values)
		recomposited = true
	}
	return recomposited
}

// uploadLightmapPages uploads all lightmap atlas pages as GL textures with LINEAR filtering.
func uploadLightmapPages(pages []WorldLightmapPage, values [64]float32) []uint32 {
	textures := make([]uint32, 0, len(pages))
	for i := range pages {
		rgba := buildLightmapPageRGBA(&pages[i], values)
		if len(rgba) == 0 {
			continue
		}
		textures = append(textures, uploadWorldLightmapTextureRGBA(pages[i].Width, pages[i].Height, rgba))
	}
	return textures
}

// dirtyBounds computes the bounding rectangle of all dirty surfaces in a page.
// Returns x, y, width, height. If no surfaces are dirty, returns zeros.
func dirtyBounds(page WorldLightmapPage) (x, y, w, h int) {
	minX, minY := page.Width, page.Height
	maxX, maxY := 0, 0
	found := false
	for _, s := range page.Surfaces {
		if !s.Dirty || s.Width <= 0 || s.Height <= 0 {
			continue
		}
		if s.X < minX {
			minX = s.X
		}
		if s.Y < minY {
			minY = s.Y
		}
		if s.X+s.Width > maxX {
			maxX = s.X + s.Width
		}
		if s.Y+s.Height > maxY {
			maxY = s.Y + s.Height
		}
		found = true
	}
	if !found {
		return 0, 0, 0, 0
	}
	return minX, minY, maxX - minX, maxY - minY
}

// updateLightmapTextures re-uploads lightmap textures for dirty pages only.
// Uses cached RGBA buffers and partial glTexSubImage2D to minimize both CPU
// recomposition and GPU upload bandwidth.
func updateLightmapTextures(textures []uint32, pages []WorldLightmapPage, values [64]float32) {
	count := len(textures)
	if len(pages) < count {
		count = len(pages)
	}
	for i := 0; i < count; i++ {
		if textures[i] == 0 || !pages[i].Dirty {
			continue
		}

		// If we have a cached RGBA buffer, do partial recomposition.
		if pages[i].rgba != nil {
			recompositeDirtySurfaces(pages[i].rgba, pages[i], values)

			// Compute dirty bounding box and upload only that region.
			dx, dy, dw, dh := dirtyBounds(pages[i])
			if dw > 0 && dh > 0 {
				gl.BindTexture(gl.TEXTURE_2D, textures[i])
				// glTexSubImage2D expects a pointer to the sub-rectangle's first pixel.
				// Since our buffer is row-major for the full page, we use GL_UNPACK_ROW_LENGTH.
				gl.PixelStorei(gl.UNPACK_ROW_LENGTH, int32(pages[i].Width))
				offset := (dy*pages[i].Width + dx) * 4
				gl.TexSubImage2D(gl.TEXTURE_2D, 0, int32(dx), int32(dy), int32(dw), int32(dh),
					gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(pages[i].rgba[offset:]))
				gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)
			}
			continue
		}

		// No cached buffer — full rebuild (first frame after level load edge case).
		rgba := buildLightmapPageRGBA(&pages[i], values)
		if len(rgba) == 0 {
			continue
		}
		gl.BindTexture(gl.TEXTURE_2D, textures[i])
		gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, int32(pages[i].Width), int32(pages[i].Height),
			gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba))
	}
	if count > 0 {
		gl.BindTexture(gl.TEXTURE_2D, 0)
	}
	clearDirtyFlags(pages)
}

// updateUploadedLightmapsLocked rebuilds and re-uploads all lightmap pages with current lightstyle values.
func (r *Renderer) updateUploadedLightmapsLocked() {
	values := r.lightStyleValues
	if r.worldData != nil {
		updateLightmapTextures(r.worldLightmaps, r.worldData.Lightmaps, values)
	}
	for _, mesh := range r.brushModels {
		if mesh == nil {
			continue
		}
		updateLightmapTextures(mesh.lightmaps, mesh.lightmapPages, values)
	}
}

// UploadWorld builds CPU geometry and uploads it to OpenGL buffers.
func (r *Renderer) UploadWorld(tree *bsp.Tree) error {
	if tree == nil {
		return fmt.Errorf("nil BSP tree")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.clearWorldLocked()
	r.worldLiquidAlphaOverrides = parseWorldspawnLiquidAlphaOverrides(tree.Entities)
	r.worldSkyFogOverride = parseWorldspawnSkyFogOverride(tree.Entities)

	renderData, err := buildWorldRenderData(tree)
	if err != nil {
		return fmt.Errorf("build world render data: %w", err)
	}
	r.worldLiquidFaceTypes = 0
	if renderData.Geometry != nil {
		r.worldLiquidFaceTypes = worldLiquidFaceTypeMask(renderData.Geometry.Faces)
	}
	if renderData.Geometry == nil || len(renderData.Geometry.Vertices) == 0 || len(renderData.Geometry.Indices) == 0 {
		r.worldData = renderData
		r.worldHasLitWater = renderData.HasLitWater
		return nil
	}

	if err := r.ensureWorldProgram(); err != nil {
		return err
	}
	if err := r.ensureWorldSkyPrograms(); err != nil {
		return err
	}
	r.worldTree = tree
	if err := r.uploadWorldTexturesLocked(tree); err != nil {
		return err
	}
	r.ensureLightmapFallbackTextureLocked()
	worldMesh := uploadWorldMesh(renderData.Geometry.Vertices, renderData.Geometry.Indices)
	if worldMesh == nil {
		return fmt.Errorf("upload world mesh: no geometry uploaded")
	}
	r.worldVAO = worldMesh.vao
	r.worldVBO = worldMesh.vbo
	r.worldEBO = worldMesh.ebo

	r.worldData = renderData
	r.worldHasLitWater = renderData.HasLitWater
	r.worldIndexCount = worldMesh.indexCount
	r.worldLightmaps = uploadLightmapPages(renderData.Lightmaps, r.lightStyleValues)

	slog.Info("OpenGL world uploaded",
		"vertices", renderData.TotalVertices,
		"indices", renderData.TotalIndices,
		"faces", renderData.TotalFaces,
		"boundsMin", renderData.BoundsMin,
		"boundsMax", renderData.BoundsMax,
	)
	return nil
}

// HasWorldData reports whether the OpenGL world path has uploaded geometry.
func (r *Renderer) HasWorldData() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData != nil && r.worldVAO != 0 && r.worldProgram != 0 && r.worldIndexCount > 0
}

// hasTranslucentWorldLiquidFaces checks if any world liquid faces would render with alpha < 1.0 at current cvar settings.
func (r *Renderer) hasTranslucentWorldLiquidFaces() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	liquidFaceTypes := r.worldLiquidFaceTypes
	liquidAlphaOverrides := r.worldLiquidAlphaOverrides
	worldTree := r.worldTree
	r.mu.RUnlock()
	if liquidFaceTypes == 0 {
		return false
	}
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(liquidAlphaOverrides, worldTree)
	return hasTranslucentWorldLiquidFaceType(liquidFaceTypes, liquidAlpha)
}

// GetWorldBounds returns the bounds of the uploaded world geometry.
func (r *Renderer) GetWorldBounds() (min [3]float32, max [3]float32, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.worldData == nil || r.worldData.TotalVertices == 0 {
		return min, max, false
	}
	return r.worldData.BoundsMin, r.worldData.BoundsMax, true
}

// clearWorldLocked releases all world GPU resources: textures, lightmaps, shader programs, VAOs/VBOs, and all cached brush/alias/sprite model data.
func (r *Renderer) clearWorldLocked() {
	if r.worldVAO != 0 {
		gl.DeleteVertexArrays(1, &r.worldVAO)
		r.worldVAO = 0
	}
	if r.worldVBO != 0 {
		gl.DeleteBuffers(1, &r.worldVBO)
		r.worldVBO = 0
	}
	if r.worldEBO != 0 {
		gl.DeleteBuffers(1, &r.worldEBO)
		r.worldEBO = 0
	}
	if r.worldProgram != 0 {
		gl.DeleteProgram(r.worldProgram)
		r.worldProgram = 0
	}
	if r.worldSkyProgram != 0 {
		gl.DeleteProgram(r.worldSkyProgram)
		r.worldSkyProgram = 0
	}
	if r.worldSkyProceduralProgram != 0 {
		gl.DeleteProgram(r.worldSkyProceduralProgram)
		r.worldSkyProceduralProgram = 0
	}
	if r.worldSkyCubemapProgram != 0 {
		gl.DeleteProgram(r.worldSkyCubemapProgram)
		r.worldSkyCubemapProgram = 0
	}
	if r.worldSkyExternalFaceProgram != 0 {
		gl.DeleteProgram(r.worldSkyExternalFaceProgram)
		r.worldSkyExternalFaceProgram = 0
	}
	for idx, mesh := range r.brushModels {
		if mesh != nil {
			mesh.destroy()
		}
		delete(r.brushModels, idx)
	}
	for textureIndex, tex := range r.worldTextures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
		}
		delete(r.worldTextures, textureIndex)
	}
	for textureIndex, tex := range r.worldFullbrightTextures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
		}
		delete(r.worldFullbrightTextures, textureIndex)
	}
	for textureIndex, tex := range r.worldSkySolidTextures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
		}
		delete(r.worldSkySolidTextures, textureIndex)
	}
	for textureIndex, tex := range r.worldSkyAlphaTextures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
		}
		delete(r.worldSkyAlphaTextures, textureIndex)
	}
	for textureIndex, tex := range r.worldSkyFlatTextures {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
		}
		delete(r.worldSkyFlatTextures, textureIndex)
	}
	r.worldTextureAnimations = nil
	for i, tex := range r.worldLightmaps {
		if tex != 0 {
			gl.DeleteTextures(1, &tex)
		}
		r.worldLightmaps[i] = 0
	}
	r.worldLightmaps = nil
	if r.worldFallbackTexture != 0 {
		gl.DeleteTextures(1, &r.worldFallbackTexture)
		r.worldFallbackTexture = 0
	}
	if r.worldLightmapFallback != 0 {
		gl.DeleteTextures(1, &r.worldLightmapFallback)
		r.worldLightmapFallback = 0
	}
	if r.worldSkyAlphaFallback != 0 {
		gl.DeleteTextures(1, &r.worldSkyAlphaFallback)
		r.worldSkyAlphaFallback = 0
	}
	if r.aliasShadowTexture != 0 {
		gl.DeleteTextures(1, &r.aliasShadowTexture)
		r.aliasShadowTexture = 0
	}
	if r.worldSkyExternalCubemap != 0 {
		gl.DeleteTextures(1, &r.worldSkyExternalCubemap)
		r.worldSkyExternalCubemap = 0
	}
	for i := range r.worldSkyExternalFaceTextures {
		if r.worldSkyExternalFaceTextures[i] != 0 {
			gl.DeleteTextures(1, &r.worldSkyExternalFaceTextures[i])
			r.worldSkyExternalFaceTextures[i] = 0
		}
	}
	r.worldVPUniform = -1
	r.worldTextureUniform = -1
	r.worldLightmapUniform = -1
	r.worldFullbrightUniform = -1
	r.worldHasFullbrightUniform = -1
	r.worldSkyVPUniform = -1
	r.worldSkySolidUniform = -1
	r.worldSkyAlphaUniform = -1
	r.worldSkyProceduralVPUniform = -1
	r.worldSkyProceduralModelOffset = -1
	r.worldSkyProceduralModelRotation = -1
	r.worldSkyProceduralModelScale = -1
	r.worldSkyProceduralCameraOrigin = -1
	r.worldSkyProceduralFogColor = -1
	r.worldSkyProceduralFogDensity = -1
	r.worldSkyProceduralHorizonColor = -1
	r.worldSkyProceduralZenithColor = -1
	r.worldSkyCubemapVPUniform = -1
	r.worldSkyCubemapUniform = -1
	r.worldSkyExternalFaceVPUniform = -1
	r.worldSkyExternalFaceRTUniform = -1
	r.worldSkyExternalFaceBKUniform = -1
	r.worldSkyExternalFaceLFUniform = -1
	r.worldSkyExternalFaceFTUniform = -1
	r.worldSkyExternalFaceUPUniform = -1
	r.worldSkyExternalFaceDNUniform = -1
	r.worldModelOffsetUniform = -1
	r.worldModelRotationUniform = -1
	r.worldModelScaleUniform = -1
	r.worldSkyModelOffsetUniform = -1
	r.worldSkyModelRotationUniform = -1
	r.worldSkyModelScaleUniform = -1
	r.worldSkyCubemapModelOffsetUniform = -1
	r.worldSkyCubemapModelRotationUniform = -1
	r.worldSkyCubemapModelScaleUniform = -1
	r.worldSkyExternalFaceModelOffset = -1
	r.worldSkyExternalFaceModelRotation = -1
	r.worldSkyExternalFaceModelScale = -1
	r.worldAlphaUniform = -1
	r.worldTimeUniform = -1
	r.worldSkyTimeUniform = -1
	r.worldSkySolidLayerSpeedUniform = -1
	r.worldSkyAlphaLayerSpeedUniform = -1
	r.worldTurbulentUniform = -1
	r.worldLitWaterUniform = -1
	r.worldCameraOriginUniform = -1
	r.worldSkyCameraOriginUniform = -1
	r.worldSkyCubemapCameraOriginUniform = -1
	r.worldSkyExternalFaceCameraOrigin = -1
	r.worldFogColorUniform = -1
	r.worldSkyFogColorUniform = -1
	r.worldSkyCubemapFogColorUniform = -1
	r.worldSkyExternalFaceFogColor = -1
	r.worldFogDensityUniform = -1
	r.worldSkyFogDensityUniform = -1
	r.worldSkyCubemapFogDensityUniform = -1
	r.worldSkyExternalFaceFogDensity = -1
	r.worldIndexCount = 0
	r.worldData = nil
	r.worldTree = nil
	r.worldHasLitWater = false
	r.worldLiquidFaceTypes = 0
	r.worldLiquidAlphaOverrides = worldLiquidAlphaOverrides{}
	r.worldSkyFogOverride = worldSkyFogOverride{}
	r.worldSkyExternalName = ""
	r.worldSkyExternalMode = externalSkyboxRenderEmbedded
	r.worldSkyExternalRequestID = 0
	r.worldFogColor = [3]float32{}
	r.worldFogDensity = 0
	for modelID, alias := range r.aliasModels {
		if alias != nil {
			for _, tex := range alias.skins {
				if tex != 0 && tex != r.worldFallbackTexture {
					gl.DeleteTextures(1, &tex)
				}
			}
			for _, tex := range alias.fullbrightSkins {
				if tex != 0 && tex != r.worldFallbackTexture {
					gl.DeleteTextures(1, &tex)
				}
			}
			for _, variants := range alias.playerSkins {
				for _, tex := range variants {
					if tex != 0 && tex != r.worldFallbackTexture {
						gl.DeleteTextures(1, &tex)
					}
				}
			}
			for _, variants := range alias.playerFullbright {
				for _, tex := range variants {
					if tex != 0 && tex != r.worldFallbackTexture {
						gl.DeleteTextures(1, &tex)
					}
				}
			}
		}
		delete(r.aliasModels, modelID)
	}
	if r.aliasScratchVAO != 0 {
		gl.DeleteVertexArrays(1, &r.aliasScratchVAO)
		r.aliasScratchVAO = 0
	}
	if r.aliasScratchVBO != 0 {
		gl.DeleteBuffers(1, &r.aliasScratchVBO)
		r.aliasScratchVBO = 0
	}
	if r.decalVAO != 0 {
		gl.DeleteVertexArrays(1, &r.decalVAO)
		r.decalVAO = 0
	}
	if r.decalVBO != 0 {
		gl.DeleteBuffers(1, &r.decalVBO)
		r.decalVBO = 0
	}
	if r.decalProgram != 0 {
		gl.DeleteProgram(r.decalProgram)
		r.decalProgram = 0
	}
	r.decalVPUniform = -1
}

// ClearWorld releases OpenGL world resources.
func (r *Renderer) ClearWorld() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clearWorldLocked()
}

// ClearTranslucentCalls resets the per-frame translucent draw call list for the next frame.
func (r *Renderer) ClearTranslucentCalls() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.translucentCalls = r.translucentCalls[:0]
}

// DrawTranslucentCalls renders the accumulated translucent draw calls sorted by distance from the camera for correct alpha blending order.
func (r *Renderer) DrawTranslucentCalls() {
	r.mu.RLock()
	if len(r.translucentCalls) == 0 {
		r.mu.RUnlock()
		return
	}
	calls := append([]worldDrawCall(nil), r.translucentCalls...)
	program := r.worldProgram
	vp := r.viewMatrices.VP
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	lightmapUniform := r.worldLightmapUniform
	fullbrightUniform := r.worldFullbrightUniform
	hasFullbrightUniform := r.worldHasFullbrightUniform
	dynamicLightUniform := r.worldDynamicLightUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	modelRotationUniform := r.worldModelRotationUniform
	modelScaleUniform := r.worldModelScaleUniform
	alphaUniform := r.worldAlphaUniform
	turbulentUniform := r.worldTurbulentUniform
	litWaterUniform := r.worldLitWaterUniform
	cameraTime := r.cameraState.Time
	cameraOrigin := r.cameraState.Origin
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity
	timeUniform := r.worldTimeUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform

	// Snapshot OIT world program state for potential use below.
	oitProg := r.oitWorldProgram
	oitVP := r.oitWorldVPUniform
	oitTex := r.oitWorldTextureUniform
	oitLM := r.oitWorldLightmapUniform
	oitFB := r.oitWorldFullbrightUniform
	oitHasFB := r.oitWorldHasFullbrightUniform
	oitDL := r.oitWorldDynamicLightUniform
	oitOff := r.oitWorldModelOffsetUniform
	oitRot := r.oitWorldModelRotationUniform
	oitScl := r.oitWorldModelScaleUniform
	oitAlpha := r.oitWorldAlphaUniform
	oitTurb := r.oitWorldTurbulentUniform
	oitLitWater := r.oitWorldLitWaterUniform
	oitTime := r.oitWorldTimeUniform
	oitCamOrig := r.oitWorldCameraOriginUniform
	oitFogCol := r.oitWorldFogColorUniform
	oitFogDen := r.oitWorldFogDensityUniform
	r.mu.RUnlock()

	if program == 0 {
		return
	}

	// When OIT is active and the OIT world program is ready,
	// switch so translucent surfaces output to the MRT.
	if oitProg != 0 && GetAlphaMode() == AlphaModeOIT {
		program = oitProg
		vpUniform = oitVP
		textureUniform = oitTex
		lightmapUniform = oitLM
		fullbrightUniform = oitFB
		hasFullbrightUniform = oitHasFB
		dynamicLightUniform = oitDL
		modelOffsetUniform = oitOff
		modelRotationUniform = oitRot
		modelScaleUniform = oitScl
		alphaUniform = oitAlpha
		turbulentUniform = oitTurb
		litWaterUniform = oitLitWater
		timeUniform = oitTime
		cameraOriginUniform = oitCamOrig
		fogColorUniform = oitFogCol
		fogDensityUniform = oitFogDen
	}

	if shouldSortTranslucentCalls(GetAlphaMode()) {
		sort.SliceStable(calls, func(i, j int) bool {
			return calls[i].distanceSq > calls[j].distanceSq
		})
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.BLEND)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.Uniform1i(fullbrightUniform, 2)
	gl.Uniform1f(hasFullbrightUniform, 0)
	gl.Uniform1f(timeUniform, cameraTime)
	gl.Uniform3f(cameraOriginUniform, cameraOrigin.X, cameraOrigin.Y, cameraOrigin.Z)
	gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
	gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))

	renderWorldDrawCalls(calls, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, false)

	gl.UseProgram(0)
}
