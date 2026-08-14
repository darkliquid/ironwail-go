package world

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// WorldGeometry holds backend-neutral BSP world data prepared for rendering.
type WorldGeometry struct {
	Vertices             []WorldVertex
	Indices              []uint32
	Faces                []WorldFace
	LeafFaces            [][]int
	Lightmaps            []WorldLightmapPage
	HasLitWater          bool
	LiquidFaceTypes      int32
	LiquidAlphaOverrides LiquidAlphaOverrides
	TransparentWaterSafe bool
	Tree                 *bsp.Tree
}

// WorldVertex matches the packed vertex layout used by world renderers.
//
// Every vertex uploaded to the GPU occupies exactly 48 bytes in a flat byte
// buffer. The fields below are laid out in declaration order with no padding,
// so `unsafe.Sizeof(WorldVertex{})` == 48. This must match:
//
//   - The stride (ArrayStride: 48) declared in every pipeline's
//     VertexBufferLayout (world_pipelines_gogpu.go, world_gogpu_alias.go,
//     world_gogpu_sprite.go).
//   - The field offsets written by every vertex-packing function
//     (createWorldVertexBuffer, appendGoGPUWorldVertexBytes, VertexBytes,
//     aliasVertexBytesInto).
//   - The @location indices in every WGSL shader's VertexInput struct.
//
// If any of these disagrees, the GPU reads vertex data at the wrong byte
// offsets, causing textures, lighting, and geometry to appear scrambled.
// See docs/VERTEX_LAYOUT.md for the full explanation and diagnostic guide.
type WorldVertex struct {
	Position      types.Vec3 // offset 0,  12 bytes — XYZ world-space coordinates
	TexCoord      [2]float32 // offset 12,  8 bytes — UV into the texture atlas
	LightmapCoord [2]float32 // offset 20,  8 bytes — UV into the lightmap texture array
	Normal        types.Vec3 // offset 28, 12 bytes — surface facing direction for lighting
	LightmapLayer float32    // offset 40,  4 bytes — which lightmap array layer (page) to sample
	MaterialID    uint32     // offset 44,  4 bytes — index into the materials uniform array
}

// WorldFace stores rendering metadata for a BSP face.
type WorldFace struct {
	FirstIndex    uint32
	NumIndices    uint32
	TextureIndex  int32
	LightmapIndex int32
	Flags         int32
	Center        types.Vec3
}

// WorldLightmapSurface describes a single face's lightmap data within an atlas page.
type WorldLightmapSurface struct {
	X       int
	Y       int
	Width   int
	Height  int
	Styles  [bsp.MaxLightmaps]uint8
	Samples []byte
	Dirty   bool
}

// WorldLightmapPage represents a shared lightmap atlas texture page.
type WorldLightmapPage struct {
	Width            int
	Height           int
	Surfaces         []WorldLightmapSurface
	Dirty            bool
	CachedRGBA       []byte
	CachedRegionRGBA []byte
}
