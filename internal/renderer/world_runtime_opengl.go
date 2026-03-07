//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/go-gl/gl/v4.6-core/gl"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/model"
)

const (
	worldVertexShaderGL = `#version 410 core
layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec2 aTexCoord;
layout(location = 2) in vec2 aLightmapCoord;
layout(location = 3) in vec3 aNormal;

uniform mat4 uViewProjection;
uniform vec3 uModelOffset;

out vec2 vTexCoord;
out vec2 vLightmapCoord;
out vec3 vNormal;
out vec3 vWorldPos;

void main() {
	vTexCoord = aTexCoord;
	vLightmapCoord = aLightmapCoord;
	vec3 worldPosition = aPosition + uModelOffset;
	vNormal = aNormal;
	vWorldPos = worldPosition;
	gl_Position = uViewProjection * vec4(worldPosition, 1.0);
}`

	worldFragmentShaderGL = `#version 410 core
in vec2 vTexCoord;
in vec2 vLightmapCoord;
in vec3 vNormal;
in vec3 vWorldPos;
out vec4 fragColor;

uniform sampler2D uTexture;
uniform sampler2D uLightmap;
uniform float uAlpha;

void main() {
	vec4 base = texture(uTexture, vTexCoord);
	vec3 light = texture(uLightmap, vLightmapCoord).rgb * 2.0;
	if (base.a < 0.1) {
		discard;
	}
	fragColor = vec4(base.rgb * light, base.a * uAlpha);
}`
)

type worldRenderPass int

const (
	worldPassSky worldRenderPass = iota
	worldPassOpaque
	worldPassAlphaTest
	worldPassTranslucent
)

type worldDrawCall struct {
	face       WorldFace
	texture    uint32
	lightmap   uint32
	alpha      float32
	distanceSq float32
	light      [3]float32
}

type worldLiquidAlphaSettings struct {
	water float32
	lava  float32
	slime float32
	tele  float32
}

type worldLiquidAlphaOverrides struct {
	hasWater bool
	water    float32
	hasLava  bool
	lava     float32
	hasSlime bool
	slime    float32
	hasTele  bool
	tele     float32
}

type glWorldMesh struct {
	vao           uint32
	vbo           uint32
	ebo           uint32
	indexCount    int32
	faces         []WorldFace
	lightmaps     []uint32
	lightmapPages []WorldLightmapPage
}

type glAliasVertexRef struct {
	vertexIndex int
	texCoord    [2]float32
}

type glAliasModel struct {
	modelID string
	flags   int
	skins   []uint32
	poses   [][]model.TriVertX
	refs    []glAliasVertexRef
}

type glAliasDraw struct {
	alias  *glAliasModel
	model  *model.Model
	pose1  int     // First pose for interpolation
	pose2  int     // Second pose for interpolation
	blend  float32 // Blend factor between pose1 and pose2 (0 = pose1, 1 = pose2)
	skin   uint32
	origin [3]float32
	angles [3]float32
	alpha  float32
	full   bool
}

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

func (r *Renderer) ensureWorldProgram() error {
	if r.worldProgram != 0 {
		return nil
	}

	vs, err := compileShader(worldVertexShaderGL, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("compile world vertex shader: %w", err)
	}
	fs, err := compileShader(worldFragmentShaderGL, gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vs)
		return fmt.Errorf("compile world fragment shader: %w", err)
	}

	program := createProgram(vs, fs)
	r.worldProgram = program
	r.worldVPUniform = gl.GetUniformLocation(program, gl.Str("uViewProjection\x00"))
	r.worldTextureUniform = gl.GetUniformLocation(program, gl.Str("uTexture\x00"))
	r.worldLightmapUniform = gl.GetUniformLocation(program, gl.Str("uLightmap\x00"))
	r.worldModelOffsetUniform = gl.GetUniformLocation(program, gl.Str("uModelOffset\x00"))
	r.worldAlphaUniform = gl.GetUniformLocation(program, gl.Str("uAlpha\x00"))
	return nil
}

func (r *Renderer) ensureAliasScratchLocked() {
	if r.aliasScratchVAO != 0 && r.aliasScratchVBO != 0 {
		return
	}

	gl.GenVertexArrays(1, &r.aliasScratchVAO)
	gl.GenBuffers(1, &r.aliasScratchVBO)

	gl.BindVertexArray(r.aliasScratchVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.aliasScratchVBO)
	gl.BufferData(gl.ARRAY_BUFFER, 4, nil, gl.DYNAMIC_DRAW)

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
}

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
	mesh.lightmapPages = append(mesh.lightmapPages, renderData.Lightmaps...)
	mesh.lightmaps = uploadLightmapPages(renderData.Lightmaps, r.lightStyleValues)
	r.brushModels[submodelIndex] = mesh
	return mesh
}

func uploadWorldTextureRGBA(width, height int, rgba []byte) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)
	gl.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba))
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.NEAREST)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)
	return tex
}

func (r *Renderer) ensureWorldFallbackTextureLocked() {
	if r.worldFallbackTexture != 0 {
		return
	}
	r.worldFallbackTexture = uploadWorldTextureRGBA(1, 1, []byte{200, 200, 200, 255})
}

