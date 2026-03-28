package model

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// ModelType represents the type of a loaded model.
type ModelType int

const (
	ModBrush  ModelType = iota // Brush model (BSP world geometry)
	ModAlias                   // Alias model (MDL character/item models)
	ModSprite                  // Sprite model (2D billboard effects)
)

// SyncType determines how model animations are synchronized.
type SyncType int

const (
	STSync      SyncType = iota // Synchronized animation
	STRand                      // Random animation
	STFrameTime                 // Sync to frame changes
)

// TextureType classifies texture surfaces for rendering.
type TextureType int

const (
	TexTypeDefault TextureType = iota
	TexTypeCutout
	TexTypeSky
	TexTypeLava
	TexTypeSlime
	TexTypeTele
	TexTypeWater
	TexTypeCount
	TexTypeFirstLiquid = TexTypeLava
	TexTypeLastLiquid  = TexTypeWater
	TexTypeNumLiquids  = TexTypeLastLiquid + 1 - TexTypeFirstLiquid
)

// IsLiquid returns true if the texture type represents a liquid surface.
func (t TextureType) IsLiquid() bool {
	return t >= TexTypeFirstLiquid && t <= TexTypeLastLiquid
}

// Surface flags control rendering behavior.
const (
	SurfPlaneBack      = 0x0002
	SurfDrawSky        = 0x0004
	SurfDrawSprite     = 0x0008
	SurfDrawTurb       = 0x0010
	SurfDrawTiled      = 0x0020
	SurfDrawBackground = 0x0040
	SurfUnderwater     = 0x0080
	SurfNoTexture      = 0x0100
	SurfDrawFence      = 0x0200
	SurfDrawLava       = 0x0400
	SurfDrawSlime      = 0x0800
	SurfDrawTele       = 0x1000
	SurfDrawWater      = 0x2000
)

// Model effect flags.
const (
	EFRocket  = 1 << iota // Leave a trail
	EFGrenade             // Leave a trail
	EFGib                 // Leave a trail
	EFRotate              // Rotate (bonus items)
	EFTracer              // Green split trail
	EFZomGib              // Small blood trail
	EFTracer2             // Orange split trail + rotate
	EFTracer3             // Purple trail
)

// Additional model flags for rendering.
const (
	ModNoLerp      = 256     // Don't lerp when animating
	ModNoShadow    = 512     // Don't cast a shadow
	ModFBrightHack = 1024    // Fullbright hack when fullbrights disabled
	MFHoley        = 1 << 14 // Make index 255 transparent on MDLs
)

const (
	MaxAliasVerts  = 0x7fff // 16-bit index buffer + onseam duplication
	MaxAliasFrames = 1024
	MaxAliasTris   = 4096
	MaxSkins       = 32
	MaxMapHulls    = 4
	MaxLightmaps   = 4
)

// MVertex represents an in-memory vertex.
type MVertex struct {
	Position [3]float32
}

// Side constants for plane testing.
const (
	SideFront = 0
	SideBack  = 1
	SideOn    = 2
)

// MPlane represents an in-memory plane used for collision and rendering.
type MPlane struct {
	Normal   [3]float32
	Dist     float32
	Type     uint8 // For texture axis selection and fast side tests
	SignBits uint8 // Signx + signy<<1 + signz<<1
}

// Texture represents an in-memory texture.
type Texture struct {
	Name           [16]byte
	Width          uint32
	Height         uint32
	Shift          uint32 // Q64 texture shift
	Type           TextureType
	AnimTotal      int // Total tenths in sequence (0 = no animation)
	AnimMin        int // Time for this frame min <= time < max
	AnimMax        int
	AnimNext       *Texture // Next texture in animation sequence
	AlternateAnims *Texture // Bmodels in frame 1 use these
}

// MEdge represents an in-memory edge connecting two vertices.
type MEdge struct {
	V [2]uint32 // Vertex indices
}

// MTexInfo represents texture mapping information for a surface.
type MTexInfo struct {
	Vecs   [2][4]float32 // Texture coordinate vectors
	TexNum int           // Texture index
	Flags  int           // Surface flags
}

