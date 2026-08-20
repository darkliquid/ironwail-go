package quakui

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/draw"
	legacymenu "github.com/darkliquid/ironwail-go/internal/menu"
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
