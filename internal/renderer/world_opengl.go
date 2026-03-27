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
	aliasimpl "github.com/ironwail/ironwail-go/internal/renderer/alias"
	worldimpl "github.com/ironwail/ironwail-go/internal/renderer/world"
	worldopengl "github.com/ironwail/ironwail-go/internal/renderer/world/opengl"
	"log/slog"
	"math"
	"sort"
	"strings"
	"unsafe"
)

// ---- merged from world_opengl.go ----
type WorldGeometry = worldimpl.WorldGeometry
type WorldVertex = worldimpl.WorldVertex
type WorldFace = worldimpl.WorldFace
type WorldLightmapSurface = worldimpl.WorldLightmapSurface
type WorldLightmapPage = worldimpl.WorldLightmapPage

// WorldRenderData holds CPU-side world data and bounds.
type WorldRenderData struct {
	Geometry      *WorldGeometry
	Lightmaps     []WorldLightmapPage
	HasLitWater   bool
	BoundsMin     [3]float32
	BoundsMax     [3]float32
	TotalVertices int
	TotalIndices  int
	TotalFaces    int
}

const worldLightmapPageSize = 1024

// BuildWorldGeometry extracts renderable geometry from BSP data.
func BuildWorldGeometry(tree *bsp.Tree) (*WorldGeometry, error) {
	return BuildModelGeometry(tree, 0)
}

// BuildModelGeometry extracts renderable geometry for a specific BSP model index.
func BuildModelGeometry(tree *bsp.Tree, modelIndex int) (*WorldGeometry, error) {
	if tree == nil {
		return nil, fmt.Errorf("nil BSP tree")
	}
	if len(tree.Models) == 0 {
		return nil, fmt.Errorf("BSP has no models")
	}
	if modelIndex < 0 || modelIndex >= len(tree.Models) {
		return nil, fmt.Errorf("model index %d out of range", modelIndex)
	}

	worldModel := tree.Models[modelIndex]
	geom := &WorldGeometry{
		Vertices: make([]WorldVertex, 0, 4096),
		Indices:  make([]uint32, 0, 16384),
		Faces:    make([]WorldFace, 0, 256),
		Tree:     tree,
	}
	lightmapAllocator, err := NewLightmapAllocator(worldLightmapPageSize, worldLightmapPageSize, false)
	if err != nil {
		return nil, fmt.Errorf("create lightmap allocator: %w", err)
	}
	lightmapPages := make([]WorldLightmapPage, 0, 4)

	textureMeta := parseWorldTextureMeta(tree)
	numFaces := int(worldModel.NumFaces)
	faceLookup := make(map[int]int, numFaces)
	firstFace := int(worldModel.FirstFace)
	for faceIdx := 0; faceIdx < numFaces; faceIdx++ {
		globalFaceIdx := firstFace + faceIdx
		if globalFaceIdx >= len(tree.Faces) {
			break
		}

		face := &tree.Faces[globalFaceIdx]
		textureIndex := int32(-1)
		textureFlags := int32(0)
		textureType := model.TexTypeDefault
		if int(face.Texinfo) >= 0 && int(face.Texinfo) < len(tree.Texinfo) {
			textureIndex = tree.Texinfo[face.Texinfo].Miptex
			if int(textureIndex) >= 0 && int(textureIndex) < len(textureMeta) {
				textureType = textureMeta[textureIndex].Type
			}
			textureFlags = deriveWorldFaceFlags(textureType, tree.Texinfo[face.Texinfo].Flags)
		}
		faceData := WorldFace{
			FirstIndex:    uint32(len(geom.Indices)),
			TextureIndex:  textureIndex,
			LightmapIndex: -1,
			Flags:         textureFlags,
		}

		faceVerts, lightmapSurface, err := extractFaceVertices(tree, face, textureMeta, lightmapAllocator, &lightmapPages)
		if err != nil {
			slog.Warn("OpenGL world: failed to extract face", "face", globalFaceIdx, "error", err)
			continue
		}
		if len(faceVerts) < 3 {
			continue
		}

		baseVertIdx := uint32(len(geom.Vertices))
		geom.Vertices = append(geom.Vertices, faceVerts...)
		for i := 1; i < len(faceVerts)-1; i++ {
			geom.Indices = append(geom.Indices,
				baseVertIdx,
				baseVertIdx+uint32(i),
				baseVertIdx+uint32(i+1),
			)
		}

		faceData.NumIndices = uint32((len(faceVerts) - 2) * 3)
		faceData.Center = worldFaceCenter(faceVerts)
		if lightmapSurface != nil {
			faceData.LightmapIndex = int32(lightmapSurface.pageIndex)
		}
		if worldFaceHasLitWater(textureFlags, lightmapSurface) {
			geom.HasLitWater = true
		}
		geom.Faces = append(geom.Faces, faceData)
		faceLookup[globalFaceIdx] = len(geom.Faces) - 1
	}
	geom.LeafFaces = buildWorldLeafFaceLookup(tree, faceLookup)
	geom.Lightmaps = lightmapPages

	return geom, nil
}

// worldFaceCenter computes the centroid of a face's vertices, used for distance-based
// sorting of translucent faces.
func worldFaceCenter(vertices []WorldVertex) [3]float32 {
	if len(vertices) == 0 {
		return [3]float32{}
	}
	var center [3]float32
	for _, vertex := range vertices {
		center[0] += vertex.Position[0]
		center[1] += vertex.Position[1]
		center[2] += vertex.Position[2]
	}
	scale := 1 / float32(len(vertices))
	center[0] *= scale
	center[1] *= scale
	center[2] *= scale
	return center
}

func worldFaceHasLitWater(textureFlags int32, lightmapSurface *faceLightmapSurface) bool {
	return textureFlags&model.SurfDrawTurb != 0 &&
		textureFlags&model.SurfDrawSky == 0 &&
		lightmapSurface != nil
}

// faceLightmapSurface is an internal result type tracking which atlas page a face's
// lightmap was allocated to during geometry building.
type faceLightmapSurface struct {
	pageIndex int
}

// extractFaceVertices extracts vertices for a BSP face by walking the surfedge table.
// Positive surfedge index means first vertex of edge, negative means second vertex (reversed
// winding). Computes both diffuse texture UVs and raw lightmap UVs.
func extractFaceVertices(tree *bsp.Tree, face *bsp.TreeFace, textureMeta []worldTextureMeta, allocator *LightmapAllocator, pages *[]WorldLightmapPage) ([]WorldVertex, *faceLightmapSurface, error) {
	if int(face.NumEdges) < 3 {
		return nil, nil, fmt.Errorf("face has < 3 edges")
	}

	vertices := make([]WorldVertex, 0, int(face.NumEdges))
	rawLightmapCoords := make([][2]float32, 0, int(face.NumEdges))
	var normal [3]float32
	if int(face.PlaneNum) < len(tree.Planes) {
		normal = tree.Planes[face.PlaneNum].Normal
		if face.Side != 0 {
			normal[0] = -normal[0]
			normal[1] = -normal[1]
			normal[2] = -normal[2]
		}
	}
	if n := float32(math.Sqrt(float64(normal[0]*normal[0] + normal[1]*normal[1] + normal[2]*normal[2]))); n > 1e-6 {
		normal[0] /= n
		normal[1] /= n
		normal[2] /= n
	} else {
		normal = [3]float32{0, 0, 1}
	}

	var texInfo *bsp.Texinfo
	textureWidth := float32(1)
	textureHeight := float32(1)
	if int(face.Texinfo) >= 0 && int(face.Texinfo) < len(tree.Texinfo) {
		texInfo = &tree.Texinfo[face.Texinfo]
		if int(texInfo.Miptex) >= 0 && int(texInfo.Miptex) < len(textureMeta) {
			meta := textureMeta[texInfo.Miptex]
			if meta.Width > 0 {
				textureWidth = float32(meta.Width)
			}
			if meta.Height > 0 {
				textureHeight = float32(meta.Height)
			}
		}
	}

	for i := int32(0); i < face.NumEdges; i++ {
		surfEdgeIdx := int(face.FirstEdge) + int(i)
		if surfEdgeIdx >= len(tree.Surfedges) {
			return nil, nil, fmt.Errorf("surfedge index %d out of range", surfEdgeIdx)
		}
		surfEdge := tree.Surfedges[surfEdgeIdx]

		var vertIdx uint32
		if surfEdge >= 0 {
			if int(surfEdge) >= len(tree.Edges) {
				return nil, nil, fmt.Errorf("edge index %d out of range", surfEdge)
			}
			vertIdx = tree.Edges[surfEdge].V[0]
		} else {
			edgeIdx := -surfEdge
			if int(edgeIdx) >= len(tree.Edges) {
				return nil, nil, fmt.Errorf("edge index %d out of range", edgeIdx)
			}
			vertIdx = tree.Edges[edgeIdx].V[1]
		}

		if int(vertIdx) >= len(tree.Vertexes) {
			return nil, nil, fmt.Errorf("vertex index %d out of range", vertIdx)
		}
		position := tree.Vertexes[vertIdx].Point

		texCoord := [2]float32{0, 0}
		lightmapCoord := [2]float32{0, 0}
		if texInfo != nil {
			u := position[0]*texInfo.Vecs[0][0] + position[1]*texInfo.Vecs[0][1] + position[2]*texInfo.Vecs[0][2] + texInfo.Vecs[0][3]
			v := position[0]*texInfo.Vecs[1][0] + position[1]*texInfo.Vecs[1][1] + position[2]*texInfo.Vecs[1][2] + texInfo.Vecs[1][3]
			texCoord[0] = u / textureWidth
			texCoord[1] = v / textureHeight
			rawLightmapCoords = append(rawLightmapCoords, [2]float32{u, v})
		}

		vertices = append(vertices, WorldVertex{
			Position:      position,
			TexCoord:      texCoord,
			LightmapCoord: lightmapCoord,
			Normal:        normal,
		})
	}

	lightmapSurface, err := assignFaceLightmap(vertices, rawLightmapCoords, face, tree, allocator, pages)
	if err != nil {
		return nil, nil, err
	}

	return vertices, lightmapSurface, nil
}

// assignFaceLightmap allocates a lightmap rectangle for a face in the atlas and computes
// lightmap texture coordinates. Each face gets a unique region in a shared lightmap page.
func assignFaceLightmap(vertices []WorldVertex, rawCoords [][2]float32, face *bsp.TreeFace, tree *bsp.Tree, allocator *LightmapAllocator, pages *[]WorldLightmapPage) (*faceLightmapSurface, error) {
	if face == nil || tree == nil || allocator == nil || len(vertices) == 0 || len(rawCoords) != len(vertices) || face.LightOfs < 0 || len(tree.Lighting) == 0 {
		return nil, nil
	}

	minU, maxU := rawCoords[0][0], rawCoords[0][0]
	minV, maxV := rawCoords[0][1], rawCoords[0][1]
	for i := 1; i < len(rawCoords); i++ {
		if rawCoords[i][0] < minU {
			minU = rawCoords[i][0]
		}
		if rawCoords[i][0] > maxU {
			maxU = rawCoords[i][0]
		}
		if rawCoords[i][1] < minV {
			minV = rawCoords[i][1]
		}
		if rawCoords[i][1] > maxV {
			maxV = rawCoords[i][1]
		}
	}

	textureMinU := float32(math.Floor(float64(minU/16.0))) * 16.0
	textureMinV := float32(math.Floor(float64(minV/16.0))) * 16.0
	extentU := int(math.Ceil(float64(maxU/16.0))*16.0 - float64(textureMinU))
	extentV := int(math.Ceil(float64(maxV/16.0))*16.0 - float64(textureMinV))
	if extentU < 0 {
		extentU = 0
	}
	if extentV < 0 {
		extentV = 0
	}
	smax := extentU/16 + 1
	tmax := extentV/16 + 1
	if smax <= 0 || tmax <= 0 {
		return nil, nil
	}

	texNum, x, y, err := allocator.AllocBlock(smax, tmax)
	if err != nil {
		return nil, fmt.Errorf("alloc face lightmap: %w", err)
	}
	for len(*pages) <= texNum {
		*pages = append(*pages, WorldLightmapPage{Width: worldLightmapPageSize, Height: worldLightmapPageSize})
	}

	styleCount := 0
	for _, style := range face.Styles {
		if style == 255 {
			break
		}
		styleCount++
	}
	if styleCount == 0 {
		styleCount = 1
	}

	sampleSize8 := smax * tmax * styleCount
	samples := expandLightmapSamples(tree.Lighting, tree.LightingRGB, int(face.LightOfs), sampleSize8)
	if samples == nil {
		return nil, nil
	}

	(*pages)[texNum].Surfaces = append((*pages)[texNum].Surfaces, WorldLightmapSurface{
		X:       x,
		Y:       y,
		Width:   smax,
		Height:  tmax,
		Styles:  face.Styles,
		Samples: samples,
	})

	for i := range vertices {
		lightS := (rawCoords[i][0]-textureMinU)/16.0 + float32(x) + 0.5
		lightT := (rawCoords[i][1]-textureMinV)/16.0 + float32(y) + 0.5
		vertices[i].LightmapCoord = [2]float32{
			lightS / float32(worldLightmapPageSize),
			lightT / float32(worldLightmapPageSize),
		}
	}

	return &faceLightmapSurface{pageIndex: texNum}, nil
}