func (r *Renderer) ensureLightmapFallbackTextureLocked() {
	if r.worldLightmapFallback != 0 {
		return
	}
	r.worldLightmapFallback = uploadWorldTextureRGBA(1, 1, []byte{255, 255, 255, 255})
}

func (r *Renderer) setLightStyleValues(values [64]float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lightStyleValues = values
	r.updateUploadedLightmapsLocked()
}

func defaultLightStyleValues() [64]float32 {
	var values [64]float32
	for i := range values {
		values[i] = 1
	}
	return values
}

func (r *Renderer) uploadWorldTexturesLocked(tree *bsp.Tree) {
	r.worldTextures = make(map[int32]uint32)
	r.ensureWorldFallbackTextureLocked()

	if tree == nil || len(tree.TextureData) < 4 {
		return
	}

	palette := append([]byte(nil), r.palette...)
	count := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if count <= 0 || len(tree.TextureData) < 4+count*4 {
		return
	}

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
		pixels, width, height, err := miptex.MipLevel(0)
		if err != nil || width <= 0 || height <= 0 {
			continue
		}
		rgba := ConvertPaletteToRGBA(pixels, palette)
		tex := uploadWorldTextureRGBA(width, height, rgba)
		if tex != 0 {
			r.worldTextures[int32(i)] = tex
		}
	}
}

func lightstyleScale(values [64]float32, style uint8) float32 {
	if int(style) < len(values) && values[style] > 0 {
		return values[style]
	}
	return 1
}

func buildLightmapPageRGBA(page WorldLightmapPage, values [64]float32) []byte {
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
		if surface.Width <= 0 || surface.Height <= 0 {
			continue
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
			continue
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
				dst := ((surface.Y+y)*page.Width + (surface.X + x)) * 4
				rgba[dst] = byte(clamp01(rSum/255.0) * 255)
				rgba[dst+1] = byte(clamp01(gSum/255.0) * 255)
				rgba[dst+2] = byte(clamp01(bSum/255.0) * 255)
			}
		}
	}

	return rgba
}

func uploadLightmapPages(pages []WorldLightmapPage, values [64]float32) []uint32 {
	textures := make([]uint32, 0, len(pages))
	for _, page := range pages {
		rgba := buildLightmapPageRGBA(page, values)
		if len(rgba) == 0 {
			continue
		}
		textures = append(textures, uploadWorldTextureRGBA(page.Width, page.Height, rgba))
	}
	return textures
}

func updateLightmapTextures(textures []uint32, pages []WorldLightmapPage, values [64]float32) {
	count := len(textures)
	if len(pages) < count {
		count = len(pages)
	}
	for i := 0; i < count; i++ {
		if textures[i] == 0 {
			continue
		}
		rgba := buildLightmapPageRGBA(pages[i], values)
		if len(rgba) == 0 {
			continue
		}
		gl.BindTexture(gl.TEXTURE_2D, textures[i])
		gl.TexSubImage2D(gl.TEXTURE_2D, 0, 0, 0, int32(pages[i].Width), int32(pages[i].Height), gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba))
	}
	if count > 0 {
		gl.BindTexture(gl.TEXTURE_2D, 0)
	}
}

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

	renderData, err := buildWorldRenderData(tree)
	if err != nil {
		return fmt.Errorf("build world render data: %w", err)
	}
	if renderData.Geometry == nil || len(renderData.Geometry.Vertices) == 0 || len(renderData.Geometry.Indices) == 0 {
		r.worldData = renderData
		return nil
	}

	if err := r.ensureWorldProgram(); err != nil {
		return err
	}
	r.worldTree = tree
	r.uploadWorldTexturesLocked(tree)
	r.ensureLightmapFallbackTextureLocked()
	worldMesh := uploadWorldMesh(renderData.Geometry.Vertices, renderData.Geometry.Indices)
	if worldMesh == nil {
		return fmt.Errorf("upload world mesh: no geometry uploaded")
	}
	r.worldVAO = worldMesh.vao
	r.worldVBO = worldMesh.vbo
	r.worldEBO = worldMesh.ebo

	r.worldData = renderData
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

