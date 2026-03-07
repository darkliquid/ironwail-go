package renderer

import "github.com/ironwail/ironwail-go/internal/model"

// DecalVariant identifies the visual style used by a projected decal mark.
type DecalVariant int

const (
	DecalVariantBullet DecalVariant = iota
	DecalVariantChip
	DecalVariantScorch
	DecalVariantMagic
)

// BrushEntity describes an inline BSP submodel instance to render.
type BrushEntity struct {
	SubmodelIndex int
	Frame         int
	Origin        [3]float32
	Angles        [3]float32
	Alpha         float32
	Scale         float32
}

// EntityEffectSource describes a runtime entity whose effect flags drive transient visuals.
type EntityEffectSource struct {
	Origin  [3]float32
	Angles  [3]float32
	Effects int
}

// AliasModelEntity describes an MDL instance to render.
type AliasModelEntity struct {
	ModelID   string
	Model     *model.Model
	Frame     int
	SkinNum   int
	FrameTime float64 // Time within current frame for animation interpolation (in seconds)
	Origin    [3]float32
	Angles    [3]float32
	Alpha     float32
	Scale     float32
}

// SpriteEntity describes a sprite (billboard) instance to render.
type SpriteEntity struct {
	ModelID string
	Model   *model.Model
	Frame   int
	Origin  [3]float32
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