// buildWorldRenderData builds complete CPU-side render data: geometry, lightmaps, and bounding box.
func buildWorldRenderData(tree *bsp.Tree) (*WorldRenderData, error) {
	return buildModelRenderData(tree, 0)
}

// buildModelRenderData builds render data for a specific BSP submodel index (used for brush entities like doors).
func buildModelRenderData(tree *bsp.Tree, modelIndex int) (*WorldRenderData, error) {
	geom, err := BuildModelGeometry(tree, modelIndex)
	if err != nil {
		return nil, err
	}

	renderData := &WorldRenderData{
		Geometry:      geom,
		Lightmaps:     append([]WorldLightmapPage(nil), geom.Lightmaps...),
		HasLitWater:   geom.HasLitWater,
		TotalVertices: len(geom.Vertices),
		TotalIndices:  len(geom.Indices),
		TotalFaces:    len(geom.Faces),
	}
	if len(geom.Vertices) == 0 {
		return renderData, nil
	}

	boundsMin := geom.Vertices[0].Position
	boundsMax := geom.Vertices[0].Position
	for i := 1; i < len(geom.Vertices); i++ {
		position := geom.Vertices[i].Position
		if position[0] < boundsMin[0] {
			boundsMin[0] = position[0]
		}
		if position[1] < boundsMin[1] {
			boundsMin[1] = position[1]
		}
		if position[2] < boundsMin[2] {
			boundsMin[2] = position[2]
		}
		if position[0] > boundsMax[0] {
			boundsMax[0] = position[0]
		}
		if position[1] > boundsMax[1] {
			boundsMax[1] = position[1]
		}
		if position[2] > boundsMax[2] {
			boundsMax[2] = position[2]
		}
	}
	renderData.BoundsMin = boundsMin
	renderData.BoundsMax = boundsMax
	return renderData, nil
}

// ---- merged from world_render_opengl_root.go ----
// ensureWorldProgram lazily compiles the world rendering shader program. The world shader performs multi-texture rendering: diffuse texture * lightmap, with optional fullbright overlay and dynamic light contribution.
func (r *Renderer) ensureWorldProgram() error {
	if r.worldProgram != 0 {
		return nil
	}

	vs, err := compileShader(worldopengl.WorldVertexShaderGL, gl.VERTEX_SHADER)
	if err != nil {
		return fmt.Errorf("compile world vertex shader: %w", err)
	}
	fs, err := compileShader(worldopengl.WorldFragmentShaderGL, gl.FRAGMENT_SHADER)
	if err != nil {
		gl.DeleteShader(vs)
		return fmt.Errorf("compile world fragment shader: %w", err)
	}

	program := createProgram(vs, fs)
	r.worldProgram = program
	r.worldVPUniform = gl.GetUniformLocation(program, gl.Str("uViewProjection\x00"))
	r.worldTextureUniform = gl.GetUniformLocation(program, gl.Str("uTexture\x00"))
	r.worldLightmapUniform = gl.GetUniformLocation(program, gl.Str("uLightmap\x00"))
	r.worldFullbrightUniform = gl.GetUniformLocation(program, gl.Str("uFullbright\x00"))
	r.worldHasFullbrightUniform = gl.GetUniformLocation(program, gl.Str("uHasFullbright\x00"))
	r.worldDynamicLightUniform = gl.GetUniformLocation(program, gl.Str("uDynamicLight\x00"))
	r.worldModelOffsetUniform = gl.GetUniformLocation(program, gl.Str("uModelOffset\x00"))
	r.worldModelRotationUniform = gl.GetUniformLocation(program, gl.Str("uModelRotation\x00"))
	r.worldModelScaleUniform = gl.GetUniformLocation(program, gl.Str("uModelScale\x00"))
	r.worldAlphaUniform = gl.GetUniformLocation(program, gl.Str("uAlpha\x00"))
	r.worldTimeUniform = gl.GetUniformLocation(program, gl.Str("uTime\x00"))
	r.worldTurbulentUniform = gl.GetUniformLocation(program, gl.Str("uTurbulent\x00"))
	r.worldLitWaterUniform = gl.GetUniformLocation(program, gl.Str("uLitWater\x00"))
	r.worldCameraOriginUniform = gl.GetUniformLocation(program, gl.Str("uCameraOrigin\x00"))
	r.worldFogColorUniform = gl.GetUniformLocation(program, gl.Str("uFogColor\x00"))
	r.worldFogDensityUniform = gl.GetUniformLocation(program, gl.Str("uFogDensity\x00"))
	return nil
}

// lightstyleScale looks up a lightstyle's current brightness from the 64-element value array. The 255 sentinel (no light) returns 0.
func lightstyleScale(values [64]float32, style uint8) float32 {
	if int(style) >= len(values) {
		return 0
	}
	return values[style]
}

// setFogState updates the fog color and density values used by world and sky shader fog calculations.
func (r *Renderer) setFogState(color [3]float32, density float32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.worldFogColor, r.worldFogDensity = blendFogStateTowards(r.worldFogColor, r.worldFogDensity, color, density, 0.2)
}

