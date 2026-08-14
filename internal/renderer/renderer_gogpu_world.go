package renderer

import (
	"encoding/binary"
	"math"
	"sort"
	"sync"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/darkliquid/ironwail-go/pkg/types"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

const (
	worldUniformBufferSize             = 128
	worldUniformAlign                  = 256
	worldUniformMaxDraws               = 8192
	gogpuWorldDynamicLightBufferMax    = 512
	gogpuWorldDynamicLightBufferStride = 32
	gogpuWorldDynamicLightHeaderSize   = 16
	gogpuWorldDynamicLightBufferSize   = gogpuWorldDynamicLightHeaderSize + gogpuWorldDynamicLightBufferMax*gogpuWorldDynamicLightBufferStride
)

type WorldGeometry = worldimpl.WorldGeometry
type WorldVertex = worldimpl.WorldVertex
type WorldFace = worldimpl.WorldFace

// Depth32FloatStencil8 is used instead of Depth24PlusStencil8 because the
// wgpu HAL maps Depth24PlusStencil8 to VK_FORMAT_D24_UNORM_S8_UINT, which
// NVIDIA GPUs do not support. Depth32FloatStencil8 maps to
// VK_FORMAT_D32_SFLOAT_S8_UINT which is universally supported.
//
// worldDepthTextureFormat is a variable (not a constant) so the renderer can
// fall back to Depth24PlusStencil8 on devices — notably browsers — that do
// not expose the depth32float-stencil8 feature. It mirrors
// pipeline.WorldDepthTextureFormat and is kept in sync via
// updateWorldDepthFormatForDevice at device discovery.
var worldDepthTextureFormat = gputypes.TextureFormatDepth32FloatStencil8

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
	BoundsMin types.Vec3
	// BoundsMax is the maximum XYZ world-space coordinate of uploaded geometry.
	BoundsMax types.Vec3

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

func worldLeafIndex(tree *bsp.Tree, cameraOrigin types.Vec3) int {
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
	return false
}

type gogpuWorldFaceDrawBucket struct {
	key   gogpuWorldFaceBatchKey
	draws []gogpuWorldFaceDraw
}

type gogpuWorldFaceBatchKey struct {
	textureBindGroup    *wgpu.BindGroup
	lightmapBindGroup   *wgpu.BindGroup
	fullbrightBindGroup *wgpu.BindGroup
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
	center     types.Vec3
	distanceSq float32
}

type gpuWorldTexture struct {
	texture   *wgpu.Texture
	view      *wgpu.TextureView
	bindGroup *wgpu.BindGroup
	// width, height, and layers record the GPU texture dimensions for
	// diagnostic verification. They are set at creation time.
	width  uint32
	height uint32
	layers uint32
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

var dynamicLightsBytesPool = sync.Pool{
	New: func() any {
		b := make([]byte, gogpuWorldDynamicLightHeaderSize+gogpuWorldDynamicLightBufferMax*gogpuWorldDynamicLightBufferStride)
		return &b
	},
}

func encodeGoGPUWorldDynamicLights(lights []DynamicLight) (*[]byte, []byte) {
	ptr := dynamicLightsBytesPool.Get().(*[]byte)
	data := *ptr

	if !dynamicLightsEnabled() || len(lights) == 0 {
		return ptr, data[:gogpuWorldDynamicLightHeaderSize]
	}
	count := 0
	for _, light := range lights {
		if count >= gogpuWorldDynamicLightBufferMax || light.Radius <= 0 {
			break
		}
		effectiveMul := light.Brightness * light.FadeMultiplier()
		if effectiveMul <= 0 {
			continue
		}
		base := gogpuWorldDynamicLightHeaderSize + count*gogpuWorldDynamicLightBufferStride
		putFloat32s(data[base:base+12], []float32{light.Position.X, light.Position.Y, light.Position.Z})
		binary.LittleEndian.PutUint32(data[base+12:base+16], math.Float32bits(light.Radius))
		color := [3]float32{
			light.Color.X * effectiveMul,
			light.Color.Y * effectiveMul,
			light.Color.Z * effectiveMul,
		}
		putFloat32s(data[base+16:base+28], color[:])
		binary.LittleEndian.PutUint32(data[base+28:base+32], math.Float32bits(light.MinLight))
		count++
	}
	binary.LittleEndian.PutUint32(data[:4], uint32(count))
	return ptr, data[:gogpuWorldDynamicLightHeaderSize+count*gogpuWorldDynamicLightBufferStride]
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
	// Match C Ironwail (gl_model.c:1359-1368): lit water is enabled for
	// turbulent (liquid) surfaces that are NOT tiled (TEX_SPECIAL) and have
	// valid lightmap sample data. Tiled surfaces (TEX_SPECIAL) are always
	// unlit, matching C behavior where SURF_DRAWTILED skips lightmap allocation.
	return textureFlags&model.SurfDrawTurb != 0 &&
		textureFlags&model.SurfDrawSky == 0 &&
		textureFlags&model.SurfDrawTiled == 0 &&
		lightmapSurface != nil
}

func worldLitWaterCvarEnabled() bool {
	if pkgCVars == nil {
		return true
	}
	cv := pkgCVars.Get(CvarRLitWater)
	if cv == nil {
		return true
	}
	return cv.Int != 0
}

func gogpuWorldLightmapArrayBindGroupForFace(face WorldFace, lightmapArray *gpuWorldTexture, fallback *wgpu.BindGroup, hasLitWater bool) (*wgpu.BindGroup, float32) {
	bindGroup := fallback
	isLiquid := face.Flags&model.SurfDrawTurb != 0 && face.Flags&model.SurfDrawSky == 0
	useLitWater := worldLitWaterCvarEnabled() && hasLitWater && isLiquid

	if face.LightmapIndex < 0 {
		if useLitWater {
			return bindGroup, 1
		}
		return bindGroup, 0
	}
	if lightmapArray == nil || lightmapArray.bindGroup == nil {
		if useLitWater {
			return bindGroup, 1
		}
		return bindGroup, 0
	}
	bindGroup = lightmapArray.bindGroup
	if useLitWater {
		return bindGroup, 1
	}
	return bindGroup, 0
}

func effectiveGoGPUAlphaMode(mode AlphaMode) AlphaMode {
	if mode == AlphaModeOIT {
		return AlphaModeSorted
	}
	return mode
}
