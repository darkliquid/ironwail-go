package renderer

import (
	"math"
	"sort"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
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
