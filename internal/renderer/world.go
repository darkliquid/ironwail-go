package renderer

import (
	"encoding/binary"
	"fmt"
	stdimage "image"
	"log/slog"
	"math"
	"os"
	"sort"
	"time"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const worldUniformBufferSize = 128

type WorldGeometry = worldimpl.WorldGeometry
type WorldVertex = worldimpl.WorldVertex
type WorldFace = worldimpl.WorldFace

// Depth32FloatStencil8 is used instead of Depth24PlusStencil8 because the
// wgpu HAL maps Depth24PlusStencil8 to VK_FORMAT_D24_UNORM_S8_UINT, which
// NVIDIA GPUs do not support. Depth32FloatStencil8 maps to
// VK_FORMAT_D32_SFLOAT_S8_UINT which is universally supported.
const worldDepthTextureFormat = gputypes.TextureFormatDepth32FloatStencil8

func gogpuNonDecalDepthStencilState(depthWrite bool) *wgpu.DepthStencilState {
	stencilFace := wgpu.StencilFaceState{
		Compare:     gputypes.CompareFunctionAlways,
		FailOp:      wgpu.StencilOperationKeep,
		DepthFailOp: wgpu.StencilOperationKeep,
		PassOp:      wgpu.StencilOperationKeep,
	}
	return &wgpu.DepthStencilState{
		Format:            worldDepthTextureFormat,
		DepthWriteEnabled: depthWrite,
		DepthCompare:      gputypes.CompareFunctionLessEqual,
		StencilFront:      stencilFace,
		StencilBack:       stencilFace,
		StencilReadMask:   0,
		StencilWriteMask:  0,
	}
}

// WorldRenderData holds GPU-side resources for world rendering.
// This is what gets uploaded to the GPU and used during rendering.
type WorldRenderData struct {
	// Geometry holds preprocessed vertex/index data
	Geometry *WorldGeometry

	// BoundsMin is the minimum XYZ world-space coordinate of uploaded geometry.
	BoundsMin [3]float32
	// BoundsMax is the maximum XYZ world-space coordinate of uploaded geometry.
	BoundsMax [3]float32

	// Backend resource status used for diagnostics and parity tracking.
	VertexBufferUploaded bool
	IndexBufferUploaded  bool
	HasDiffuseTextures   bool
	HasLightmapTextures  bool
	HasDepthBuffer       bool

	// Stats for debugging
	TotalVertices int
	TotalIndices  int
	TotalFaces    int
}

type gogpuWorldFaceStats struct {
	TotalFaces                 int
	TotalTriangles             int
	LightmappedFaces           int
	LitWaterFaces              int
	TurbulentFaces             int
	TurbulentTriangles         int
	SkyFaces                   int
	SkyTriangles               int
	OpaqueFaces                int
	OpaqueTriangles            int
	AlphaTestFaces             int
	AlphaTestTriangles         int
	OpaqueLiquidFaces          int
	OpaqueLiquidTriangles      int
	TranslucentLiquidFaces     int
	TranslucentLiquidTriangles int
	UnclassifiedFaces          int
	UnclassifiedTriangles      int
}

type gogpuWorldFaceDraw struct {
	face                WorldFace
	textureBindGroup    *wgpu.BindGroup
	lightmapBindGroup   *wgpu.BindGroup
	fullbrightBindGroup *wgpu.BindGroup
	dynamicLight        [3]float32
	litWater            float32
}

func summarizeGoGPUWorldFaceStats(faces []WorldFace, liquidAlpha worldLiquidAlphaSettings) gogpuWorldFaceStats {
	var stats gogpuWorldFaceStats
	for _, face := range faces {
		if face.NumIndices == 0 {
			continue
		}
		triangles := int(face.NumIndices / 3)
		stats.TotalFaces++
		stats.TotalTriangles += triangles
		if face.LightmapIndex >= 0 {
			stats.LightmappedFaces++
		}
		if face.Flags&model.SurfDrawTurb != 0 && face.Flags&model.SurfDrawSky == 0 {
			stats.TurbulentFaces++
			stats.TurbulentTriangles += triangles
			if face.LightmapIndex >= 0 {
				stats.LitWaterFaces++
			}
		}
		switch {
		case shouldDrawGoGPUSkyWorldFace(face):
			stats.SkyFaces++
			stats.SkyTriangles += triangles
		case shouldDrawGoGPUAlphaTestWorldFace(face):
			stats.AlphaTestFaces++
			stats.AlphaTestTriangles += triangles
		case shouldDrawGoGPUOpaqueLiquidFace(face, liquidAlpha):
			stats.OpaqueLiquidFaces++
			stats.OpaqueLiquidTriangles += triangles
		case shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha):
			stats.TranslucentLiquidFaces++
			stats.TranslucentLiquidTriangles += triangles
		case shouldDrawGoGPUOpaqueWorldFace(face):
			stats.OpaqueFaces++
			stats.OpaqueTriangles += triangles
		default:
			stats.UnclassifiedFaces++
			stats.UnclassifiedTriangles += triangles
		}
	}
	return stats
}

func gogpuBindGroupSortKey(bindGroup *wgpu.BindGroup) uintptr {
	return uintptr(unsafe.Pointer(bindGroup))
}

func worldLeafIndex(tree *bsp.Tree, cameraOrigin [3]float32) int {
	if tree == nil || len(tree.Leafs) == 0 {
		return -1
	}
	cameraLeaf := tree.PointInLeaf(cameraOrigin)
	if cameraLeaf == nil {
		return -1
	}
	for i := range tree.Leafs {
		if &tree.Leafs[i] == cameraLeaf {
			return i
		}
	}
	return -1
}

func gogpuWorldDynamicLightSignature(lights []DynamicLight) uint64 {
	var h uint64 = 1469598103934665603
	mix := func(v uint32) {
		h ^= uint64(v)
		h *= 1099511628211
	}
	mix(uint32(len(lights)))
	for _, light := range lights {
		mix(math.Float32bits(light.Position[0]))
		mix(math.Float32bits(light.Position[1]))
		mix(math.Float32bits(light.Position[2]))
		mix(math.Float32bits(light.Radius))
		effectiveMul := light.Brightness * light.FadeMultiplier()
		mix(math.Float32bits(quantizeGoGPUWorldDynamicLightScalar(light.Color[0] * effectiveMul)))
		mix(math.Float32bits(quantizeGoGPUWorldDynamicLightScalar(light.Color[1] * effectiveMul)))
		mix(math.Float32bits(quantizeGoGPUWorldDynamicLightScalar(light.Color[2] * effectiveMul)))
		mix(uint32(light.Type))
	}
	return h
}

func gogpuWorldFaceBatchKeyLess(a, b gogpuWorldFaceBatchKey) bool {
	if ak, bk := gogpuBindGroupSortKey(a.textureBindGroup), gogpuBindGroupSortKey(b.textureBindGroup); ak != bk {
		return ak < bk
	}
	if ak, bk := gogpuBindGroupSortKey(a.lightmapBindGroup), gogpuBindGroupSortKey(b.lightmapBindGroup); ak != bk {
		return ak < bk
	}
	if ak, bk := gogpuBindGroupSortKey(a.fullbrightBindGroup), gogpuBindGroupSortKey(b.fullbrightBindGroup); ak != bk {
		return ak < bk
	}
	if a.litWater != b.litWater {
		return a.litWater < b.litWater
	}
	if a.dynamicLight[0] != b.dynamicLight[0] {
		return a.dynamicLight[0] < b.dynamicLight[0]
	}
	if a.dynamicLight[1] != b.dynamicLight[1] {
		return a.dynamicLight[1] < b.dynamicLight[1]
	}
	return a.dynamicLight[2] < b.dynamicLight[2]
}

type gogpuWorldFaceDrawBucket struct {
	key   gogpuWorldFaceBatchKey
	draws []gogpuWorldFaceDraw
}

type gogpuWorldFaceBatchKey struct {
	textureBindGroup    *wgpu.BindGroup
	lightmapBindGroup   *wgpu.BindGroup
	fullbrightBindGroup *wgpu.BindGroup
	dynamicLight        [3]float32
	litWater            float32
}

type gogpuWorldFaceBatch struct {
	key        gogpuWorldFaceBatchKey
	firstIndex uint32
	numIndices uint32
}

type gogpuTranslucentLiquidFaceDraw struct {
	face       WorldFace
	alpha      float32
	center     [3]float32
	distanceSq float32
}

type gpuWorldTexture struct {
	texture   *wgpu.Texture
	view      *wgpu.TextureView
	bindGroup *wgpu.BindGroup
}

type WorldLightmapSurface = worldimpl.WorldLightmapSurface
type WorldLightmapPage = worldimpl.WorldLightmapPage

type faceLightmapSurface struct {
	pageIndex int
}

type gogpuWorldMaterialBindState struct {
	initialized bool
	texture     *wgpu.BindGroup
	lightmap    *wgpu.BindGroup
	fullbright  *wgpu.BindGroup
}

func (s *gogpuWorldMaterialBindState) invalidate() {
	s.initialized = false
	s.texture = nil
	s.lightmap = nil
	s.fullbright = nil
}

func (s *gogpuWorldMaterialBindState) update(texture, lightmap, fullbright *wgpu.BindGroup) (setTexture, setLightmap, setFullbright bool) {
	if !s.initialized || s.texture != texture {
		setTexture = true
		s.texture = texture
	}
	if !s.initialized || s.lightmap != lightmap {
		setLightmap = true
		s.lightmap = lightmap
	}
	if !s.initialized || s.fullbright != fullbright {
		setFullbright = true
		s.fullbright = fullbright
	}
	s.initialized = true
	return setTexture, setLightmap, setFullbright
}

func gogpuWorldFaceBatchKeyForDraw(draw gogpuWorldFaceDraw) gogpuWorldFaceBatchKey {
	return gogpuWorldFaceBatchKey{
		textureBindGroup:    draw.textureBindGroup,
		lightmapBindGroup:   draw.lightmapBindGroup,
		fullbrightBindGroup: draw.fullbrightBindGroup,
		dynamicLight:        draw.dynamicLight,
		litWater:            draw.litWater,
	}
}

func appendGoGPUOpaqueWorldFaceBatches(dstIndices []uint32, dstBatches []gogpuWorldFaceBatch, draws []gogpuWorldFaceDraw, worldIndices []uint32) ([]uint32, []gogpuWorldFaceBatch) {
	if len(draws) == 0 {
		return dstIndices, dstBatches
	}
	bucketIndex := make(map[gogpuWorldFaceBatchKey]int, len(draws))
	buckets := make([]gogpuWorldFaceDrawBucket, 0, len(draws))
	for _, draw := range draws {
		key := gogpuWorldFaceBatchKeyForDraw(draw)
		index, ok := bucketIndex[key]
		if !ok {
			index = len(buckets)
			bucketIndex[key] = index
			buckets = append(buckets, gogpuWorldFaceDrawBucket{key: key})
		}
		buckets[index].draws = append(buckets[index].draws, draw)
	}
	sort.Slice(buckets, func(i, j int) bool {
		return gogpuWorldFaceBatchKeyLess(buckets[i].key, buckets[j].key)
	})
	for _, bucket := range buckets {
		firstIndex := uint32(len(dstIndices))
		numIndices := uint32(0)
		for _, draw := range bucket.draws {
			first := int(draw.face.FirstIndex)
			end := first + int(draw.face.NumIndices)
			if first < 0 || end > len(worldIndices) || first > end {
				continue
			}
			dstIndices = append(dstIndices, worldIndices[first:end]...)
			numIndices += draw.face.NumIndices
		}
		if numIndices == 0 {
			continue
		}
		dstBatches = append(dstBatches, gogpuWorldFaceBatch{
			key:        bucket.key,
			firstIndex: firstIndex,
			numIndices: numIndices,
		})
	}
	return dstIndices, dstBatches
}

func worldFaceHasLitWater(textureFlags int32, lightmapSurface *faceLightmapSurface) bool {
	return textureFlags&model.SurfDrawTurb != 0 &&
		textureFlags&model.SurfDrawSky == 0 &&
		lightmapSurface != nil
}

func worldLitWaterCvarEnabled() bool {
	cv := cvar.Get(CvarRLitWater)
	if cv == nil {
		return true
	}
	return cv.Int != 0
}

func gogpuWorldLightmapBindGroupForFace(face WorldFace, lightmaps []*gpuWorldTexture, fallback *wgpu.BindGroup, hasLitWater bool) (*wgpu.BindGroup, float32) {
	bindGroup := fallback
	if face.LightmapIndex < 0 || int(face.LightmapIndex) >= len(lightmaps) {
		return bindGroup, 0
	}
	lightmapPage := lightmaps[face.LightmapIndex]
	if lightmapPage == nil || lightmapPage.bindGroup == nil {
		return bindGroup, 0
	}
	bindGroup = lightmapPage.bindGroup
	if worldLitWaterCvarEnabled() && hasLitWater && face.Flags&model.SurfDrawTurb != 0 && face.Flags&model.SurfDrawSky == 0 {
		return bindGroup, 1
	}
	return bindGroup, 0
}

func gogpuFacesHaveLitWater(faces []WorldFace) bool {
	for _, face := range faces {
		if face.Flags&model.SurfDrawTurb != 0 && face.Flags&model.SurfDrawSky == 0 && face.LightmapIndex >= 0 {
			return true
		}
	}
	return false
}

func sortGoGPUTranslucentLiquidFaces(mode AlphaMode, faces []gogpuTranslucentLiquidFaceDraw) {
	if !shouldSortTranslucentCalls(mode) {
		return
	}
	sort.SliceStable(faces, func(i, j int) bool {
		return faces[i].distanceSq > faces[j].distanceSq
	})
}

func effectiveGoGPUAlphaMode(mode AlphaMode) AlphaMode {
	if mode == AlphaModeOIT {
		return AlphaModeSorted
	}
	return mode
}

const worldLightmapPageSize = 1024

// BuildWorldGeometry extracts renderable geometry from a BSP tree.
// This converts the BSP's face/edge/vertex structure into a simple
// vertex buffer + index buffer suitable for GPU rendering.
//
// The function:
// - Iterates all faces in the world model (model 0)
// - Extracts vertices via the edge/surfedge indirection
// - Computes texture coordinates from TexInfo
// - Triangulates faces using fan triangulation
// - Computes normals from plane data
//
// For MVP implementation, this processes ALL faces without culling.
// Future optimization: PVS culling, frustum culling, face sorting.
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
		Vertices:             make([]WorldVertex, 0, 4096),
		Indices:              make([]uint32, 0, 16384),
		Faces:                make([]WorldFace, 0, 256),
		LiquidAlphaOverrides: worldimpl.ParseWorldspawnLiquidAlphaOverrides(tree.Entities),
		TransparentWaterSafe: worldimpl.MapVisTransparentWaterSafe(tree),
		Tree:                 tree,
	}
	lightmapAllocator, err := NewLightmapAllocator(worldLightmapPageSize, worldLightmapPageSize, false)
	if err != nil {
		return nil, fmt.Errorf("create lightmap allocator: %w", err)
	}
	lightmapPages := make([]WorldLightmapPage, 0, 4)
	textureMeta := parseWorldTextureMeta(tree)

	// Process all faces in the selected model.
	numFaces := int(worldModel.NumFaces)
	firstFace := int(worldModel.FirstFace)
	faceLookup := make(map[int]int, numFaces)

	slog.Debug("Building world geometry",
		"numFaces", numFaces,
		"numVertices", len(tree.Vertexes),
		"numEdges", len(tree.Edges))

	for faceIdx := 0; faceIdx < numFaces; faceIdx++ {
		globalFaceIdx := firstFace + faceIdx
		if globalFaceIdx >= len(tree.Faces) {
			break
		}

		face := &tree.Faces[globalFaceIdx]

		// Extract face metadata
		faceData := WorldFace{
			FirstIndex:    uint32(len(geom.Indices)),
			NumIndices:    0, // Will be computed during triangulation
			TextureIndex:  worldFaceTextureIndex(tree, face),
			LightmapIndex: -1,
			Flags:         worldFaceFlags(textureMeta, tree, face),
		}

		// Extract vertices for this face
		faceVerts, lightmapSurface, err := extractFaceVertices(tree, face, lightmapAllocator, &lightmapPages)
		if err != nil {
			slog.Warn("Failed to extract face vertices",
				"faceIdx", globalFaceIdx,
				"error", err)
			continue
		}

		if len(faceVerts) < 3 {
			// Skip degenerate faces
			continue
		}
		if lightmapSurface != nil {
			faceData.LightmapIndex = int32(lightmapSurface.pageIndex)
		}
		faceData.Center = worldFaceCenter(faceVerts)
		if worldFaceHasLitWater(faceData.Flags, lightmapSurface) {
			geom.HasLitWater = true
		}
		if faceData.Flags&model.SurfDrawTurb != 0 {
			geom.LiquidFaceTypes |= faceData.Flags & (model.SurfDrawLava | model.SurfDrawSlime | model.SurfDrawTele | model.SurfDrawWater)
		}

		// Triangulate face using fan triangulation
		// Face with N vertices becomes (N-2) triangles
		baseVertIdx := uint32(len(geom.Vertices))

		// Add all vertices for this face
		geom.Vertices = append(geom.Vertices, faceVerts...)

		// Generate triangle indices (fan triangulation around vertex 0)
		for i := 1; i < len(faceVerts)-1; i++ {
			geom.Indices = append(geom.Indices,
				baseVertIdx,             // Vertex 0 (fan center)
				baseVertIdx+uint32(i),   // Vertex i
				baseVertIdx+uint32(i+1)) // Vertex i+1
		}

		faceData.NumIndices = uint32((len(faceVerts) - 2) * 3)
		geom.Faces = append(geom.Faces, faceData)
		faceLookup[globalFaceIdx] = len(geom.Faces) - 1
	}

	slog.Debug("World geometry built",
		"vertices", len(geom.Vertices),
		"indices", len(geom.Indices),
		"faces", len(geom.Faces),
		"triangles", len(geom.Indices)/3)

	geom.LeafFaces = buildWorldLeafFaceLookup(tree, faceLookup)
	geom.Lightmaps = lightmapPages
	return geom, nil
}

