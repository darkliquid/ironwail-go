package quakui

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/hud"
	legacymenu "github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/event"
)

type dummyHost struct{}

func (d *dummyHost) CVar(string) float64       { return 0 }
func (d *dummyHost) PlaySound(string)          {}
func (d *dummyHost) ExecuteCommandText(string) {}
func (d *dummyHost) Quit()                     {}

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
	err := r.DrawOverlay(gpucontext.TextureView{}, 640, 480)
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
	palette := make([]byte, 768)

	r := NewOverlayRenderer(host, mgr, con, nil, drawMgr, conchars, palette)
	if !r.MenuRoot().IsVisible() {
		t.Fatal("expected MenuRoot to be visible when MenuMain active")
	}

	err := r.DrawOverlay(gpucontext.TextureView{}, 640, 480)
	if err != nil {
		t.Fatalf("expected nil error on DrawOverlay: %v", err)
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

	err := r.DrawOverlay(gpucontext.TextureView{}, 640, 480)
	if err != nil {
		t.Fatalf("expected nil error on DrawOverlay: %v", err)
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
		err := r.DrawOverlay(gpucontext.TextureView{}, size[0], size[1])
		if err != nil {
			t.Fatalf("expected nil error on DrawOverlay at %dx%d: %v", size[0], size[1], err)
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
