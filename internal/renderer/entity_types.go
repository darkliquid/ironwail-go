package renderer

import "github.com/ironwail/ironwail-go/internal/model"

// BrushEntity describes an inline BSP submodel instance to render.
type BrushEntity struct {
	SubmodelIndex int
	Origin        [3]float32
	Angles        [3]float32
}

// AliasModelEntity describes an MDL instance to render.
type AliasModelEntity struct {
	ModelID string
	Model   *model.Model
	Frame   int
	SkinNum int
	Origin  [3]float32
	Angles  [3]float32
	Alpha   float32
}
