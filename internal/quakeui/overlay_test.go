package quakeui

import (
	"image"
	"testing"
	"unsafe"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/hud"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	legacymenu "github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	"github.com/gogpu/gg"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gputypes"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/render"
)

type dummyHost struct{}

func (d *dummyHost) CVar(string) float64       { return 0 }
func (d *dummyHost) PlaySound(string)          {}
func (d *dummyHost) ExecuteCommandText(string) {}
func (d *dummyHost) Quit()                     {}
func (d *dummyHost) GPUContextProvider() gpucontext.DeviceProvider {
	return nil // software fallback path in tests
}
func (d *dummyHost) SurfaceView() gpucontext.TextureView {
	return gpucontext.TextureView{}
}

type testOverlayRenderContext struct {
	drawRGBACalled bool
	lastImage      *image.RGBA
}

func (t *testOverlayRenderContext) Clear(r, g, b, a float32)            {}
func (t *testOverlayRenderContext) DrawTriangle(r, g, b, a float32)     {}
func (t *testOverlayRenderContext) SurfaceView() any                    { return nil }
func (t *testOverlayRenderContext) Gamma() float32                      { return 1 }
func (t *testOverlayRenderContext) DrawPic(x, y int, pic *qimage.QPic)  {}
func (t *testOverlayRenderContext) DrawMenuPic(x, y int, pic *qimage.QPic) {}
func (t *testOverlayRenderContext) DrawFill(x, y, w, h int, color byte) {}
func (t *testOverlayRenderContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {}
func (t *testOverlayRenderContext) DrawCharacter(x, y int, num int)     {}
func (t *testOverlayRenderContext) DrawMenuCharacter(x, y int, num int) {}
func (t *testOverlayRenderContext) DrawRGBA(x, y int, img *image.RGBA) {
	t.drawRGBACalled = true
	t.lastImage = img
}
func (t *testOverlayRenderContext) SetCanvas(ct renderer.CanvasType)     {}
func (t *testOverlayRenderContext) Canvas() renderer.CanvasState         { return renderer.CanvasState{} }

func setupTestOverlay() *OverlayRenderer {
	host := &dummyHost{}
	mgr := legacymenu.NewManager(nil, nil, nil)
	con := console.NewConsole(1024)
	_ = con.Init(0)
	drawMgr := draw.NewManager()
	conchars := make([]byte, 128*128)
	palette := make([]byte, 768)

	return NewOverlayRenderer(host, mgr, con, nil, drawMgr, conchars, palette)
}

func TestOverlayRenderer_Creation(t *testing.T) {
	r := setupTestOverlay()
	if r == nil {
		t.Fatal("expected non-nil OverlayRenderer")
	}
	if r.MenuRoot() == nil {
		t.Fatal("expected non-nil MenuRoot")
	}
	if r.ConsoleRoot() == nil {
		t.Fatal("expected non-nil ConsoleRoot")
	}
	if r.HUDRoot() == nil {
		t.Fatal("expected non-nil HUDRoot")
	}
}

func TestOverlayRenderer_DrawOverlay_NilTarget(t *testing.T) {
	r := setupTestOverlay()
	err := r.DrawOverlay(nil, 640, 480)
	if err != nil {
		t.Fatalf("expected nil error for nil target view, got: %v", err)
	}
}

func TestOverlayRenderer_Event(t *testing.T) {
	r := setupTestOverlay()
	r.SetConsoleSlideFraction(1.0)

	// Key event while console is open
	ke := event.NewKeyEvent(event.KeyPress, event.KeyA, 'a', event.ModNone)
	handled := r.Event(ke)
	if !handled {
		t.Fatal("expected console to handle key event")
	}
}

func TestOverlayRenderer_Menu_DrawAndEvent(t *testing.T) {
	host := &dummyHost{}
	mgr := legacymenu.NewManager(nil, nil, nil)
	mgr.ShowState(legacymenu.MenuMain)
	con := console.NewConsole(1024)
	_ = con.Init(0)
	drawMgr := draw.NewManager()
	conchars := make([]byte, 128*128)
	for i := range conchars {
		conchars[i] = byte(i%255 + 1)
	}
	palette := make([]byte, 768)
	for i := range palette {
		palette[i] = 255
	}

	r := NewOverlayRenderer(host, mgr, con, nil, drawMgr, conchars, palette)
	if !r.MenuRoot().IsVisible() {
		t.Fatal("expected MenuRoot to be visible when MenuMain active")
	}

	rc := &testOverlayRenderContext{}
	err := r.DrawOverlay(rc, 640, 480)
	if err != nil {
		t.Fatalf("expected nil error on DrawOverlay: %v", err)
	}
	if !rc.drawRGBACalled {
		t.Fatal("expected DrawRGBA to be called on render context")
	}
	var nonZero int
	if rc.lastImage != nil {
		for _, b := range rc.lastImage.Pix {
			if b != 0 {
				nonZero++
			}
		}
	}
	if nonZero == 0 {
		t.Fatal("expected non-zero pixels in rendered overlay image")
	}

	// Down arrow should navigate menu
	ke := event.NewKeyEvent(event.KeyPress, event.KeyDown, 0, event.ModNone)
	handled := r.Event(ke)
	if !handled {
		t.Fatal("expected menu to handle DownArrow key event")
	}
}

func TestOverlayRenderer_Console_FullFlow(t *testing.T) {
	host := &dummyHost{}
	mgr := legacymenu.NewManager(nil, nil, nil)
	con := console.NewConsole(1024)
	_ = con.Init(0)
	drawMgr := draw.NewManager()
	conchars := make([]byte, 128*128)
	palette := make([]byte, 768)

	r := NewOverlayRenderer(host, mgr, con, nil, drawMgr, conchars, palette)
	r.SetConsoleSlideFraction(1.0)
	r.SetConsoleForcedUp(true)

	var executedCmd string
	r.ConsoleRoot().SetOnCommand(func(cmd string) {
		executedCmd = cmd
	})

	// Type characters
	for _, ch := range "echo hello" {
		ke := event.NewKeyEvent(event.KeyPress, event.KeyUnknown, ch, event.ModNone)
		r.Event(ke)
	}

	// Press Enter
	enterEvent := event.NewKeyEvent(event.KeyPress, event.KeyEnter, 0, event.ModNone)
	r.Event(enterEvent)

	if executedCmd != "echo hello" {
		t.Fatalf("expected executedCmd %q, got %q", "echo hello", executedCmd)
	}

	con.Printf("Console output line\n")

	rc := &testOverlayRenderContext{}
	err := r.DrawOverlay(rc, 640, 480)
	if err != nil {
		t.Fatalf("expected nil error on DrawOverlay: %v", err)
	}
	if !rc.drawRGBACalled {
		t.Fatal("expected DrawRGBA to be called on render context")
	}
}

type testHUDProvider struct {
	drawCalled bool
}

func (p *testHUDProvider) State() hud.State {
	return hud.State{
		Health: 100,
		Armor:  50,
		Ammo:   25,
	}
}

func (p *testHUDProvider) Style() hud.HUDStyle {
	return hud.HUDStyleClassic
}

func (p *testHUDProvider) Draw(rc renderer.RenderContext) {
	p.drawCalled = true
}

func TestOverlayRenderer_HUD_DrawAndFallthrough(t *testing.T) {
	host := &dummyHost{}
	mgr := legacymenu.NewManager(nil, nil, nil)
	con := console.NewConsole(1024)
	_ = con.Init(0)
	hudProv := &testHUDProvider{}
	drawMgr := draw.NewManager()
	conchars := make([]byte, 128*128)
	palette := make([]byte, 768)

	r := NewOverlayRenderer(host, mgr, con, hudProv, drawMgr, conchars, palette)
	r.SetConsoleSlideFraction(0)

	if !r.HUDRoot().IsVisible() {
		t.Fatal("expected HUDRoot to be visible")
	}

	// Draw at multiple resolutions to test dynamic resize adaptation
	for _, size := range [][2]int{{640, 480}, {1280, 720}, {1920, 1080}} {
		rc := &testOverlayRenderContext{}
		err := r.DrawOverlay(rc, size[0], size[1])
		if err != nil {
			t.Fatalf("expected nil error on DrawOverlay at %dx%d: %v", size[0], size[1], err)
		}
		if !rc.drawRGBACalled {
			t.Fatalf("expected DrawRGBA to be called at %dx%d", size[0], size[1])
		}
	}

	if !hudProv.drawCalled {
		t.Fatal("expected HUD Draw to be called")
	}

	// When only HUD is visible (no console/menu), key events must NOT be consumed (fallthrough)
	ke := event.NewKeyEvent(event.KeyPress, event.KeyW, 'w', event.ModNone)
	handled := r.Event(ke)
	if handled {
		t.Fatal("expected gameplay key event to fall through when only HUD is visible")
	}
}

func TestGGDrawImage(t *testing.T) {
	dc := gg.NewContext(640, 480)
	dc.Clear()
	canvas := render.NewCanvas(dc, 640, 480)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	canvas.DrawImage(img, geometry.Pt(50, 50))

	outImg := dc.Image()
	var nonZero int
	if rgba, ok := outImg.(*image.RGBA); ok {
		for _, b := range rgba.Pix {
			if b != 0 {
				nonZero++
			}
		}
	}
	t.Logf("nonZero pixels: %d", nonZero)
	if nonZero == 0 {
		t.Fatal("expected nonZero > 0")
	}
}

func TestOverlayRenderer_RealPak_DrawOverlay(t *testing.T) {
	quakeDir, err := testutil.LocateQuakeDir()
	if err != nil || quakeDir == "" {
		t.Skip("quake dir not found")
	}
	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(quakeDir, "id1"); err != nil {
		t.Fatalf("fs init: %v", err)
	}
	drawMgr := draw.NewManager()
	if err := drawMgr.Init(fileSys); err != nil {
		t.Fatalf("draw init: %v", err)
	}
	host := &dummyHost{}
	mgr := legacymenu.NewManager(nil, nil, nil)
	mgr.ShowState(legacymenu.MenuMain)
	con := console.NewConsole(1024)
	_ = con.Init(0)
	hudProv := hud.NewHUD(drawMgr, nil)

	r := NewOverlayRenderer(host, mgr, con, hudProv, drawMgr, drawMgr.ConcharsData(), drawMgr.Palette())
	r.SetConsoleForcedUp(true)
	rc := &testOverlayRenderContext{}
	if err := r.DrawOverlay(rc, 1892, 1072); err != nil {
		t.Fatalf("DrawOverlay error: %v", err)
	}
	var nonZero int
	if rc.lastImage != nil {
		for i := 3; i < len(rc.lastImage.Pix); i += 4 {
			if rc.lastImage.Pix[i] > 0 {
				nonZero++
			}
		}
	}
	t.Logf("nonZero alpha pixels: %d out of %d", nonZero, len(rc.lastImage.Pix)/4)
	if nonZero == 0 {
		t.Fatal("expected nonZero > 0 with real pak")
	}
}

// gpuAwareHost is a Host whose GPUContextProvider returns a mock device
// provider (so the overlay takes the GPU canvas path) and whose SurfaceView
// returns the view RenderDirect should target.
type gpuAwareHost struct {
	*dummyHost
	provider gpucontext.DeviceProvider
	view     gpucontext.TextureView
}

func (h *gpuAwareHost) GPUContextProvider() gpucontext.DeviceProvider { return h.provider }
func (h *gpuAwareHost) SurfaceView() gpucontext.TextureView           { return h.view }

// gpuCaptureAccelerator is a GPUAccelerator + GPURenderContextProvider that
// hands each gg.Context a per-context ops stub (gpuContextOps shape) whose
// Flush increments flushes — proving RenderDirect reached the GPU flush path.
type gpuCaptureAccelerator struct {
	flushes *int
}

func (a *gpuCaptureAccelerator) Name() string          { return "quakeui-gpu-capture" }
func (a *gpuCaptureAccelerator) Init() error            { return nil }
func (a *gpuCaptureAccelerator) Close()                 {}
func (a *gpuCaptureAccelerator) CanAccelerate(gg.AcceleratedOp) bool {
	return true
}
func (a *gpuCaptureAccelerator) FillPath(gg.GPURenderTarget, *gg.Path, *gg.Paint) error {
	return gg.ErrFallbackToCPU
}
func (a *gpuCaptureAccelerator) StrokePath(gg.GPURenderTarget, *gg.Path, *gg.Paint) error {
	return gg.ErrFallbackToCPU
}
func (a *gpuCaptureAccelerator) FillShape(gg.GPURenderTarget, gg.DetectedShape, *gg.Paint) error {
	return gg.ErrFallbackToCPU
}
func (a *gpuCaptureAccelerator) StrokeShape(gg.GPURenderTarget, gg.DetectedShape, *gg.Paint) error {
	return gg.ErrFallbackToCPU
}
func (a *gpuCaptureAccelerator) Flush(gg.GPURenderTarget) error { return nil }

// NewGPURenderContext implements gg.GPURenderContextProvider with a shared
// ops stub whose Flush counts into the shared counter.
func (a *gpuCaptureAccelerator) NewGPURenderContext() any {
	return &gpuCaptureOps{flushes: a.flushes}
}

// gpuCaptureOps implements the gg gpuContextOps contract (structural) with a
// no-op Flush that increments the shared counter. All draw ops fall back to
// CPU so the capture only observes the flush path.
type gpuCaptureOps struct {
	flushes *int
}

func (o *gpuCaptureOps) FillShape(gg.GPURenderTarget, gg.DetectedShape, *gg.Paint) error {
	return gg.ErrFallbackToCPU
}
func (o *gpuCaptureOps) StrokeShape(gg.GPURenderTarget, gg.DetectedShape, *gg.Paint) error {
	return gg.ErrFallbackToCPU
}
func (o *gpuCaptureOps) FillPath(gg.GPURenderTarget, *gg.Path, *gg.Paint) error {
	return gg.ErrFallbackToCPU
}
func (o *gpuCaptureOps) StrokePath(gg.GPURenderTarget, *gg.Path, *gg.Paint) error {
	return gg.ErrFallbackToCPU
}
func (o *gpuCaptureOps) DrawText(gg.GPURenderTarget, any, string, float64, float64, gg.RGBA, gg.Matrix, float64) error {
	return gg.ErrFallbackToCPU
}
func (o *gpuCaptureOps) DrawGlyphMaskText(gg.GPURenderTarget, any, string, float64, float64, gg.RGBA, gg.Matrix, float64) error {
	return gg.ErrFallbackToCPU
}
func (o *gpuCaptureOps) QueueImageDraw(gg.GPURenderTarget, []byte, uint64, int, int, int, float32, float32, float32, float32, float32, uint32, uint32, float32, float32, float32, float32) {
}
func (o *gpuCaptureOps) QueueGPUTextureDraw(gg.GPURenderTarget, gpucontext.TextureView, float32, float32, float32, float32, float32, uint32, uint32) {
}
func (o *gpuCaptureOps) QueueBaseLayer(gg.GPURenderTarget, gpucontext.TextureView, float32, float32, float32, float32, float32, uint32, uint32) {
}
func (o *gpuCaptureOps) Flush(gg.GPURenderTarget) error {
	if o.flushes != nil {
		*o.flushes++
	}
	return nil
}
func (o *gpuCaptureOps) SetClipRect(uint32, uint32, uint32, uint32) {}
func (o *gpuCaptureOps) ClearClipRect()                             {}
func (o *gpuCaptureOps) SetClipRRect(float32, float32, float32, float32, float32) {
}
func (o *gpuCaptureOps) ClearClipRRect()     {}
func (o *gpuCaptureOps) SetClipPath(*gg.Path) {}
func (o *gpuCaptureOps) ClearClipPath()      {}
func (o *gpuCaptureOps) BeginFrame()         {}
func (o *gpuCaptureOps) MarkFrameRendered()  {}
func (o *gpuCaptureOps) SetPipelineMode(gg.PipelineMode) {}
func (o *gpuCaptureOps) SetAntiAlias(bool)   {}
func (o *gpuCaptureOps) PendingCount() int   { return 0 }
func (o *gpuCaptureOps) Close()              {}

// mockDeviceProvider satisfies gpucontext.DeviceProvider with opaque handles.
type mockDeviceProvider struct{}

func (m *mockDeviceProvider) Device() gpucontext.Device             { return gpucontext.Device{} }
func (m *mockDeviceProvider) Queue() gpucontext.Queue               { return gpucontext.Queue{} }
func (m *mockDeviceProvider) Adapter() gpucontext.Adapter           { return gpucontext.Adapter{} }
func (m *mockDeviceProvider) SurfaceFormat() gputypes.TextureFormat { return gputypes.TextureFormatBGRA8Unorm }
func (m *mockDeviceProvider) AdapterInfo() gpucontext.AdapterInfo {
	return gpucontext.AdapterInfo{Type: gpucontext.AdapterTypeUnknown}
}

// TestOverlayRenderer_GPUCAnvasRendersStackToSurface drives the Scenario A
// GPU canvas path (ADR-0011): with a device provider present, DrawOverlay must
// create the GPU ggcanvas, draw the widget stack into it, and composite onto
// the surface view via RenderDirect. The capture accelerator's Flush receives
// the target holding the provided surface view.
func TestOverlayRenderer_GPUCAnvasRendersStackToSurface(t *testing.T) {
	var flushCount int
	acc := &gpuCaptureAccelerator{flushes: &flushCount}
	if err := gg.RegisterAccelerator(acc); err != nil {
		t.Fatalf("RegisterAccelerator: %v", err)
	}
	t.Cleanup(gg.CloseAccelerator)

	host := &gpuAwareHost{
		dummyHost: &dummyHost{},
		provider:  &mockDeviceProvider{},
		// A non-nil view handle so RenderDirect does not early-return as "no
		// live surface" (matches the in-frame engine path where the view is
		// acquired).
		view: gpucontext.NewTextureView(unsafe.Pointer(&gpuSurfaceProbe)),
	}
	mgr := legacymenu.NewManager(nil, nil, nil)
	mgr.ShowState(legacymenu.MenuMain)
	con := console.NewConsole(1024)
	_ = con.Init(0)
	r := NewOverlayRenderer(host, mgr, con, nil, draw.NewManager(), make([]byte, 128*128), make([]byte, 768))

	flushCount = 0
	if err := r.DrawOverlay(&testOverlayRenderContext{}, 640, 480); err != nil {
		t.Fatalf("DrawOverlay: %v", err)
	}
	if flushCount == 0 {
		t.Fatal("expected RenderDirect to flush the GPU canvas (accelerator Flush called)")
	}
}

// gpuSurfaceProbe is a fake *wgpu.TextureView-backed handle so the mock
// surface view passed to RenderDirect is non-nil.
var gpuSurfaceProbe byte