// renderWorld renders the world BSP geometry using the specified pass selector. Binds the world shader, sets the view-projection matrix and camera uniforms, buckets faces by type (sky, opaque, liquid, translucent), and issues draw calls with per-face diffuse + lightmap + fullbright texture binds.
func (r *Renderer) renderWorld(selector worldBrushPassSelector) {
	selector = normalizeWorldBrushPassSelector(selector)
	drawSky := selector.includesSky()
	drawNonLiquid := selector.includesNonLiquid()
	drawLiquidOpaque := selector.includesLiquidOpaque()
	drawLiquidTranslucent := selector.includesLiquidTranslucent()

	r.mu.RLock()
	program := r.worldProgram
	skyProgram := r.worldSkyProgram
	skyProceduralProgram := r.worldSkyProceduralProgram
	skyCubemapProgram := r.worldSkyCubemapProgram
	skyExternalFaceProgram := r.worldSkyExternalFaceProgram
	vao := r.worldVAO
	indexCount := r.worldIndexCount
	vp := r.viewMatrices.VP
	camera := r.cameraState
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
	timeUniform := r.worldTimeUniform
	turbulentUniform := r.worldTurbulentUniform
	litWaterUniform := r.worldLitWaterUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform
	skyVPUniform := r.worldSkyVPUniform
	skySolidUniform := r.worldSkySolidUniform
	skyAlphaUniform := r.worldSkyAlphaUniform
	skyProceduralVPUniform := r.worldSkyProceduralVPUniform
	skyCubemapVPUniform := r.worldSkyCubemapVPUniform
	skyCubemapUniform := r.worldSkyCubemapUniform
	skyExternalFaceVPUniform := r.worldSkyExternalFaceVPUniform
	skyExternalFaceRTUniform := r.worldSkyExternalFaceRTUniform
	skyExternalFaceBKUniform := r.worldSkyExternalFaceBKUniform
	skyExternalFaceLFUniform := r.worldSkyExternalFaceLFUniform
	skyExternalFaceFTUniform := r.worldSkyExternalFaceFTUniform
	skyExternalFaceUPUniform := r.worldSkyExternalFaceUPUniform
	skyExternalFaceDNUniform := r.worldSkyExternalFaceDNUniform
	skyModelOffsetUniform := r.worldSkyModelOffsetUniform
	skyModelRotationUniform := r.worldSkyModelRotationUniform
	skyModelScaleUniform := r.worldSkyModelScaleUniform
	skyProceduralModelOffsetUniform := r.worldSkyProceduralModelOffset
	skyProceduralModelRotationUniform := r.worldSkyProceduralModelRotation
	skyProceduralModelScaleUniform := r.worldSkyProceduralModelScale
	skyCubemapModelOffsetUniform := r.worldSkyCubemapModelOffsetUniform
	skyCubemapModelRotationUniform := r.worldSkyCubemapModelRotationUniform
	skyCubemapModelScaleUniform := r.worldSkyCubemapModelScaleUniform
	skyExternalFaceModelOffsetUniform := r.worldSkyExternalFaceModelOffset
	skyExternalFaceModelRotationUniform := r.worldSkyExternalFaceModelRotation
	skyExternalFaceModelScaleUniform := r.worldSkyExternalFaceModelScale
	skyTimeUniform := r.worldSkyTimeUniform
	skySolidLayerSpeedUniform := r.worldSkySolidLayerSpeedUniform
	skyAlphaLayerSpeedUniform := r.worldSkyAlphaLayerSpeedUniform
	skyCameraOriginUniform := r.worldSkyCameraOriginUniform
	skyProceduralCameraOriginUniform := r.worldSkyProceduralCameraOrigin
	skyCubemapCameraOriginUniform := r.worldSkyCubemapCameraOriginUniform
	skyExternalFaceCameraOriginUniform := r.worldSkyExternalFaceCameraOrigin
	skyFogColorUniform := r.worldSkyFogColorUniform
	skyProceduralFogColorUniform := r.worldSkyProceduralFogColor
	skyCubemapFogColorUniform := r.worldSkyCubemapFogColorUniform
	skyExternalFaceFogColorUniform := r.worldSkyExternalFaceFogColor
	skyFogDensityUniform := r.worldSkyFogDensityUniform
	skyProceduralFogDensityUniform := r.worldSkyProceduralFogDensity
	skyProceduralHorizonColorUniform := r.worldSkyProceduralHorizonColor
	skyProceduralZenithColorUniform := r.worldSkyProceduralZenithColor
	skyCubemapFogDensityUniform := r.worldSkyCubemapFogDensityUniform
	skyExternalFaceFogDensityUniform := r.worldSkyExternalFaceFogDensity
	fallbackTexture := r.worldFallbackTexture
	skyFallbackAlpha := r.worldSkyAlphaFallback
	worldFastSky := readWorldFastSkyEnabled()
	worldProceduralSky := readWorldProceduralSkyEnabled()
	skySolidLayerSpeed := readWorldSkySolidSpeedCvar()
	skyAlphaLayerSpeed := readWorldSkyAlphaSpeedCvar()
	skyExternalCubemap := r.worldSkyExternalCubemap
	skyExternalFaceTextures := r.worldSkyExternalFaceTextures
	skyExternalMode := r.worldSkyExternalMode
	fallbackLightmap := r.worldLightmapFallback
	liquidAlphaOverrides := r.worldLiquidAlphaOverrides
	skyFogOverride := r.worldSkyFogOverride
	worldTree := r.worldTree
	worldHasLitWater := r.worldHasLitWater
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity
	proceduralSkyHorizon, proceduralSkyZenith := proceduralSkyGradientColors()
	allFaces := []WorldFace(nil)
	leafFaces := [][]int(nil)
	if r.worldData != nil && r.worldData.Geometry != nil {
		allFaces = append(allFaces, r.worldData.Geometry.Faces...)
		leafFaces = r.worldData.Geometry.LeafFaces
	}
	worldTextures := make(map[int32]uint32, len(r.worldTextures))
	for k, v := range r.worldTextures {
		worldTextures[k] = v
	}
	worldFullbrightTextures := make(map[int32]uint32, len(r.worldFullbrightTextures))
	for k, v := range r.worldFullbrightTextures {
		worldFullbrightTextures[k] = v
	}
	worldSkySolidTextures := make(map[int32]uint32, len(r.worldSkySolidTextures))
	for k, v := range r.worldSkySolidTextures {
		worldSkySolidTextures[k] = v
	}
	worldSkyAlphaTextures := make(map[int32]uint32, len(r.worldSkyAlphaTextures))
	for k, v := range r.worldSkyAlphaTextures {
		worldSkyAlphaTextures[k] = v
	}
	worldSkyFlatTextures := make(map[int32]uint32, len(r.worldSkyFlatTextures))
	for k, v := range r.worldSkyFlatTextures {
		worldSkyFlatTextures[k] = v
	}
	worldTextureAnimations := append([]*SurfaceTexture(nil), r.worldTextureAnimations...)
	worldLightmaps := append([]uint32(nil), r.worldLightmaps...)
	lightPool := r.lightPool // Get light pool for light evaluation
	r.mu.RUnlock()

	if program == 0 || skyProgram == 0 || skyCubemapProgram == 0 || skyExternalFaceProgram == 0 || vao == 0 || indexCount <= 0 {
		return
	}
	faces := selectVisibleWorldFaces(worldTree, allFaces, leafFaces, [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z})

	liquidAlpha := worldLiquidAlphaSettingsFromCvars(liquidAlphaOverrides, worldTree)
	skyFogFactor := resolveWorldSkyFogMix(readWorldSkyFogCvar(0.5), skyFogOverride, fogDensity)
	skyFaces, opaqueFaces, alphaTestFaces, liquidOpaqueFaces, liquidTranslucentFaces, translucentFaces := bucketWorldFacesWithLights(faces, worldHasLitWater, worldTextures, worldFullbrightTextures, worldTextureAnimations, worldLightmaps, fallbackTexture, fallbackLightmap, vao, [3]float32{}, identityModelRotationMatrix, 1, 1, 0, float64(camera.Time), camera, liquidAlpha, lightPool)
	bindWorldProgram := func() {
		gl.UseProgram(program)
		gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
		gl.Uniform1i(textureUniform, 0)
		gl.Uniform1i(lightmapUniform, 1)
		gl.Uniform1i(fullbrightUniform, 2)
		gl.Uniform1f(hasFullbrightUniform, 0)
		gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
		gl.Uniform3f(modelOffsetUniform, 0, 0, 0)
		gl.UniformMatrix4fv(modelRotationUniform, 1, false, &identityModelRotationMatrix[0])
		gl.Uniform1f(modelScaleUniform, 1)
		gl.Uniform1f(timeUniform, camera.Time)
		gl.Uniform1f(turbulentUniform, 0)
		gl.Uniform1f(litWaterUniform, 0)
		gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
		gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
		gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
		gl.BindVertexArray(vao)
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	bindWorldProgram()
	if len(faces) == 0 {
		if drawNonLiquid {
			gl.DepthMask(true)
			gl.Disable(gl.BLEND)
			gl.Uniform1f(alphaUniform, 1)
			gl.ActiveTexture(gl.TEXTURE0)
			gl.BindTexture(gl.TEXTURE_2D, fallbackTexture)
			gl.ActiveTexture(gl.TEXTURE1)
			gl.BindTexture(gl.TEXTURE_2D, fallbackLightmap)
			gl.ActiveTexture(gl.TEXTURE0)
			gl.DrawElements(gl.TRIANGLES, indexCount, gl.UNSIGNED_INT, unsafe.Pointer(nil))
		}
	} else {
		if drawSky {
			renderSkyPass(skyFaces, skyPassState{
				program:                     skyProgram,
				proceduralProgram:           skyProceduralProgram,
				cubemapProgram:              skyCubemapProgram,
				vpUniform:                   skyVPUniform,
				solidUniform:                skySolidUniform,
				alphaUniform:                skyAlphaUniform,
				proceduralVPUniform:         skyProceduralVPUniform,
				cubemapVPUniform:            skyCubemapVPUniform,
				cubemapUniform:              skyCubemapUniform,
				externalFaceVPUniform:       skyExternalFaceVPUniform,
				externalFaceRTUniform:       skyExternalFaceRTUniform,
				externalFaceBKUniform:       skyExternalFaceBKUniform,
				externalFaceLFUniform:       skyExternalFaceLFUniform,
				externalFaceFTUniform:       skyExternalFaceFTUniform,
				externalFaceUPUniform:       skyExternalFaceUPUniform,
				externalFaceDNUniform:       skyExternalFaceDNUniform,
				modelOffsetUniform:          skyModelOffsetUniform,
				modelRotationUniform:        skyModelRotationUniform,
				modelScaleUniform:           skyModelScaleUniform,
				proceduralModelOffset:       skyProceduralModelOffsetUniform,
				proceduralModelRotation:     skyProceduralModelRotationUniform,
				proceduralModelScale:        skyProceduralModelScaleUniform,
				cubemapModelOffsetUniform:   skyCubemapModelOffsetUniform,
				cubemapModelRotationUniform: skyCubemapModelRotationUniform,
				cubemapModelScaleUniform:    skyCubemapModelScaleUniform,
				externalFaceModelOffset:     skyExternalFaceModelOffsetUniform,
				externalFaceModelRotation:   skyExternalFaceModelRotationUniform,
				externalFaceModelScale:      skyExternalFaceModelScaleUniform,
				timeUniform:                 skyTimeUniform,
				solidLayerSpeedUniform:      skySolidLayerSpeedUniform,
				alphaLayerSpeedUniform:      skyAlphaLayerSpeedUniform,
				cameraOriginUniform:         skyCameraOriginUniform,
				proceduralCameraOrigin:      skyProceduralCameraOriginUniform,
				cubemapCameraOriginUniform:  skyCubemapCameraOriginUniform,
				externalFaceCameraOrigin:    skyExternalFaceCameraOriginUniform,
				fogColorUniform:             skyFogColorUniform,
				proceduralFogColor:          skyProceduralFogColorUniform,
				cubemapFogColorUniform:      skyCubemapFogColorUniform,
				externalFaceFogColor:        skyExternalFaceFogColorUniform,
				fogDensityUniform:           skyFogDensityUniform,
				proceduralFogDensity:        skyProceduralFogDensityUniform,
				proceduralHorizonColor:      skyProceduralHorizonColorUniform,
				proceduralZenithColor:       skyProceduralZenithColorUniform,
				cubemapFogDensityUniform:    skyCubemapFogDensityUniform,
				externalFaceFogDensity:      skyExternalFaceFogDensityUniform,
				vp:                          vp,
				time:                        camera.Time,
				solidLayerSpeed:             skySolidLayerSpeed,
				alphaLayerSpeed:             skyAlphaLayerSpeed,
				cameraOrigin:                [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
				modelOffset:                 [3]float32{0, 0, 0},
				modelRotation:               identityModelRotationMatrix,
				modelScale:                  1,
				fogColor:                    fogColor,
				proceduralHorizon:           proceduralSkyHorizon,
				proceduralZenith:            proceduralSkyZenith,
				fogDensity:                  skyFogFactor,
				solidTextures:               worldSkySolidTextures,
				alphaTextures:               worldSkyAlphaTextures,
				flatTextures:                worldSkyFlatTextures,
				textureAnimations:           worldTextureAnimations,
				fallbackSolid:               fallbackTexture,
				fallbackAlpha:               skyFallbackAlpha,
				externalFaceProgram:         skyExternalFaceProgram,
				externalCubemap:             skyExternalCubemap,
				externalFaceTextures:        skyExternalFaceTextures,
				externalSkyMode:             skyExternalMode,
				fastSky:                     worldFastSky,
				proceduralSky:               shouldUseProceduralSky(worldFastSky, worldProceduralSky, skyExternalMode),
			})
			if drawNonLiquid || drawLiquidOpaque || drawLiquidTranslucent {
				bindWorldProgram()
			}
		}
		if drawNonLiquid {
			renderWorldDrawCalls(opaqueFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
			renderWorldDrawCalls(alphaTestFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
			r.mu.Lock()
			r.translucentCalls = append(r.translucentCalls, translucentFaces...)
			r.mu.Unlock()
		}
		if drawLiquidOpaque {
			renderWorldDrawCalls(liquidOpaqueFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
		}
		if drawLiquidTranslucent {
			r.mu.Lock()
			r.translucentCalls = append(r.translucentCalls, liquidTranslucentFaces...)
			r.mu.Unlock()
		}
	}
	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)

	gl.Enable(gl.BLEND)
}

// renderBrushEntities renders BSP brush entities (doors, platforms, lifts). Each entity has a model offset, rotation matrix, and optional alpha. Uses the same world shader with model transform uniforms.
func (r *Renderer) renderBrushEntities(entities []BrushEntity, selector worldBrushPassSelector) {
	if len(entities) == 0 {
		return
	}
	selector = normalizeWorldBrushPassSelector(selector)
	drawSky := selector.includesSky()
	drawNonLiquid := selector.includesNonLiquid()
	drawLiquidOpaque := selector.includesLiquidOpaque()
	drawLiquidTranslucent := selector.includesLiquidTranslucent()

	r.mu.Lock()
	program := r.worldProgram
	skyProgram := r.worldSkyProgram
	skyProceduralProgram := r.worldSkyProceduralProgram
	skyCubemapProgram := r.worldSkyCubemapProgram
	skyExternalFaceProgram := r.worldSkyExternalFaceProgram
	vp := r.viewMatrices.VP
	camera := r.cameraState
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
	timeUniform := r.worldTimeUniform
	turbulentUniform := r.worldTurbulentUniform
	litWaterUniform := r.worldLitWaterUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform
	skyVPUniform := r.worldSkyVPUniform
	skySolidUniform := r.worldSkySolidUniform
	skyAlphaUniform := r.worldSkyAlphaUniform
	skyProceduralVPUniform := r.worldSkyProceduralVPUniform
	skyCubemapVPUniform := r.worldSkyCubemapVPUniform
	skyCubemapUniform := r.worldSkyCubemapUniform
	skyExternalFaceVPUniform := r.worldSkyExternalFaceVPUniform
	skyExternalFaceRTUniform := r.worldSkyExternalFaceRTUniform
	skyExternalFaceBKUniform := r.worldSkyExternalFaceBKUniform
	skyExternalFaceLFUniform := r.worldSkyExternalFaceLFUniform
	skyExternalFaceFTUniform := r.worldSkyExternalFaceFTUniform
	skyExternalFaceUPUniform := r.worldSkyExternalFaceUPUniform
	skyExternalFaceDNUniform := r.worldSkyExternalFaceDNUniform
	skyModelOffsetUniform := r.worldSkyModelOffsetUniform
	skyModelRotationUniform := r.worldSkyModelRotationUniform
	skyModelScaleUniform := r.worldSkyModelScaleUniform
	skyProceduralModelOffsetUniform := r.worldSkyProceduralModelOffset
	skyProceduralModelRotationUniform := r.worldSkyProceduralModelRotation
	skyProceduralModelScaleUniform := r.worldSkyProceduralModelScale
	skyCubemapModelOffsetUniform := r.worldSkyCubemapModelOffsetUniform
	skyCubemapModelRotationUniform := r.worldSkyCubemapModelRotationUniform
	skyCubemapModelScaleUniform := r.worldSkyCubemapModelScaleUniform
	skyExternalFaceModelOffsetUniform := r.worldSkyExternalFaceModelOffset
	skyExternalFaceModelRotationUniform := r.worldSkyExternalFaceModelRotation
	skyExternalFaceModelScaleUniform := r.worldSkyExternalFaceModelScale
	skyTimeUniform := r.worldSkyTimeUniform
	skySolidLayerSpeedUniform := r.worldSkySolidLayerSpeedUniform
	skyAlphaLayerSpeedUniform := r.worldSkyAlphaLayerSpeedUniform
	skyCameraOriginUniform := r.worldSkyCameraOriginUniform
	skyProceduralCameraOriginUniform := r.worldSkyProceduralCameraOrigin
	skyCubemapCameraOriginUniform := r.worldSkyCubemapCameraOriginUniform
	skyExternalFaceCameraOriginUniform := r.worldSkyExternalFaceCameraOrigin
	skyFogColorUniform := r.worldSkyFogColorUniform
	skyProceduralFogColorUniform := r.worldSkyProceduralFogColor
	skyCubemapFogColorUniform := r.worldSkyCubemapFogColorUniform
	skyExternalFaceFogColorUniform := r.worldSkyExternalFaceFogColor
	skyFogDensityUniform := r.worldSkyFogDensityUniform
	skyProceduralFogDensityUniform := r.worldSkyProceduralFogDensity
	skyProceduralHorizonColorUniform := r.worldSkyProceduralHorizonColor
	skyProceduralZenithColorUniform := r.worldSkyProceduralZenithColor
	skyCubemapFogDensityUniform := r.worldSkyCubemapFogDensityUniform
	skyExternalFaceFogDensityUniform := r.worldSkyExternalFaceFogDensity
	fallbackTexture := r.worldFallbackTexture
	skyFallbackAlpha := r.worldSkyAlphaFallback
	worldFastSky := readWorldFastSkyEnabled()
	worldProceduralSky := readWorldProceduralSkyEnabled()
	skySolidLayerSpeed := readWorldSkySolidSpeedCvar()
	skyAlphaLayerSpeed := readWorldSkyAlphaSpeedCvar()
	skyExternalCubemap := r.worldSkyExternalCubemap
	skyExternalFaceTextures := r.worldSkyExternalFaceTextures
	skyExternalMode := r.worldSkyExternalMode
	fallbackLightmap := r.worldLightmapFallback
	liquidAlphaOverrides := r.worldLiquidAlphaOverrides
	skyFogOverride := r.worldSkyFogOverride
	worldTree := r.worldTree
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity
	proceduralSkyHorizon, proceduralSkyZenith := proceduralSkyGradientColors()
	worldTextures := make(map[int32]uint32, len(r.worldTextures))
	for k, v := range r.worldTextures {
		worldTextures[k] = v
	}
	worldFullbrightTextures := make(map[int32]uint32, len(r.worldFullbrightTextures))
	for k, v := range r.worldFullbrightTextures {
		worldFullbrightTextures[k] = v
	}
	worldSkySolidTextures := make(map[int32]uint32, len(r.worldSkySolidTextures))
	for k, v := range r.worldSkySolidTextures {
		worldSkySolidTextures[k] = v
	}
	worldSkyAlphaTextures := make(map[int32]uint32, len(r.worldSkyAlphaTextures))
	for k, v := range r.worldSkyAlphaTextures {
		worldSkyAlphaTextures[k] = v
	}
	worldSkyFlatTextures := make(map[int32]uint32, len(r.worldSkyFlatTextures))
	for k, v := range r.worldSkyFlatTextures {
		worldSkyFlatTextures[k] = v
	}
	worldTextureAnimations := append([]*SurfaceTexture(nil), r.worldTextureAnimations...)
	lightPool := r.lightPool // Get light pool for light evaluation
	type drawBrush struct {
		frame       int
		origin      [3]float32
		rotation    [16]float32
		alpha       float32
		scale       float32
		hasLitWater bool
		mesh        *glWorldMesh
	}
	brushes := make([]drawBrush, 0, len(entities))
	for _, entity := range entities {
		mesh := r.ensureBrushModelLocked(entity.SubmodelIndex)
		if mesh == nil {
			continue
		}
		brushes = append(brushes, drawBrush{
			frame:       entity.Frame,
			origin:      entity.Origin,
			rotation:    buildBrushRotationMatrix(entity.Angles),
			alpha:       entity.Alpha,
			scale:       entity.Scale,
			hasLitWater: mesh.hasLitWater,
			mesh:        mesh,
		})
	}
	r.mu.Unlock()

	if program == 0 || skyProgram == 0 || skyCubemapProgram == 0 || skyExternalFaceProgram == 0 || len(brushes) == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.Uniform1i(fullbrightUniform, 2)
	gl.Uniform1f(hasFullbrightUniform, 0)
	gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.Uniform1f(timeUniform, camera.Time)
	gl.Uniform1f(turbulentUniform, 0)
	gl.Uniform1f(litWaterUniform, 0)
	gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
	gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
	gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
	liquidAlpha := worldLiquidAlphaSettingsFromCvars(liquidAlphaOverrides, worldTree)
	skyFogFactor := resolveWorldSkyFogMix(readWorldSkyFogCvar(0.5), skyFogOverride, fogDensity)

	for _, brush := range brushes {
		skyFaces, opaqueFaces, alphaTestFaces, liquidOpaqueFaces, liquidTranslucentFaces, translucentFaces := bucketWorldFacesWithLights(brush.mesh.faces, brush.hasLitWater, worldTextures, worldFullbrightTextures, worldTextureAnimations, brush.mesh.lightmaps, fallbackTexture, fallbackLightmap, brush.mesh.vao, brush.origin, brush.rotation, brush.scale, brush.alpha, brush.frame, float64(camera.Time), camera, liquidAlpha, lightPool)
		bindBrushWorldProgram := func() {
			gl.UseProgram(program)
			gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
			gl.Uniform1i(textureUniform, 0)
			gl.Uniform1i(lightmapUniform, 1)
			gl.Uniform1i(fullbrightUniform, 2)
			gl.Uniform1f(hasFullbrightUniform, 0)
			gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
			gl.Uniform1f(timeUniform, camera.Time)
			gl.Uniform1f(turbulentUniform, 0)
			gl.Uniform1f(litWaterUniform, 0)
			gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
			gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
			gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
			gl.Uniform3f(modelOffsetUniform, brush.origin[0], brush.origin[1], brush.origin[2])
			gl.UniformMatrix4fv(modelRotationUniform, 1, false, &brush.rotation[0])
			gl.Uniform1f(modelScaleUniform, brush.scale)
			gl.BindVertexArray(brush.mesh.vao)
		}
		bindBrushWorldProgram()
		if drawSky {
			renderSkyPass(skyFaces, skyPassState{
				program:                     skyProgram,
				proceduralProgram:           skyProceduralProgram,
				cubemapProgram:              skyCubemapProgram,
				vpUniform:                   skyVPUniform,
				solidUniform:                skySolidUniform,
				alphaUniform:                skyAlphaUniform,
				proceduralVPUniform:         skyProceduralVPUniform,
				cubemapVPUniform:            skyCubemapVPUniform,
				cubemapUniform:              skyCubemapUniform,
				externalFaceVPUniform:       skyExternalFaceVPUniform,
				externalFaceRTUniform:       skyExternalFaceRTUniform,
				externalFaceBKUniform:       skyExternalFaceBKUniform,
				externalFaceLFUniform:       skyExternalFaceLFUniform,
				externalFaceFTUniform:       skyExternalFaceFTUniform,
				externalFaceUPUniform:       skyExternalFaceUPUniform,
				externalFaceDNUniform:       skyExternalFaceDNUniform,
				modelOffsetUniform:          skyModelOffsetUniform,
				modelRotationUniform:        skyModelRotationUniform,
				modelScaleUniform:           skyModelScaleUniform,
				proceduralModelOffset:       skyProceduralModelOffsetUniform,
				proceduralModelRotation:     skyProceduralModelRotationUniform,
				proceduralModelScale:        skyProceduralModelScaleUniform,
				cubemapModelOffsetUniform:   skyCubemapModelOffsetUniform,
				cubemapModelRotationUniform: skyCubemapModelRotationUniform,
				cubemapModelScaleUniform:    skyCubemapModelScaleUniform,
				externalFaceModelOffset:     skyExternalFaceModelOffsetUniform,
				externalFaceModelRotation:   skyExternalFaceModelRotationUniform,
				externalFaceModelScale:      skyExternalFaceModelScaleUniform,
				timeUniform:                 skyTimeUniform,
				solidLayerSpeedUniform:      skySolidLayerSpeedUniform,
				alphaLayerSpeedUniform:      skyAlphaLayerSpeedUniform,
				cameraOriginUniform:         skyCameraOriginUniform,
				proceduralCameraOrigin:      skyProceduralCameraOriginUniform,
				cubemapCameraOriginUniform:  skyCubemapCameraOriginUniform,
				externalFaceCameraOrigin:    skyExternalFaceCameraOriginUniform,
				fogColorUniform:             skyFogColorUniform,
				proceduralFogColor:          skyProceduralFogColorUniform,
				cubemapFogColorUniform:      skyCubemapFogColorUniform,
				externalFaceFogColor:        skyExternalFaceFogColorUniform,
				fogDensityUniform:           skyFogDensityUniform,
				proceduralFogDensity:        skyProceduralFogDensityUniform,
				proceduralHorizonColor:      skyProceduralHorizonColorUniform,
				proceduralZenithColor:       skyProceduralZenithColorUniform,
				cubemapFogDensityUniform:    skyCubemapFogDensityUniform,
				externalFaceFogDensity:      skyExternalFaceFogDensityUniform,
				vp:                          vp,
				time:                        camera.Time,
				solidLayerSpeed:             skySolidLayerSpeed,
				alphaLayerSpeed:             skyAlphaLayerSpeed,
				cameraOrigin:                [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
				modelOffset:                 brush.origin,
				modelRotation:               brush.rotation,
				modelScale:                  brush.scale,
				fogColor:                    fogColor,
				proceduralHorizon:           proceduralSkyHorizon,
				proceduralZenith:            proceduralSkyZenith,
				fogDensity:                  skyFogFactor,
				solidTextures:               worldSkySolidTextures,
				alphaTextures:               worldSkyAlphaTextures,
				flatTextures:                worldSkyFlatTextures,
				textureAnimations:           worldTextureAnimations,
				fallbackSolid:               fallbackTexture,
				fallbackAlpha:               skyFallbackAlpha,
				externalFaceProgram:         skyExternalFaceProgram,
				externalCubemap:             skyExternalCubemap,
				externalFaceTextures:        skyExternalFaceTextures,
				externalSkyMode:             skyExternalMode,
				frame:                       brush.frame,
				fastSky:                     worldFastSky,
				proceduralSky:               shouldUseProceduralSky(worldFastSky, worldProceduralSky, skyExternalMode),
			})
			if drawNonLiquid || drawLiquidOpaque || drawLiquidTranslucent {
				bindBrushWorldProgram()
			}
		}
		if drawNonLiquid {
			renderWorldDrawCalls(opaqueFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
			renderWorldDrawCalls(alphaTestFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
			r.mu.Lock()
			r.translucentCalls = append(r.translucentCalls, translucentFaces...)
			r.mu.Unlock()
		}
		if drawLiquidOpaque {
			renderWorldDrawCalls(liquidOpaqueFaces, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform, true)
		}
		if drawLiquidTranslucent {
			r.mu.Lock()
			r.translucentCalls = append(r.translucentCalls, liquidTranslucentFaces...)
			r.mu.Unlock()
		}
	}

	gl.BindVertexArray(0)
	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	gl.Enable(gl.BLEND)
}

// bucketWorldFacesWithLights is like bucketWorldFaces but also evaluates dynamic lights.
// This variant accepts a light pool and computes light contributions for each face.
func bucketWorldFacesWithLights(faces []WorldFace, hasLitWater bool, textures map[int32]uint32, fullbrightTextures map[int32]uint32, textureAnimations []*SurfaceTexture, lightmaps []uint32, fallbackTexture, fallbackLightmap, vao uint32, modelOffset [3]float32, modelRotation [16]float32, modelScale, entityAlpha float32, entityFrame int, timeSeconds float64, camera CameraState, liquidAlpha worldLiquidAlphaSettings, lightPool *glLightPool) (sky, opaque, alphaTest, liquidOpaque, liquidTranslucent, translucent []worldDrawCall) {
	var evaluateLights worldopengl.LightEvaluator
	if lightPool != nil {
		evaluateLights = func(point [3]float32) [3]float32 {
			return lightPool.EvaluateLightsAtPoint(point)
		}
	}
	return worldopengl.BucketFacesWithLights(
		faces,
		hasLitWater,
		textures,
		fullbrightTextures,
		textureAnimations,
		lightmaps,
		fallbackTexture,
		fallbackLightmap,
		vao,
		modelOffset,
		modelRotation,
		modelScale,
		entityAlpha,
		entityFrame,
		timeSeconds,
		[3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z},
		liquidAlpha.toWorld(),
		TextureAnimation,
		evaluateLights,
	)
}

// renderWorldDrawCalls issues GL draw calls for bucketed world faces. Each call binds its diffuse + lightmap + fullbright textures and draws the face's index range from the VAO.
func renderWorldDrawCalls(calls []worldDrawCall, alphaUniform, turbulentUniform, litWaterUniform, dynamicLightUniform, modelOffsetUniform, modelRotationUniform, modelScaleUniform, hasFullbrightUniform int32, depthWrite bool) {
	if len(calls) == 0 {
		return
	}
	litWaterEnabled := worldLitWaterCvarEnabled()
	gl.DepthMask(depthWrite)
	if depthWrite {
		gl.Disable(gl.BLEND)
	} else {
		gl.Enable(gl.BLEND)
	}

	lastVAO := uint32(0xFFFFFFFF)
	lastLitWaterValue := float32(-1)
	for _, call := range calls {
		if call.VAO != lastVAO {
			gl.BindVertexArray(call.VAO)
			lastVAO = call.VAO
		}
		gl.Uniform3f(modelOffsetUniform, call.ModelOffset[0], call.ModelOffset[1], call.ModelOffset[2])
		gl.UniformMatrix4fv(modelRotationUniform, 1, false, &call.ModelRotation[0])
		gl.Uniform1f(modelScaleUniform, call.ModelScale)

		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, call.Texture)
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, call.Lightmap)

		// Bind fullbright texture if available
		gl.ActiveTexture(gl.TEXTURE2)
		if call.FullbrightTexture != 0 {
			gl.BindTexture(gl.TEXTURE_2D, call.FullbrightTexture)
			gl.Uniform1f(hasFullbrightUniform, 1.0)
		} else {
			gl.BindTexture(gl.TEXTURE_2D, 0)
			gl.Uniform1f(hasFullbrightUniform, 0.0)
		}

		gl.ActiveTexture(gl.TEXTURE0)
		if call.Turbulent {
			gl.Uniform1f(turbulentUniform, 1)
		} else {
			gl.Uniform1f(turbulentUniform, 0)
		}
		if litWaterUniform >= 0 {
			litWaterValue := float32(0)
			if litWaterEnabled && call.HasLitWater {
				litWaterValue = 1
			}
			if litWaterValue != lastLitWaterValue {
				gl.Uniform1f(litWaterUniform, litWaterValue)
				lastLitWaterValue = litWaterValue
			}
		}
		gl.Uniform3f(dynamicLightUniform, call.Light[0], call.Light[1], call.Light[2])
		gl.Uniform1f(alphaUniform, call.Alpha)
		//lint:ignore SA1019 OpenGL indexed draws require byte offsets into the bound element array buffer.
		gl.DrawElements(gl.TRIANGLES, int32(call.Face.NumIndices), gl.UNSIGNED_INT, gl.PtrOffset(int(call.Face.FirstIndex*4)))
	}
}

func worldLitWaterCvarEnabled() bool {
	cv := cvar.Get(CvarRLitWater)
	if cv == nil {
		return true
	}
	return cv.Int != 0
}

// ---- merged from world_alias_opengl_root.go ----
// ensureAliasScratchLocked creates a scratch VAO/VBO for alias model rendering. Alias models re-upload interpolated vertex data each frame, so the buffer uses GL_DYNAMIC_DRAW.
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

// ensureAliasModelLocked lazily creates GPU data for an alias (MDL) model. Parses triangles, vertices, and texture coordinates, stores all pose vertices for CPU-side interpolation, and uploads the skin texture.
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
	palette := append([]byte(nil), r.palette...)
	skins := make([]uint32, 0, len(hdr.Skins))
	fullbrightSkins := make([]uint32, 0, len(hdr.Skins))
	for _, skin := range hdr.Skins {
		if len(skin) != hdr.SkinWidth*hdr.SkinHeight {
			skins = append(skins, r.worldFallbackTexture)
			fullbrightSkins = append(fullbrightSkins, r.worldFallbackTexture)
			continue
		}
		rgba, fullbright := aliasSkinVariantRGBA(skin, palette, 0, false)
		tex := uploadWorldTextureRGBA(hdr.SkinWidth, hdr.SkinHeight, rgba)
		if tex == 0 {
			tex = r.worldFallbackTexture
		}
		fullbrightTex := uploadWorldTextureRGBA(hdr.SkinWidth, hdr.SkinHeight, fullbright)
		if fullbrightTex == 0 {
			fullbrightTex = r.worldFallbackTexture
		}
		skins = append(skins, tex)
		fullbrightSkins = append(fullbrightSkins, fullbrightTex)
	}

	refs := make([]aliasimpl.MeshRef, 0, len(hdr.Triangles)*3)
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
			refs = append(refs, aliasimpl.MeshRef{
				VertexIndex: idx,
				TexCoord: [2]float32{
					s / float32(hdr.SkinWidth),
					(float32(st.T) + 0.5) / float32(hdr.SkinHeight),
				},
			})
		}
	}

	alias := &glAliasModel{
		modelID:          modelID,
		flags:            hdr.Flags,
		skins:            skins,
		fullbrightSkins:  fullbrightSkins,
		playerSkins:      make(map[uint32][]uint32),
		playerFullbright: make(map[uint32][]uint32),
		poses:            hdr.Poses,
		refs:             refs,
	}
	r.aliasModels[modelID] = alias
	return alias
}

func uploadAliasSkinTextures(width, height int, baseRGBA, fullbrightRGBA []byte, fallback uint32) (uint32, uint32) {
	base := uploadWorldTextureRGBA(width, height, baseRGBA)
	if base == 0 {
		base = fallback
	}
	fullbright := uploadWorldTextureRGBA(width, height, fullbrightRGBA)
	if fullbright == 0 {
		fullbright = fallback
	}
	return base, fullbright
}

func (r *Renderer) resolveAliasSkinTexturesLocked(alias *glAliasModel, entity AliasModelEntity, skinSlot int) (uint32, uint32) {
	if alias == nil {
		return r.worldFallbackTexture, r.worldFallbackTexture
	}
	if entity.IsPlayer {
		if skins, ok := alias.playerSkins[entity.ColorMap]; ok && skinSlot >= 0 && skinSlot < len(skins) {
			return skins[skinSlot], alias.playerFullbright[entity.ColorMap][skinSlot]
		}
		hdr := entity.Model.AliasHeader
		palette := append([]byte(nil), r.palette...)
		playerSkins := make([]uint32, len(hdr.Skins))
		playerFullbright := make([]uint32, len(hdr.Skins))
		for i, skinPixels := range hdr.Skins {
			baseRGBA, fullbrightRGBA := aliasSkinVariantRGBA(skinPixels, palette, entity.ColorMap, true)
			playerSkins[i], playerFullbright[i] = uploadAliasSkinTextures(hdr.SkinWidth, hdr.SkinHeight, baseRGBA, fullbrightRGBA, r.worldFallbackTexture)
		}
		alias.playerSkins[entity.ColorMap] = playerSkins
		alias.playerFullbright[entity.ColorMap] = playerFullbright
		if skinSlot >= 0 && skinSlot < len(playerSkins) {
			return playerSkins[skinSlot], playerFullbright[skinSlot]
		}
	}
	if skinSlot >= 0 && skinSlot < len(alias.skins) {
		return alias.skins[skinSlot], alias.fullbrightSkins[skinSlot]
	}
	return r.worldFallbackTexture, r.worldFallbackTexture
}

// buildAliasDrawLocked prepares a complete alias model draw command: resolves the model, computes pose interpolation, builds interpolated vertices, and uploads to the scratch VBO.
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

	state := r.ensureAliasStateLocked(entity)
	state.Frame = frame
	aliasHdr := aliasHeaderFromModel(hdr)
	aliasHdr.Flags = applyAliasNoLerpListFlags(aliasHdr.Flags, entity.ModelID)
	interpData, err := SetupAliasFrame(state, aliasHdr, entity.TimeSeconds, true, false, 1)
	if err != nil {
		return nil
	}
	interpData.Origin, interpData.Angles = SetupEntityTransform(
		state,
		entity.TimeSeconds,
		true,
		entity.EntityKey == AliasViewModelEntityKey,
		false,
		false,
		1,
	)

	pose1 := interpData.Pose1
	pose2 := interpData.Pose2
	if pose1 < 0 || pose1 >= len(alias.poses) {
		pose1 = 0
	}
	if pose2 < 0 || pose2 >= len(alias.poses) {
		pose2 = 0
	}

	skin := r.worldFallbackTexture
	fullbrightSkin := r.worldFallbackTexture
	if len(alias.skins) > 0 {
		slot := resolveAliasSkinSlot(entity.Model.AliasHeader, entity.SkinNum, entity.TimeSeconds, len(alias.skins))
		skin, fullbrightSkin = r.resolveAliasSkinTexturesLocked(alias, entity, slot)
	}

	alpha, visible := visibleEntityAlpha(entity.Alpha)
	if !visible {
		return nil
	}

	return &glAliasDraw{
		alias:          alias,
		model:          entity.Model,
		pose1:          pose1,
		pose2:          pose2,
		blend:          interpData.Blend,
		skin:           skin,
		fullbrightSkin: fullbrightSkin,
		origin:         interpData.Origin,
		angles:         interpData.Angles,
		alpha:          alpha,
		scale:          entity.Scale,
		full:           fullAngles,
	}
}

// renderAliasDraws renders a batch of alias model draw commands. Sets up GL state with depth test, backface culling, and the world shader. For view model rendering, narrows the depth range to prevent the weapon from clipping into nearby walls.
func (r *Renderer) renderAliasDraws(draws []glAliasDraw, useViewModelDepthRange bool) {
	if len(draws) == 0 {
		return
	}

	r.mu.Lock()
	program := r.worldProgram
	vp := r.viewMatrices.VP
	camera := r.cameraState
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
	timeUniform := r.worldTimeUniform
	turbulentUniform := r.worldTurbulentUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform
	r.ensureAliasScratchLocked()
	scratchVAO := r.aliasScratchVAO
	scratchVBO := r.aliasScratchVBO
	fallbackLightmap := r.worldLightmapFallback
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity
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
	gl.Uniform1i(fullbrightUniform, 2)
	gl.Uniform1f(hasFullbrightUniform, 1)
	gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
	gl.Uniform3f(modelOffsetUniform, 0, 0, 0)
	gl.UniformMatrix4fv(modelRotationUniform, 1, false, &identityModelRotationMatrix[0])
	gl.Uniform1f(modelScaleUniform, 1)
	gl.Uniform1f(timeUniform, camera.Time)
	gl.Uniform1f(turbulentUniform, 0)
	gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
	gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
	gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
	gl.ActiveTexture(gl.TEXTURE0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, fallbackLightmap)
	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D, r.worldFallbackTexture)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindVertexArray(scratchVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, scratchVBO)

	for _, draw := range draws {
		// Use interpolated vertex building with two poses
		vertices := buildAliasVerticesInterpolated(draw.alias, draw.model, draw.pose1, draw.pose2, draw.blend, draw.origin, draw.angles, draw.scale, draw.full)
		if len(vertices) == 0 {
			continue
		}
		vertexData := worldopengl.FlattenWorldVertices(vertices)
		gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.DYNAMIC_DRAW)
		gl.BindTexture(gl.TEXTURE_2D, draw.skin)
		gl.ActiveTexture(gl.TEXTURE2)
		gl.BindTexture(gl.TEXTURE_2D, draw.fullbrightSkin)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.Uniform1f(alphaUniform, draw.alpha)
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(vertices)))
	}

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	if useViewModelDepthRange {
		gl.DepthRange(0.0, 1.0)
	}
}

