package world

import "github.com/darkliquid/ironwail-go/internal/bsp"

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
type WorldVertex struct {
	Position      [3]float32
	TexCoord      [2]float32
	LightmapCoord [2]float32
	Normal        [3]float32
}

// WorldFace stores rendering metadata for a BSP face.
type WorldFace struct {
	FirstIndex    uint32
	NumIndices    uint32
	TextureIndex  int32
	LightmapIndex int32
	Flags         int32
	Center        [3]float32
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
