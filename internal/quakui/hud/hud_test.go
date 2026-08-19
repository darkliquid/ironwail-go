package hud

import (
	"image"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/hud"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

type testContext struct {
	widget.Context
	winSize geometry.Size
}

func (c *testContext) WindowSize() geometry.Size {
	if c.winSize.Width <= 0 || c.winSize.Height <= 0 {
		return geometry.Sz(640, 480)
	}
	return c.winSize
}

type testCanvas struct {
	widget.Canvas
	images []image.Image
	rects  []geometry.Rect
}

func (c *testCanvas) PushTransform(offset geometry.Point) {}
func (c *testCanvas) PopTransform()                       {}
func (c *testCanvas) TransformOffset() geometry.Point     { return geometry.Pt(0, 0) }
func (c *testCanvas) DrawImage(img image.Image, at geometry.Point) {
	c.images = append(c.images, img)
}
func (c *testCanvas) DrawRect(bounds geometry.Rect, col widget.Color) {
	c.rects = append(c.rects, bounds)
}

type mockHUDProvider struct {
	state hud.State
	style hud.HUDStyle
}

func (m *mockHUDProvider) State() hud.State     { return m.state }
func (m *mockHUDProvider) Style() hud.HUDStyle { return m.style }
func (m *mockHUDProvider) Draw(rc renderer.RenderContext) {
	rc.SetCanvas(renderer.CanvasSbar)
	pic := &qimage.QPic{Width: 16, Height: 16, Pixels: make([]byte, 16*16)}
	rc.DrawPic(10, 10, pic)
	rc.DrawCharacter(50, 50, 65)
	rc.DrawFill(0, 0, 100, 20, 15)
}

func setupTestHUDRoot() *HUDRoot {
	rawConchars := make([]byte, 128*128)
	for i := range rawConchars {
		rawConchars[i] = 1
	}
	palette := draw.DefaultQuakePalette()
	provider := &mockHUDProvider{
		state: hud.State{Health: 100, Armor: 50, Ammo: 25},
		style: hud.HUDStyleClassic,
	}
	return NewHUDRoot(provider, nil, rawConchars, palette)
}

func TestHUDRoot_Layout(t *testing.T) {
	root := setupTestHUDRoot()
	ctx := &testContext{winSize: geometry.Sz(640, 480)}
	sz := root.Layout(ctx, geometry.Loose(geometry.Sz(640, 480)))
	if sz.Width != 640 || sz.Height != 480 {
		t.Fatalf("Layout size = %+v, want (640, 480)", sz)
	}
}

func TestHUDRoot_Draw(t *testing.T) {
	root := setupTestHUDRoot()
	ctx := &testContext{winSize: geometry.Sz(640, 480)}
	canvas := &testCanvas{}

	root.Draw(ctx, canvas)

	if len(canvas.images) == 0 {
		t.Fatal("expected image draw calls from HUD drawing")
	}
	if len(canvas.rects) == 0 {
		t.Fatal("expected rect draw calls from HUD DrawFill")
	}
}

func TestHUDRoot_Event_Fallthrough(t *testing.T) {
	root := setupTestHUDRoot()
	ctx := &testContext{}

	handled := root.Event(ctx, &event.KeyEvent{
		KeyType: event.KeyPress,
		Key:     event.KeySpace,
	})
	if handled {
		t.Fatal("expected HUDRoot.Event to return false for input fallthrough (ADR-0007)")
	}
}

func TestHUDRoot_RepaintBoundary(t *testing.T) {
	root := setupTestHUDRoot()
	if !root.IsRepaintBoundary() {
		t.Fatal("expected HUDRoot to be a RepaintBoundary")
	}
}