func (r *Renderer) renderWorld() {
	r.mu.RLock()
	program := r.worldProgram
	vao := r.worldVAO
	indexCount := r.worldIndexCount
	vp := r.viewMatrices.VP
	camera := r.cameraState
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	lightmapUniform := r.worldLightmapUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	alphaUniform := r.worldAlphaUniform
	fallbackTexture := r.worldFallbackTexture
	fallbackLightmap := r.worldLightmapFallback
	liquidAlphaOverrides := r.worldLiquidAlphaOverrides
	worldTree := r.worldTree
	faces := []WorldFace(nil)
	if r.worldData != nil && r.worldData.Geometry != nil {
		faces = append(faces, r.worldData.Geometry.Faces...)
	}
	worldTextures := make(map[int32]uint32, len(r.worldTextures))
	for k, v := range r.worldTextures {
		worldTextures[k] = v
	}
	worldLightmaps := append([]uint32(nil), r.worldLightmaps...)
	lightPool := r.lightPool // Get light pool for light evaluation
	r.mu.RUnlock()

	if program == 0 || vao == 0 || indexCount <= 0 {
		return
	}

	liquidAlpha := worldLiquidAlphaSettingsFromCvars(liquidAlphaOverrides, worldTree)
	skyFaces, opaqueFaces, alphaTestFaces, translucentFaces := bucketWorldFacesWithLights(faces, worldTextures, worldLightmaps, fallbackTexture, fallbackLightmap, [3]float32{}, camera, liquidAlpha, lightPool)

	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)

	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.Uniform3f(modelOffsetUniform, 0, 0, 0)
	gl.BindVertexArray(vao)
	if len(faces) == 0 {
		gl.DepthMask(true)
		gl.Disable(gl.BLEND)
		gl.Uniform1f(alphaUniform, 1)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, fallbackTexture)
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, fallbackLightmap)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.DrawElements(gl.TRIANGLES, indexCount, gl.UNSIGNED_INT, unsafe.Pointer(nil))
	} else {
		renderSkyPass(skyFaces, alphaUniform)
		renderWorldDrawCalls(opaqueFaces, alphaUniform, true)
		renderWorldDrawCalls(alphaTestFaces, alphaUniform, true)
		renderWorldDrawCalls(translucentFaces, alphaUniform, false)
	}
	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)

	gl.Enable(gl.BLEND)
}

func (r *Renderer) renderBrushEntities(entities []BrushEntity) {
	if len(entities) == 0 {
		return
	}

	r.mu.Lock()
	program := r.worldProgram
	vp := r.viewMatrices.VP
	camera := r.cameraState
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	lightmapUniform := r.worldLightmapUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	alphaUniform := r.worldAlphaUniform
	fallbackTexture := r.worldFallbackTexture
	fallbackLightmap := r.worldLightmapFallback
	liquidAlphaOverrides := r.worldLiquidAlphaOverrides
	worldTree := r.worldTree
	worldTextures := make(map[int32]uint32, len(r.worldTextures))
	for k, v := range r.worldTextures {
		worldTextures[k] = v
	}
	lightPool := r.lightPool // Get light pool for light evaluation
	type drawBrush struct {
		origin [3]float32
		mesh   *glWorldMesh
	}
	brushes := make([]drawBrush, 0, len(entities))
	for _, entity := range entities {
		mesh := r.ensureBrushModelLocked(entity.SubmodelIndex)
		if mesh == nil {
			continue
		}
		brushes = append(brushes, drawBrush{origin: entity.Origin, mesh: mesh})
	}
	r.mu.Unlock()

	if program == 0 || len(brushes) == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.ActiveTexture(gl.TEXTURE0)
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(liquidAlphaOverrides, worldTree)

	for _, brush := range brushes {
		skyFaces, opaqueFaces, alphaTestFaces, translucentFaces := bucketWorldFacesWithLights(brush.mesh.faces, worldTextures, brush.mesh.lightmaps, fallbackTexture, fallbackLightmap, brush.origin, camera, liquidAlpha, lightPool)
		gl.Uniform3f(modelOffsetUniform, brush.origin[0], brush.origin[1], brush.origin[2])
		gl.BindVertexArray(brush.mesh.vao)
		renderSkyPass(skyFaces, alphaUniform)
		renderWorldDrawCalls(opaqueFaces, alphaUniform, true)
		renderWorldDrawCalls(alphaTestFaces, alphaUniform, true)
		renderWorldDrawCalls(translucentFaces, alphaUniform, false)
	}

	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	gl.Enable(gl.BLEND)
}

// bucketWorldFacesWithLights is like bucketWorldFaces but also evaluates dynamic lights.
// This variant accepts a light pool and computes light contributions for each face.
func bucketWorldFacesWithLights(faces []WorldFace, textures map[int32]uint32, lightmaps []uint32, fallbackTexture, fallbackLightmap uint32, modelOffset [3]float32, camera CameraState, liquidAlpha worldLiquidAlphaSettings, lightPool *glLightPool) (sky, opaque, alphaTest, translucent []worldDrawCall) {
	for _, face := range faces {
		call := worldDrawCall{
			face:       face,
			texture:    worldTextureForFace(face, textures, fallbackTexture),
			lightmap:   worldLightmapForFace(face, lightmaps, fallbackLightmap),
			alpha:      worldFaceAlpha(face.Flags, liquidAlpha),
			distanceSq: worldFaceDistanceSq(face.Center, modelOffset, camera),
			light:      [3]float32{}, // Will be populated below
		}

		// Evaluate dynamic lights at this face's center
		if lightPool != nil {
			call.light = lightPool.EvaluateLightsAtPoint(face.Center)
		}

		switch worldFacePass(face.Flags, call.alpha) {
		case worldPassSky:
			sky = append(sky, call)
		case worldPassAlphaTest:
			alphaTest = append(alphaTest, call)
		case worldPassTranslucent:
			translucent = append(translucent, call)
		default:
			opaque = append(opaque, call)
		}
	}

	sort.SliceStable(translucent, func(i, j int) bool {
		return translucent[i].distanceSq > translucent[j].distanceSq
	})

	return sky, opaque, alphaTest, translucent
}

