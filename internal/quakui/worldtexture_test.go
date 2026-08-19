package quakui

import (
	"testing"
	"unsafe"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

type mockHostForTexture struct {
	renderedView gpucontext.TextureView
	renderCount  int
}

func (m *mockHostForTexture) GogpuApp() *gogpu.App { return nil }
func (m *mockHostForTexture) WorldTexture() gpucontext.TextureView {
	return m.renderedView
}
func (m *mockHostForTexture) RenderIntoWorldTexture(view gpucontext.TextureView) error {
	m.renderedView = view
	return nil
}
func (m *mockHostForTexture) RenderFrame() error {
	m.renderCount++
	return nil
}
func (m *mockHostForTexture) CVar(name string) float64 { return 0 }
func (m *mockHostForTexture) KeyDest() KeyDest          { return KeyDestMenu }
func (m *mockHostForTexture) ExecuteCommandText(text string) {}
func (m *mockHostForTexture) PlaySound(name string) {}
func (m *mockHostForTexture) Quit()                 {}
func (m *mockHostForTexture) AttachKeyForwarder(f Forwarder) {}

func TestWorldTexture_RepaintBoundaryAndExternalTextureWidget(t *testing.T) {
	host := &mockHostForTexture{}
	wt := NewWorldTexture(host, 320, 200)

	if !wt.IsRepaintBoundary() {
		t.Fatal("expected WorldTexture.IsRepaintBoundary() == true")
	}

	w, h := wt.ViewportSize()
	if w != 320 || h != 200 {
		t.Fatalf("ViewportSize = (%d, %d), want (320, 200)", w, h)
	}
}

func TestWorldTexture_DynamicLayoutResizing(t *testing.T) {
	host := &mockHostForTexture{}
	wt := NewWorldTexture(host, 320, 200)
	ctx := widget.NewContext()

	var createdW, createdH int
	ctx.SetOnCreateGPUTexture(func(width, height int) (any, func()) {
		createdW = width
		createdH = height
		dummy := uint64(0x1234)
		view := gpucontext.NewTextureView(unsafe.Pointer(&dummy))
		return view, func() {}
	})

	var animFrameScheduled bool
	ctx.SetOnScheduleAnimation(func() {
		animFrameScheduled = true
	})

	// Initial draw: creates 320x200 texture
	wt.Draw(ctx, nil)
	if createdW != 320 || createdH != 200 {
		t.Fatalf("initial texture size = (%d, %d), want (320, 200)", createdW, createdH)
	}
	if host.renderCount != 1 {
		t.Fatalf("renderCount = %d, want 1", host.renderCount)
	}
	if !animFrameScheduled {
		t.Fatal("expected ScheduleAnimationFrame() to be called")
	}

	// Resize window to 1892x1072
	wt.Layout(ctx, geometry.Loose(geometry.Sz(1892, 1072)))
	w, h := wt.ViewportSize()
	if w != 1892 || h != 1072 {
		t.Fatalf("resized ViewportSize = (%d, %d), want (1892, 1072)", w, h)
	}

	// Second draw: reallocates 1892x1072 texture
	wt.Draw(ctx, nil)
	if createdW != 1892 || createdH != 1072 {
		t.Fatalf("resized texture created size = (%d, %d), want (1892, 1072)", createdW, createdH)
	}
	if host.renderCount != 2 {
		t.Fatalf("renderCount = %d, want 2", host.renderCount)
	}
}