// renderAliasEntities renders all alias model entities by building draw commands and dispatching them to renderAliasDraws.
func (r *Renderer) renderAliasEntities(entities []AliasModelEntity) {
	r.mu.Lock()
	r.pruneAliasStatesLocked(entities)
	draws := make([]glAliasDraw, 0, len(entities))
	for _, entity := range entities {
		if draw := r.buildAliasDrawLocked(entity, false); draw != nil {
			draws = append(draws, *draw)
		}
	}
	r.mu.Unlock()
	r.renderAliasDraws(draws, false)
}

// renderAliasShadows renders simple projected ground shadows under alias model entities as darkened, flattened copies projected onto a plane below each entity.
func (r *Renderer) renderAliasShadows(entities []AliasModelEntity) {
	if len(entities) == 0 {
		return
	}
	if cvar.FloatValue(CvarRShadows) <= 0 {
		return
	}

	excludedModels := parseAliasShadowExclusions(cvar.StringValue(CvarRNoshadowList))

	const (
		shadowSegments = 16
		shadowAlpha    = 0.5
		shadowLift     = 0.1
		minShadowSize  = 8.0
		maxShadowSize  = 48.0
	)

	r.mu.Lock()
	program := r.worldProgram
	vp := r.viewMatrices.VP
	camera := r.cameraState
	vpUniform := r.worldVPUniform
	textureUniform := r.worldTextureUniform
	lightmapUniform := r.worldLightmapUniform
	dynamicLightUniform := r.worldDynamicLightUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	modelRotationUniform := r.worldModelRotationUniform
	modelScaleUniform := r.worldModelScaleUniform
	alphaUniform := r.worldAlphaUniform
	timeUniform := r.worldTimeUniform
	turbulentUniform := r.worldTurbulentUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform
	r.ensureAliasScratchLocked()
	r.ensureAliasShadowTextureLocked()
	scratchVAO := r.aliasScratchVAO
	scratchVBO := r.aliasScratchVBO
	shadowTexture := r.aliasShadowTexture
	fallbackLightmap := r.worldLightmapFallback
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity
	r.mu.Unlock()

	if program == 0 || scratchVAO == 0 || scratchVBO == 0 || shadowTexture == 0 || fallbackLightmap == 0 {
		return
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthMask(false)
	gl.Disable(gl.CULL_FACE)
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.UseProgram(program)
	gl.UniformMatrix4fv(vpUniform, 1, false, &vp[0])
	gl.Uniform1i(textureUniform, 0)
	gl.Uniform1i(lightmapUniform, 1)
	gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
	gl.Uniform3f(modelOffsetUniform, 0, 0, 0)
	gl.UniformMatrix4fv(modelRotationUniform, 1, false, &identityModelRotationMatrix[0])
	gl.Uniform1f(modelScaleUniform, 1)
	gl.Uniform1f(timeUniform, camera.Time)
	gl.Uniform1f(turbulentUniform, 0)
	gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
	gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
	gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, shadowTexture)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, fallbackLightmap)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindVertexArray(scratchVAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, scratchVBO)

	for _, entity := range entities {
		modelID := strings.ToLower(entity.ModelID)
		if _, skip := excludedModels[modelID]; skip {
			continue
		}
		if entity.Model == nil || entity.Model.AliasHeader == nil {
			continue
		}
		if _, visible := visibleEntityAlpha(entity.Alpha); !visible {
			continue
		}

		modelScale := entity.Scale
		if modelScale == 0 {
			modelScale = 1
		}
		mins := entity.Model.Mins
		maxs := entity.Model.Maxs
		spanX := (maxs[0] - mins[0]) * modelScale
		spanY := (maxs[1] - mins[1]) * modelScale
		shadowRadius := 0.5 * float32(math.Max(float64(spanX), float64(spanY)))
		if shadowRadius < minShadowSize {
			shadowRadius = minShadowSize
		}
		if shadowRadius > maxShadowSize {
			shadowRadius = maxShadowSize
		}

		shadowZ := entity.Origin[2] + mins[2]*modelScale + shadowLift
		center := WorldVertex{
			Position:      [3]float32{entity.Origin[0], entity.Origin[1], shadowZ},
			TexCoord:      [2]float32{0.5, 0.5},
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
		vertices := make([]WorldVertex, 0, shadowSegments*3)
		for i := 0; i < shadowSegments; i++ {
			a0 := float32(i) * 2 * float32(math.Pi) / shadowSegments
			a1 := float32(i+1) * 2 * float32(math.Pi) / shadowSegments
			p0 := WorldVertex{
				Position: [3]float32{
					entity.Origin[0] + float32(math.Cos(float64(a0)))*shadowRadius,
					entity.Origin[1] + float32(math.Sin(float64(a0)))*shadowRadius,
					shadowZ,
				},
				TexCoord:      [2]float32{0, 0},
				LightmapCoord: [2]float32{},
				Normal:        [3]float32{0, 0, 1},
			}
			p1 := WorldVertex{
				Position: [3]float32{
					entity.Origin[0] + float32(math.Cos(float64(a1)))*shadowRadius,
					entity.Origin[1] + float32(math.Sin(float64(a1)))*shadowRadius,
					shadowZ,
				},
				TexCoord:      [2]float32{1, 1},
				LightmapCoord: [2]float32{},
				Normal:        [3]float32{0, 0, 1},
			}
			vertices = append(vertices, center, p0, p1)
		}

		vertexData := worldopengl.FlattenWorldVertices(vertices)
		gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.DYNAMIC_DRAW)
		gl.Uniform1f(alphaUniform, shadowAlpha)
		gl.DrawArrays(gl.TRIANGLES, 0, int32(len(vertices)))
	}

	gl.BindVertexArray(0)
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	gl.DepthMask(true)
}