func bucketWorldFaces(faces []WorldFace, textures map[int32]uint32, lightmaps []uint32, fallbackTexture, fallbackLightmap uint32, modelOffset [3]float32, camera CameraState, liquidAlpha worldLiquidAlphaSettings) (sky, opaque, alphaTest, translucent []worldDrawCall) {
	return bucketWorldFacesWithLights(faces, textures, lightmaps, fallbackTexture, fallbackLightmap, modelOffset, camera, liquidAlpha, nil)
}

func worldTextureForFace(face WorldFace, textures map[int32]uint32, fallbackTexture uint32) uint32 {
	tex := textures[face.TextureIndex]
	if tex == 0 {
		tex = fallbackTexture
	}
	return tex
}

func worldLightmapForFace(face WorldFace, lightmaps []uint32, fallbackLightmap uint32) uint32 {
	if face.LightmapIndex >= 0 && int(face.LightmapIndex) < len(lightmaps) && lightmaps[face.LightmapIndex] != 0 {
		return lightmaps[face.LightmapIndex]
	}
	return fallbackLightmap
}

func worldFaceAlpha(flags int32, liquidAlpha worldLiquidAlphaSettings) float32 {
	if flags&model.SurfDrawTurb == 0 {
		return 1
	}
	if flags&model.SurfDrawLava != 0 {
		return liquidAlpha.lava
	}
	if flags&model.SurfDrawSlime != 0 {
		return liquidAlpha.slime
	}
	if flags&model.SurfDrawTele != 0 {
		return liquidAlpha.tele
	}
	if flags&model.SurfDrawWater != 0 {
		return liquidAlpha.water
	}
	return 1
}

func worldLiquidAlphaSettingsFromCvars(overrides worldLiquidAlphaOverrides, tree *bsp.Tree) worldLiquidAlphaSettings {
	return resolveWorldLiquidAlphaSettings(
		readWorldAlphaCvar(CvarRWaterAlpha, 1),
		readWorldAlphaCvar(CvarRLavaAlpha, 0),
		readWorldAlphaCvar(CvarRSlimeAlpha, 0),
		readWorldAlphaCvar(CvarRTeleAlpha, 0),
		overrides,
		tree,
	)
}

func resolveWorldLiquidAlphaSettings(cvarWater, cvarLava, cvarSlime, cvarTele float32, overrides worldLiquidAlphaOverrides, tree *bsp.Tree) worldLiquidAlphaSettings {
	water := clamp01(cvarWater)
	if overrides.hasWater {
		water = clamp01(overrides.water)
	}
	fallback := water

	lava := fallback
	if cvarLava > 0 {
		lava = clamp01(cvarLava)
	}
	if overrides.hasLava {
		if overrides.lava > 0 {
			lava = clamp01(overrides.lava)
		} else {
			lava = fallback
		}
	}

	slime := fallback
	if cvarSlime > 0 {
		slime = clamp01(cvarSlime)
	}
	if overrides.hasSlime {
		if overrides.slime > 0 {
			slime = clamp01(overrides.slime)
		} else {
			slime = fallback
		}
	}

	tele := fallback
	if cvarTele > 0 {
		tele = clamp01(cvarTele)
	}
	if overrides.hasTele {
		if overrides.tele > 0 {
			tele = clamp01(overrides.tele)
		} else {
			tele = fallback
		}
	}

	settings := worldLiquidAlphaSettings{water: water, lava: lava, slime: slime, tele: tele}

	// Force opaque if map is not vis-safe for transparent water
	if !mapVisTransparentWaterSafe(tree) {
		settings.water = 1.0
		settings.lava = 1.0
		settings.slime = 1.0
		settings.tele = 1.0
	}

	return settings
}

func parseWorldspawnLiquidAlphaOverrides(entities []byte) worldLiquidAlphaOverrides {
	if len(entities) == 0 {
		return worldLiquidAlphaOverrides{}
	}

	entity, ok := firstEntityLumpObject(string(entities))
	if !ok {
		return worldLiquidAlphaOverrides{}
	}

	fields := parseEntityFields(entity)
	if !strings.EqualFold(fields["classname"], "worldspawn") {
		return worldLiquidAlphaOverrides{}
	}

	var overrides worldLiquidAlphaOverrides
	if value, ok := parseEntityAlphaField(fields, "wateralpha"); ok {
		overrides.hasWater = true
		overrides.water = value
	}
	if value, ok := parseEntityAlphaField(fields, "lavaalpha"); ok {
		overrides.hasLava = true
		overrides.lava = value
	}
	if value, ok := parseEntityAlphaField(fields, "slimealpha"); ok {
		overrides.hasSlime = true
		overrides.slime = value
	}
	if value, ok := parseEntityAlphaField(fields, "telealpha"); ok {
		overrides.hasTele = true
		overrides.tele = value
	}

	return overrides
}