// MSurface represents an in-memory rendering surface.
type MSurface struct {
	Plane              *MPlane
	Mins               [3]float32 // For frustum culling
	Maxs               [3]float32
	Flags              int
	VBOFirstVert       int // Index of this surface's first vert in the VBO
	FirstEdge          int // Look up in model->surfedges[], negative numbers are backwards edges
	NumEdges           int16
	LightmapTextureNum int16
	Extents            [2]int16
	LightS             int16 // GL lightmap coordinates
	LightT             int16
	Styles             [MaxLightmaps]byte
	Samples            []byte
	TextureMins        [2]int
	TexInfo            *MTexInfo
}

// MNode represents an in-memory BSP node.
type MNode struct {
	Contents     int        // 0, to differentiate from leafs
	VisFrame     int        // Node needs to be traversed if current
	MinMaxs      [6]float32 // For bounding box culling
	Parent       *MNode
	Plane        *MPlane
	Children     [2]*MNode
	FirstSurface uint32
	NumSurfaces  uint32
}

// MLeaf represents an in-memory BSP leaf.
type MLeaf struct {
	Contents          int
	VisFrame          int
	MinMaxs           [6]float32
	Parent            *MNode
	CompressedVis     []byte
	FirstMarkSurface  []int
	NumMarkSurfaces   int
	Key               int // BSP sequence number for leaf's contents
	AmbientSoundLevel [4]byte
}

// MClipNode represents an in-memory clipnode for collision.
type MClipNode struct {
	PlaneNum int
	Children [2]int // Negative numbers are contents
}

// Hull represents a collision hull.
type Hull struct {
	ClipNodes     []MClipNode
	Planes        []MPlane
	FirstClipNode int
	LastClipNode  int
	ClipMins      [3]float32
	ClipMaxs      [3]float32
}

// MSpriteFrame represents a single sprite frame.
type MSpriteFrame struct {
	Width, Height         int
	Up, Down, Left, Right float32
	SMax, TMax            float32 // Image might be padded
	Pixels                []byte
}

// MSpriteGroup represents a group of animated sprite frames.
type MSpriteGroup struct {
	NumFrames int
	Intervals []float32
	Frames    []*MSpriteFrame
}

// MSpriteFrameDesc describes a sprite frame with its type.
type MSpriteFrameDesc struct {
	Type     int         // spriteframetype_t
	FramePtr interface{} // *MSpriteFrame or *MSpriteGroup
}

// MSprite represents an in-memory sprite model.
type MSprite struct {
	Type      int
	MaxWidth  int
	MaxHeight int
	NumFrames int
	SyncType  SyncType
	Frames    []MSpriteFrameDesc
}

// AliasFrameDesc describes an alias model frame.
type AliasFrameDesc struct {
	FirstPose int
	NumPoses  int
	Interval  float32
	BBoxMin   [4]byte // trivertx_t
	BBoxMax   [4]byte // trivertx_t
	Frame     int
	Name      [16]byte
}

// AliasSkinDesc describes a logical alias skin entry and the flat skin-frame
// range it owns inside AliasHeader.Skins.
type AliasSkinDesc struct {
	FirstFrame int
	NumFrames  int
	Intervals  []float32
}

// AliasHeader represents an in-memory alias model header.
type AliasHeader struct {
	Ident          int
	Version        int
	Scale          [3]float32
	ScaleOrigin    [3]float32
	BoundingRadius float32
	EyePosition    [3]float32
	NumSkins       int
	SkinWidth      int
	SkinHeight     int
	NumVerts       int
	NumTris        int
	NumFrames      int
	SyncType       SyncType
	Flags          int
	Size           float32
	NumVertsVBO    int
	NumPoses       int
	PoseVertType   int // PV_QUAKE1, PV_IQM, PV_MD3
	Skins          [][]byte
	SkinDescs      []AliasSkinDesc
	STVerts        []STVert
	Triangles      []DTriangle
	Poses          [][]TriVertX
	Frames         []AliasFrameDesc
}