// renderViewModel renders the first-person weapon model with a narrower depth range (0..0.3) to prevent it from clipping into nearby walls. This depth range trick is a classic Quake rendering technique.
func (r *Renderer) renderViewModel(entity AliasModelEntity) {
	r.mu.Lock()
	draw := r.buildAliasDrawLocked(entity, true)
	r.mu.Unlock()
	if draw == nil {
		return
	}
	r.renderAliasDraws([]glAliasDraw{*draw}, true)
}

// parseAliasShadowExclusions parses the r_noshadow_list cvar into a set of model names that should not cast ground shadows.
func parseAliasShadowExclusions(value string) map[string]struct{} {
	return parseAliasModelList(value)
}

// ensureAliasShadowTextureLocked creates a 1x1 dark semi-transparent texture used for alias model ground shadows.
func (r *Renderer) ensureAliasShadowTextureLocked() {
	if r.aliasShadowTexture != 0 {
		return
	}
	r.aliasShadowTexture = uploadWorldTextureRGBA(1, 1, []byte{0, 0, 0, 255})
}

// ---- merged from world_upload_opengl_root.go ----
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
	return worldopengl.ParseTextureMode(cvar.StringValue(CvarGLTextureMode))
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
	return worldopengl.UploadTextureRGBA(width, height, rgba, worldopengl.TextureUploadOptions{
		MinFilter:  minFilter,
		MagFilter:  magFilter,
		LodBias:    readTextureLodBias(),
		Anisotropy: readTextureAnisotropy(),
	})
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