// mapVisTransparentWaterSafe determines if the map's visibility data is compiled for transparent water.
// In Quake 1, transparent water requires special VIS-time flags; maps without them render water opaque to prevent rendering errors.
// Returns true if map is safe for transparent liquids, false if opaque should be forced.
func mapVisTransparentWaterSafe(tree *bsp.Tree) bool {
	// TODO: Check worldspawn flags or BSP header for transparency bit
	// For now, assume safe by default (matches original Ironwail conservative approach)
	// Real implementation would check VIS compilation flags or worldspawn "transpwater" key
	if tree == nil {
		return true
	}
	return true
}

func parseEntityAlphaField(fields map[string]string, key string) (float32, bool) {
	value, ok := fields[key]
	if !ok {
		value, ok = fields["_"+key]
		if !ok {
			return 0, false
		}
	}
	f, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return 0, false
	}
	return float32(f), true
}

func firstEntityLumpObject(data string) (string, bool) {
	start := strings.IndexByte(data, '{')
	if start < 0 {
		return "", false
	}
	end := strings.IndexByte(data[start+1:], '}')
	if end < 0 {
		return "", false
	}
	return data[start+1 : start+1+end], true
}

func parseEntityFields(data string) map[string]string {
	fields := make(map[string]string)
	pos := 0
	for {
		key, next, ok := nextQuotedEntityToken(data, pos)
		if !ok {
			break
		}
		value, nextValue, ok := nextQuotedEntityToken(data, next)
		if !ok {
			break
		}
		fields[strings.ToLower(key)] = value
		pos = nextValue
	}
	return fields
}

func nextQuotedEntityToken(data string, pos int) (string, int, bool) {
	start := strings.IndexByte(data[pos:], '"')
	if start < 0 {
		return "", pos, false
	}
	start += pos
	end := strings.IndexByte(data[start+1:], '"')
	if end < 0 {
		return "", pos, false
	}
	end += start + 1
	return data[start+1 : end], end + 1, true
}

func readWorldAlphaCvar(name string, fallback float32) float32 {
	cv := cvar.Get(name)
	if cv == nil {
		return clamp01(fallback)
	}
	return clamp01(cv.Float32())
}

func worldFacePass(flags int32, alpha float32) worldRenderPass {
	switch {
	case flags&model.SurfDrawSky != 0:
		return worldPassSky
	case flags&model.SurfDrawFence != 0:
		return worldPassAlphaTest
	case flags&model.SurfDrawTurb != 0 && alpha < 1:
		return worldPassTranslucent
	default:
		return worldPassOpaque
	}
}

func worldFaceDistanceSq(center, modelOffset [3]float32, camera CameraState) float32 {
	dx := center[0] + modelOffset[0] - camera.Origin.X
	dy := center[1] + modelOffset[1] - camera.Origin.Y
	dz := center[2] + modelOffset[2] - camera.Origin.Z
	return dx*dx + dy*dy + dz*dz
}

func renderSkyPass(calls []worldDrawCall, alphaUniform int32) {
	if len(calls) == 0 {
		return
	}
	// Sky is rendered at maximum depth but doesn't write to depth buffer
	gl.DepthFunc(gl.LEQUAL)
	gl.DepthMask(false)
	gl.Disable(gl.BLEND)

	for _, call := range calls {
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, call.texture)
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, call.lightmap)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.Uniform1f(alphaUniform, call.alpha)
		gl.DrawElements(gl.TRIANGLES, int32(call.face.NumIndices), gl.UNSIGNED_INT, unsafe.Pointer(uintptr(call.face.FirstIndex*4)))
	}

	// Restore depth state
	gl.DepthFunc(gl.LESS)
	gl.DepthMask(true)
}

func renderWorldDrawCalls(calls []worldDrawCall, alphaUniform int32, depthWrite bool) {
	if len(calls) == 0 {
		return
	}
	gl.DepthMask(depthWrite)
	if depthWrite {
		gl.Disable(gl.BLEND)
	} else {
		gl.Enable(gl.BLEND)
	}
	for _, call := range calls {
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, call.texture)
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, call.lightmap)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.Uniform1f(alphaUniform, call.alpha)
		gl.DrawElements(gl.TRIANGLES, int32(call.face.NumIndices), gl.UNSIGNED_INT, unsafe.Pointer(uintptr(call.face.FirstIndex*4)))
	}
}