// ResolveSkinFrame maps a logical skin selection and time value to a concrete
// flattened skin-frame index inside Skins.
func (a *AliasHeader) ResolveSkinFrame(skinNum int, timeSeconds float64) int {
	if a == nil || len(a.Skins) == 0 {
		return 0
	}

	descCount := len(a.SkinDescs)
	if descCount == 0 {
		if skinNum < 0 {
			skinNum = 0
		}
		return skinNum % len(a.Skins)
	}
	if skinNum < 0 {
		skinNum = 0
	}
	skinNum %= descCount
	desc := a.SkinDescs[skinNum]
	if desc.NumFrames <= 1 {
		return clampAliasSkinFrame(desc.FirstFrame, len(a.Skins))
	}
	if len(desc.Intervals) >= desc.NumFrames {
		fullInterval := float64(desc.Intervals[desc.NumFrames-1])
		if fullInterval > 0 {
			target := math.Mod(timeSeconds, fullInterval)
			if target < 0 {
				target += fullInterval
			}
			for i, interval := range desc.Intervals[:desc.NumFrames] {
				if float64(interval) > target {
					return clampAliasSkinFrame(desc.FirstFrame+i, len(a.Skins))
				}
			}
			return clampAliasSkinFrame(desc.FirstFrame+desc.NumFrames-1, len(a.Skins))
		}
	}

	frame := desc.FirstFrame + int(timeSeconds*10)%desc.NumFrames
	if frame < desc.FirstFrame {
		frame = desc.FirstFrame
	}
	return clampAliasSkinFrame(frame, len(a.Skins))
}

func clampAliasSkinFrame(index, count int) int {
	if count <= 0 {
		return 0
	}
	if index < 0 {
		return 0
	}
	if index >= count {
		return count - 1
	}
	return index
}

// Model represents a loaded Quake model (brush, alias, or sprite).
type Model struct {
	Name     string
	PathID   uint32 // Path ID of the game directory this model came from
	NeedLoad bool   // Bmodels and sprites don't cache normally

	Type      ModelType
	NumFrames int
	SyncType  SyncType
	Flags     int
	SortKey   uint32

	// Volume occupied by the model graphics
	Mins, Maxs   [3]float32
	YMins, YMaxs [3]float32 // Bounds for entities with nonzero yaw
	RMins, RMaxs [3]float32 // Bounds for entities with nonzero pitch or roll

	// Solid volume for clipping
	ClipBox            bool
	ClipMins, ClipMaxs [3]float32

	// Brush model specific
	FirstModelSurface, NumModelSurfaces int
	NumSubModels                        int
	SubModels                           []bsp.DModel

	NumPlanes int
	Planes    []MPlane

	NumLeafs int // Number of visible leafs, not counting 0
	Leafs    []MLeaf

	NumVertexes int
	Vertexes    []MVertex

	NumEdges int
	Edges    []MEdge

	NumNodes int
	Nodes    []MNode

	NumTexInfo int
	TexInfo    []MTexInfo

	NumSurfaces int
	Surfaces    []MSurface

	NumSurfEdges int
	SurfEdges    []int

	NumClipNodes int
	ClipNodes    []MClipNode

	NumMarkSurfaces int
	MarkSurfaces    []int

	Hulls [MaxMapHulls]Hull

	NumTextures int
	Textures    []*Texture

	VisData   []byte
	LightData []byte
	Entities  string

	LitFile             bool
	VisWarn             bool
	BSPVersion          int
	ContentsTransparent int
	HasLitWater         bool

	// Alias model specific
	AliasHeader *AliasHeader

	// Sprite model specific
	SpriteData *MSprite
}

// SideFromPlane determines which side of a plane a point is on.
func SideFromPlane(plane *MPlane, point [3]float32) int {
	dot := plane.Normal[0]*point[0] + plane.Normal[1]*point[1] + plane.Normal[2]*point[2]
	if dot < plane.Dist {
		return SideBack
	}
	return SideFront
}