func lightStylesChanged(old, new_ [64]float32) [64]bool {
	var changed [64]bool
	for i := range old {
		if old[i] != new_[i] {
			changed[i] = true
		}
	}
	return changed
}

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
	return rgba
}

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

func updateLightmapTextures(textures []uint32, pages []WorldLightmapPage, values [64]float32) {
	count := len(textures)
	if len(pages) < count {
		count = len(pages)
	}
	for i := 0; i < count; i++ {
		if textures[i] == 0 || !pages[i].Dirty {
			continue
		}

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

// ---- merged from world_runtime_opengl_root.go ----
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

// clearExternalSkyboxLocked deletes external skybox GL textures and resets to the embedded sky rendering mode.
func (r *Renderer) clearExternalSkyboxLocked() {
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
	r.worldSkyExternalMode = externalSkyboxRenderEmbedded
	r.worldSkyExternalName = ""
}

// SetExternalSkybox loads an external skybox by name, attempting cubemap first and falling back to individual face textures.
func (r *Renderer) SetExternalSkybox(name string, loadFile func(string) ([]byte, error)) {
	normalized := normalizeSkyboxBaseName(name)

	r.mu.Lock()
	r.worldSkyExternalRequestID++
	requestID := r.worldSkyExternalRequestID
	if normalized == r.worldSkyExternalName {
		r.mu.Unlock()
		return
	}
	r.mu.Unlock()

	faces, loaded := loadExternalSkyboxFaces(normalized, loadFile)
	faceSize, cubemapEligible := externalSkyboxCubemapFaceSize(faces, loaded)
	renderMode := selectExternalSkyboxRenderMode(loaded, cubemapEligible)

	r.mu.Lock()
	defer r.mu.Unlock()
	if requestID != r.worldSkyExternalRequestID {
		return
	}

	r.clearExternalSkyboxLocked()
	if normalized == "" || renderMode == externalSkyboxRenderEmbedded {
		return
	}
	if renderMode == externalSkyboxRenderCubemap {
		cubemap := uploadSkyboxCubemap(faces, faceSize)
		if cubemap == 0 {
			slog.Debug("external skybox cubemap upload failed; falling back to embedded sky", "name", normalized)
			return
		}
		r.worldSkyExternalCubemap = cubemap
		r.worldSkyExternalMode = externalSkyboxRenderCubemap
		r.worldSkyExternalName = normalized
		return
	}
	faceTextures, ok := uploadSkyboxFaceTextures(faces)
	if !ok {
		slog.Debug("external skybox face upload failed; falling back to embedded sky", "name", normalized)
		return
	}
	r.worldSkyExternalFaceTextures = faceTextures
	r.worldSkyExternalMode = externalSkyboxRenderFaces
	r.worldSkyExternalName = normalized
}

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
			return calls[i].DistanceSq > calls[j].DistanceSq
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

// ---- merged from world_sky_pass_opengl_root.go ----
type skyPassState struct {
	program                     uint32
	proceduralProgram           uint32
	cubemapProgram              uint32
	externalFaceProgram         uint32
	vpUniform                   int32
	solidUniform                int32
	alphaUniform                int32
	proceduralVPUniform         int32
	cubemapVPUniform            int32
	cubemapUniform              int32
	externalFaceVPUniform       int32
	externalFaceRTUniform       int32
	externalFaceBKUniform       int32
	externalFaceLFUniform       int32
	externalFaceFTUniform       int32
	externalFaceUPUniform       int32
	externalFaceDNUniform       int32
	modelOffsetUniform          int32
	modelRotationUniform        int32
	modelScaleUniform           int32
	proceduralModelOffset       int32
	proceduralModelRotation     int32
	proceduralModelScale        int32
	cubemapModelOffsetUniform   int32
	cubemapModelRotationUniform int32
	cubemapModelScaleUniform    int32
	externalFaceModelOffset     int32
	externalFaceModelRotation   int32
	externalFaceModelScale      int32
	timeUniform                 int32
	solidLayerSpeedUniform      int32
	alphaLayerSpeedUniform      int32
	cameraOriginUniform         int32
	proceduralCameraOrigin      int32
	cubemapCameraOriginUniform  int32
	externalFaceCameraOrigin    int32
	fogColorUniform             int32
	proceduralFogColor          int32
	cubemapFogColorUniform      int32
	externalFaceFogColor        int32
	fogDensityUniform           int32
	proceduralFogDensity        int32
	proceduralHorizonColor      int32
	proceduralZenithColor       int32
	cubemapFogDensityUniform    int32
	externalFaceFogDensity      int32
	vp                          [16]float32
	time                        float32
	solidLayerSpeed             float32
	alphaLayerSpeed             float32
	cameraOrigin                [3]float32
	modelOffset                 [3]float32
	modelRotation               [16]float32
	modelScale                  float32
	fogColor                    [3]float32
	proceduralHorizon           [3]float32
	proceduralZenith            [3]float32
	fogDensity                  float32
	solidTextures               map[int32]uint32
	alphaTextures               map[int32]uint32
	flatTextures                map[int32]uint32
	textureAnimations           []*SurfaceTexture
	fallbackSolid               uint32
	fallbackAlpha               uint32
	externalSkyMode             externalSkyboxRenderMode
	externalCubemap             uint32
	externalFaceTextures        [6]uint32
	frame                       int
	fastSky                     bool
	proceduralSky               bool
}

// renderSkyPass renders sky surfaces using one of three sky shader programs: embedded two-layer scrolling sky, cubemap sky, or individual face textures. Draws sky as a backdrop with depth clamped to the far plane.
func renderSkyPass(calls []worldDrawCall, state skyPassState) {
	if len(calls) == 0 {
		return
	}
	useCubemap := state.externalSkyMode == externalSkyboxRenderCubemap && state.externalCubemap != 0
	useExternalFaces := state.externalSkyMode == externalSkyboxRenderFaces
	useProcedural := state.proceduralSky && !useCubemap && !useExternalFaces
	if useProcedural {
		if state.proceduralProgram == 0 {
			return
		}
		gl.UseProgram(state.proceduralProgram)
		gl.UniformMatrix4fv(state.proceduralVPUniform, 1, false, &state.vp[0])
		gl.Uniform3f(state.proceduralModelOffset, state.modelOffset[0], state.modelOffset[1], state.modelOffset[2])
		gl.UniformMatrix4fv(state.proceduralModelRotation, 1, false, &state.modelRotation[0])
		gl.Uniform1f(state.proceduralModelScale, state.modelScale)
		gl.Uniform3f(state.proceduralCameraOrigin, state.cameraOrigin[0], state.cameraOrigin[1], state.cameraOrigin[2])
		gl.Uniform3f(state.proceduralFogColor, state.fogColor[0], state.fogColor[1], state.fogColor[2])
		gl.Uniform1f(state.proceduralFogDensity, state.fogDensity)
		gl.Uniform3f(state.proceduralHorizonColor, state.proceduralHorizon[0], state.proceduralHorizon[1], state.proceduralHorizon[2])
		gl.Uniform3f(state.proceduralZenithColor, state.proceduralZenith[0], state.proceduralZenith[1], state.proceduralZenith[2])
	} else if useCubemap {
		if state.cubemapProgram == 0 {
			return
		}
		gl.UseProgram(state.cubemapProgram)
		gl.UniformMatrix4fv(state.cubemapVPUniform, 1, false, &state.vp[0])
		gl.Uniform1i(state.cubemapUniform, 2)
		gl.Uniform3f(state.cubemapModelOffsetUniform, state.modelOffset[0], state.modelOffset[1], state.modelOffset[2])
		gl.UniformMatrix4fv(state.cubemapModelRotationUniform, 1, false, &state.modelRotation[0])
		gl.Uniform1f(state.cubemapModelScaleUniform, state.modelScale)
		gl.Uniform3f(state.cubemapCameraOriginUniform, state.cameraOrigin[0], state.cameraOrigin[1], state.cameraOrigin[2])
		gl.Uniform3f(state.cubemapFogColorUniform, state.fogColor[0], state.fogColor[1], state.fogColor[2])
		gl.Uniform1f(state.cubemapFogDensityUniform, state.fogDensity)
		gl.ActiveTexture(gl.TEXTURE2)
		gl.BindTexture(gl.TEXTURE_CUBE_MAP, state.externalCubemap)
		gl.ActiveTexture(gl.TEXTURE0)
	} else if useExternalFaces {
		if state.externalFaceProgram == 0 {
			return
		}
		gl.UseProgram(state.externalFaceProgram)
		gl.UniformMatrix4fv(state.externalFaceVPUniform, 1, false, &state.vp[0])
		gl.Uniform1i(state.externalFaceRTUniform, 2)
		gl.Uniform1i(state.externalFaceBKUniform, 3)
		gl.Uniform1i(state.externalFaceLFUniform, 4)
		gl.Uniform1i(state.externalFaceFTUniform, 5)
		gl.Uniform1i(state.externalFaceUPUniform, 6)
		gl.Uniform1i(state.externalFaceDNUniform, 7)
		gl.Uniform3f(state.externalFaceModelOffset, state.modelOffset[0], state.modelOffset[1], state.modelOffset[2])
		gl.UniformMatrix4fv(state.externalFaceModelRotation, 1, false, &state.modelRotation[0])
		gl.Uniform1f(state.externalFaceModelScale, state.modelScale)
		gl.Uniform3f(state.externalFaceCameraOrigin, state.cameraOrigin[0], state.cameraOrigin[1], state.cameraOrigin[2])
		gl.Uniform3f(state.externalFaceFogColor, state.fogColor[0], state.fogColor[1], state.fogColor[2])
		gl.Uniform1f(state.externalFaceFogDensity, state.fogDensity)
		for i, texture := range state.externalFaceTextures {
			gl.ActiveTexture(gl.TEXTURE2 + uint32(i))
			gl.BindTexture(gl.TEXTURE_2D, texture)
		}
		gl.ActiveTexture(gl.TEXTURE0)
	} else {
		if state.program == 0 {
			return
		}
		gl.UseProgram(state.program)
		gl.UniformMatrix4fv(state.vpUniform, 1, false, &state.vp[0])
		gl.Uniform1i(state.solidUniform, 0)
		gl.Uniform1i(state.alphaUniform, 1)
		gl.Uniform3f(state.modelOffsetUniform, state.modelOffset[0], state.modelOffset[1], state.modelOffset[2])
		gl.UniformMatrix4fv(state.modelRotationUniform, 1, false, &state.modelRotation[0])
		gl.Uniform1f(state.modelScaleUniform, state.modelScale)
		gl.Uniform1f(state.timeUniform, state.time)
		gl.Uniform1f(state.solidLayerSpeedUniform, state.solidLayerSpeed)
		gl.Uniform1f(state.alphaLayerSpeedUniform, state.alphaLayerSpeed)
		gl.Uniform3f(state.cameraOriginUniform, state.cameraOrigin[0], state.cameraOrigin[1], state.cameraOrigin[2])
		gl.Uniform3f(state.fogColorUniform, state.fogColor[0], state.fogColor[1], state.fogColor[2])
		gl.Uniform1f(state.fogDensityUniform, state.fogDensity)
	}

	// Sky is rendered at maximum depth but doesn't write to depth buffer
	gl.DepthFunc(gl.LEQUAL)
	gl.DepthMask(false)
	gl.Disable(gl.BLEND)

	for _, call := range calls {
		if !useProcedural && !useCubemap && !useExternalFaces {
			solid, alpha := worldopengl.SkyTexturesForFace(
				call.Face,
				state.solidTextures,
				state.alphaTextures,
				state.textureAnimations,
				state.fallbackSolid,
				state.fallbackAlpha,
				state.frame,
				float64(state.time),
				TextureAnimation,
			)
			if state.fastSky {
				solid = worldopengl.SkyFlatTextureForFace(
					call.Face,
					state.flatTextures,
					state.textureAnimations,
					state.fallbackSolid,
					state.frame,
					float64(state.time),
					TextureAnimation,
				)
				alpha = state.fallbackAlpha
			}
			gl.ActiveTexture(gl.TEXTURE0)
			gl.BindTexture(gl.TEXTURE_2D, solid)
			gl.ActiveTexture(gl.TEXTURE1)
			gl.BindTexture(gl.TEXTURE_2D, alpha)
			gl.ActiveTexture(gl.TEXTURE0)
		}
		//lint:ignore SA1019 OpenGL indexed draws require byte offsets into the bound element array buffer.
		gl.DrawElements(gl.TRIANGLES, int32(call.Face.NumIndices), gl.UNSIGNED_INT, gl.PtrOffset(int(call.Face.FirstIndex*4)))
	}
	if useCubemap {
		gl.ActiveTexture(gl.TEXTURE2)
		gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
		gl.ActiveTexture(gl.TEXTURE0)
	} else if useExternalFaces {
		for i := range state.externalFaceTextures {
			gl.ActiveTexture(gl.TEXTURE2 + uint32(i))
			gl.BindTexture(gl.TEXTURE_2D, 0)
		}
		gl.ActiveTexture(gl.TEXTURE0)
	}

	// Restore depth state
	gl.DepthFunc(gl.LESS)
	gl.DepthMask(true)
}

// ---- merged from world_sky_support_opengl_root.go ----

// uploadSkyboxCubemap uploads 6 skybox face images as a GL_TEXTURE_CUBE_MAP, reordering faces from Quake convention (rt/bk/lf/ft/up/dn) to OpenGL convention (+X/-X/+Y/-Y/+Z/-Z).
func uploadSkyboxCubemap(faces [6]externalSkyboxFace, faceSize int) uint32 {
	if faceSize <= 0 {
		return 0
	}
	var cubemap uint32
	gl.GenTextures(1, &cubemap)
	if cubemap == 0 {
		return 0
	}
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, cubemap)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_CUBE_MAP, gl.TEXTURE_WRAP_R, gl.CLAMP_TO_EDGE)
	zeroFace := make([]byte, faceSize*faceSize*4)
	for i, target := range skyboxCubemapTargets {
		face := faces[skyboxCubemapFaceOrder[i]]
		faceData := zeroFace
		if face.Width > 0 && face.Height > 0 && len(face.RGBA) > 0 {
			if face.Width != faceSize || face.Height != faceSize || len(face.RGBA) != faceSize*faceSize*4 {
				gl.DeleteTextures(1, &cubemap)
				gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
				return 0
			}
			faceData = face.RGBA
		}
		if len(faceData) != faceSize*faceSize*4 {
			gl.DeleteTextures(1, &cubemap)
			gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
			return 0
		}
		gl.TexImage2D(target, 0, gl.RGBA8, int32(faceSize), int32(faceSize), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(faceData))
	}
	gl.BindTexture(gl.TEXTURE_CUBE_MAP, 0)
	return cubemap
}