func (r *Renderer) ensureAliasModelLocked(modelID string, mdl *model.Model) *glAliasModel {
	if modelID == "" || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}
	if cached, ok := r.aliasModels[modelID]; ok {
		return cached
	}

	hdr := mdl.AliasHeader
	if len(hdr.STVerts) != hdr.NumVerts || len(hdr.Triangles) != hdr.NumTris || len(hdr.Poses) == 0 {
		return nil
	}

	r.ensureWorldFallbackTextureLocked()
	r.ensureLightmapFallbackTextureLocked()
	palette := append([]byte(nil), r.palette...)
	skins := make([]uint32, 0, len(hdr.Skins))
	for _, skin := range hdr.Skins {
		if len(skin) != hdr.SkinWidth*hdr.SkinHeight {
			skins = append(skins, r.worldFallbackTexture)
			continue
		}
		rgba := ConvertPaletteToRGBA(skin, palette)
		tex := uploadWorldTextureRGBA(hdr.SkinWidth, hdr.SkinHeight, rgba)
		if tex == 0 {
			tex = r.worldFallbackTexture
		}
		skins = append(skins, tex)
	}

	refs := make([]glAliasVertexRef, 0, len(hdr.Triangles)*3)
	for _, tri := range hdr.Triangles {
		for vertexIndex := 0; vertexIndex < 3; vertexIndex++ {
			idx := int(tri.VertIndex[vertexIndex])
			if idx < 0 || idx >= len(hdr.STVerts) {
				continue
			}
			st := hdr.STVerts[idx]
			s := float32(st.S) + 0.5
			if tri.FacesFront == 0 && st.OnSeam != 0 {
				s += float32(hdr.SkinWidth) * 0.5
			}
			refs = append(refs, glAliasVertexRef{
				vertexIndex: idx,
				texCoord: [2]float32{
					s / float32(hdr.SkinWidth),
					(float32(st.T) + 0.5) / float32(hdr.SkinHeight),
				},
			})
		}
	}

	alias := &glAliasModel{
		modelID: modelID,
		flags:   hdr.Flags,
		skins:   skins,
		poses:   hdr.Poses,
		refs:    refs,
	}
	r.aliasModels[modelID] = alias
	return alias
}

func buildAliasVertices(alias *glAliasModel, mdl *model.Model, poseIndex int, origin, angles [3]float32, fullAngles bool) []WorldVertex {
	if alias == nil || mdl == nil || mdl.AliasHeader == nil || poseIndex < 0 || poseIndex >= len(alias.poses) {
		return nil
	}
	pose := alias.poses[poseIndex]
	vertices := make([]WorldVertex, 0, len(alias.refs))
	for _, ref := range alias.refs {
		if ref.vertexIndex < 0 || ref.vertexIndex >= len(pose) {
			continue
		}
		compressed := pose[ref.vertexIndex]
		position := model.DecodeVertex(compressed, mdl.AliasHeader.Scale, mdl.AliasHeader.ScaleOrigin)
		normal := model.GetNormal(compressed.LightNormalIndex)
		if fullAngles {
			position = rotateAliasAngles(position, angles)
			normal = rotateAliasAngles(normal, angles)
		} else {
			position = rotateAliasYaw(position, angles[1])
			normal = rotateAliasYaw(normal, angles[1])
		}
		position[0] += origin[0]
		position[1] += origin[1]
		position[2] += origin[2]
		vertices = append(vertices, WorldVertex{
			Position:      position,
			TexCoord:      ref.texCoord,
			LightmapCoord: [2]float32{},
			Normal:        normal,
		})
	}
	return vertices
}

func rotateAliasAngles(v [3]float32, angles [3]float32) [3]float32 {
	v = rotateAliasYaw(v, angles[1])
	v = rotateAliasPitch(v, angles[0])
	v = rotateAliasRoll(v, angles[2])
	return v
}

func rotateAliasYaw(v [3]float32, yawDegrees float32) [3]float32 {
	if yawDegrees == 0 {
		return v
	}
	yaw := float32(math.Pi) * yawDegrees / 180.0
	sinYaw := float32(math.Sin(float64(yaw)))
	cosYaw := float32(math.Cos(float64(yaw)))
	return [3]float32{
		v[0]*cosYaw - v[1]*sinYaw,
		v[0]*sinYaw + v[1]*cosYaw,
		v[2],
	}
}

func rotateAliasPitch(v [3]float32, pitchDegrees float32) [3]float32 {
	if pitchDegrees == 0 {
		return v
	}
	pitch := float32(math.Pi) * pitchDegrees / 180.0
	sinPitch := float32(math.Sin(float64(pitch)))
	cosPitch := float32(math.Cos(float64(pitch)))
	return [3]float32{
		v[0],
		v[1]*cosPitch - v[2]*sinPitch,
		v[1]*sinPitch + v[2]*cosPitch,
	}
}

func rotateAliasRoll(v [3]float32, rollDegrees float32) [3]float32 {
	if rollDegrees == 0 {
		return v
	}
	roll := float32(math.Pi) * rollDegrees / 180.0
	sinRoll := float32(math.Sin(float64(roll)))
	cosRoll := float32(math.Cos(float64(roll)))
	return [3]float32{
		v[0]*cosRoll + v[2]*sinRoll,
		v[1],
		-v[0]*sinRoll + v[2]*cosRoll,
	}
}

