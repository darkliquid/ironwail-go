package quakui

import (
	"image"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/hud"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	legacymenu "github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/gogpu/ui/event"
)

type dummyHost struct{}

func (d *dummyHost) CVar(string) float64       { return 0 }
func (d *dummyHost) PlaySound(string)          {}
func (d *dummyHost) ExecuteCommandText(string) {}
func (d *dummyHost) Quit()                     {}

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
	con.Init(0)
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
	con.Init(0)
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
	con.Init(0)
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
	con.Init(0)
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
		t.Fatal("expected HUD provider Draw method to be called")
	}

	// When only HUD is visible (no console/menu), key events must NOT be consumed (fallthrough)
	ke := event.NewKeyEvent(event.KeyPress, event.KeyW, 'w', event.ModNone)
	handled := r.Event(ke)
	if handled {
		t.Fatal("expected gameplay key event to fall through when only HUD is visible")
	}
}