// uploadSkyboxFaceTextures uploads each skybox face as an individual GL_TEXTURE_2D, used as fallback when faces aren't all square and can't form a cubemap.
func uploadSkyboxFaceTextures(faces [6]externalSkyboxFace) (textures [6]uint32, ok bool) {
	fallbackPixel := [4]byte{0, 0, 0, 255}
	for i := range textures {
		gl.GenTextures(1, &textures[i])
		if textures[i] == 0 {
			for j := 0; j < i; j++ {
				if textures[j] != 0 {
					gl.DeleteTextures(1, &textures[j])
					textures[j] = 0
				}
			}
			return textures, false
		}
		gl.BindTexture(gl.TEXTURE_2D, textures[i])
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

		face := faces[i]
		width := face.Width
		height := face.Height
		data := face.RGBA
		if width <= 0 || height <= 0 || len(data) != width*height*4 {
			width, height = 1, 1
			data = fallbackPixel[:]
		}
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, int32(width), int32(height), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(data))
	}
	gl.BindTexture(gl.TEXTURE_2D, 0)
	return textures, true
}

// ---- merged from world_probe_opengl_root.go ----
// WorldFaceProbeStats captures opt-in world extraction diagnostics for a bbox.
type WorldFaceProbeStats struct {
	ModelIndex int
	BoundsMin  [3]float32
	BoundsMax  [3]float32

	RawInBBox       int
	ExtractedInBBox int

	PassSky         int
	PassOpaque      int
	PassAlphaTest   int
	PassTranslucent int

	Faces []WorldFaceProbeEntry
}

// WorldFaceProbeEntry captures one suspect face from raw BSP through extraction and pass routing.
type WorldFaceProbeEntry struct {
	SourceFaceIndex int
	NumEdges        int32
	TexInfoIndex    int32
	TexInfoFlags    int32
	TextureIndex    int32
	TextureName     string
	DerivedFlags    int32
	Center          [3]float32
	Normal          [3]float32
	InBBox          bool
	Extracted       bool
	ExtractError    string
	VertexCount     int
	Pass            worldRenderPass
}

func worldRenderPassName(pass worldRenderPass) string {
	switch pass {
	case worldPassSky:
		return "sky"
	case worldPassOpaque:
		return "opaque"
	case worldPassAlphaTest:
		return "alpha-test"
	case worldPassTranslucent:
		return "translucent"
	default:
		return fmt.Sprintf("unknown(%d)", pass)
	}
}

// ProbeWorldFacesInBBox inspects world/model faces in an opt-in diagnostic path.
func ProbeWorldFacesInBBox(tree *bsp.Tree, modelIndex int, boundsMin, boundsMax [3]float32, liquidAlpha worldLiquidAlphaSettings) (*WorldFaceProbeStats, error) {
	if tree == nil {
		return nil, fmt.Errorf("nil BSP tree")
	}
	if len(tree.Models) == 0 {
		return nil, fmt.Errorf("BSP has no models")
	}
	if modelIndex < 0 || modelIndex >= len(tree.Models) {
		return nil, fmt.Errorf("model index %d out of range", modelIndex)
	}

	meta := parseWorldTextureMeta(tree)
	modelDef := tree.Models[modelIndex]
	out := &WorldFaceProbeStats{
		ModelIndex: modelIndex,
		BoundsMin:  boundsMin,
		BoundsMax:  boundsMax,
		Faces:      make([]WorldFaceProbeEntry, 0, 64),
	}

	for localFace := 0; localFace < int(modelDef.NumFaces); localFace++ {
		faceIndex := int(modelDef.FirstFace) + localFace
		if faceIndex < 0 || faceIndex >= len(tree.Faces) {
			break
		}
		face := &tree.Faces[faceIndex]
		center, normal, err := worldFaceCenterAndNormal(tree, face)
		if err != nil || !pointInBounds(center, boundsMin, boundsMax) {
			continue
		}

		entry := WorldFaceProbeEntry{
			SourceFaceIndex: faceIndex,
			NumEdges:        face.NumEdges,
			TexInfoIndex:    face.Texinfo,
			Center:          center,
			Normal:          normal,
			InBBox:          true,
		}
		out.RawInBBox++

		if int(face.Texinfo) >= 0 && int(face.Texinfo) < len(tree.Texinfo) {
			texInfo := tree.Texinfo[face.Texinfo]
			entry.TexInfoFlags = texInfo.Flags
			entry.TextureIndex = texInfo.Miptex
			if int(texInfo.Miptex) >= 0 && int(texInfo.Miptex) < len(meta) {
				entry.TextureName = meta[texInfo.Miptex].Name
				entry.DerivedFlags = deriveWorldFaceFlags(meta[texInfo.Miptex].Type, texInfo.Flags)
			} else {
				entry.DerivedFlags = deriveWorldFaceFlags(model.TexTypeDefault, texInfo.Flags)
			}
		}

		verts, _, extractErr := extractFaceVertices(tree, face, meta, nil, nil)
		if extractErr != nil {
			entry.ExtractError = extractErr.Error()
			out.Faces = append(out.Faces, entry)
			continue
		}
		entry.Extracted = true
		entry.VertexCount = len(verts)
		out.ExtractedInBBox++
		entry.Pass = worldFacePass(entry.DerivedFlags, worldFaceAlpha(entry.DerivedFlags, liquidAlpha))
		switch entry.Pass {
		case worldPassSky:
			out.PassSky++
		case worldPassOpaque:
			out.PassOpaque++
		case worldPassAlphaTest:
			out.PassAlphaTest++
		case worldPassTranslucent:
			out.PassTranslucent++
		}

		out.Faces = append(out.Faces, entry)
	}

	return out, nil
}

func worldFaceCenterAndNormal(tree *bsp.Tree, face *bsp.TreeFace) ([3]float32, [3]float32, error) {
	var center [3]float32
	var normal [3]float32
	if face == nil {
		return center, normal, fmt.Errorf("nil face")
	}
	if int(face.PlaneNum) < 0 || int(face.PlaneNum) >= len(tree.Planes) {
		return center, normal, fmt.Errorf("invalid plane num %d", face.PlaneNum)
	}
	normal = tree.Planes[face.PlaneNum].Normal
	if face.Side != 0 {
		normal[0] = -normal[0]
		normal[1] = -normal[1]
		normal[2] = -normal[2]
	}
	if face.NumEdges <= 0 {
		return center, normal, fmt.Errorf("face has no edges")
	}
	for i := int32(0); i < face.NumEdges; i++ {
		surfEdgeIdx := int(face.FirstEdge + i)
		if surfEdgeIdx < 0 || surfEdgeIdx >= len(tree.Surfedges) {
			return center, normal, fmt.Errorf("surfedge index %d out of range", surfEdgeIdx)
		}
		surfEdge := tree.Surfedges[surfEdgeIdx]
		var vertIdx uint32
		if surfEdge >= 0 {
			if int(surfEdge) >= len(tree.Edges) {
				return center, normal, fmt.Errorf("edge index %d out of range", surfEdge)
			}
			vertIdx = tree.Edges[surfEdge].V[0]
		} else {
			edgeIdx := -surfEdge
			if int(edgeIdx) >= len(tree.Edges) {
				return center, normal, fmt.Errorf("edge index %d out of range", edgeIdx)
			}
			vertIdx = tree.Edges[edgeIdx].V[1]
		}
		if int(vertIdx) >= len(tree.Vertexes) {
			return center, normal, fmt.Errorf("vertex index %d out of range", vertIdx)
		}
		v := tree.Vertexes[vertIdx]
		center[0] += v.Point[0]
		center[1] += v.Point[1]
		center[2] += v.Point[2]
	}
	inv := 1.0 / float32(face.NumEdges)
	center[0] *= inv
	center[1] *= inv
	center[2] *= inv
	return center, normal, nil
}

func pointInBounds(point, boundsMin, boundsMax [3]float32) bool {
	return point[0] >= boundsMin[0] && point[0] <= boundsMax[0] &&
		point[1] >= boundsMin[1] && point[1] <= boundsMax[1] &&
		point[2] >= boundsMin[2] && point[2] <= boundsMax[2]
}

// ---- merged from world_sprite_opengl_root.go ----
// renderSpriteEntities renders sprite entities as textured billboard quads with alpha blending, no depth write, and no backface culling. Sprite vertex positions are computed on CPU based on the sprite's orientation type.
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
	dynamicLightUniform := r.worldDynamicLightUniform
	modelOffsetUniform := r.worldModelOffsetUniform
	modelRotationUniform := r.worldModelRotationUniform
	modelScaleUniform := r.worldModelScaleUniform
	alphaUniform := r.worldAlphaUniform
	timeUniform := r.worldTimeUniform
	turbulentUniform := r.worldTurbulentUniform
	cameraOriginUniform := r.worldCameraOriginUniform
	fogColorUniform := r.worldFogColorUniform
	fogDensityUniform := r.worldFogDensityUniform
	fallbackLightmap := r.worldLightmapFallback
	fogColor := r.worldFogColor
	fogDensity := r.worldFogDensity

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
	gl.Uniform3f(dynamicLightUniform, 0, 0, 0)
	gl.UniformMatrix4fv(modelRotationUniform, 1, false, &identityModelRotationMatrix[0])
	gl.Uniform1f(modelScaleUniform, 1)
	gl.Uniform1f(timeUniform, camera.Time)
	gl.Uniform1f(turbulentUniform, 0)
	gl.Uniform3f(cameraOriginUniform, camera.Origin.X, camera.Origin.Y, camera.Origin.Z)
	gl.Uniform3f(fogColorUniform, fogColor[0], fogColor[1], fogColor[2])
	gl.Uniform1f(fogDensityUniform, worldFogUniformDensity(fogDensity))
	gl.ActiveTexture(gl.TEXTURE0)
	cameraForward, cameraRight, cameraUp := spriteCameraBasis([3]float32{
		camera.Angles.X,
		camera.Angles.Y,
		camera.Angles.Z,
	})

	for _, draw := range draws {
		r.renderSpriteDraw(draw, camera, cameraForward, cameraRight, cameraUp, program, modelOffsetUniform, alphaUniform, fallbackLightmap)
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
	angles [3]float32
	alpha  float32
	scale  float32
}