func (r *Renderer) buildAliasDrawLocked(entity AliasModelEntity, fullAngles bool) *glAliasDraw {
	alias := r.ensureAliasModelLocked(entity.ModelID, entity.Model)
	if alias == nil || entity.Model == nil || entity.Model.AliasHeader == nil || len(alias.refs) == 0 {
		return nil
	}

	hdr := entity.Model.AliasHeader
	frame := entity.Frame
	if frame < 0 || frame >= len(hdr.Frames) {
		frame = 0
	}

	// Convert model.AliasFrameDesc to our internal AliasFrameDesc
	frameDescs := make([]AliasFrameDesc, len(hdr.Frames))
	for i, f := range hdr.Frames {
		frameDescs[i] = AliasFrameDesc{
			FirstPose: f.FirstPose,
			NumPoses:  f.NumPoses,
			Interval:  f.Interval,
			BBoxMin:   f.BBoxMin,
			BBoxMax:   f.BBoxMax,
			Frame:     f.Frame,
			Name:      f.Name,
		}
	}

	// Get animation time from entity state
	// FrameTime is accumulated by the game logic and indicates how far into
	// the current animation frame we are
	currentTime := entity.FrameTime

	// Setup frame interpolation
	interpData := setupAliasFrameInterpolation(frame, frameDescs, currentTime, true, hdr.Flags)

	// Validate poses
	pose1 := interpData.Pose1
	pose2 := interpData.Pose2
	if pose1 < 0 || pose1 >= len(alias.poses) {
		pose1 = 0
	}
	if pose2 < 0 || pose2 >= len(alias.poses) {
		pose2 = 0
	}

	skin := r.worldFallbackTexture
	if len(alias.skins) > 0 {
		skinIndex := entity.SkinNum
		if skinIndex < 0 {
			skinIndex = 0
		}
		skin = alias.skins[skinIndex%len(alias.skins)]
		if skin == 0 {
			skin = r.worldFallbackTexture
		}
	}

	alpha := entity.Alpha
	if alpha <= 0 {
		alpha = 1
	}

	return &glAliasDraw{
		alias:  alias,
		model:  entity.Model,
		pose1:  pose1,
		pose2:  pose2,
		blend:  interpData.Blend,
		skin:   skin,
		origin: entity.Origin,
		angles: entity.Angles,
		alpha:  alpha,
		full:   fullAngles,
	}
}

func (r *Renderer) renderAliasDraws(draws []glAliasDraw, useViewModelDepthRange bool) {
	if len(draws) == 0 {
		return
	}

	r.mu.Lock()
	program := r.worldProgram
	vp := r.viewMatrices.VP
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	lightmapUniform := r.worldLightmapUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	alphaUniform := r.worldAlphaUniform
	r.ensureAliasScratchLocked()
	scratchVAO := r.aliasScratchVAO
	scratchVBO := r.aliasScratchVBO
	fallbackLightmap := r.worldLightmapFallback
	r.mu.Unlock()

	if program == 0 || scratchVAO == 0 || scratchVBO == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(true)
	gl.Disable(gl.CULL_FACE)
	gl.Enable(gl.BLEND)
	if useViewModelDepthRange {
		gl.DepthRange(0.0, 0.3)
	}
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.Uniform3f(modelOffsetUniform, 0, 0, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, fallbackLightmap)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindVertexArray(scratchVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, scratchVBO)

	for _, draw := range draws {
		// Use interpolated vertex building with two poses
		vertices := buildAliasVerticesInterpolated(draw.alias, draw.model, draw.pose1, draw.pose2, draw.blend, draw.origin, draw.angles, draw.full)
		if len(vertices) == 0 {
			continue
		}
		vertexData := flattenWorldVertices(vertices)
		gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.DYNAMIC_DRAW)
		gl.BindTexture(gl.TEXTURE_2D, draw.skin)
		gl.Uniform1f(alphaUniform, draw.alpha)
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(vertices)))
	}

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	if useViewModelDepthRange {
		gl.DepthRange(0.0, 1.0)
	}
}

func (r *Renderer) renderAliasEntities(entities []AliasModelEntity) {
	if len(entities) == 0 {
		return
	}

	r.mu.Lock()
	draws := make([]glAliasDraw, 0, len(entities))
	for _, entity := range entities {
		if draw := r.buildAliasDrawLocked(entity, false); draw != nil {
			draws = append(draws, *draw)
		}
	}
	r.mu.Unlock()
	r.renderAliasDraws(draws, false)
}

func (r *Renderer) renderViewModel(entity AliasModelEntity) {
	r.mu.Lock()
	draw := r.buildAliasDrawLocked(entity, true)
	r.mu.Unlock()
	if draw == nil {
		return
	}
	r.renderAliasDraws([]glAliasDraw{*draw}, true)
}

func (r *Renderer) renderSpriteEntities(entities []SpriteEntity) {
	if len(entities) == 0 {
		return
	}

	r.mu.Lock()
	program := r.worldProgram
	vp := r.viewMatrices.VP
	camera := r.cameraState
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	lightmapUniform := r.worldLightmapUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	alphaUniform := r.worldAlphaUniform
	fallbackLightmap := r.worldLightmapFallback

	draws := make([]glSpriteDraw, 0, len(entities))
	for _, entity := range entities {
		if draw := r.buildSpriteDrawLocked(entity); draw != nil {
			draws = append(draws, *draw)
		}
	}
	r.mu.Unlock()

	if program == 0 || len(draws) == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.ActiveTexture(gl.TEXTURE0)

	for _, draw := range draws {
		r.renderSpriteDraw(draw, camera, program, modelOffsetUniform, alphaUniform, fallbackLightmap)
	}

	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
}