// extractFaceVertices extracts all vertices for a BSP face.
// It follows the edge/surfedge indirection to get vertex positions,
// then computes texture/lightmap coords and normals.
func extractFaceVertices(tree *bsp.Tree, face *bsp.TreeFace, allocator *LightmapAllocator, pages *[]WorldLightmapPage) ([]WorldVertex, *faceLightmapSurface, error) {
	numEdges := int(face.NumEdges)
	if numEdges < 3 {
		return nil, nil, fmt.Errorf("face has < 3 edges")
	}

	vertices := make([]WorldVertex, 0, numEdges)
	rawLightmapCoords := make([][2]float64, 0, numEdges)

	// Get plane normal for this face
	var normal [3]float32
	if int(face.PlaneNum) < len(tree.Planes) {
		planeNormal := tree.Planes[face.PlaneNum].Normal
		normal = planeNormal
		// If face is on back side of plane, flip normal
		if face.Side != 0 {
			normal[0] = -normal[0]
			normal[1] = -normal[1]
			normal[2] = -normal[2]
		}
	} else {
		// Invalid plane number - log warning
		slog.Warn("Invalid plane number for face",
			"planeNum", face.PlaneNum,
			"numPlanes", len(tree.Planes))
	}

	// Check if normal is valid (not all zeros)
	normalLen := float32(math.Sqrt(float64(normal[0]*normal[0] + normal[1]*normal[1] + normal[2]*normal[2])))
	if normalLen < 0.01 {
		slog.Warn("Invalid normal for face",
			"faceIdx", face,
			"normalLen", normalLen)
	}

	texInfo := worldFaceTexInfo(tree, face)
	textureWidth, textureHeight := worldTextureDimensions(tree, texInfo)

	// Iterate through edges to extract vertex positions
	for i := int32(0); i < face.NumEdges; i++ {
		surfEdgeIdx := int(face.FirstEdge) + int(i)
		if surfEdgeIdx >= len(tree.Surfedges) {
			return nil, nil, fmt.Errorf("surfedge index %d out of range", surfEdgeIdx)
		}

		surfEdge := tree.Surfedges[surfEdgeIdx]

		// Surfedge is signed: positive = use edge V[0], negative = use edge V[1]
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

		texCoord := [2]float32{0.0, 0.0}
		lightmapCoord := [2]float32{0.0, 0.0}
		if texInfo != nil {
			u := worldTexCoordDouble(position, texInfo.Vecs[0])
			v := worldTexCoordDouble(position, texInfo.Vecs[1])
			texCoord = [2]float32{float32(u) / textureWidth, float32(v) / textureHeight}
			rawLightmapCoords = append(rawLightmapCoords, [2]float64{u, v})
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

// worldFaceTexInfo resolves the texture-info record for a BSP face, which maps geometric vertices into texture/lightmap UV space.
func worldFaceTexInfo(tree *bsp.Tree, face *bsp.TreeFace) *bsp.Texinfo {
	if tree == nil || face == nil {
		return nil
	}
	if int(face.Texinfo) < 0 || int(face.Texinfo) >= len(tree.Texinfo) {
		return nil
	}
	return &tree.Texinfo[face.Texinfo]
}

// worldFaceTextureIndex resolves the diffuse texture atlas slot for a face so world pass shaders can sample the correct base map.
func worldFaceTextureIndex(tree *bsp.Tree, face *bsp.TreeFace) int32 {
	texInfo := worldFaceTexInfo(tree, face)
	if texInfo == nil || texInfo.Miptex < 0 {
		return -1
	}
	return texInfo.Miptex
}

// worldFaceLightmapIndex returns the lightmap atlas page/index used for static lighting lookup during world shading.
func worldFaceLightmapIndex(face *bsp.TreeFace) int32 {
	if face == nil || face.LightOfs < 0 || face.Styles[0] == 255 {
		return -1
	}
	// gogpu path does not allocate lightmap pages yet; keep a stable "present" sentinel.
	return 0
}

// worldFaceFlags exposes per-face material/render flags (sky, liquid, turbulent, etc.) that drive pass routing and shader behavior.
func worldFaceFlags(textureMeta []worldTextureMeta, tree *bsp.Tree, face *bsp.TreeFace) int32 {
	texInfo := worldFaceTexInfo(tree, face)
	if texInfo == nil {
		return 0
	}
	textureType := classifyWorldTextureName("")
	if int(texInfo.Miptex) >= 0 && int(texInfo.Miptex) < len(textureMeta) {
		textureType = textureMeta[texInfo.Miptex].Type
	}
	return deriveWorldFaceFlags(textureType, texInfo.Flags)
}

// worldTextureDimensions fetches source texture dimensions for texel-density and UV conversion computations.
func worldTextureDimensions(tree *bsp.Tree, texInfo *bsp.Texinfo) (float32, float32) {
	textureWidth := float32(1)
	textureHeight := float32(1)
	if tree == nil || texInfo == nil || texInfo.Miptex < 0 || len(tree.TextureData) < 4 {
		return textureWidth, textureHeight
	}

	textureCount := int(int32(binary.LittleEndian.Uint32(tree.TextureData[:4])))
	miptexIndex := int(texInfo.Miptex)
	if miptexIndex < 0 || miptexIndex >= textureCount {
		return textureWidth, textureHeight
	}
	offsetTableEnd := 4 + textureCount*4
	if len(tree.TextureData) < offsetTableEnd {
		return textureWidth, textureHeight
	}

	offsetPos := 4 + miptexIndex*4
	offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[offsetPos : offsetPos+4])))
	if offset <= 0 || offset >= len(tree.TextureData) {
		return textureWidth, textureHeight
	}

	miptex, err := image.ParseMipTex(tree.TextureData[offset:])
	if err != nil {
		return textureWidth, textureHeight
	}

	if miptex.Width > 0 {
		textureWidth = float32(miptex.Width)
	}
	if miptex.Height > 0 {
		textureHeight = float32(miptex.Height)
	}
	return textureWidth, textureHeight
}

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

// worldVertexShaderWGSL is the WGSL source for world vertex shader
const worldVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
}

struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    time: f32,
    alpha: f32,
    dynamicLight: vec3<f32>,
    litWater: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) clipPos: vec4<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    
    let worldPos = vec4<f32>(input.position, 1.0);
    let clipPos = uniforms.viewProjection * worldPos;
    output.clipPosition = clipPos;
    
    output.texCoord = input.texCoord;
    output.lightmapCoord = input.lightmapCoord;
    output.worldPos = input.position;
    output.normal = input.normal;
    output.clipPos = clipPos;
    
    return output;
}
`

// worldFragmentShaderWGSL is the WGSL source for the GoGPU world fragment shader.
// Keep its lightmap/fullbright/fog math aligned with the canonical world-shader
// behavior so BSP world surfaces look the same across renderer paths.
const worldFragmentShaderWGSL = `
struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    time: f32,
    alpha: f32,
    dynamicLight: vec3<f32>,
    litWater: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) clipPos: vec4<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(1) @binding(0)
var worldSampler: sampler;

@group(1) @binding(1)
var worldTexture: texture_2d<f32>;

@group(2) @binding(0)
var worldLightmapSampler: sampler;

@group(2) @binding(1)
var worldLightmap: texture_2d<f32>;

@group(3) @binding(0)
var worldFullbrightSampler: sampler;

@group(3) @binding(1)
var worldFullbrightTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
	let sampled = textureSample(worldTexture, worldSampler, input.texCoord);
	if (sampled.a < 0.1) {
		discard;
	}
	let lightmap = textureSample(worldLightmap, worldLightmapSampler, input.lightmapCoord).rgb;
	let fullbright = textureSample(worldFullbrightTexture, worldFullbrightSampler, input.texCoord);
	let lit = sampled.rgb * (lightmap + uniforms.dynamicLight) * 2.0 + fullbright.rgb * fullbright.a;
	let fogPosition = input.worldPos - uniforms.cameraOrigin;
	let fog = clamp(exp2(-uniforms.fogDensity * dot(fogPosition, fogPosition)), 0.0, 1.0);
	let fogged = mix(uniforms.fogColor, lit, vec3<f32>(fog));
	return vec4<f32>(fogged, sampled.a * uniforms.alpha);
}
`

const worldSkyVertexShaderWGSL = `
struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
    @location(2) lightmapCoord: vec2<f32>,
    @location(3) normal: vec3<f32>,
}

struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    time: f32,
    alpha: f32,
    dynamicLight: vec3<f32>,
    litWater: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) dir: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@vertex
fn vs_main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    let worldPos = vec4<f32>(input.position, 1.0);
    output.clipPosition = uniforms.viewProjection * worldPos;
    output.dir = vec3<f32>(
        input.position.x - uniforms.cameraOrigin.x,
        input.position.y - uniforms.cameraOrigin.y,
        (input.position.z - uniforms.cameraOrigin.z) * 3.0,
    );
    return output;
}
`

const worldTurbulentFragmentShaderWGSL = `
struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    time: f32,
    alpha: f32,
    dynamicLight: vec3<f32>,
    litWater: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
    @location(1) lightmapCoord: vec2<f32>,
    @location(2) worldPos: vec3<f32>,
    @location(3) normal: vec3<f32>,
    @location(4) clipPos: vec4<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(1) @binding(0)
var worldSampler: sampler;

@group(1) @binding(1)
var worldTexture: texture_2d<f32>;

@group(2) @binding(0)
var worldLightmapSampler: sampler;

@group(2) @binding(1)
var worldLightmap: texture_2d<f32>;

@group(3) @binding(0)
var worldFullbrightSampler: sampler;

@group(3) @binding(1)
var worldFullbrightTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let uv = input.texCoord * 2.0 + 0.125 * sin(input.texCoord.yx * (3.14159265 * 2.0) + vec2<f32>(uniforms.time, uniforms.time));
    let sampled = textureSample(worldTexture, worldSampler, uv);
    let fullbright = textureSample(worldFullbrightTexture, worldFullbrightSampler, uv);
    var lightmap = vec3<f32>(0.5);
    if (uniforms.litWater > 0.5) {
        lightmap = textureSample(worldLightmap, worldLightmapSampler, input.lightmapCoord).rgb;
    }
    let lit = sampled.rgb * (lightmap + uniforms.dynamicLight) * 2.0 + fullbright.rgb * fullbright.a;
    let fogPosition = input.worldPos - uniforms.cameraOrigin;
    let fog = clamp(exp2(-uniforms.fogDensity * dot(fogPosition, fogPosition)), 0.0, 1.0);
    let fogged = mix(uniforms.fogColor, lit, vec3<f32>(fog));
    return vec4<f32>(fogged, sampled.a * uniforms.alpha);
}
`

const worldSkyFragmentShaderWGSL = `
struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    time: f32,
    alpha: f32,
    dynamicLight: vec3<f32>,
    litWater: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) dir: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(1) @binding(0)
var skySolidSampler: sampler;

@group(1) @binding(1)
var skySolidTexture: texture_2d<f32>;

@group(2) @binding(0)
var skyAlphaSampler: sampler;

@group(2) @binding(1)
var skyAlphaTexture: texture_2d<f32>;

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    let dir = normalize(input.dir);
    let uv = dir.xy * (189.0 / 64.0);
    var result = textureSample(skySolidTexture, skySolidSampler, uv + vec2<f32>(uniforms.time / 16.0, uniforms.time / 16.0));
    let layer = textureSample(skyAlphaTexture, skyAlphaSampler, uv + vec2<f32>(uniforms.time / 8.0, uniforms.time / 8.0));
    result = vec4<f32>(mix(result.rgb, layer.rgb, vec3<f32>(layer.a)), 1.0);
    result = vec4<f32>(mix(result.rgb, uniforms.fogColor, vec3<f32>(uniforms.fogDensity)), 1.0);
    return result;
}
`

const worldSkyExternalFaceFragmentShaderWGSL = `
struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    time: f32,
    alpha: f32,
    dynamicLight: vec3<f32>,
    litWater: f32,
}

struct VertexOutput {
    @builtin(position) clipPosition: vec4<f32>,
    @location(0) dir: vec3<f32>,
}

@group(0) @binding(0)
var<uniform> uniforms: Uniforms;

@group(1) @binding(0)
var skySampler: sampler;

@group(1) @binding(1)
var skyRT: texture_2d<f32>;

@group(1) @binding(2)
var skyBK: texture_2d<f32>;

@group(1) @binding(3)
var skyLF: texture_2d<f32>;

@group(1) @binding(4)
var skyFT: texture_2d<f32>;

@group(1) @binding(5)
var skyUP: texture_2d<f32>;

@group(1) @binding(6)
var skyDN: texture_2d<f32>;