// buildSpriteDrawLocked prepares a sprite for rendering (must be called with mutex held).
func (r *Renderer) buildSpriteDrawLocked(entity SpriteEntity) *glSpriteDraw {
	if entity.ModelID == "" || entity.Model == nil || entity.Model.Type != model.ModSprite {
		return nil
	}

	var spr *glSpriteModel
	if entity.SpriteData != nil {
		spr = uploadSpriteModel(entity.ModelID, entity.SpriteData)
	} else {
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
		angles: entity.Angles,
		alpha:  entity.Alpha,
		scale:  entity.Scale,
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

	spr := spriteDataFromModel(mdl)
	glsprite := uploadSpriteModel(modelID, spr)
	if glsprite == nil {
		return nil
	}

	r.spriteModels[modelID] = glsprite
	return glsprite
}

// renderSpriteDraw renders a single sprite billboard.
func (r *Renderer) renderSpriteDraw(draw glSpriteDraw, camera CameraState, cameraForward, cameraRight, cameraUp [3]float32, program uint32, modelOffsetUniform, alphaUniform int32, fallbackLightmap uint32) {
	if draw.sprite == nil || draw.frame < 0 || draw.frame >= len(draw.sprite.frames) {
		return
	}

	vertices := buildSpriteQuadVertices(draw.sprite, draw.frame, [3]float32{
		camera.Origin.X,
		camera.Origin.Y,
		camera.Origin.Z,
	}, draw.origin, draw.angles, cameraForward, cameraRight, cameraUp, draw.scale)

	if len(vertices) == 0 {
		return
	}

	triangleVertices := expandSpriteQuadVertices(vertices)
	if len(triangleVertices) == 0 {
		return
	}
	worldVertices := spriteQuadVerticesToWorldVertices(triangleVertices)

	r.ensureAliasScratchLocked()

	worldopengl.DrawSpriteWorldVertices(worldVertices, worldopengl.SpriteDrawParams{
		VBO:                r.aliasScratchVBO,
		VAO:                r.aliasScratchVAO,
		ModelOffset:        draw.origin,
		Alpha:              draw.alpha,
		ModelOffsetUniform: modelOffsetUniform,
		AlphaUniform:       alphaUniform,
	})
}

func spriteQuadVerticesToWorldVertices(vertices []spriteQuadVertex) []WorldVertex {
	out := make([]WorldVertex, len(vertices))
	for i, vertex := range vertices {
		out[i] = WorldVertex{
			Position:      vertex.Position,
			TexCoord:      vertex.TexCoord,
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{0, 0, 1},
		}
	}
	return out
}

// ---- merged from world_support_opengl_root.go ----
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

func uploadWorldMesh(vertices []WorldVertex, indices []uint32) *glWorldMesh {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil
	}

	vertexData := worldopengl.FlattenWorldVertices(vertices)
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

type worldDrawCall = worldopengl.DrawCall

type glAliasModel struct {
	modelID          string
	flags            int
	skins            []uint32
	fullbrightSkins  []uint32
	playerSkins      map[uint32][]uint32
	playerFullbright map[uint32][]uint32
	poses            [][]model.TriVertX
	refs             []aliasimpl.MeshRef
}

type glAliasDraw struct {
	alias          *glAliasModel
	model          *model.Model
	pose1          int
	pose2          int
	blend          float32
	skin           uint32
	fullbrightSkin uint32
	origin         [3]float32
	angles         [3]float32
	alpha          float32
	scale          float32
	full           bool
}

// ---- merged from world_sky_opengl_root.go ----
// ensureWorldSkyPrograms lazily compiles all three sky shader variants: embedded two-layer scrolling sky, cubemap sky (GL_TEXTURE_CUBE_MAP for external skybox), and individual-face sky (fallback for non-uniform face sizes).
func (r *Renderer) ensureWorldSkyPrograms() error {
	if r.worldSkyProgram == 0 {
		vs, err := compileShader(worldopengl.WorldSkyVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky vertex shader: %w", err)
		}
		fs, err := compileShader(worldopengl.WorldSkyFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vs)
			return fmt.Errorf("compile world sky fragment shader: %w", err)
		}

		program := createProgram(vs, fs)
		r.worldSkyProgram = program
		r.worldSkyVPUniform = gl.GetUniformLocation(program, gl.Str("uViewProjection\x00"))
		r.worldSkySolidUniform = gl.GetUniformLocation(program, gl.Str("uSolidLayer\x00"))
		r.worldSkyAlphaUniform = gl.GetUniformLocation(program, gl.Str("uAlphaLayer\x00"))
		r.worldSkyModelOffsetUniform = gl.GetUniformLocation(program, gl.Str("uModelOffset\x00"))
		r.worldSkyModelRotationUniform = gl.GetUniformLocation(program, gl.Str("uModelRotation\x00"))
		r.worldSkyModelScaleUniform = gl.GetUniformLocation(program, gl.Str("uModelScale\x00"))
		r.worldSkyTimeUniform = gl.GetUniformLocation(program, gl.Str("uTime\x00"))
		r.worldSkySolidLayerSpeedUniform = gl.GetUniformLocation(program, gl.Str("uSolidLayerSpeed\x00"))
		r.worldSkyAlphaLayerSpeedUniform = gl.GetUniformLocation(program, gl.Str("uAlphaLayerSpeed\x00"))
		r.worldSkyCameraOriginUniform = gl.GetUniformLocation(program, gl.Str("uCameraOrigin\x00"))
		r.worldSkyFogColorUniform = gl.GetUniformLocation(program, gl.Str("uFogColor\x00"))
		r.worldSkyFogDensityUniform = gl.GetUniformLocation(program, gl.Str("uFogDensity\x00"))
	}

	if r.worldSkyProceduralProgram == 0 {
		vs, err := compileShader(worldopengl.WorldSkyVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky procedural vertex shader: %w", err)
		}
		fs, err := compileShader(worldopengl.WorldSkyProceduralFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vs)
			return fmt.Errorf("compile world sky procedural fragment shader: %w", err)
		}

		program := createProgram(vs, fs)
		r.worldSkyProceduralProgram = program
		r.worldSkyProceduralVPUniform = gl.GetUniformLocation(program, gl.Str("uViewProjection\x00"))
		r.worldSkyProceduralModelOffset = gl.GetUniformLocation(program, gl.Str("uModelOffset\x00"))
		r.worldSkyProceduralModelRotation = gl.GetUniformLocation(program, gl.Str("uModelRotation\x00"))
		r.worldSkyProceduralModelScale = gl.GetUniformLocation(program, gl.Str("uModelScale\x00"))
		r.worldSkyProceduralCameraOrigin = gl.GetUniformLocation(program, gl.Str("uCameraOrigin\x00"))
		r.worldSkyProceduralFogColor = gl.GetUniformLocation(program, gl.Str("uFogColor\x00"))
		r.worldSkyProceduralFogDensity = gl.GetUniformLocation(program, gl.Str("uFogDensity\x00"))
		r.worldSkyProceduralHorizonColor = gl.GetUniformLocation(program, gl.Str("uHorizonColor\x00"))
		r.worldSkyProceduralZenithColor = gl.GetUniformLocation(program, gl.Str("uZenithColor\x00"))
	}

	if r.worldSkyCubemapProgram == 0 {
		vsCubemap, err := compileShader(worldopengl.WorldSkyCubemapVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky cubemap vertex shader: %w", err)
		}
		fsCubemap, err := compileShader(worldopengl.WorldSkyCubemapFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vsCubemap)
			return fmt.Errorf("compile world sky cubemap fragment shader: %w", err)
		}
		cubemapProgram := createProgram(vsCubemap, fsCubemap)
		r.worldSkyCubemapProgram = cubemapProgram
		r.worldSkyCubemapVPUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uViewProjection\x00"))
		r.worldSkyCubemapUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uCubeMap\x00"))
		r.worldSkyCubemapModelOffsetUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uModelOffset\x00"))
		r.worldSkyCubemapModelRotationUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uModelRotation\x00"))
		r.worldSkyCubemapModelScaleUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uModelScale\x00"))
		r.worldSkyCubemapCameraOriginUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uCameraOrigin\x00"))
		r.worldSkyCubemapFogColorUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uFogColor\x00"))
		r.worldSkyCubemapFogDensityUniform = gl.GetUniformLocation(cubemapProgram, gl.Str("uFogDensity\x00"))
	}
	if r.worldSkyExternalFaceProgram == 0 {
		vsExternalFaces, err := compileShader(worldopengl.WorldSkyCubemapVertexShaderGL, gl.VERTEX_SHADER)
		if err != nil {
			return fmt.Errorf("compile world sky external-face vertex shader: %w", err)
		}
		fsExternalFaces, err := compileShader(worldopengl.WorldSkyExternalFaceFragmentShaderGL, gl.FRAGMENT_SHADER)
		if err != nil {
			gl.DeleteShader(vsExternalFaces)
			return fmt.Errorf("compile world sky external-face fragment shader: %w", err)
		}
		externalFaceProgram := createProgram(vsExternalFaces, fsExternalFaces)
		r.worldSkyExternalFaceProgram = externalFaceProgram
		r.worldSkyExternalFaceVPUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uViewProjection\x00"))
		r.worldSkyExternalFaceRTUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyRT\x00"))
		r.worldSkyExternalFaceBKUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyBK\x00"))
		r.worldSkyExternalFaceLFUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyLF\x00"))
		r.worldSkyExternalFaceFTUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyFT\x00"))
		r.worldSkyExternalFaceUPUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyUP\x00"))
		r.worldSkyExternalFaceDNUniform = gl.GetUniformLocation(externalFaceProgram, gl.Str("uSkyDN\x00"))
		r.worldSkyExternalFaceModelOffset = gl.GetUniformLocation(externalFaceProgram, gl.Str("uModelOffset\x00"))
		r.worldSkyExternalFaceModelRotation = gl.GetUniformLocation(externalFaceProgram, gl.Str("uModelRotation\x00"))
		r.worldSkyExternalFaceModelScale = gl.GetUniformLocation(externalFaceProgram, gl.Str("uModelScale\x00"))
		r.worldSkyExternalFaceCameraOrigin = gl.GetUniformLocation(externalFaceProgram, gl.Str("uCameraOrigin\x00"))
		r.worldSkyExternalFaceFogColor = gl.GetUniformLocation(externalFaceProgram, gl.Str("uFogColor\x00"))
		r.worldSkyExternalFaceFogDensity = gl.GetUniformLocation(externalFaceProgram, gl.Str("uFogDensity\x00"))
	}
	return nil
}

// ensureWorldSkyFallbackTexturesLocked creates fallback sky textures: dark blue for the solid layer, transparent black for the alpha layer.
func (r *Renderer) ensureWorldSkyFallbackTexturesLocked() {
	r.ensureWorldFallbackTextureLocked()
	if r.worldSkyAlphaFallback != 0 {
		return
	}
	r.worldSkyAlphaFallback = uploadWorldTextureRGBA(1, 1, []byte{0, 0, 0, 0})
}

// ---- merged from world_sky_texture_opengl_root.go ----
func shouldSplitAsQuake64Sky(treeVersion int32, width, height int) bool {
	return bsp.IsQuake64(treeVersion) || (width == 32 && height == 64)
}

func indexedOpaqueToRGBA(pixels []byte, palette []byte) []byte {
	rgba := make([]byte, len(pixels)*4)
	for i, p := range pixels {
		r, g, b := GetPaletteColor(p, palette)
		rgba[i*4] = r
		rgba[i*4+1] = g
		rgba[i*4+2] = b
		rgba[i*4+3] = 255
	}
	return rgba
}

func extractEmbeddedSkyLayers(pixels []byte, width, height int, palette []byte, quake64 bool) (solidRGBA, alphaRGBA []byte, layerWidth, layerHeight int, ok bool) {
	if width <= 0 || height <= 0 || len(pixels) < width*height {
		return nil, nil, 0, 0, false
	}

	if quake64 {
		if height < 2 {
			return nil, nil, 0, 0, false
		}
		halfHeight := height / 2
		if halfHeight <= 0 {
			return nil, nil, 0, 0, false
		}
		layerWidth = width
		layerHeight = halfHeight
		layerSize := layerWidth * layerHeight
		front := pixels[:layerSize]
		back := pixels[layerSize : layerSize*2]
		solidRGBA = indexedOpaqueToRGBA(back, palette)
		alphaRGBA = make([]byte, layerSize*4)
		for i, p := range front {
			r, g, b := GetPaletteColor(p, palette)
			alphaRGBA[i*4] = r
			alphaRGBA[i*4+1] = g
			alphaRGBA[i*4+2] = b
			alphaRGBA[i*4+3] = 128
		}
		return solidRGBA, alphaRGBA, layerWidth, layerHeight, true
	}

	if width < 2 {
		return nil, nil, 0, 0, false
	}
	halfWidth := width / 2
	if halfWidth <= 0 {
		return nil, nil, 0, 0, false
	}
	layerWidth = halfWidth
	layerHeight = height
	layerSize := layerWidth * layerHeight
	backIndexed := make([]byte, layerSize)
	frontIndexed := make([]byte, layerSize)
	for y := 0; y < height; y++ {
		row := y * width
		copy(backIndexed[y*halfWidth:(y+1)*halfWidth], pixels[row+halfWidth:row+width])
		copy(frontIndexed[y*halfWidth:(y+1)*halfWidth], pixels[row:row+halfWidth])
	}
	solidRGBA = indexedOpaqueToRGBA(backIndexed, palette)
	alphaRGBA = make([]byte, layerSize*4)
	for i, p := range frontIndexed {
		if p == 0 || p == 255 {
			r, g, b := GetPaletteColor(255, palette)
			alphaRGBA[i*4] = r
			alphaRGBA[i*4+1] = g
			alphaRGBA[i*4+2] = b
			alphaRGBA[i*4+3] = 0
			continue
		}
		r, g, b := GetPaletteColor(p, palette)
		alphaRGBA[i*4] = r
		alphaRGBA[i*4+1] = g
		alphaRGBA[i*4+2] = b
		alphaRGBA[i*4+3] = 255
	}
	return solidRGBA, alphaRGBA, layerWidth, layerHeight, true
}

// ---- merged from world_shader_opengl_root.go ----