// glSpriteDraw holds data for rendering a single sprite.
type glSpriteDraw struct {
	sprite *glSpriteModel
	model  *model.Model
	frame  int
	origin [3]float32
	alpha  float32
}

// buildSpriteDrawLocked prepares a sprite for rendering (must be called with mutex held).
func (r *Renderer) buildSpriteDrawLocked(entity SpriteEntity) *glSpriteDraw {
	if entity.ModelID == "" || entity.Model == nil || entity.Model.Type != model.ModSprite {
		return nil
	}

	var spr *glSpriteModel
	if entity.SpriteData != nil {
		// Use sprite data directly from entity
		spr = uploadSpriteModel(entity.ModelID, entity.SpriteData)
	} else {
		// Fall back to cache
		spr = r.ensureSpriteLocked(entity.ModelID, entity.Model)
	}

	if spr == nil {
		return nil
	}

	frame := entity.Frame
	if frame < 0 || frame >= len(spr.frames) {
		frame = 0
	}

	return &glSpriteDraw{
		sprite: spr,
		model:  entity.Model,
		frame:  frame,
		origin: entity.Origin,
		alpha:  entity.Alpha,
	}
}

// ensureSpriteLocked retrieves or creates a cached sprite model (must be called with mutex held).
func (r *Renderer) ensureSpriteLocked(modelID string, mdl *model.Model) *glSpriteModel {
	if modelID == "" || mdl == nil || mdl.Type != model.ModSprite {
		return nil
	}

	if cached, ok := r.spriteModels[modelID]; ok {
		return cached
	}

	// Extract sprite data - MSprite should have been populated during model loading
	if !isModelSprite(mdl) {
		return nil
	}

	// For now, we'll construct sprite data from the model structure
	// In a real implementation, the model loader would populate a Sprite field
	spr := &model.MSprite{
		Type:      int(mdl.Type),
		MaxWidth:  int(mdl.Maxs[0] - mdl.Mins[0]),
		MaxHeight: int(mdl.Maxs[2] - mdl.Mins[2]),
	}

	if mdl.Maxs[0] == 0 && mdl.Maxs[2] == 0 {
		// Fallback dimensions
		spr.MaxWidth = 64
		spr.MaxHeight = 64
	}

	// For basic implementation, create a simple frame
	// A complete implementation would load actual sprite frames from model data
	spr.NumFrames = 1
	spr.Frames = make([]model.MSpriteFrameDesc, 1)

	glsprite := uploadSpriteModel(modelID, spr)
	if glsprite == nil {
		return nil
	}

	r.spriteModels[modelID] = glsprite
	return glsprite
}

// isModelSprite checks if a model is a sprite type.
func isModelSprite(mdl *model.Model) bool {
	return mdl != nil && mdl.Type == model.ModSprite
}

// renderSpriteDraw renders a single sprite billboard.
func (r *Renderer) renderSpriteDraw(draw glSpriteDraw, camera CameraState, program uint32, modelOffsetUniform, alphaUniform int32, fallbackLightmap uint32) {
	if draw.sprite == nil || draw.frame < 0 || draw.frame >= len(draw.sprite.frames) {
		return
	}

	// Build sprite quad vertices
	vertices := buildSpriteQuadVertices(draw.sprite, draw.frame, [3]float32{
		camera.Angles.X,
		camera.Angles.Y,
		camera.Angles.Z,
	})

	if len(vertices) == 0 {
		return
	}

	// Generate quad indices
	indices := generateSpriteQuadIndices()

	// Ensure scratch VAO/VBO for transient geometry
	r.ensureAliasScratchLocked()

	// Upload vertices to scratch VBO
	vertexData := flattenWorldVertices(vertices)
	gl.BindBuffer(gl.ARRAY_BUFFER, r.aliasScratchVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.DYNAMIC_DRAW)

	// Set model offset (sprite origin)
	gl.Uniform3f(modelOffsetUniform, draw.origin[0], draw.origin[1], draw.origin[2])

	// Set alpha
	gl.Uniform1f(alphaUniform, draw.alpha)

	// Bind vertex array
	gl.BindVertexArray(r.aliasScratchVAO)

	// Draw sprite quad (2 triangles = 6 indices)
	gl.DrawElements(gl.TRIANGLES, int32(len(indices)), gl.UNSIGNED_INT, gl.PtrOffset(0))
}

// HasWorldData reports whether the OpenGL world path has uploaded geometry.
func (r *Renderer) HasWorldData() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData != nil && r.worldVAO != 0 && r.worldProgram != 0 && r.worldIndexCount > 0
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
	r.worldVPUniform = -1
	r.worldTextureUniform = -1
	r.worldLightmapUniform = -1
	r.worldModelOffsetUniform = -1
	r.worldAlphaUniform = -1
	r.worldIndexCount = 0
	r.worldData = nil
	r.worldTree = nil
	r.worldLiquidAlphaOverrides = worldLiquidAlphaOverrides{}
	for modelID, alias := range r.aliasModels {
		if alias != nil {
			for _, tex := range alias.skins {
				if tex != 0 && tex != r.worldFallbackTexture {
					gl.DeleteTextures(1, &tex)
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