fn sampleExternalSky(dir: vec3<f32>) -> vec4<f32> {
    let absDir = abs(dir);
    var ma: f32;
    var uv: vec2<f32>;
    if (absDir.x >= absDir.y && absDir.x >= absDir.z) {
        ma = absDir.x;
        if (dir.x > 0.0) {
            uv = vec2<f32>((-dir.z / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
            return textureSample(skyFT, skySampler, uv);
        }
        uv = vec2<f32>((dir.z / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
        return textureSample(skyBK, skySampler, uv);
    }
    if (absDir.y >= absDir.x && absDir.y >= absDir.z) {
        ma = absDir.y;
        if (dir.y > 0.0) {
            uv = vec2<f32>((dir.x / ma + 1.0) * 0.5, (dir.z / ma + 1.0) * 0.5);
            return textureSample(skyUP, skySampler, uv);
        }
        uv = vec2<f32>((dir.x / ma + 1.0) * 0.5, (-dir.z / ma + 1.0) * 0.5);
        return textureSample(skyDN, skySampler, uv);
    }
    ma = absDir.z;
    if (dir.z > 0.0) {
        uv = vec2<f32>((dir.x / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
        return textureSample(skyRT, skySampler, uv);
    }
    uv = vec2<f32>((-dir.x / ma + 1.0) * 0.5, (-dir.y / ma + 1.0) * 0.5);
    return textureSample(skyLF, skySampler, uv);
}

@fragment
fn fs_main(input: VertexOutput) -> @location(0) vec4<f32> {
    var result = sampleExternalSky(normalize(input.dir));
    result = vec4<f32>(mix(result.rgb, uniforms.fogColor, vec3<f32>(uniforms.fogDensity)), result.a);
    return result;
}
`

// compileWorldShader compiles a WGSL shader to SPIR-V bytecode
// For now, we pass WGSL directly to HAL which handles compilation internally
func compileWorldShader(source string) string {
	// Return WGSL source directly - HAL will compile it
	return source
}

// createWorldShaderModule creates a HAL shader module from WGSL source
func createWorldShaderModule(device *wgpu.Device, wgslSource string, label string) (*wgpu.ShaderModule, error) {
	shaderModule, err := device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: label,
		WGSL:  wgslSource,
	})
	if err != nil {
		return nil, fmt.Errorf("create shader module: %w", err)
	}

	return shaderModule, nil
}

// createWorldVertexBuffer uploads vertex data to GPU
func (r *Renderer) createWorldVertexBuffer(device *wgpu.Device, queue *wgpu.Queue, vertices []WorldVertex) (*wgpu.Buffer, error) {
	if len(vertices) == 0 {
		return nil, fmt.Errorf("no vertices to upload")
	}

	// Calculate size
	vertexSize := uint64(len(vertices)) * 44 // sizeof(WorldVertex) = 44 bytes

	slog.Debug("Creating world vertex buffer",
		"vertexCount", len(vertices),
		"sizeBytes", vertexSize)

	// Create GPU buffer
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Vertices",
		Size:             vertexSize,
		Usage:            gputypes.BufferUsageVertex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create vertex buffer: %w", err)
	}

	// Write vertex data to buffer
	vertexData := make([]byte, vertexSize)
	for i, v := range vertices {
		offset := uint64(i) * 44

		// Write position (3 float32 = 12 bytes)
		posBytes := float32ToBytes(v.Position[:])
		copy(vertexData[offset:offset+12], posBytes)

		// Write texCoord (2 float32 = 8 bytes)
		texBytes := float32ToBytes(v.TexCoord[:])
		copy(vertexData[offset+12:offset+20], texBytes)

		// Write lightmapCoord (2 float32 = 8 bytes)
		lightBytes := float32ToBytes(v.LightmapCoord[:])
		copy(vertexData[offset+20:offset+28], lightBytes)

		// Write normal (3 float32 = 12 bytes)
		normBytes := float32ToBytes(v.Normal[:])
		copy(vertexData[offset+28:offset+40], normBytes)
	}

	queue.WriteBuffer(buffer, 0, vertexData)

	slog.Debug("World vertex buffer uploaded", "vertices", len(vertices))

	return buffer, nil
}

// createWorldIndexBuffer uploads index data to GPU
func (r *Renderer) createWorldIndexBuffer(device *wgpu.Device, queue *wgpu.Queue, indices []uint32) (*wgpu.Buffer, uint32, error) {
	if len(indices) == 0 {
		return nil, 0, fmt.Errorf("no indices to upload")
	}

	indexSize := uint64(len(indices)) * 4 // uint32 = 4 bytes

	slog.Debug("Creating world index buffer",
		"indexCount", len(indices),
		"sizeBytes", indexSize)

	// Create GPU buffer
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Indices",
		Size:             indexSize,
		Usage:            gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("create index buffer: %w", err)
	}

	// Write index data to buffer
	indexData := make([]byte, indexSize)
	for i, idx := range indices {
		offset := uint64(i) * 4
		binary.LittleEndian.PutUint32(indexData[offset:offset+4], idx)
	}

	queue.WriteBuffer(buffer, 0, indexData)

	slog.Debug("World index buffer uploaded", "indices", len(indices))

	return buffer, uint32(len(indices)), nil
}

func (r *Renderer) ensureGoGPUWorldDynamicIndexBuffer(device *wgpu.Device, size uint64) (*wgpu.Buffer, error) {
	if size == 0 {
		return nil, nil
	}
	if r.worldDynamicIndexBuffer != nil && r.worldDynamicIndexBufferSize >= size {
		return r.worldDynamicIndexBuffer, nil
	}
	if r.worldDynamicIndexBuffer != nil {
		r.worldDynamicIndexBuffer.Release()
		r.worldDynamicIndexBuffer = nil
		r.worldDynamicIndexBufferSize = 0
	}
	allocSize := size
	if allocSize < 4096 {
		allocSize = 4096
	}
	buffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Dynamic Indices",
		Size:             allocSize,
		Usage:            gputypes.BufferUsageIndex | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create dynamic world index buffer: %w", err)
	}
	r.worldDynamicIndexBuffer = buffer
	r.worldDynamicIndexBufferSize = allocSize
	return buffer, nil
}

// createWorldRenderTarget ensures the GoGPU world scene target exists for the current framebuffer size.
func (r *Renderer) createWorldRenderTarget() error {
	width, height := r.Size()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid window size: %dx%d", width, height)
	}
	device := r.getWGPUDevice()
	if device == nil {
		return fmt.Errorf("nil wgpu device")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ensureWorldRenderTargetLocked(device, width, height)
}

// createWorldPipeline creates the render pipeline for world rendering.
// Configures all pipeline state: vertex layout, shaders, depth-stencil, primitive topology, etc.
func (r *Renderer) createWorldPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule) (*wgpu.RenderPipeline, *wgpu.PipelineLayout, error) {
	if device == nil || vertexShader == nil || fragmentShader == nil {
		return nil, nil, fmt.Errorf("invalid shader modules or device")
	}

	// Create bind group layout for @group(0) @binding(0) uniform buffer.
	uniformLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "World Uniform BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageVertex | gputypes.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             gputypes.BufferBindingTypeUniform,
					HasDynamicOffset: false,
					MinBindingSize:   worldUniformBufferSize,
				},
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create uniform bind group layout: %w", err)
	}

	textureLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "World Texture BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Sampler: &gputypes.SamplerBindingLayout{
					Type: gputypes.SamplerBindingTypeFiltering,
				},
			},
			{
				Binding:    1,
				Visibility: gputypes.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    gputypes.TextureSampleTypeFloat,
					ViewDimension: gputypes.TextureViewDimension2D,
					Multisampled:  false,
				},
			},
		},
	})
	if err != nil {
		uniformLayout.Release()
		return nil, nil, fmt.Errorf("create texture bind group layout: %w", err)
	}

	// Create pipeline layout with the uniform bind group layout.
	pipelineLayoutDesc := &wgpu.PipelineLayoutDescriptor{
		Label:            "World Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{uniformLayout, textureLayout, textureLayout, textureLayout},
	}

	pipelineLayout, err := device.CreatePipelineLayout(pipelineLayoutDesc)
	if err != nil {
		textureLayout.Release()
		uniformLayout.Release()
		return nil, nil, fmt.Errorf("create pipeline layout: %w", err)
	}

	r.mu.Lock()
	r.uniformBindGroupLayout = uniformLayout
	r.textureBindGroupLayout = textureLayout
	r.mu.Unlock()

	pipeline, err := r.createWorldOpaquePipeline(device, vertexShader, fragmentShader, pipelineLayout)
	if err != nil {
		textureLayout.Release()
		uniformLayout.Release()
		pipelineLayout.Release()
		return nil, nil, fmt.Errorf("create render pipeline: %w", err)
	}

	slog.Debug("World render pipeline created")
	return pipeline, pipelineLayout, nil
}

func (r *Renderer) createWorldOpaquePipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
		},
	}
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: gogpuNonDecalDepthStencilState(true),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
}

func (r *Renderer) createWorldSkyPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
		},
	}
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Sky Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: gogpuNonDecalDepthStencilState(false),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
}

func (r *Renderer) createWorldExternalSkyPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule) (*wgpu.RenderPipeline, *wgpu.PipelineLayout, *wgpu.BindGroupLayout, error) {
	if device == nil || vertexShader == nil || fragmentShader == nil || r.uniformBindGroupLayout == nil {
		return nil, nil, nil, fmt.Errorf("missing external sky pipeline inputs")
	}
	textureLayout, err := device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Label: "World External Sky Texture BGL",
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: gputypes.ShaderStageFragment,
				Sampler: &gputypes.SamplerBindingLayout{
					Type: gputypes.SamplerBindingTypeFiltering,
				},
			},
			{Binding: 1, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 2, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 3, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 4, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 5, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
			{Binding: 6, Visibility: gputypes.ShaderStageFragment, Texture: &gputypes.TextureBindingLayout{SampleType: gputypes.TextureSampleTypeFloat, ViewDimension: gputypes.TextureViewDimension2D, Multisampled: false}},
		},
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create external sky bind group layout: %w", err)
	}
	layout, err := device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		Label:            "World External Sky Pipeline Layout",
		BindGroupLayouts: []*wgpu.BindGroupLayout{r.uniformBindGroupLayout, textureLayout},
	})
	if err != nil {
		textureLayout.Release()
		return nil, nil, nil, fmt.Errorf("create external sky pipeline layout: %w", err)
	}
	pipeline, err := r.createWorldSkyPipeline(device, vertexShader, fragmentShader, layout)
	if err != nil {
		layout.Release()
		textureLayout.Release()
		return nil, nil, nil, fmt.Errorf("create external sky pipeline: %w", err)
	}
	return pipeline, layout, textureLayout, nil
}

func (r *Renderer) createWorldTurbulentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
		},
	}
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Turbulent Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: gogpuNonDecalDepthStencilState(true),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorZero, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
}

func (r *Renderer) createWorldTranslucentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
		},
	}
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Translucent Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: gogpuNonDecalDepthStencilState(false),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
}

func (r *Renderer) createWorldTranslucentTurbulentPipeline(device *wgpu.Device, vertexShader, fragmentShader *wgpu.ShaderModule, layout *wgpu.PipelineLayout) (*wgpu.RenderPipeline, error) {
	vertexBufferLayout := gputypes.VertexBufferLayout{
		ArrayStride: 44,
		StepMode:    gputypes.VertexStepModeVertex,
		Attributes: []gputypes.VertexAttribute{
			{Format: gputypes.VertexFormatFloat32x3, Offset: 0, ShaderLocation: 0},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 12, ShaderLocation: 1},
			{Format: gputypes.VertexFormatFloat32x2, Offset: 20, ShaderLocation: 2},
			{Format: gputypes.VertexFormatFloat32x3, Offset: 28, ShaderLocation: 3},
		},
	}
	surfaceFormat := gputypes.TextureFormatBGRA8Unorm
	if r.app != nil {
		if provider := r.app.DeviceProvider(); provider != nil {
			surfaceFormat = provider.SurfaceFormat()
		}
	}
	return validatedGoGPURenderPipeline(device, &wgpu.RenderPipelineDescriptor{
		Label:  "World Translucent Turbulent Render Pipeline",
		Layout: layout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
			Buffers:    []gputypes.VertexBufferLayout{vertexBufferLayout},
		},
		Primitive: gputypes.PrimitiveState{
			Topology:  gputypes.PrimitiveTopologyTriangleList,
			FrontFace: gputypes.FrontFaceCCW,
			CullMode:  gputypes.CullModeNone,
		},
		DepthStencil: gogpuNonDecalDepthStencilState(false),
		Multisample: gputypes.MultisampleState{
			Count:                  1,
			Mask:                   0xFFFFFFFF,
			AlphaToCoverageEnabled: false,
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []gputypes.ColorTargetState{{
				Format: surfaceFormat,
				Blend: &gputypes.BlendState{
					Color: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorSrcAlpha, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
					Alpha: gputypes.BlendComponent{SrcFactor: gputypes.BlendFactorOne, DstFactor: gputypes.BlendFactorOneMinusSrcAlpha, Operation: gputypes.BlendOperationAdd},
				},
				WriteMask: gputypes.ColorWriteMaskAll,
			}},
		},
	})
}

// createWorldWhiteTexture creates a simple 1x1 white texture for fallback.
// Used when actual textures are not yet available for rendering.
func (r *Renderer) createWorldWhiteTexture(device *wgpu.Device, queue *wgpu.Queue) (*wgpu.Texture, *wgpu.TextureView, error) {
	if device == nil || queue == nil {
		return nil, nil, fmt.Errorf("invalid device or queue")
	}

	// Create 1x1 RGBA texture descriptor
	textureDesc := &wgpu.TextureDescriptor{
		Label:         "World White Texture",
		Size:          wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	}

	// Create the texture
	texture, err := device.CreateTexture(textureDesc)
	if err != nil {
		return nil, nil, fmt.Errorf("create white texture: %w", err)
	}

	// Create white pixel data (RGBA: 255,255,255,255)
	whitePixel := []byte{255, 255, 255, 255}

	// Write white pixel to texture using queue
	err = queue.WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture:  texture,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   gputypes.TextureAspectAll,
		},
		whitePixel,
		&wgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  4, // 1 pixel × 4 bytes
			RowsPerImage: 1,
		},
		&wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
	)
	if err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("write white texture data: %w", err)
	}

	// Create texture view
	textureViewDesc := &wgpu.TextureViewDescriptor{
		Label:           "World White Texture View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	}

	textureView, err := device.CreateTextureView(texture, textureViewDesc)
	if err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("create white texture view: %w", err)
	}

	slog.Debug("World white texture created")
	return texture, textureView, nil
}

func (r *Renderer) createWorldTextureFromRGBA(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, label string, rgba []byte, width, height int) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil {
		return nil, fmt.Errorf("invalid world texture upload inputs")
	}
	if width <= 0 || height <= 0 || len(rgba) != width*height*4 {
		return nil, fmt.Errorf("invalid world texture size/data %dx%d (%d bytes)", width, height, len(rgba))
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create world texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Release()
		return nil, fmt.Errorf("write world texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           label + " View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Release()
		return nil, fmt.Errorf("create world texture view: %w", err)
	}
	bindGroup, err := r.createWorldTextureBindGroup(device, sampler, view)
	if err != nil {
		view.Release()
		texture.Release()
		return nil, fmt.Errorf("create world texture bind group: %w", err)
	}
	return &gpuWorldTexture{texture: texture, view: view, bindGroup: bindGroup}, nil
}

func shouldDrawGoGPUOpaqueWorldFace(face WorldFace) bool {
	if face.NumIndices == 0 {
		return false
	}
	if face.Flags&(model.SurfDrawSky|model.SurfDrawTurb|model.SurfDrawFence) != 0 {
		return false
	}
	return true
}

func shouldDrawGoGPUAlphaTestWorldFace(face WorldFace) bool {
	return face.NumIndices > 0 && worldFacePass(face.Flags, 1) == worldPassAlphaTest
}

func shouldDrawGoGPUSkyWorldFace(face WorldFace) bool {
	return face.NumIndices > 0 && face.Flags&model.SurfDrawSky != 0
}

func shouldDrawGoGPUOpaqueLiquidFace(face WorldFace, liquidAlpha worldLiquidAlphaSettings) bool {
	return face.NumIndices > 0 && worldFaceIsLiquid(face.Flags) && worldFacePass(face.Flags, worldFaceAlpha(face.Flags, liquidAlpha)) == worldPassOpaque
}

func shouldDrawGoGPUTranslucentLiquidFace(face WorldFace, liquidAlpha worldLiquidAlphaSettings) bool {
	return face.NumIndices > 0 && worldFaceIsLiquid(face.Flags) && worldFacePass(face.Flags, worldFaceAlpha(face.Flags, liquidAlpha)) == worldPassTranslucent
}

func (r *Renderer) createWorldTextureSampler(device *wgpu.Device) (*wgpu.Sampler, error) {
	return device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "World Texture Sampler",
		AddressModeU: gputypes.AddressModeRepeat,
		AddressModeV: gputypes.AddressModeRepeat,
		AddressModeW: gputypes.AddressModeRepeat,
		MagFilter:    gputypes.FilterModeNearest,
		MinFilter:    gputypes.FilterModeNearest,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
}

func (r *Renderer) createWorldLightmapSampler(device *wgpu.Device) (*wgpu.Sampler, error) {
	return device.CreateSampler(&wgpu.SamplerDescriptor{
		Label:        "World Lightmap Sampler",
		AddressModeU: gputypes.AddressModeClampToEdge,
		AddressModeV: gputypes.AddressModeClampToEdge,
		AddressModeW: gputypes.AddressModeClampToEdge,
		MagFilter:    gputypes.FilterModeLinear,
		MinFilter:    gputypes.FilterModeLinear,
		MipmapFilter: gputypes.FilterModeNearest,
		LodMinClamp:  0,
		LodMaxClamp:  0,
	})
}

func (r *Renderer) createWorldTextureBindGroup(device *wgpu.Device, sampler *wgpu.Sampler, view *wgpu.TextureView) (*wgpu.BindGroup, error) {
	if device == nil || sampler == nil || view == nil || r.textureBindGroupLayout == nil {
		return nil, fmt.Errorf("missing world texture bind group resources")
	}
	return device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "World Texture BG",
		Layout: r.textureBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: sampler},
			{Binding: 1, TextureView: view},
		},
	})
}

func (r *Renderer) createWorldExternalSkyBindGroup(device *wgpu.Device, sampler *wgpu.Sampler, views [6]*wgpu.TextureView) (*wgpu.BindGroup, error) {
	if device == nil || sampler == nil || r.worldSkyExternalBindGroupLayout == nil {
		return nil, fmt.Errorf("missing external sky bind group resources")
	}
	for i, view := range views {
		if view == nil {
			return nil, fmt.Errorf("missing external sky texture view %d", i)
		}
	}
	return device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Label:  "World External Sky BG",
		Layout: r.worldSkyExternalBindGroupLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Sampler: sampler},
			{Binding: 1, TextureView: views[0]},
			{Binding: 2, TextureView: views[1]},
			{Binding: 3, TextureView: views[2]},
			{Binding: 4, TextureView: views[3]},
			{Binding: 5, TextureView: views[4]},
			{Binding: 6, TextureView: views[5]},
		},
	})
}

func (r *Renderer) createWorldExternalSkyFaceTexture(device *wgpu.Device, queue *wgpu.Queue, label string, rgba []byte, width, height int) (*wgpu.Texture, *wgpu.TextureView, error) {
	if device == nil || queue == nil {
		return nil, nil, fmt.Errorf("invalid external sky texture upload inputs")
	}
	if width <= 0 || height <= 0 || len(rgba) != width*height*4 {
		return nil, nil, fmt.Errorf("invalid external sky texture size/data %dx%d (%d bytes)", width, height, len(rgba))
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         label,
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create external sky texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &wgpu.ImageDataLayout{BytesPerRow: uint32(width * 4), RowsPerImage: uint32(height)}, &wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1}); err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("write external sky texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           label + " View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("create external sky texture view: %w", err)
	}
	return texture, view, nil
}

func (r *Renderer) ensureGoGPUExternalSkyboxLocked(device *wgpu.Device, queue *wgpu.Queue) error {
	if r.worldSkyExternalMode != externalSkyboxRenderFaces || r.worldSkyExternalLoaded == 0 {
		return nil
	}
	if device == nil || queue == nil || r.worldLightmapSampler == nil || r.worldSkyExternalBindGroupLayout == nil {
		return fmt.Errorf("external sky resources not ready")
	}
	r.destroyGoGPUExternalSkyboxResourcesLocked()
	fallbackPixel := [4]byte{0, 0, 0, 255}
	var views [6]*wgpu.TextureView
	for i, face := range r.worldSkyExternalFaces {
		width := face.Width
		height := face.Height
		data := face.RGBA
		if width <= 0 || height <= 0 || len(data) != width*height*4 {
			width, height = 1, 1
			data = fallbackPixel[:]
		}
		texture, view, err := r.createWorldExternalSkyFaceTexture(device, queue, fmt.Sprintf("World External Sky %s", skyboxFaceSuffixes[i]), data, width, height)
		if err != nil {
			r.destroyGoGPUExternalSkyboxResourcesLocked()
			return err
		}
		r.worldSkyExternalTextures[i] = texture
		r.worldSkyExternalViews[i] = view
		views[i] = view
	}
	bindGroup, err := r.createWorldExternalSkyBindGroup(device, r.worldLightmapSampler, views)
	if err != nil {
		r.destroyGoGPUExternalSkyboxResourcesLocked()
		return fmt.Errorf("create external sky bind group: %w", err)
	}
	r.worldSkyExternalBindGroup = bindGroup
	return nil
}

func (r *Renderer) createWorldDiffuseTexture(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, textureType model.TextureType, rgba []byte, width, height int) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil {
		return nil, fmt.Errorf("invalid world texture upload inputs")
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid world texture size %dx%d", width, height)
	}
	rgba := ConvertPaletteToRGBA(pixels, r.palette)
	if classifyWorldTextureName(miptex.Name) == model.TexTypeCutout {
		cutout := &stdimage.RGBA{
			Pix:    rgba,
			Stride: width * 4,
			Rect:   stdimage.Rect(0, 0, width, height),
		}
		image.AlphaEdgeFix(cutout)
	}
	return r.createWorldTextureFromRGBA(device, queue, sampler, "World Diffuse Texture", rgba, width, height)
}

func (r *Renderer) uploadWorldMaterialTextures(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, tree *bsp.Tree) (map[int32]*gpuWorldTexture, map[int32]*gpuWorldTexture, []*SurfaceTexture) {
	if tree == nil || device == nil || queue == nil || sampler == nil || len(tree.TextureData) < 4 {
		return nil, nil, nil
	}
	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if textureCount <= 0 || len(tree.TextureData) < 4+textureCount*4 {
		return nil, nil, nil
	}
	textures := make(map[int32]*gpuWorldTexture, textureCount)
	fullbright := make(map[int32]*gpuWorldTexture)
	textureNames := make([]string, textureCount)
	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil {
			continue
		}
		textureNames[i] = miptex.Name
		pixels, width, height, err := miptex.MipLevel(0)
		if err != nil || width <= 0 || height <= 0 {
			continue
		}
		textureType := classifyWorldTextureName(miptex.Name)
		materialRGBA := worldimpl.BuildMaterialTextureRGBA(pixels, r.palette, textureType)
		worldTexture, err := r.createWorldDiffuseTexture(device, queue, sampler, textureType, materialRGBA.DiffuseRGBA, width, height)
		if err != nil {
			slog.Warn("failed to upload world diffuse texture", "texture", miptex.Name, "error", err)
			continue
		}
		textures[int32(i)] = worldTexture
		if !materialRGBA.HasFullbright {
			continue
		}
		fullbrightTexture, err := r.createWorldTextureFromRGBA(device, queue, sampler, "World Fullbright Texture", materialRGBA.FullbrightRGBA, width, height)
		if err != nil {
			slog.Warn("failed to upload world fullbright texture", "texture", miptex.Name, "error", err)
			continue
		}
		fullbright[int32(i)] = fullbrightTexture
	}
	animations, err := BuildTextureAnimations(textureNames)
	if err != nil {
		slog.Warn("failed to build world texture animations", "error", err)
	}
	return textures, fullbright, animations
}

func shouldSplitAsQuake64Sky(treeVersion int32, width, height int) bool {
	return worldimpl.ShouldSplitAsQuake64Sky(treeVersion, width, height)
}

func extractEmbeddedSkyLayers(pixels []byte, width, height int, palette []byte, quake64 bool) (solidRGBA, alphaRGBA []byte, layerWidth, layerHeight int, ok bool) {
	return worldimpl.ExtractEmbeddedSkyLayers(pixels, width, height, palette, quake64)
}

func (r *Renderer) uploadWorldEmbeddedSkyTextures(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, tree *bsp.Tree) (map[int32]*gpuWorldTexture, map[int32]*gpuWorldTexture) {
	if tree == nil || device == nil || queue == nil || sampler == nil || len(tree.TextureData) < 4 {
		return nil, nil
	}
	textureCount := int(binary.LittleEndian.Uint32(tree.TextureData[:4]))
	if textureCount <= 0 || len(tree.TextureData) < 4+textureCount*4 {
		return nil, nil
	}
	solid := make(map[int32]*gpuWorldTexture)
	alpha := make(map[int32]*gpuWorldTexture)
	for i := 0; i < textureCount; i++ {
		offset := int(int32(binary.LittleEndian.Uint32(tree.TextureData[4+i*4:])))
		if offset <= 0 || offset >= len(tree.TextureData) {
			continue
		}
		miptex, err := image.ParseMipTex(tree.TextureData[offset:])
		if err != nil || classifyWorldTextureName(miptex.Name) != model.TexTypeSky {
			continue
		}
		pixels, width, height, err := miptex.MipLevel(0)
		if err != nil {
			continue
		}
		solidRGBA, alphaRGBA, layerWidth, layerHeight, ok := extractEmbeddedSkyLayers(pixels, width, height, r.palette, shouldSplitAsQuake64Sky(tree.Version, width, height))
		if !ok {
			continue
		}
		solidTexture, err := r.createWorldTextureFromRGBA(device, queue, sampler, "World Sky Solid Texture", solidRGBA, layerWidth, layerHeight)
		if err != nil {
			slog.Warn("failed to upload world sky solid texture", "texture", miptex.Name, "error", err)
			continue
		}
		alphaTexture, err := r.createWorldTextureFromRGBA(device, queue, sampler, "World Sky Alpha Texture", alphaRGBA, layerWidth, layerHeight)
		if err != nil {
			if solidTexture.bindGroup != nil {
				solidTexture.bindGroup.Release()
			}
			if solidTexture.view != nil {
				solidTexture.view.Release()
			}
			if solidTexture.texture != nil {
				solidTexture.texture.Release()
			}
			slog.Warn("failed to upload world sky alpha texture", "texture", miptex.Name, "error", err)
			continue
		}
		solid[int32(i)] = solidTexture
		alpha[int32(i)] = alphaTexture
	}
	return solid, alpha
}

func gogpuWorldTextureForFace(face WorldFace, textures map[int32]*gpuWorldTexture, textureAnimations []*SurfaceTexture, fallback *gpuWorldTexture, frame int, timeSeconds float64) *gpuWorldTexture {
	textureIndex := face.TextureIndex
	if textureIndex >= 0 && int(textureIndex) < len(textureAnimations) && textureAnimations[textureIndex] != nil {
		if animated, err := TextureAnimation(textureAnimations[textureIndex], frame, timeSeconds); err == nil && animated != nil {
			textureIndex = animated.TextureIndex
		}
	}
	worldTexture := textures[textureIndex]
	if worldTexture == nil && textureIndex != face.TextureIndex {
		worldTexture = textures[face.TextureIndex]
	}
	if worldTexture == nil {
		return fallback
	}
	return worldTexture
}

func (r *Renderer) createWorldLightmapPageTexture(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, page *WorldLightmapPage, values [64]float32) (*gpuWorldTexture, error) {
	if device == nil || queue == nil || sampler == nil || page == nil {
		return nil, fmt.Errorf("invalid world lightmap upload inputs")
	}
	rgba := buildWorldLightmapPageRGBA(page, values)
	if len(rgba) == 0 {
		return nil, fmt.Errorf("empty world lightmap page")
	}
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "World Lightmap Texture",
		Size:          wgpu.Extent3D{Width: uint32(page.Width), Height: uint32(page.Height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        gputypes.TextureFormatRGBA8Unorm,
		Usage:         gputypes.TextureUsageTextureBinding | gputypes.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, fmt.Errorf("create world lightmap texture: %w", err)
	}
	if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
		Texture:  texture,
		MipLevel: 0,
		Aspect:   gputypes.TextureAspectAll,
	}, rgba, &wgpu.ImageDataLayout{BytesPerRow: uint32(page.Width * 4), RowsPerImage: uint32(page.Height)}, &wgpu.Extent3D{Width: uint32(page.Width), Height: uint32(page.Height), DepthOrArrayLayers: 1}); err != nil {
		texture.Release()
		return nil, fmt.Errorf("write world lightmap texture: %w", err)
	}
	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           "World Lightmap Texture View",
		Format:          gputypes.TextureFormatRGBA8Unorm,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Release()
		return nil, fmt.Errorf("create world lightmap view: %w", err)
	}
	bindGroup, err := r.createWorldTextureBindGroup(device, sampler, view)
	if err != nil {
		view.Release()
		texture.Release()
		return nil, fmt.Errorf("create world lightmap bind group: %w", err)
	}
	return &gpuWorldTexture{texture: texture, view: view, bindGroup: bindGroup}, nil
}

func (r *Renderer) uploadWorldLightmapPages(device *wgpu.Device, queue *wgpu.Queue, sampler *wgpu.Sampler, pages []WorldLightmapPage, values [64]float32) []*gpuWorldTexture {
	if device == nil || queue == nil || sampler == nil || len(pages) == 0 {
		return nil
	}
	out := make([]*gpuWorldTexture, len(pages))
	for i := range pages {
		pageTexture, err := r.createWorldLightmapPageTexture(device, queue, sampler, &pages[i], values)
		if err != nil {
			slog.Warn("failed to upload world lightmap page", "page", i, "error", err)
			continue
		}
		out[i] = pageTexture
	}
	return out
}

func lightmapDirtyBounds(page WorldLightmapPage) (x, y, w, h int) {
	minX, minY := page.Width, page.Height
	maxX, maxY := 0, 0
	found := false
	for _, surface := range page.Surfaces {
		if !surface.Dirty || surface.Width <= 0 || surface.Height <= 0 {
			continue
		}
		if surface.X < minX {
			minX = surface.X
		}
		if surface.Y < minY {
			minY = surface.Y
		}
		if surface.X+surface.Width > maxX {
			maxX = surface.X + surface.Width
		}
		if surface.Y+surface.Height > maxY {
			maxY = surface.Y + surface.Height
		}
		found = true
	}
	if !found {
		return 0, 0, 0, 0
	}
	return minX, minY, maxX - minX, maxY - minY
}

func extractLightmapRegionRGBA(dst, rgba []byte, pageWidth, x, y, w, h int) []byte {
	if len(rgba) == 0 || pageWidth <= 0 || w <= 0 || h <= 0 {
		return nil
	}
	size := w * h * 4
	if cap(dst) < size {
		dst = make([]byte, size)
	} else {
		dst = dst[:size]
	}
	srcStride := pageWidth * 4
	dstStride := w * 4
	for row := 0; row < h; row++ {
		srcStart := ((y + row) * srcStride) + x*4
		srcEnd := srcStart + dstStride
		dstStart := row * dstStride
		copy(dst[dstStart:dstStart+dstStride], rgba[srcStart:srcEnd])
	}
	return dst
}

func updateUploadedLightmapsLocked(queue *wgpu.Queue, uploaded []*gpuWorldTexture, pages []WorldLightmapPage, values [64]float32) {
	if queue == nil || len(pages) == 0 || len(uploaded) == 0 {
		return
	}
	count := len(uploaded)
	if len(pages) < count {
		count = len(pages)
	}
	for i := 0; i < count; i++ {
		if !pages[i].Dirty || uploaded[i] == nil || uploaded[i].texture == nil {
			continue
		}
		rgba := buildWorldLightmapPageRGBA(&pages[i], values)
		if len(rgba) == 0 {
			continue
		}
		x, y, w, h := lightmapDirtyBounds(pages[i])
		if w == 0 || h == 0 {
			continue
		}
		region := extractLightmapRegionRGBA(pages[i].CachedRegionRGBA, rgba, pages[i].Width, x, y, w, h)
		if len(region) == 0 {
			continue
		}
		pages[i].CachedRegionRGBA = region
		if err := queue.WriteTexture(&wgpu.ImageCopyTexture{
			Texture:  uploaded[i].texture,
			MipLevel: 0,
			Aspect:   gputypes.TextureAspectAll,
			Origin:   wgpu.Origin3D{X: uint32(x), Y: uint32(y)},
		}, region, &wgpu.ImageDataLayout{BytesPerRow: uint32(w * 4), RowsPerImage: uint32(h)}, &wgpu.Extent3D{Width: uint32(w), Height: uint32(h), DepthOrArrayLayers: 1}); err != nil {
			slog.Warn("failed to update world lightmap page", "page", i, "error", err)
		}
	}
	clearDirtyFlags(pages)
}

func (r *Renderer) setGoGPUWorldLightStyleValues(values [64]float32) {
	queue := r.getWGPUQueue()
	if queue == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	changed := lightStylesChanged(r.worldLightStyleValues, values)
	if !anyLightStyleChanged(changed) {
		return
	}
	if r.worldData != nil && r.worldData.Geometry != nil {
		markDirtyLightmapPages(r.worldData.Geometry.Lightmaps, changed)
		updateUploadedLightmapsLocked(queue, r.worldLightmapPages, r.worldData.Geometry.Lightmaps, values)
	}
	for submodelIndex, geom := range r.brushModelGeometry {
		if geom == nil || len(geom.Lightmaps) == 0 {
			continue
		}
		markDirtyLightmapPages(geom.Lightmaps, changed)
		updateUploadedLightmapsLocked(queue, r.brushModelLightmaps[submodelIndex], geom.Lightmaps, values)
	}
	r.worldLightStyleValues = values
}

func defaultWorldLightStyleValues() [64]float32 {
	var values [64]float32
	values[0] = 1
	return values
}

func worldLightstyleScale(values [64]float32, style uint8) float32 {
	if int(style) >= len(values) {
		return 0
	}
	return values[style]
}

func compositeWorldLightmapSurfaceRGBA(rgba []byte, pageWidth int, surface WorldLightmapSurface, values [64]float32) {
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
				scale := worldLightstyleScale(values, surface.Styles[styleIndex])
				rSum += float32(surface.Samples[offset]) * scale
				gSum += float32(surface.Samples[offset+1]) * scale
				bSum += float32(surface.Samples[offset+2]) * scale
			}
			dst := ((surface.Y+y)*pageWidth + (surface.X + x)) * 4
			rgba[dst] = byte(clamp01(rSum/255.0) * 255)
			rgba[dst+1] = byte(clamp01(gSum/255.0) * 255)
			rgba[dst+2] = byte(clamp01(bSum/255.0) * 255)
			rgba[dst+3] = 255
		}
	}
}

func buildWorldLightmapPageRGBA(page *WorldLightmapPage, values [64]float32) []byte {
	if page.Width <= 0 || page.Height <= 0 {
		return nil
	}
	size := page.Width * page.Height * 4
	if len(page.CachedRGBA) != size {
		page.CachedRGBA = make([]byte, size)
		for i := 0; i < len(page.CachedRGBA); i += 4 {
			page.CachedRGBA[i+3] = 255
		}
		for _, surface := range page.Surfaces {
			compositeWorldLightmapSurfaceRGBA(page.CachedRGBA, page.Width, surface, values)
		}
		return page.CachedRGBA
	}
	if page.Dirty {
		recompositeDirtySurfaces(page.CachedRGBA, *page, values)
	}
	return page.CachedRGBA
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

func anyLightStyleChanged(changed [64]bool) bool {
	for _, dirty := range changed {
		if dirty {
			return true
		}
	}
	return false
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

func recompositeDirtySurfaces(rgba []byte, page WorldLightmapPage, values [64]float32) bool {
	recomposited := false
	for _, surface := range page.Surfaces {
		if !surface.Dirty {
			continue
		}
		compositeWorldLightmapSurfaceRGBA(rgba, page.Width, surface, values)
		recomposited = true
	}
	return recomposited
}

func worldLiquidAlphaSettingsForGeometry(geom *WorldGeometry) worldLiquidAlphaSettings {
	if geom == nil {
		return worldLiquidAlphaSettingsFromCvars(worldLiquidAlphaOverrides{}, nil)
	}
	settings := resolveWorldLiquidAlphaSettings(
		worldimpl.ReadAlphaCvar(CvarRWaterAlpha, 1),
		worldimpl.ReadAlphaCvar(CvarRLavaAlpha, 0),
		worldimpl.ReadAlphaCvar(CvarRSlimeAlpha, 0),
		worldimpl.ReadAlphaCvar(CvarRTeleAlpha, 0),
		worldLiquidAlphaOverridesFromWorld(geom.LiquidAlphaOverrides),
		nil,
	)
	if !geom.TransparentWaterSafe {
		settings.water = 1
		settings.lava = 1
		settings.slime = 1
		settings.tele = 1
	}
	return settings
}

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

	textureMinU := math.Floor(minU/16.0) * 16.0
	textureMinV := math.Floor(minV/16.0) * 16.0
	extentU := int(math.Ceil(maxU/16.0)*16.0 - textureMinU)
	extentV := int(math.Ceil(maxV/16.0)*16.0 - textureMinV)
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
		lightS := float32((rawCoords[i][0]-textureMinU)/16.0 + float64(x) + 0.5)
		lightT := float32((rawCoords[i][1]-textureMinV)/16.0 + float64(y) + 0.5)
		vertices[i].LightmapCoord = [2]float32{
			lightS / float32(worldLightmapPageSize),
			lightT / float32(worldLightmapPageSize),
		}
	}

	return &faceLightmapSurface{pageIndex: texNum}, nil
}

func worldTexCoordDouble(position [3]float32, vec [4]float32) float64 {
	return float64(position[0])*float64(vec[0]) +
		float64(position[1])*float64(vec[1]) +
		float64(position[2])*float64(vec[2]) +
		float64(vec[3])
}

// Helper functions to convert Go types to byte slices
func float32ToBytes(f []float32) []byte {
	result := make([]byte, len(f)*4)
	for i, v := range f {
		binary.LittleEndian.PutUint32(result[i*4:i*4+4], math.Float32bits(v))
	}
	return result
}

// uint32ToBytes expands packed integer data into byte form for uploads to APIs expecting byte-addressable buffers/textures.
func uint32ToBytes(u uint32) []byte {
	result := make([]byte, 4)
	binary.LittleEndian.PutUint32(result, u)
	return result
}

func uint32SliceToBytes(values []uint32) []byte {
	if len(values) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(values))), len(values)*4)
}

// UploadWorld prepares BSP world geometry for rendering.
// This should be called once when a map is loaded.
//
// Uploads vertex and index buffers to GPU, compiles shaders,
// creates the render pipeline, and prepares for rendering.
func (r *Renderer) UploadWorld(tree *bsp.Tree) error {
	if tree == nil {
		return fmt.Errorf("nil BSP tree")
	}
	r.worldFirstFrameStatsLogged.Store(false)
	r.mu.Lock()
	r.brushModelGeometry = make(map[int]*WorldGeometry)
	r.resetGoGPUWorldBatchCache()
	r.mu.Unlock()

	slog.Debug("Uploading world geometry to GPU")

	// Build geometry from BSP
	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		return fmt.Errorf("build world geometry: %w", err)
	}
	liquidAlpha := worldLiquidAlphaSettingsForGeometry(geom)
	faceStats := summarizeGoGPUWorldFaceStats(geom.Faces, liquidAlpha)

	// Create render data
	renderData := &WorldRenderData{
		Geometry:      geom,
		TotalVertices: len(geom.Vertices),
		TotalIndices:  len(geom.Indices),
		TotalFaces:    len(geom.Faces),
	}

	if len(geom.Vertices) > 0 {
		boundsMin := geom.Vertices[0].Position
		boundsMax := geom.Vertices[0].Position
		for index := 1; index < len(geom.Vertices); index++ {
			position := geom.Vertices[index].Position
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
	}

	// Get HAL device and queue from gogpu renderer
	device := r.getWGPUDevice()
	queue := r.getWGPUQueue()
	if device == nil || queue == nil {
		slog.Warn("HAL device or queue not available, skipping GPU upload")
		// Store geometry anyway for later use
		r.mu.Lock()
		r.worldData = renderData
		r.mu.Unlock()
		slog.Info("GoGPU world upload stats",
			"gpu_upload", false,
			"bsp_version", tree.Version,
			"lighting_rgb", tree.LightingRGB,
			"raw_faces", len(tree.Faces),
			"built_faces", faceStats.TotalFaces,
			"built_triangles", faceStats.TotalTriangles,
			"vertices", renderData.TotalVertices,
			"lightmap_pages", len(geom.Lightmaps),
			"lightmapped_faces", faceStats.LightmappedFaces,
			"lit_water_faces", faceStats.LitWaterFaces,
			"turbulent_faces", faceStats.TurbulentFaces,
			"sky_faces", faceStats.SkyFaces,
			"opaque_faces", faceStats.OpaqueFaces,
			"alpha_test_faces", faceStats.AlphaTestFaces,
			"opaque_liquid_faces", faceStats.OpaqueLiquidFaces,
			"translucent_liquid_faces", faceStats.TranslucentLiquidFaces,
			"textures", tree.NumTextures,
			"leafs", len(tree.Leafs),
			"models", len(tree.Models),
		)
		return nil
	}

	// Upload vertex buffer
	vertexBuffer, err := r.createWorldVertexBuffer(device, queue, geom.Vertices)
	if err != nil {
		return fmt.Errorf("upload vertex buffer: %w", err)
	}

	// Upload index buffer
	indexBuffer, indexCount, err := r.createWorldIndexBuffer(device, queue, geom.Indices)
	if err != nil {
		return fmt.Errorf("upload index buffer: %w", err)
	}

	// Create shader modules (WGSL is compiled by HAL internally)
	vertexShader, err := createWorldShaderModule(device, worldVertexShaderWGSL, "World Vertex Shader")
	if err != nil {
		slog.Warn("Failed to create vertex shader", "error", err)
		vertexShader = nil
	}

	fragmentShader, err := createWorldShaderModule(device, worldFragmentShaderWGSL, "World Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create fragment shader", "error", err)
		fragmentShader = nil
	}
	alphaTestFragmentShader, err := createWorldShaderModule(device, worldAlphaTestFragmentShaderWGSL, "World Alpha Test Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create alpha-test fragment shader", "error", err)
		alphaTestFragmentShader = nil
	}
	skyVertexShader, err := createWorldShaderModule(device, worldSkyVertexShaderWGSL, "World Sky Vertex Shader")
	if err != nil {
		slog.Warn("Failed to create sky vertex shader", "error", err)
		skyVertexShader = nil
	}
	skyFragmentShader, err := createWorldShaderModule(device, worldSkyFragmentShaderWGSL, "World Sky Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create sky fragment shader", "error", err)
		skyFragmentShader = nil
	}
	externalSkyFragmentShader, err := createWorldShaderModule(device, worldSkyExternalFaceFragmentShaderWGSL, "World External Sky Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create external sky fragment shader", "error", err)
		externalSkyFragmentShader = nil
	}
	turbulentFragmentShader, err := createWorldShaderModule(device, worldTurbulentFragmentShaderWGSL, "World Turbulent Fragment Shader")
	if err != nil {
		slog.Warn("Failed to create turbulent fragment shader", "error", err)
		turbulentFragmentShader = nil
	}

	// Create render pipeline (may fail if gogpu API not fully exposed)
	var pipeline *wgpu.RenderPipeline
	var pipelineLayout *wgpu.PipelineLayout
	var skyPipeline *wgpu.RenderPipeline
	var externalSkyPipeline *wgpu.RenderPipeline
	var externalSkyPipelineLayout *wgpu.PipelineLayout
	var externalSkyBindGroupLayout *wgpu.BindGroupLayout
	var alphaTestPipeline *wgpu.RenderPipeline
	var translucentPipeline *wgpu.RenderPipeline
	var turbulentPipeline *wgpu.RenderPipeline
	var translucentTurbulentPipeline *wgpu.RenderPipeline
	if vertexShader != nil && fragmentShader != nil {
		var err2 error
		pipeline, pipelineLayout, err2 = r.createWorldPipeline(device, vertexShader, fragmentShader)
		if err2 != nil {
			slog.Warn("Failed to create render pipeline", "error", err2)
		}
	}
	if pipelineLayout != nil && skyVertexShader != nil && skyFragmentShader != nil {
		skyPipeline, err = r.createWorldSkyPipeline(device, skyVertexShader, skyFragmentShader, pipelineLayout)
		if err != nil {
			slog.Warn("Failed to create world sky pipeline", "error", err)
			skyPipeline = nil
		}
	}
	if pipelineLayout != nil && vertexShader != nil && alphaTestFragmentShader != nil {
		alphaTestPipeline, err = r.createWorldOpaquePipeline(device, vertexShader, alphaTestFragmentShader, pipelineLayout)
		if err != nil {
			slog.Warn("Failed to create world alpha-test pipeline", "error", err)
			alphaTestPipeline = nil
		}
	}
	if skyVertexShader != nil && externalSkyFragmentShader != nil {
		externalSkyPipeline, externalSkyPipelineLayout, externalSkyBindGroupLayout, err = r.createWorldExternalSkyPipeline(device, skyVertexShader, externalSkyFragmentShader)
		if err != nil {
			slog.Warn("Failed to create external world sky pipeline", "error", err)
			externalSkyPipeline = nil
			externalSkyPipelineLayout = nil
			externalSkyBindGroupLayout = nil
		}
	}
	if pipelineLayout != nil && vertexShader != nil && fragmentShader != nil {
		translucentPipeline, err = r.createWorldTranslucentPipeline(device, vertexShader, fragmentShader, pipelineLayout)
		if err != nil {
			slog.Warn("Failed to create world translucent pipeline", "error", err)
			translucentPipeline = nil
		}
	}
	if pipelineLayout != nil && vertexShader != nil && turbulentFragmentShader != nil {
		turbulentPipeline, err = r.createWorldTurbulentPipeline(device, vertexShader, turbulentFragmentShader, pipelineLayout)
		if err != nil {
			slog.Warn("Failed to create world turbulent pipeline", "error", err)
			turbulentPipeline = nil
		}
		translucentTurbulentPipeline, err = r.createWorldTranslucentTurbulentPipeline(device, vertexShader, turbulentFragmentShader, pipelineLayout)
		if err != nil {
			slog.Warn("Failed to create world translucent turbulent pipeline", "error", err)
			translucentTurbulentPipeline = nil
		}
	}

	// Create uniform buffer for VP matrix
	uniformBuffer, err := device.CreateBuffer(&wgpu.BufferDescriptor{
		Label:            "World Uniforms",
		Size:             worldUniformBufferSize,
		Usage:            gputypes.BufferUsageUniform | gputypes.BufferUsageCopyDst,
		MappedAtCreation: false,
	})
	if err != nil {
		return fmt.Errorf("create uniform buffer: %w", err)
	}

	// Create bind group for world uniform buffer.
	uniformLayout := r.uniformBindGroupLayout
	if uniformLayout != nil {
		uniformBindGroup, bindErr := device.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Label:  "World Uniform BG",
			Layout: uniformLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, Buffer: uniformBuffer, Offset: 0, Size: worldUniformBufferSize},
			},
		})
		if bindErr != nil {
			slog.Warn("Failed to create world uniform bind group", "error", bindErr)
		} else {
			r.uniformBindGroup = uniformBindGroup
			r.worldBindGroup = uniformBindGroup
		}
	}

	// Create white texture for fallback
	whiteTexture, whiteTextureView, err := r.createWorldWhiteTexture(device, queue)
	if err != nil {
		slog.Warn("Failed to create white texture", "error", err)
		// Don't fail completely, will use fallback rendering
	}
	var worldTextureSampler *wgpu.Sampler
	var whiteTextureBindGroup *wgpu.BindGroup
	var transparentTexture *wgpu.Texture
	var transparentTextureView *wgpu.TextureView
	var transparentBindGroup *wgpu.BindGroup
	if r.textureBindGroupLayout != nil {
		worldTextureSampler, err = r.createWorldTextureSampler(device)
		if err != nil {
			slog.Warn("Failed to create world texture sampler", "error", err)
		} else if whiteTextureView != nil {
			whiteTextureBindGroup, err = r.createWorldTextureBindGroup(device, worldTextureSampler, whiteTextureView)
			if err != nil {
				slog.Warn("Failed to create white world texture bind group", "error", err)
			}
		}
		if worldTextureSampler != nil {
			transparentTextureResource, transparentViewResource, transparentErr := r.createWorldWhiteTexture(device, queue)
			if transparentErr != nil {
				slog.Warn("Failed to create transparent fallback texture", "error", transparentErr)
			} else {
				transparentTexture = transparentTextureResource
				transparentTextureView = transparentViewResource
				if queueErr := queue.WriteTexture(&wgpu.ImageCopyTexture{
					Texture:  transparentTexture,
					MipLevel: 0,
					Aspect:   gputypes.TextureAspectAll,
				}, []byte{0, 0, 0, 0}, &wgpu.ImageDataLayout{BytesPerRow: 4, RowsPerImage: 1}, &wgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1}); queueErr != nil {
					slog.Warn("Failed to zero transparent fallback texture", "error", queueErr)
				} else {
					transparentBindGroup, err = r.createWorldTextureBindGroup(device, worldTextureSampler, transparentTextureView)
					if err != nil {
						slog.Warn("Failed to create transparent world texture bind group", "error", err)
					}
				}
			}
		}
	}
	worldTextures, worldFullbrightTextures, worldTextureAnimations := r.uploadWorldMaterialTextures(device, queue, worldTextureSampler, tree)
	worldSkySolidTextures, worldSkyAlphaTextures := r.uploadWorldEmbeddedSkyTextures(device, queue, worldTextureSampler, tree)
	lightstyleValues := defaultWorldLightStyleValues()
	var worldLightmapSampler *wgpu.Sampler
	var whiteLightmapBindGroup *wgpu.BindGroup
	if r.textureBindGroupLayout != nil {
		worldLightmapSampler, err = r.createWorldLightmapSampler(device)
		if err != nil {
			slog.Warn("Failed to create world lightmap sampler", "error", err)
		} else if whiteTextureView != nil {
			whiteLightmapBindGroup, err = r.createWorldTextureBindGroup(device, worldLightmapSampler, whiteTextureView)
			if err != nil {
				slog.Warn("Failed to create white world lightmap bind group", "error", err)
			}
		}
	}
	worldLightmapPages := r.uploadWorldLightmapPages(device, queue, worldLightmapSampler, geom.Lightmaps, lightstyleValues)

	// Create offscreen render target for world rendering
	if err := r.createWorldRenderTarget(); err != nil {
		slog.Warn("Failed to create world render target", "error", err)
		// Don't fail completely, will use direct rendering fallback
	}

	width, height := r.Size()
	var depthTexture *wgpu.Texture
	var depthTextureView *wgpu.TextureView
	if width > 0 && height > 0 {
		depthTexture, depthTextureView, err = r.createWorldDepthTexture(device, width, height)
		if err != nil {
			slog.Warn("Failed to create world depth texture", "error", err)
		}
	}

	// Store GPU resources in renderer
	r.mu.Lock()
	r.worldData = renderData
	r.worldVertexBuffer = vertexBuffer
	r.worldIndexBuffer = indexBuffer
	r.worldIndexCount = indexCount
	r.worldPipeline = pipeline
	r.worldAlphaTestPipeline = alphaTestPipeline
	r.worldTranslucentPipeline = translucentPipeline
	r.worldTurbulentPipeline = turbulentPipeline
	r.worldTranslucentTurbulentPipeline = translucentTurbulentPipeline
	r.worldSkyPipeline = skyPipeline
	r.worldSkyExternalPipeline = externalSkyPipeline
	r.worldPipelineLayout = pipelineLayout
	r.worldSkyExternalPipelineLayout = externalSkyPipelineLayout
	r.worldShader = vertexShader
	r.uniformBuffer = uniformBuffer
	r.whiteTexture = whiteTexture
	r.whiteTextureView = whiteTextureView
	r.worldTextureSampler = worldTextureSampler
	r.worldTextures = worldTextures
	r.worldFullbrightTextures = worldFullbrightTextures
	r.worldSkySolidTextures = worldSkySolidTextures
	r.worldSkyAlphaTextures = worldSkyAlphaTextures
	r.worldTextureAnimations = worldTextureAnimations
	r.worldSkyExternalBindGroupLayout = externalSkyBindGroupLayout
	r.whiteTextureBindGroup = whiteTextureBindGroup
	r.transparentTexture = transparentTexture
	r.transparentTextureView = transparentTextureView
	r.transparentBindGroup = transparentBindGroup
	r.worldLightmapSampler = worldLightmapSampler
	r.worldLightmapPages = worldLightmapPages
	r.whiteLightmapBindGroup = whiteLightmapBindGroup
	r.worldLightStyleValues = lightstyleValues
	r.worldDepthTexture = depthTexture
	r.worldDepthTextureView = depthTextureView
	if depthTexture != nil {
		r.worldDepthWidth = width
		r.worldDepthHeight = height
	}
	if err := r.ensureGoGPUExternalSkyboxLocked(device, queue); err != nil && r.worldSkyExternalMode == externalSkyboxRenderFaces {
		slog.Debug("external gogpu skybox remains deferred", "name", r.worldSkyExternalName, "error", err)
	}
	renderData.VertexBufferUploaded = vertexBuffer != nil
	renderData.IndexBufferUploaded = indexBuffer != nil
	renderData.HasDiffuseTextures = len(worldTextures) > 0
	renderData.HasLightmapTextures = len(worldLightmapPages) > 0
	renderData.HasDepthBuffer = depthTextureView != nil
	r.mu.Unlock()

	slog.Debug("World geometry uploaded to GPU",
		"vertices", renderData.TotalVertices,
		"indices", renderData.TotalIndices,
		"faces", renderData.TotalFaces,
		"triangles", renderData.TotalIndices/3,
		"boundsMin", renderData.BoundsMin,
		"boundsMax", renderData.BoundsMax,
		"vertexBufferSize", uint64(len(geom.Vertices))*44,
		"indexBufferSize", uint64(len(geom.Indices))*4)
	slog.Info("GoGPU world upload stats",
		"gpu_upload", true,
		"bsp_version", tree.Version,
		"lighting_rgb", tree.LightingRGB,
		"raw_faces", len(tree.Faces),
		"built_faces", faceStats.TotalFaces,
		"built_triangles", faceStats.TotalTriangles,
		"vertices", renderData.TotalVertices,
		"lightmap_pages", len(geom.Lightmaps),
		"gpu_lightmap_pages", len(worldLightmapPages),
		"lightmapped_faces", faceStats.LightmappedFaces,
		"lit_water_faces", faceStats.LitWaterFaces,
		"turbulent_faces", faceStats.TurbulentFaces,
		"sky_faces", faceStats.SkyFaces,
		"opaque_faces", faceStats.OpaqueFaces,
		"alpha_test_faces", faceStats.AlphaTestFaces,
		"opaque_liquid_faces", faceStats.OpaqueLiquidFaces,
		"translucent_liquid_faces", faceStats.TranslucentLiquidFaces,
		"textures", tree.NumTextures,
		"gpu_textures", len(worldTextures),
		"gpu_fullbright_textures", len(worldFullbrightTextures),
		"gpu_sky_solid_textures", len(worldSkySolidTextures),
		"gpu_sky_alpha_textures", len(worldSkyAlphaTextures),
		"leafs", len(tree.Leafs),
		"models", len(tree.Models),
	)

	return nil
}

// renderWorldInternal implements world rendering.
// This records render commands to draw the world geometry with the configured pipeline,
// shaders, textures, and matrices.
func (dc *DrawContext) renderWorldInternal(state *RenderFrameState) {
	worldData := dc.renderer.GetWorldData()
	if worldData == nil || worldData.Geometry == nil {
		slog.Debug("renderWorldInternal: no world data")
		return
	}
	hostSpeeds := cvar.BoolValue("host_speeds")
	var (
		visibleSelectMS float64
		classifyFacesMS float64
		batchBuildMS    float64
		batchUploadMS   float64
		skyDrawMS       float64
		opaqueDrawMS    float64
		submitMS        float64
	)

	slog.Debug("renderWorldInternal: starting world render")

	// Ensure depth texture matches current surface dimensions (handles window resize).
	// Must happen before the RLock below since ensureAliasDepthTextureLocked needs a write lock.
	device := dc.renderer.getWGPUDevice()
	if device != nil {
		dc.renderer.mu.Lock()
		dc.renderer.ensureAliasDepthTextureLocked(device)
		dc.renderer.mu.Unlock()
	}

	dc.renderer.mu.RLock()
	defer dc.renderer.mu.RUnlock()

	// Check if GPU resources are ready
	if dc.renderer.worldVertexBuffer == nil || dc.renderer.worldIndexBuffer == nil {
		if worldData.TotalFaces > 0 {
			slog.Debug("renderWorldInternal: World GPU buffers not ready",
				"faces", worldData.TotalFaces,
				"triangles", worldData.TotalIndices/3)
		}
		return
	}

	if dc.renderer.worldPipeline == nil {
		slog.Debug("renderWorldInternal: World pipeline not ready")
		return
	}

	// Get HAL device and queue (device already fetched above, just need queue)
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		slog.Debug("renderWorldInternal: HAL device or queue not available for world rendering")
		return
	}

	// Create command encoder
	slog.Debug("renderWorldInternal: creating command encoder")
	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{
		Label: "World Render Command Encoder",
	})
	if err != nil {
		slog.Error("renderWorldInternal: Failed to create command encoder", "error", err)
		return
	}

	slog.Debug("renderWorldInternal: command encoder started")

	// Use the current surface view for zero-copy rendering (per gogpu design)
	// This allows HAL to render directly to the same surface that gogpu will composite onto
	slog.Debug("renderWorldInternal: getting surface view from gogpu context")
	textureView := dc.currentWGPURenderTargetView()
	if textureView == nil {
		slog.Debug("renderWorldInternal: Render target view not available, skipping world rendering")
		return
	}
	slog.Debug("renderWorldInternal: render target view acquired", "view_type", fmt.Sprintf("%T", textureView), "queue_type", fmt.Sprintf("%T", queue))

	// Create render pass descriptor with color and depth attachments.
	// Use LoadOpClear to handle the clear ourselves since we skip gogpu's Clear().
	clearColor := gogpuWorldClearColor(state.ClearColor)
	slog.Debug("renderWorldInternal: creating render pass descriptor")
	renderPassDesc := &wgpu.RenderPassDescriptor{
		Label: "World Render Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:       textureView,
				LoadOp:     gputypes.LoadOpClear,
				StoreOp:    gputypes.StoreOpStore,
				ClearValue: clearColor,
			},
		},
		DepthStencilAttachment: worldDepthAttachmentForView(dc.renderer.worldDepthTextureView),
	}

	// Begin render pass
	slog.Debug("renderWorldInternal: beginning render pass")
	renderPass, err := encoder.BeginRenderPass(renderPassDesc)
	if err != nil {
		slog.Error("renderWorldInternal: Failed to begin render pass", "error", err)
		return
	}
	slog.Debug("renderWorldInternal: render pass created", "pass", fmt.Sprintf("%T", renderPass))

	// Set pipeline
	slog.Debug("renderWorldInternal: setting pipeline", "pipeline", fmt.Sprintf("%T", dc.renderer.worldPipeline))
	renderPass.SetPipeline(dc.renderer.worldPipeline)

	// Explicit viewport/scissor to avoid backend defaults that can yield zero-area rasterization.
	w, h := dc.renderer.Size()
	if w > 0 && h > 0 {
		slog.Debug("renderWorldInternal: setting viewport", "x", 0, "y", 0, "w", w, "h", h)
		renderPass.SetViewport(0, 0, float32(w), float32(h), 0.0, 1.0)
		renderPass.SetScissorRect(0, 0, uint32(w), uint32(h))
	} else {
		slog.Warn("renderWorldInternal: invalid viewport size", "w", w, "h", h)
	}

	// Update uniform buffer with VP matrix
	vpMatrix := dc.renderer.GetViewProjectionMatrix()
	camera := dc.renderer.cameraState
	cameraOrigin, fogDensity, timeValue := gogpuWorldUniformInputs(state, camera)
	var currentDynamicLight [3]float32
	currentLitWater := float32(0)
	var uniformBytes [worldUniformBufferSize]byte
	fillWorldSceneUniformBytes(uniformBytes[:], vpMatrix, cameraOrigin, state.FogColor, fogDensity, timeValue, 1, currentDynamicLight, currentLitWater)
	slog.Debug("renderWorldInternal: VP matrix",
		"m00", vpMatrix[0], "m11", vpMatrix[5], "m22", vpMatrix[10], "m33", vpMatrix[15])
	slog.Debug("renderWorldInternal: writing uniform buffer", "bytes_len", len(uniformBytes))
	err = queue.WriteBuffer(dc.renderer.uniformBuffer, 0, uniformBytes[:])
	if err != nil {
		slog.Error("renderWorldInternal: Failed to update uniform buffer", "error", err)
		renderPass.End()
		return
	}

	// Set vertex buffer
	slog.Debug("renderWorldInternal: setting vertex buffer", "buffer", fmt.Sprintf("%T", dc.renderer.worldVertexBuffer))
	renderPass.SetVertexBuffer(0, dc.renderer.worldVertexBuffer, 0)

	// Set index buffer (uint32 format for indices)
	slog.Debug("renderWorldInternal: setting index buffer", "buffer", fmt.Sprintf("%T", dc.renderer.worldIndexBuffer), "count", dc.renderer.worldIndexCount)
	renderPass.SetIndexBuffer(dc.renderer.worldIndexBuffer, gputypes.IndexFormatUint32, 0)

	// Set uniform bind group.
	if dc.renderer.uniformBindGroup != nil {
		slog.Debug("renderWorldInternal: setting bind group", "group", fmt.Sprintf("%T", dc.renderer.uniformBindGroup))
		renderPass.SetBindGroup(0, dc.renderer.uniformBindGroup, nil)
	} else {
		slog.Warn("renderWorldInternal: NO uniform bind group set")
	}

	if dc.renderer.whiteTextureBindGroup == nil || dc.renderer.whiteLightmapBindGroup == nil {
		slog.Warn("renderWorldInternal: no world texture/lightmap bind group available")
		renderPass.End()
		return
	}
	timeSeconds := float64(camera.Time)
	liquidAlpha := worldLiquidAlphaSettingsForGeometry(worldData.Geometry)
	worldHasLitWater := worldData.Geometry.HasLitWater
	skyFogDensity := gogpuWorldSkyFogDensity(worldData.Geometry.Tree.Entities, fogDensity)
	var activeDynamicLights []DynamicLight
	dc.renderer.mu.RLock()
	if dc.renderer.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, dc.renderer.lightPool.ActiveLights()...)
	}
	dc.renderer.mu.RUnlock()
	currentAlpha := float32(1)
	currentFogDensity := fogDensity
	writeWorldUniformWithFog := func(alpha float32, dynamicLight [3]float32, litWater float32, activeFogDensity float32) bool {
		if currentAlpha == alpha && currentDynamicLight == dynamicLight && currentLitWater == litWater && currentFogDensity == activeFogDensity {
			return true
		}
		currentAlpha = alpha
		currentDynamicLight = dynamicLight
		currentLitWater = litWater
		currentFogDensity = activeFogDensity
		fillWorldSceneUniformBytes(uniformBytes[:], vpMatrix, cameraOrigin, state.FogColor, activeFogDensity, timeValue, alpha, dynamicLight, litWater)
		return queue.WriteBuffer(dc.renderer.uniformBuffer, 0, uniformBytes[:]) == nil
	}
	writeWorldUniform := func(alpha float32, dynamicLight [3]float32, litWater float32) bool {
		return writeWorldUniformWithFog(alpha, dynamicLight, litWater, fogDensity)
	}
	cameraOriginWorld := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	cameraLeafIndex := worldLeafIndex(worldData.Geometry.Tree, cameraOriginWorld)
	dynamicLightSig := gogpuWorldDynamicLightSignature(activeDynamicLights)
	cacheEntry := dc.renderer.gogpuWorldBatchCacheEntry(cameraLeafIndex, dynamicLightSig)
	cacheHit := cacheEntry != nil
	visibleFaceCount := 0
	var skyFaces []WorldFace
	var translucentLiquidFaces []WorldFace
	var batchedIndices []uint32
	var opaqueBatches []gogpuWorldFaceBatch
	var alphaTestBatches []gogpuWorldFaceBatch
	var opaqueLiquidBatches []gogpuWorldFaceBatch
	if cacheHit {
		visibleFaceCount = cacheEntry.faceCount
		skyFaces = cacheEntry.skyFaces
		translucentLiquidFaces = cacheEntry.translucentLiquid
		batchedIndices = cacheEntry.indices
		opaqueBatches = cacheEntry.opaque
		alphaTestBatches = cacheEntry.alpha
		opaqueLiquidBatches = cacheEntry.liquid
	} else {
		selectStart := time.Now()
		visibleFaces := dc.renderer.worldVisibleFacesScratch.selectVisibleWorldFaces(
			worldData.Geometry.Tree,
			worldData.Geometry.Faces,
			worldData.Geometry.LeafFaces,
			cameraOriginWorld,
		)
		visibleSelectMS = float64(time.Since(selectStart)) / float64(time.Millisecond)
		visibleFaceCount = len(visibleFaces)
		skyFaces = dc.renderer.worldSkyFacesScratch[:0]
		translucentLiquidFaces = dc.renderer.worldTranslucentLiquidScratch[:0]
		opaqueDraws := dc.renderer.worldOpaqueDrawsScratch[:0]
		alphaTestDraws := dc.renderer.worldAlphaDrawsScratch[:0]
		opaqueLiquidDraws := dc.renderer.worldLiquidDrawsScratch[:0]
		batchedIndices = dc.renderer.worldBatchedIndexScratch[:0]
		opaqueBatches = dc.renderer.worldOpaqueBatchScratch[:0]
		alphaTestBatches = dc.renderer.worldAlphaBatchScratch[:0]
		opaqueLiquidBatches = dc.renderer.worldLiquidBatchScratch[:0]
		defer func() {
			dc.renderer.worldSkyFacesScratch = skyFaces[:0]
			dc.renderer.worldTranslucentLiquidScratch = translucentLiquidFaces[:0]
			dc.renderer.worldOpaqueDrawsScratch = opaqueDraws[:0]
			dc.renderer.worldAlphaDrawsScratch = alphaTestDraws[:0]
			dc.renderer.worldLiquidDrawsScratch = opaqueLiquidDraws[:0]
			dc.renderer.worldBatchedIndexScratch = batchedIndices[:0]
			dc.renderer.worldOpaqueBatchScratch = opaqueBatches[:0]
			dc.renderer.worldAlphaBatchScratch = alphaTestBatches[:0]
			dc.renderer.worldLiquidBatchScratch = opaqueLiquidBatches[:0]
		}()
		classifyStart := time.Now()
		for _, face := range visibleFaces {
			switch {
			case shouldDrawGoGPUSkyWorldFace(face):
				skyFaces = append(skyFaces, face)
			case shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha):
				translucentLiquidFaces = append(translucentLiquidFaces, face)
			case shouldDrawGoGPUOpaqueWorldFace(face), shouldDrawGoGPUAlphaTestWorldFace(face), shouldDrawGoGPUOpaqueLiquidFace(face, liquidAlpha):
				textureBindGroup := dc.renderer.whiteTextureBindGroup
				if worldTexture := gogpuWorldTextureForFace(face, dc.renderer.worldTextures, dc.renderer.worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
					textureBindGroup = worldTexture.bindGroup
				}
				lightmapBindGroup := dc.renderer.whiteLightmapBindGroup
				litWater := float32(0)
				if shouldDrawGoGPUOpaqueLiquidFace(face, liquidAlpha) {
					lightmapBindGroup, litWater = gogpuWorldLightmapBindGroupForFace(face, dc.renderer.worldLightmapPages, dc.renderer.whiteLightmapBindGroup, worldHasLitWater)
				} else if face.LightmapIndex >= 0 && int(face.LightmapIndex) < len(dc.renderer.worldLightmapPages) {
					if lightmapPage := dc.renderer.worldLightmapPages[face.LightmapIndex]; lightmapPage != nil && lightmapPage.bindGroup != nil {
						lightmapBindGroup = lightmapPage.bindGroup
					}
				}
				fullbrightBindGroup := dc.renderer.transparentBindGroup
				if fullbrightBindGroup == nil {
					fullbrightBindGroup = dc.renderer.whiteTextureBindGroup
				}
				if worldTexture := gogpuWorldTextureForFace(face, dc.renderer.worldFullbrightTextures, dc.renderer.worldTextureAnimations, nil, 0, timeSeconds); worldTexture != nil && worldTexture.bindGroup != nil {
					fullbrightBindGroup = worldTexture.bindGroup
				}
				draw := gogpuWorldFaceDraw{
					face:                face,
					textureBindGroup:    textureBindGroup,
					lightmapBindGroup:   lightmapBindGroup,
					fullbrightBindGroup: fullbrightBindGroup,
					dynamicLight:        quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(activeDynamicLights, face.Center)),
					litWater:            litWater,
				}
				switch {
				case shouldDrawGoGPUOpaqueWorldFace(face):
					opaqueDraws = append(opaqueDraws, draw)
				case shouldDrawGoGPUAlphaTestWorldFace(face):
					alphaTestDraws = append(alphaTestDraws, draw)
				default:
					opaqueLiquidDraws = append(opaqueLiquidDraws, draw)
				}
			}
		}
		classifyFacesMS = float64(time.Since(classifyStart)) / float64(time.Millisecond)
		batchBuildStart := time.Now()
		batchedIndices, opaqueBatches = appendGoGPUOpaqueWorldFaceBatches(batchedIndices, opaqueBatches, opaqueDraws, worldData.Geometry.Indices)
		batchedIndices, alphaTestBatches = appendGoGPUOpaqueWorldFaceBatches(batchedIndices, alphaTestBatches, alphaTestDraws, worldData.Geometry.Indices)
		batchedIndices, opaqueLiquidBatches = appendGoGPUOpaqueWorldFaceBatches(batchedIndices, opaqueLiquidBatches, opaqueLiquidDraws, worldData.Geometry.Indices)
		batchBuildMS = float64(time.Since(batchBuildStart)) / float64(time.Millisecond)
		dc.renderer.storeGoGPUWorldBatchCacheEntry(cameraLeafIndex, dynamicLightSig, visibleFaceCount, skyFaces, translucentLiquidFaces, batchedIndices, opaqueBatches, alphaTestBatches, opaqueLiquidBatches)
	}
	var opaqueBatchBuffer *wgpu.Buffer
	if len(batchedIndices) > 0 {
		batchUploadStart := time.Now()
		opaqueBatchBuffer, err = dc.renderer.ensureGoGPUWorldDynamicIndexBuffer(device, uint64(len(batchedIndices))*4)
		if err != nil {
			slog.Error("renderWorldInternal: Failed to allocate batched world index buffer", "error", err)
			renderPass.End()
			return
		}
		if err := queue.WriteBuffer(opaqueBatchBuffer, 0, uint32SliceToBytes(batchedIndices)); err != nil {
			slog.Error("renderWorldInternal: Failed to upload batched world indices", "error", err)
			renderPass.End()
			return
		}
		batchUploadMS = float64(time.Since(batchUploadStart)) / float64(time.Millisecond)
	}

	skyDrawnIndices := uint32(0)
	var materialBindState gogpuWorldMaterialBindState
	skyDrawStart := time.Now()
	if dc.renderer.worldSkyExternalMode == externalSkyboxRenderFaces && dc.renderer.worldSkyExternalPipeline != nil && dc.renderer.worldSkyExternalBindGroup != nil {
		if !writeWorldUniformWithFog(1, [3]float32{}, 0, skyFogDensity) {
			slog.Error("renderWorldInternal: Failed to update sky fog uniform")
			renderPass.End()
			return
		}
		renderPass.SetPipeline(dc.renderer.worldSkyExternalPipeline)
		renderPass.SetBindGroup(1, dc.renderer.worldSkyExternalBindGroup, nil)
		for _, face := range skyFaces {
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
			skyDrawnIndices += face.NumIndices
		}
	} else if dc.renderer.worldSkyPipeline != nil {
		if !writeWorldUniformWithFog(1, [3]float32{}, 0, skyFogDensity) {
			slog.Error("renderWorldInternal: Failed to update sky fog uniform")
			renderPass.End()
			return
		}
		renderPass.SetPipeline(dc.renderer.worldSkyPipeline)
		materialBindState.invalidate()
		for _, face := range skyFaces {
			textureIndex := resolveWorldSkyTextureIndex(face, dc.renderer.worldTextureAnimations, 0, timeSeconds)
			solidBindGroup := dc.renderer.whiteTextureBindGroup
			if worldTexture := dc.renderer.worldSkySolidTextures[textureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
				solidBindGroup = worldTexture.bindGroup
			}
			alphaBindGroup := dc.renderer.transparentBindGroup
			if alphaBindGroup == nil {
				alphaBindGroup = dc.renderer.whiteTextureBindGroup
			}
			if worldTexture := dc.renderer.worldSkyAlphaTextures[textureIndex]; worldTexture != nil && worldTexture.bindGroup != nil {
				alphaBindGroup = worldTexture.bindGroup
			}
			setTexture, setLightmap, setFullbright := materialBindState.update(solidBindGroup, alphaBindGroup, dc.renderer.whiteTextureBindGroup)
			if setTexture {
				renderPass.SetBindGroup(1, solidBindGroup, nil)
			}
			if setLightmap {
				renderPass.SetBindGroup(2, alphaBindGroup, nil)
			}
			// Bind group 3 (fullbright/lightmap) is required by the shared pipeline
			// layout even though the sky shader doesn't use it.
			if setFullbright {
				renderPass.SetBindGroup(3, dc.renderer.whiteTextureBindGroup, nil)
			}
			renderPass.DrawIndexed(face.NumIndices, 1, face.FirstIndex, 0, 0)
			skyDrawnIndices += face.NumIndices
		}
	}
	skyDrawMS = float64(time.Since(skyDrawStart)) / float64(time.Millisecond)

	if !writeWorldUniform(1, [3]float32{}, 0) {
		slog.Error("renderWorldInternal: Failed to restore world fog uniform after sky pass")
		renderPass.End()
		return
	}

	renderPass.SetPipeline(dc.renderer.worldPipeline)
	materialBindState.invalidate()
	drawnIndices := uint32(0)
	alphaTestDrawnIndices := uint32(0)
	liquidDrawnIndices := uint32(0)
	if opaqueBatchBuffer != nil {
		renderPass.SetIndexBuffer(opaqueBatchBuffer, gputypes.IndexFormatUint32, 0)
	}
	opaqueDrawStart := time.Now()
	for _, batch := range opaqueBatches {
		if !writeWorldUniform(1, batch.key.dynamicLight, batch.key.litWater) {
			slog.Error("renderWorldInternal: Failed to update world dynamic-light uniform")
			renderPass.End()
			return
		}
		setTexture, setLightmap, setFullbright := materialBindState.update(batch.key.textureBindGroup, batch.key.lightmapBindGroup, batch.key.fullbrightBindGroup)
		if setTexture {
			renderPass.SetBindGroup(1, batch.key.textureBindGroup, nil)
		}
		if setLightmap {
			renderPass.SetBindGroup(2, batch.key.lightmapBindGroup, nil)
		}
		if setFullbright {
			renderPass.SetBindGroup(3, batch.key.fullbrightBindGroup, nil)
		}
		renderPass.DrawIndexed(batch.numIndices, 1, batch.firstIndex, 0, 0)
		drawnIndices += batch.numIndices
	}
	for _, batch := range alphaTestBatches {
		if !writeWorldUniform(1, batch.key.dynamicLight, batch.key.litWater) {
			slog.Error("renderWorldInternal: Failed to update alpha-test world dynamic-light uniform")
			renderPass.End()
			return
		}
		setTexture, setLightmap, setFullbright := materialBindState.update(batch.key.textureBindGroup, batch.key.lightmapBindGroup, batch.key.fullbrightBindGroup)
		if setTexture {
			renderPass.SetBindGroup(1, batch.key.textureBindGroup, nil)
		}
		if setLightmap {
			renderPass.SetBindGroup(2, batch.key.lightmapBindGroup, nil)
		}
		if setFullbright {
			renderPass.SetBindGroup(3, batch.key.fullbrightBindGroup, nil)
		}
		renderPass.DrawIndexed(batch.numIndices, 1, batch.firstIndex, 0, 0)
		alphaTestDrawnIndices += batch.numIndices
	}
	if dc.renderer.worldTurbulentPipeline != nil {
		renderPass.SetPipeline(dc.renderer.worldTurbulentPipeline)
		materialBindState.invalidate()
		for _, batch := range opaqueLiquidBatches {
			if !writeWorldUniform(1, batch.key.dynamicLight, batch.key.litWater) {
				slog.Error("renderWorldInternal: Failed to update liquid lighting uniform")
				renderPass.End()
				return
			}
			setTexture, setLightmap, setFullbright := materialBindState.update(batch.key.textureBindGroup, batch.key.lightmapBindGroup, batch.key.fullbrightBindGroup)
			if setTexture {
				renderPass.SetBindGroup(1, batch.key.textureBindGroup, nil)
			}
			if setLightmap {
				renderPass.SetBindGroup(2, batch.key.lightmapBindGroup, nil)
			}
			if setFullbright {
				renderPass.SetBindGroup(3, batch.key.fullbrightBindGroup, nil)
			}
			renderPass.DrawIndexed(batch.numIndices, 1, batch.firstIndex, 0, 0)
			liquidDrawnIndices += batch.numIndices
		}
	}
	opaqueDrawMS = float64(time.Since(opaqueDrawStart)) / float64(time.Millisecond)
	if drawnIndices > 0 {
		slog.Debug("World rendered",
			"indices", drawnIndices,
			"triangles", drawnIndices/3,
			"vertices", worldData.TotalVertices)
	} else {
		slog.Debug("renderWorldInternal: No opaque world faces selected for textured draw")
	}
	if skyDrawnIndices > 0 {
		slog.Debug("GoGPU world sky rendered", "indices", skyDrawnIndices, "triangles", skyDrawnIndices/3)
	}
	if alphaTestDrawnIndices > 0 {
		slog.Debug("GoGPU alpha-test world faces rendered", "indices", alphaTestDrawnIndices, "triangles", alphaTestDrawnIndices/3)
	}
	if liquidDrawnIndices > 0 {
		slog.Debug("GoGPU opaque liquids rendered", "indices", liquidDrawnIndices, "triangles", liquidDrawnIndices/3)
	}

	// End render pass
	slog.Debug("renderWorldInternal: ending render pass")
	if err := renderPass.End(); err != nil {
		slog.Warn("renderWorldInternal: render pass end error", "error", err)
	}

	// Finish encoding and get command buffer
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Error("renderWorldInternal: Failed to finish command encoding", "error", err)
		return
	}

	// Submit to queue
	slog.Debug("renderWorldInternal: submitting to queue")
	submitStart := time.Now()
	_, err = queue.Submit(cmdBuffer)
	submitMS = float64(time.Since(submitStart)) / float64(time.Millisecond)
	if err != nil {
		slog.Error("renderWorldInternal: Failed to submit render commands", "error", err)
		return
	}

	if hostSpeeds {
		slog.Info("render_world_speeds",
			"visible_select_ms", visibleSelectMS,
			"classify_faces_ms", classifyFacesMS,
			"batch_build_ms", batchBuildMS,
			"batch_upload_ms", batchUploadMS,
			"sky_draw_ms", skyDrawMS,
			"opaque_draw_ms", opaqueDrawMS,
			"submit_ms", submitMS,
			"cache_hit", cacheHit,
			"visible_faces", visibleFaceCount,
			"sky_faces", len(skyFaces),
			"opaque_batches", len(opaqueBatches),
			"alpha_test_batches", len(alphaTestBatches),
			"opaque_liquid_batches", len(opaqueLiquidBatches),
			"batched_indices", len(batchedIndices),
		)
	}
	slog.Debug("World render commands submitted successfully")
}

// matrixToBytes converts a types.Mat4 to bytes (column-major, little-endian).
func matrixToBytes(m types.Mat4) []byte {
	b := types.Mat4ToBytes(m)
	return b[:]
}

func (r *Renderer) resetGoGPUWorldBatchCache() {
	for i := range r.worldBatchCacheEntries {
		entry := &r.worldBatchCacheEntries[i]
		entry.valid = false
		entry.leaf = 0
		entry.lightSig = 0
		entry.faceCount = 0
		entry.skyFaces = nil
		entry.translucentLiquid = nil
		entry.indices = nil
		entry.opaque = nil
		entry.alpha = nil
		entry.liquid = nil
	}
	r.worldBatchCacheNext = 0
}

func (r *Renderer) gogpuWorldBatchCacheEntry(leaf int, lightSig uint64) *gogpuWorldBatchCacheEntry {
	for i := range r.worldBatchCacheEntries {
		entry := &r.worldBatchCacheEntries[i]
		if entry.valid && entry.leaf == leaf && entry.lightSig == lightSig {
			return entry
		}
	}
	return nil
}

func (r *Renderer) storeGoGPUWorldBatchCacheEntry(leaf int, lightSig uint64, faceCount int, skyFaces, translucentLiquid []WorldFace, batchedIndices []uint32, opaqueBatches, alphaTestBatches, opaqueLiquidBatches []gogpuWorldFaceBatch) {
	if leaf < 0 {
		return
	}
	entry := r.gogpuWorldBatchCacheEntry(leaf, lightSig)
	if entry == nil {
		entry = &r.worldBatchCacheEntries[r.worldBatchCacheNext]
		r.worldBatchCacheNext = (r.worldBatchCacheNext + 1) % len(r.worldBatchCacheEntries)
	}
	entry.valid = true
	entry.leaf = leaf
	entry.lightSig = lightSig
	entry.faceCount = faceCount
	entry.skyFaces = append(entry.skyFaces[:0], skyFaces...)
	entry.translucentLiquid = append(entry.translucentLiquid[:0], translucentLiquid...)
	entry.indices = append(entry.indices[:0], batchedIndices...)
	entry.opaque = append(entry.opaque[:0], opaqueBatches...)
	entry.alpha = append(entry.alpha[:0], alphaTestBatches...)
	entry.liquid = append(entry.liquid[:0], opaqueLiquidBatches...)
}

func fillWorldSceneUniformBytes(dst []byte, vp types.Mat4, cameraOrigin [3]float32, fogColor [3]float32, fogDensity float32, time float32, alpha float32, dynamicLight [3]float32, litWater float32) {
	clear(dst[:worldUniformBufferSize])
	matrixBytes := matrixToBytes(vp)
	copy(dst[:64], matrixBytes)
	putFloat32s(dst[64:76], cameraOrigin[:])
	binary.LittleEndian.PutUint32(dst[76:80], math.Float32bits(worldFogUniformDensity(fogDensity)))
	putFloat32s(dst[80:92], fogColor[:])
	binary.LittleEndian.PutUint32(dst[92:96], math.Float32bits(time))
	binary.LittleEndian.PutUint32(dst[96:100], math.Float32bits(alpha))
	putFloat32s(dst[112:124], dynamicLight[:])
	binary.LittleEndian.PutUint32(dst[124:128], math.Float32bits(litWater))
}

func worldSceneUniformBytes(vp types.Mat4, cameraOrigin [3]float32, fogColor [3]float32, fogDensity float32, time float32, alpha float32, dynamicLight [3]float32, litWater float32) []byte {
	data := make([]byte, worldUniformBufferSize)
	fillWorldSceneUniformBytes(data, vp, cameraOrigin, fogColor, fogDensity, time, alpha, dynamicLight, litWater)
	return data
}

func gogpuWorldUniformInputs(state *RenderFrameState, camera CameraState) ([3]float32, float32, float32) {
	cameraOrigin := [3]float32{camera.Origin.X, camera.Origin.Y, camera.Origin.Z}
	return cameraOrigin, state.FogDensity, camera.Time
}

func gogpuWorldClearColor(clear [4]float32) gputypes.Color {
	if os.Getenv("IRONWAIL_DEBUG_WORLD_CLEAR_GREEN") == "1" {
		return gputypes.Color{R: 0.0, G: 1.0, B: 0.0, A: 1.0}
	}
	return gputypes.Color{
		R: float64(clear[0]),
		G: float64(clear[1]),
		B: float64(clear[2]),
		A: float64(clear[3]),
	}
}

func (dc *DrawContext) clearGoGPUSharedDepthStencil() {
	if dc == nil || dc.renderer == nil {
		return
	}
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	textureView := dc.currentWGPURenderTargetView()
	if device == nil || queue == nil || textureView == nil {
		return
	}

	dc.renderer.mu.Lock()
	dc.renderer.ensureAliasDepthTextureLocked(device)
	depthView := dc.renderer.worldDepthTextureView
	dc.renderer.mu.Unlock()
	attachment := gogpuSharedDepthStencilClearAttachmentForView(depthView)
	if attachment == nil {
		return
	}

	encoder, err := device.CreateCommandEncoder(&wgpu.CommandEncoderDescriptor{Label: "GoGPU Shared Depth Clear Encoder"})
	if err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: failed to create encoder", "error", err)
		return
	}

	renderPass, err := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		Label: "GoGPU Shared Depth Clear Pass",
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:    textureView,
			LoadOp:  gputypes.LoadOpLoad,
			StoreOp: gputypes.StoreOpStore,
		}},
		DepthStencilAttachment: attachment,
	})
	if err != nil {
		slog.Error("clearGoGPUSharedDepthStencil: Failed to begin render pass", "error", err)
		return
	}
	if err := renderPass.End(); err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: failed to finish encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Warn("clearGoGPUSharedDepthStencil: failed to submit clear pass", "error", err)
	}
}

// TransformVertex applies model-view-projection transformation to a vertex.
// This is a helper for software rendering fallback.
func TransformVertex(pos [3]float32, mvp types.Mat4) types.Vec4 {
	v := types.Vec4{X: pos[0], Y: pos[1], Z: pos[2], W: 1.0}
	return types.Mat4MulVec4(mvp, v)
}

// createWorldDepthTexture allocates a depth attachment used by multi-pass world rendering so later passes can depth-test against the opaque world.
func (r *Renderer) createWorldDepthTexture(device *wgpu.Device, width, height int) (*wgpu.Texture, *wgpu.TextureView, error) {
	texture, err := device.CreateTexture(&wgpu.TextureDescriptor{
		Label:         "World Depth Texture",
		Size:          wgpu.Extent3D{Width: uint32(width), Height: uint32(height), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     gputypes.TextureDimension2D,
		Format:        worldDepthTextureFormat,
		Usage:         gputypes.TextureUsageRenderAttachment,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create depth texture: %w", err)
	}

	view, err := device.CreateTextureView(texture, &wgpu.TextureViewDescriptor{
		Label:           "World Depth Texture View",
		Format:          worldDepthTextureFormat,
		Dimension:       gputypes.TextureViewDimension2D,
		Aspect:          gputypes.TextureAspectAll,
		BaseMipLevel:    0,
		MipLevelCount:   1,
		BaseArrayLayer:  0,
		ArrayLayerCount: 1,
	})
	if err != nil {
		texture.Release()
		return nil, nil, fmt.Errorf("create depth texture view: %w", err)
	}

	return texture, view, nil
}

func (dc *DrawContext) renderWorldTranslucentLiquidsHAL(state *RenderFrameState) {
	if dc == nil || dc.renderer == nil || state == nil {
		return
	}
	device := dc.renderer.getWGPUDevice()
	queue := dc.renderer.getWGPUQueue()
	if device == nil || queue == nil {
		return
	}

	dc.renderer.mu.RLock()
	worldData := dc.renderer.worldData
	textureView := dc.currentWGPURenderTargetView()
	depthView := dc.renderer.worldDepthTextureView
	uniformBuffer := dc.renderer.uniformBuffer
	uniformBindGroup := dc.renderer.uniformBindGroup
	translucentPipeline := dc.renderer.worldTranslucentTurbulentPipeline
	vertexBuffer := dc.renderer.worldVertexBuffer
	indexBuffer := dc.renderer.worldIndexBuffer
	worldTextures := dc.renderer.worldTextures
	worldFullbrightTextures := dc.renderer.worldFullbrightTextures
	worldTextureAnimations := dc.renderer.worldTextureAnimations
	worldLightmapPages := dc.renderer.worldLightmapPages
	whiteTextureBindGroup := dc.renderer.whiteTextureBindGroup
	transparentBindGroup := dc.renderer.transparentBindGroup
	whiteLightmapBindGroup := dc.renderer.whiteLightmapBindGroup
	var activeDynamicLights []DynamicLight
	if dc.renderer.lightPool != nil {
		activeDynamicLights = append(activeDynamicLights, dc.renderer.lightPool.ActiveLights()...)
	}
	dc.renderer.mu.RUnlock()

	if worldData == nil || textureView == nil || uniformBuffer == nil || uniformBindGroup == nil || translucentPipeline == nil || vertexBuffer == nil || indexBuffer == nil {
		return
	}

	liquidAlpha := worldLiquidAlphaSettingsForGeometry(worldData.Geometry)
	worldHasLitWater := worldData.Geometry.HasLitWater
	if !hasTranslucentWorldLiquidFaceType(worldData.Geometry.LiquidFaceTypes, liquidAlpha) {
		return
	}

	renderPassDescriptor := &wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{{
			View:       textureView,
			LoadOp:     gputypes.LoadOpLoad,
			StoreOp:    gputypes.StoreOpStore,
			ClearValue: gputypes.Color{},
		}},
		DepthStencilAttachment: aliasDepthAttachmentForView(depthView),
	}
	encoder, err := device.CreateCommandEncoder(nil)
	if err != nil {
		slog.Error("renderWorldTranslucentLiquidsHAL: failed to create command encoder", "error", err)
		return
	}
	renderPass, err := encoder.BeginRenderPass(renderPassDescriptor)
	if err != nil {
		slog.Error("renderWorldTranslucentLiquidsHAL: Failed to begin render pass", "error", err)
		return
	}
	w, h := dc.renderer.Size()
	renderPass.SetViewport(0, 0, float32(w), float32(h), 0, 1)
	renderPass.SetScissorRect(0, 0, uint32(w), uint32(h))
	renderPass.SetPipeline(translucentPipeline)
	renderPass.SetVertexBuffer(0, vertexBuffer, 0)
	renderPass.SetIndexBuffer(indexBuffer, gputypes.IndexFormatUint32, 0)

	cameraState := dc.renderer.cameraState
	camera, fogDensity, timeValue := gogpuWorldUniformInputs(state, cameraState)
	vp := dc.renderer.GetViewProjectionMatrix()
	var uniformData [worldUniformBufferSize]byte
	writeWorldUniform := func(alpha float32, dynamicLight [3]float32, litWater float32) bool {
		fillWorldSceneUniformBytes(uniformData[:], vp, camera, state.FogColor, fogDensity, timeValue, alpha, dynamicLight, litWater)
		if err := queue.WriteBuffer(uniformBuffer, 0, uniformData[:]); err != nil {
			slog.Error("renderWorldTranslucentLiquidsHAL: failed to update world uniform", "error", err)
			return false
		}
		renderPass.SetBindGroup(0, uniformBindGroup, nil)
		return true
	}

	translucentFaces := make([]gogpuTranslucentLiquidFaceDraw, 0, 8)
	for _, face := range worldData.Geometry.Faces {
		if !shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha) {
			continue
		}
		translucentFaces = append(translucentFaces, gogpuTranslucentLiquidFaceDraw{
			face:       face,
			alpha:      worldFaceAlpha(face.Flags, liquidAlpha),
			center:     face.Center,
			distanceSq: worldFaceDistanceSq(face.Center, cameraState),
		})
	}
	sortGoGPUTranslucentLiquidFaces(effectiveGoGPUAlphaMode(GetAlphaMode()), translucentFaces)

	translucentLiquidDrawnIndices := uint32(0)
	for _, draw := range translucentFaces {
		lightmapBindGroup, litWater := gogpuWorldLightmapBindGroupForFace(draw.face, worldLightmapPages, whiteLightmapBindGroup, worldHasLitWater)
		dynamicLight := quantizeGoGPUWorldDynamicLight(evaluateDynamicLightsAtPoint(activeDynamicLights, draw.center))
		if !writeWorldUniform(draw.alpha, dynamicLight, litWater) {
			renderPass.End()
			return
		}
		textureBindGroup := whiteTextureBindGroup
		if worldTexture := gogpuWorldTextureForFace(draw.face, worldTextures, worldTextureAnimations, nil, 0, float64(timeValue)); worldTexture != nil && worldTexture.bindGroup != nil {
			textureBindGroup = worldTexture.bindGroup
		}
		fullbrightBindGroup := transparentBindGroup
		if fullbrightBindGroup == nil {
			fullbrightBindGroup = whiteTextureBindGroup
		}
		if worldTexture := gogpuWorldTextureForFace(draw.face, worldFullbrightTextures, worldTextureAnimations, nil, 0, float64(timeValue)); worldTexture != nil && worldTexture.bindGroup != nil {
			fullbrightBindGroup = worldTexture.bindGroup
		}
		renderPass.SetBindGroup(1, textureBindGroup, nil)
		renderPass.SetBindGroup(2, lightmapBindGroup, nil)
		renderPass.SetBindGroup(3, fullbrightBindGroup, nil)
		renderPass.DrawIndexed(draw.face.NumIndices, 1, draw.face.FirstIndex, 0, 0)
		translucentLiquidDrawnIndices += draw.face.NumIndices
	}

	if err := renderPass.End(); err != nil {
		slog.Warn("renderWorldTranslucentLiquidsHAL: render pass end error", "error", err)
	}
	cmdBuffer, err := encoder.Finish()
	if err != nil {
		slog.Error("renderWorldTranslucentLiquidsHAL: failed to finish encoding", "error", err)
		return
	}
	if _, err := queue.Submit(cmdBuffer); err != nil {
		slog.Error("renderWorldTranslucentLiquidsHAL: failed to submit render commands", "error", err)
		return
	}
	if translucentLiquidDrawnIndices > 0 {
		slog.Debug("GoGPU translucent liquids rendered", "indices", translucentLiquidDrawnIndices, "triangles", translucentLiquidDrawnIndices/3)
	}
}

func (r *Renderer) hasTranslucentWorldLiquidFacesGoGPU() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	worldData := r.worldData
	r.mu.RUnlock()
	if worldData == nil {
		return false
	}
	return hasTranslucentWorldLiquidFaceType(
		worldData.Geometry.LiquidFaceTypes,
		worldLiquidAlphaSettingsForGeometry(worldData.Geometry),
	)
}

// worldDepthAttachmentForView picks the correct depth target for the current view configuration and pass sequence.
func worldDepthAttachmentForView(view *wgpu.TextureView) *wgpu.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &wgpu.RenderPassDepthStencilAttachment{
		View:              view,
		DepthLoadOp:       gputypes.LoadOpClear,
		DepthStoreOp:      gputypes.StoreOpStore,
		DepthClearValue:   1.0,
		DepthReadOnly:     false,
		StencilLoadOp:     gputypes.LoadOpClear,
		StencilStoreOp:    gputypes.StoreOpStore,
		StencilClearValue: 0,
		StencilReadOnly:   false, // Must be false when StencilLoadOp is Clear (WebGPU spec)
	}
}

func gogpuSharedDepthStencilClearAttachmentForView(view *wgpu.TextureView) *wgpu.RenderPassDepthStencilAttachment {
	if view == nil {
		return nil
	}
	return &wgpu.RenderPassDepthStencilAttachment{
		View:              view,
		DepthLoadOp:       gputypes.LoadOpClear,
		DepthStoreOp:      gputypes.StoreOpStore,
		DepthClearValue:   1.0,
		DepthReadOnly:     false,
		StencilLoadOp:     gputypes.LoadOpClear,
		StencilStoreOp:    gputypes.StoreOpStore,
		StencilClearValue: 0,
		StencilReadOnly:   false,
	}
}

// ClearWorld releases world geometry resources.
// Called when switching maps or shutting down.
func (r *Renderer) ClearWorld() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.worldFirstFrameStatsLogged.Store(false)

	if r.worldData != nil {
		// Release GPU buffers
		if r.worldVertexBuffer != nil {
			r.worldVertexBuffer.Release()
		}
		if r.worldIndexBuffer != nil {
			r.worldIndexBuffer.Release()
		}
		if r.worldDynamicIndexBuffer != nil {
			r.worldDynamicIndexBuffer.Release()
		}
		if r.uniformBuffer != nil {
			r.uniformBuffer.Release()
		}
		if r.worldSkyPipeline != nil {
			r.worldSkyPipeline.Release()
		}
		if r.worldSkyExternalPipeline != nil {
			r.worldSkyExternalPipeline.Release()
		}
		if r.worldTurbulentPipeline != nil {
			r.worldTurbulentPipeline.Release()
		}
		if r.worldTranslucentPipeline != nil {
			r.worldTranslucentPipeline.Release()
		}
		if r.worldTranslucentTurbulentPipeline != nil {
			r.worldTranslucentTurbulentPipeline.Release()
		}
		if r.worldPipeline != nil {
			r.worldPipeline.Release()
		}
		if r.worldAlphaTestPipeline != nil {
			r.worldAlphaTestPipeline.Release()
		}
		if r.worldPipelineLayout != nil {
			r.worldPipelineLayout.Release()
		}
		if r.worldSkyExternalPipelineLayout != nil {
			r.worldSkyExternalPipelineLayout.Release()
		}
		if r.uniformBindGroup != nil {
			r.uniformBindGroup.Release()
		}
		if r.uniformBindGroupLayout != nil {
			r.uniformBindGroupLayout.Release()
		}
		if r.textureBindGroupLayout != nil {
			r.textureBindGroupLayout.Release()
		}
		if r.worldSkyExternalBindGroupLayout != nil {
			r.worldSkyExternalBindGroupLayout.Release()
		}
		if r.whiteTextureBindGroup != nil {
			r.whiteTextureBindGroup.Release()
		}
		if r.whiteLightmapBindGroup != nil {
			r.whiteLightmapBindGroup.Release()
		}
		if r.worldTextureSampler != nil {
			r.worldTextureSampler.Release()
		}
		if r.worldLightmapSampler != nil {
			r.worldLightmapSampler.Release()
		}
		for textureIndex, worldTexture := range r.worldTextures {
			if worldTexture == nil {
				delete(r.worldTextures, textureIndex)
				continue
			}
			if worldTexture.bindGroup != nil {
				worldTexture.bindGroup.Release()
			}
			if worldTexture.view != nil {
				worldTexture.view.Release()
			}
			if worldTexture.texture != nil {
				worldTexture.texture.Release()
			}
			delete(r.worldTextures, textureIndex)
		}
		for textureIndex, worldTexture := range r.worldSkySolidTextures {
			if worldTexture == nil {
				delete(r.worldSkySolidTextures, textureIndex)
				continue
			}
			if worldTexture.bindGroup != nil {
				worldTexture.bindGroup.Release()
			}
			if worldTexture.view != nil {
				worldTexture.view.Release()
			}
			if worldTexture.texture != nil {
				worldTexture.texture.Release()
			}
			delete(r.worldSkySolidTextures, textureIndex)
		}
		for textureIndex, worldTexture := range r.worldSkyAlphaTextures {
			if worldTexture == nil {
				delete(r.worldSkyAlphaTextures, textureIndex)
				continue
			}
			if worldTexture.bindGroup != nil {
				worldTexture.bindGroup.Release()
			}
			if worldTexture.view != nil {
				worldTexture.view.Release()
			}
			if worldTexture.texture != nil {
				worldTexture.texture.Release()
			}
			delete(r.worldSkyAlphaTextures, textureIndex)
		}
		for textureIndex, worldTexture := range r.worldFullbrightTextures {
			if worldTexture == nil {
				delete(r.worldFullbrightTextures, textureIndex)
				continue
			}
			if worldTexture.bindGroup != nil {
				worldTexture.bindGroup.Release()
			}
			if worldTexture.view != nil {
				worldTexture.view.Release()
			}
			if worldTexture.texture != nil {
				worldTexture.texture.Release()
			}
			delete(r.worldFullbrightTextures, textureIndex)
		}
		for index, worldLightmap := range r.worldLightmapPages {
			if worldLightmap == nil {
				continue
			}
			if worldLightmap.bindGroup != nil {
				worldLightmap.bindGroup.Release()
			}
			if worldLightmap.view != nil {
				worldLightmap.view.Release()
			}
			if worldLightmap.texture != nil {
				worldLightmap.texture.Release()
			}
			r.worldLightmapPages[index] = nil
		}
		if r.whiteTexture != nil {
			r.whiteTexture.Release()
		}
		if r.transparentBindGroup != nil {
			r.transparentBindGroup.Release()
		}
		if r.transparentTextureView != nil {
			r.transparentTextureView.Release()
		}
		if r.transparentTexture != nil {
			r.transparentTexture.Release()
		}
		if r.worldDepthTexture != nil {
			r.worldDepthTexture.Release()
		}
		for submodelIndex, lightmaps := range r.brushModelLightmaps {
			for _, lightmap := range lightmaps {
				if lightmap == nil {
					continue
				}
				if lightmap.bindGroup != nil {
					lightmap.bindGroup.Release()
				}
				if lightmap.view != nil {
					lightmap.view.Release()
				}
				if lightmap.texture != nil {
					lightmap.texture.Release()
				}
			}
			delete(r.brushModelLightmaps, submodelIndex)
		}
		r.destroyGoGPUExternalSkyboxResourcesLocked()

		r.worldData = nil
		r.worldVertexBuffer = nil
		r.worldIndexBuffer = nil
		r.worldDynamicIndexBuffer = nil
		r.worldDynamicIndexBufferSize = 0
		r.worldPipeline = nil
		r.worldAlphaTestPipeline = nil
		r.worldTranslucentPipeline = nil
		r.worldTurbulentPipeline = nil
		r.worldTranslucentTurbulentPipeline = nil
		r.worldSkyPipeline = nil
		r.worldSkyExternalPipeline = nil
		r.worldPipelineLayout = nil
		r.worldSkyExternalPipelineLayout = nil
		r.worldShader = nil
		r.uniformBuffer = nil
		r.uniformBindGroup = nil
		r.uniformBindGroupLayout = nil
		r.textureBindGroupLayout = nil
		r.worldSkyExternalBindGroupLayout = nil
		r.worldTextureSampler = nil
		r.worldTextures = nil
		r.worldFullbrightTextures = nil
		r.worldSkySolidTextures = nil
		r.worldSkyAlphaTextures = nil
		r.worldTextureAnimations = nil
		r.whiteTextureBindGroup = nil
		r.transparentTexture = nil
		r.transparentTextureView = nil
		r.transparentBindGroup = nil
		r.worldLightmapSampler = nil
		r.worldLightmapPages = nil
		r.whiteLightmapBindGroup = nil
		r.worldBindGroup = nil
		r.worldSkyExternalBindGroup = nil
		r.whiteTexture = nil
		r.whiteTextureView = nil
		r.worldDepthTexture = nil
		r.worldDepthTextureView = nil
		r.worldDepthWidth = 0
		r.worldDepthHeight = 0
		r.worldVisibleFacesScratch = worldVisibilityScratch{}
		r.worldSkyFacesScratch = nil
		r.worldTranslucentLiquidScratch = nil
		r.worldOpaqueDrawsScratch = nil
		r.worldAlphaDrawsScratch = nil
		r.worldLiquidDrawsScratch = nil
		r.worldBatchedIndexScratch = nil
		r.worldOpaqueBatchScratch = nil
		r.worldAlphaBatchScratch = nil
		r.worldLiquidBatchScratch = nil
		r.resetGoGPUWorldBatchCache()
		r.brushModelGeometry = make(map[int]*WorldGeometry)
		r.brushModelLightmaps = make(map[int][]*gpuWorldTexture)

		slog.Debug("World geometry cleared")
	}
}

// GetWorldData returns the current world render data (for debugging).
func (r *Renderer) GetWorldData() *WorldRenderData {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData
}

// GetWorldBounds returns uploaded world geometry bounds when available.
func (r *Renderer) GetWorldBounds() (min [3]float32, max [3]float32, ok bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.worldData == nil || r.worldData.TotalVertices == 0 {
		return min, max, false
	}

	return r.worldData.BoundsMin, r.worldData.BoundsMax, true
}
