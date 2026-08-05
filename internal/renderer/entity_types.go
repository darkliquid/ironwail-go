package renderer

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
)

// DecalVariant identifies the visual style used by a projected decal mark.
type DecalVariant int

// DecalOrigin implements decal.MarkEntity.
func (m DecalMarkEntity) DecalOrigin() [3]float32 { return m.Origin }

// DecalNormal implements decal.MarkEntity.
func (m DecalMarkEntity) DecalNormal() [3]float32 { return m.Normal }

// DecalSize implements decal.MarkEntity.
func (m DecalMarkEntity) DecalSize() float32 { return m.Size }

// DecalRotation implements decal.MarkEntity.
func (m DecalMarkEntity) DecalRotation() float32 { return m.Rotation }

const (
	DecalVariantBullet DecalVariant = iota
	DecalVariantChip
	DecalVariantScorch
	DecalVariantMagic
)

// BrushEntity describes an inline BSP submodel instance to render.
//
// When ExternalKey is non-empty, the entity references a standalone BSP
// file (e.g. "maps/b_rock0.bsp") rather than an inline world submodel:
// SubmodelIndex is treated as 0 against ExternalTree's own model table,
// and the renderer caches geometry keyed by ExternalKey.
type BrushEntity struct {
	SubmodelIndex int
	Frame         int
	Origin        [3]float32
	Angles        [3]float32
	Alpha         float32
	Scale         float32
	ExternalKey   string
	ExternalTree  *bsp.Tree
}

// EntityEffectSource describes a runtime entity whose effect flags drive transient visuals.
type EntityEffectSource struct {
	Origin     [3]float32
	Angles     [3]float32
	Effects    int
	ModelFlags int
	EntityNum  int // Entity index — used as EntityKey for per-entity dlight slot reuse
}

// AliasModelEntity describes an MDL instance to render.
type AliasModelEntity struct {
	ModelID     string
	Model       *model.Model
	EntityKey   int
	Frame       int
	SkinNum     int
	ColorMap    uint32
	IsPlayer    bool
	FrameTime   float64 // Legacy per-frame animation time; world alias rendering uses TimeSeconds.
	TimeSeconds float64 // Absolute client/render time for persistent alias interpolation.
	LerpFlags   int
	LerpFinish  float64
	Origin      [3]float32
	Angles      [3]float32
	Alpha       float32
	Scale       float32
}

const AliasViewModelEntityKey = -1

func AliasStaticEntityKey(index int) int {
	return -2 - index
}

// SpriteEntity describes a sprite (billboard) instance to render.
type SpriteEntity struct {
	ModelID string
	Model   *model.Model
	Frame   int
	Origin  [3]float32
	Angles  [3]float32
	Alpha   float32
	Scale   float32
	// SpriteData holds the actual sprite loading data (optional, used internally)
	SpriteData *model.MSprite
}

// DecalMarkEntity describes a projected mark (bullet hole, scorch mark) in world space.
type DecalMarkEntity struct {
	Origin   [3]float32
	Normal   [3]float32
	Size     float32
	Rotation float32
	Color    [3]float32
	Alpha    float32
	Variant  DecalVariant
}
