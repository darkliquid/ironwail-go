package hud

import (
	"image"

	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/hud"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/quakui/gfx"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// HUDStateProvider provides point-in-time HUD state and style for drawing.
type HUDStateProvider interface {
	State() hud.State
	Style() hud.HUDStyle
	Draw(rc renderer.RenderContext)
}

// HUDRoot is the top-level gogpu/ui widget that renders the Quake heads-up display
// (status bar, crosshair, centerprint) over the game view (ADR-0008, M3.2).
//
// It is a draw-only widget: Event returns false so all input falls through to the
// game engine (ADR-0007).
type HUDRoot struct {
	widget.WidgetBase

	provider HUDStateProvider
	drawMgr  *draw.Manager
	atlas    *gfx.ConcharsAtlas
	palette  []byte
}

// NewHUDRoot constructs a new HUDRoot widget.
func NewHUDRoot(provider HUDStateProvider, drawMgr *draw.Manager, conchars []byte, palette []byte) *HUDRoot {
	if len(palette) < 768 && drawMgr != nil {
		palette = drawMgr.Palette()
	}
	if len(palette) < 768 {
		palette = draw.DefaultQuakePalette()
	}
	if len(conchars) < 128*128 && drawMgr != nil {
		conchars = drawMgr.ConcharsData()
	}
	atlas := gfx.NewConcharsAtlas(conchars, palette)

	r := &HUDRoot{
		provider: provider,
		drawMgr:  drawMgr,
		atlas:    atlas,
		palette:  palette,
	}
	r.SetVisible(true)
	r.SetEnabled(true)
	r.SetRepaintBoundary(true)
	return r
}

// IsVisible reports whether the heads-up display is active and should be drawn.
func (r *HUDRoot) IsVisible() bool {
	if r == nil || r.provider == nil {
		return false
	}
	return true
}

// Layout fills the window constraints.
func (r *HUDRoot) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.MaxWidth
	h := c.MaxHeight
	if w <= 0 {
		w = 320
	}
	if h <= 0 {
		h = 200
	}
	sz := geometry.Sz(w, h)
	r.SetBounds(geometry.NewRect(0, 0, w, h))
	r.SetNeedsRedraw(true)
	r.InvalidateScene()
	return sz
}

// Draw renders the status bar, crosshair, and centerprint.
func (r *HUDRoot) Draw(ctx widget.Context, canvas widget.Canvas) {
	if r == nil || r.provider == nil || canvas == nil {
		return
	}

	bounds := r.Bounds()
	w, h := int(bounds.Width()), int(bounds.Height())
	if (w <= 0 || h <= 0) && ctx != nil {
		winSize := ctx.WindowSize()
		w = int(winSize.Width)
		h = int(winSize.Height)
	}
	if w <= 0 {
		w = 320
	}
	if h <= 0 {
		h = 200
	}

	if sizer, ok := r.provider.(interface{ SetScreenSize(int, int) }); ok {
		sizer.SetScreenSize(w, h)
	}

	if sched, ok := ctx.(interface{ ScheduleAnimationFrame() }); ok {
		sched.ScheduleAnimationFrame()
		r.SetNeedsRedraw(true)
	}

	rc := &canvasRenderContext{
		canvas:  canvas,
		atlas:   r.atlas,
		palette: r.palette,
		width:   w,
		height:  h,
		picMap:  make(map[*qimage.QPic]*image.RGBA),
	}

	r.provider.Draw(rc)
}

// Event returns false so all input falls through to the game simulation (ADR-0007).
func (r *HUDRoot) Event(ctx widget.Context, e event.Event) bool {
	return false
}

// canvasRenderContext bridges renderer.RenderContext calls to widget.Canvas.
type canvasRenderContext struct {
	canvas  widget.Canvas
	atlas   *gfx.ConcharsAtlas
	palette []byte
	width   int
	height  int
	params  renderer.CanvasTransformParams
	curType renderer.CanvasType
	picMap  map[*qimage.QPic]*image.RGBA
}

func (c *canvasRenderContext) Clear(r, g, b, a float32)            {}
func (c *canvasRenderContext) DrawTriangle(r, g, b, a float32)     {}
func (c *canvasRenderContext) SurfaceView() any                    { return nil }
func (c *canvasRenderContext) Gamma() float32                      { return 1.0 }

