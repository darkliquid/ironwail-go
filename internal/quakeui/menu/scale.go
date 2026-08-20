package menu

import (
	"github.com/gogpu/ui/geometry"
)

// MenuScaleParams holds the parameters for computing the menu layout transform.
type MenuScaleParams struct {
	WindowWidth  float32
	WindowHeight float32
	MenuScale    float32 // value of scr_menuscale (0 = default 1.0)
}

// MenuTransform represents the computed position and scale for the 320x200 menu.
type MenuTransform struct {
	Scale   float32
	OriginX float32
	OriginY float32
}

// ComputeMenuTransform computes the scale and centering offset for the 320x200
// virtual menu viewport (spec §4.4, C-lineage Draw_Transform in gl_draw.c).
//
// offset = ((guiwidth - 320*s)/2, (guiheight - 200*s)/2)
func ComputeMenuTransform(params MenuScaleParams) MenuTransform {
	w := params.WindowWidth
	h := params.WindowHeight
	if w <= 0 {
		w = 320
	}
	if h <= 0 {
		h = 200
	}

	scale := float32(1.0)
	if params.MenuScale > 0 {
		scale = params.MenuScale
	}

	originX := (w - 320.0*scale) * 0.5
	originY := (h - 200.0*scale) * 0.5
	if originX < 0 {
		originX = 0
	}
	if originY < 0 {
		originY = 0
	}

	return MenuTransform{
		Scale:   scale,
		OriginX: originX,
		OriginY: originY,
	}
}

// TransformPoint maps a (x, y) point from 320x200 menu virtual space into window space.
func (t MenuTransform) TransformPoint(pt geometry.Point) geometry.Point {
	return geometry.Pt(t.OriginX+pt.X*t.Scale, t.OriginY+pt.Y*t.Scale)
}

// InverseTransformPoint maps a point from window space back into 320x200 menu virtual space.
func (t MenuTransform) InverseTransformPoint(pt geometry.Point) geometry.Point {
	if t.Scale <= 0 {
		return pt
	}
	return geometry.Pt((pt.X-t.OriginX)/t.Scale, (pt.Y-t.OriginY)/t.Scale)
}