func (c *canvasRenderContext) DrawRGBA(x, y int, img *image.RGBA) {
	if c.canvas != nil && img != nil {
		c.canvas.DrawImage(img, geometry.Pt(float32(x), float32(y)))
	}
}

func (c *canvasRenderContext) SetCanvasParams(params renderer.CanvasTransformParams) {
	c.params = params
}

func (c *canvasRenderContext) SetCanvas(canvasType renderer.CanvasType) {
	c.curType = canvasType
}

func (c *canvasRenderContext) getImage(pic *qimage.QPic) *image.RGBA {
	if pic == nil {
		return nil
	}
	if img, ok := c.picMap[pic]; ok {
		return img
	}
	img := gfx.QPicToImage(pic, c.palette)
	c.picMap[pic] = img
	return img
}

func (c *canvasRenderContext) Canvas() renderer.CanvasState {
	return renderer.CanvasState{
		Type:   c.curType,
		Left:   0,
		Right:  float32(c.width),
		Top:    0,
		Bottom: float32(c.height),
	}
}

func (c *canvasRenderContext) transformPos(x, y int) (float32, float32) {
	if c.curType == renderer.CanvasSbar {
		// CanvasSbar: 320x48 fixed status bar centered at the bottom of the screen
		originX := (float32(c.width) - 320.0) * 0.5
		originY := float32(c.height) - 48.0
		if originX < 0 {
			originX = 0
		}
		if originY < 0 {
			originY = 0
		}
		return originX + float32(x), originY + float32(y)
	} else if c.curType == renderer.CanvasCrosshair {
		// Centered at screen center
		originX := float32(c.width) * 0.5
		originY := float32(c.height) * 0.5
		return originX + float32(x), originY + float32(y)
	}

	return float32(x), float32(y)
}

func (c *canvasRenderContext) DrawPic(x, y int, pic *qimage.QPic) {
	if pic == nil || c.canvas == nil {
		return
	}
	img := c.getImage(pic)
	if img != nil {
		tx, ty := c.transformPos(x, y)
		c.canvas.DrawImage(img, geometry.Pt(tx, ty))
	}
}

func (c *canvasRenderContext) DrawMenuPic(x, y int, pic *qimage.QPic) {
	c.DrawPic(x, y, pic)
}

func (c *canvasRenderContext) DrawMenuCharacter(x, y int, num int) {
	c.DrawCharacter(x, y, num)
}

func (c *canvasRenderContext) DrawPicAlpha(x, y int, pic *qimage.QPic, alpha float32) {
	if pic == nil || c.canvas == nil || alpha <= 0 {
		return
	}
	img := c.getImage(pic)
	if img != nil {
		tx, ty := c.transformPos(x, y)
		c.canvas.DrawImage(img, geometry.Pt(tx, ty))
	}
}

func (c *canvasRenderContext) DrawCharacter(x, y int, num int) {
	if c == nil || c.atlas == nil || c.canvas == nil {
		return
	}
	if num <= 0 || num == ' ' {
		return
	}
	if num > 255 {
		num = '?'
	}
	if img := c.atlas.GlyphImage(byte(num)); img != nil {
		tx, ty := c.transformPos(x, y)
		c.canvas.DrawImage(img, geometry.Pt(tx, ty))
	}
}

func (c *canvasRenderContext) DrawString(x, y int, str string) {
	for i, ch := range []byte(str) {
		c.DrawCharacter(x+i*8, y, int(ch))
	}
}

func (c *canvasRenderContext) DrawFill(x, y, w, h int, col byte) {
	c.DrawFillAlpha(x, y, w, h, col, 1.0)
}

func (c *canvasRenderContext) DrawFillAlpha(x, y, w, h int, col byte, alpha float32) {
	if c.canvas == nil || w <= 0 || h <= 0 || alpha <= 0 {
		return
	}
	off := int(col) * 3
	var r, g, b byte
	if off+2 < len(c.palette) {
		r = c.palette[off]
		g = c.palette[off+1]
		b = c.palette[off+2]
	}
	a := byte(alpha*255.0 + 0.5)
	colRGBA := widget.RGBA8(r, g, b, a)
	tx, ty := c.transformPos(x, y)
	c.canvas.DrawRect(geometry.NewRect(tx, ty, float32(w), float32(h)), colRGBA)
}
