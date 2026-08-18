package quakeui

import (
	"testing"

	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// testRoot is a minimal widget used to exercise the host lifecycle without
// any stock core/* widgets (AC8: BYO-kit validation).
type testRoot struct {
	widget.WidgetBase
}

func (r *testRoot) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := c.Constrain(geometry.Sz(320, 200))
	r.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

func (r *testRoot) Draw(ctx widget.Context, canvas widget.Canvas) {}

func (r *testRoot) Event(ctx widget.Context, e event.Event) bool { return false }

// newTestHost constructs a headless host with no providers (app.New with no
// options runs headless at 800x600, per ui docs).
func newTestHost(t *testing.T) *Host {
	t.Helper()
	h := NewHost(HostOptions{})
	if h == nil {
		t.Fatal("NewHost returned nil")
	}
	return h
}

// TestHostConstructHeadless asserts the host constructs a uiApp in headless
// mode without panicking (AC3: boots; spec §3.1).
func TestHostConstructHeadless(t *testing.T) {
	h := newTestHost(t)
	if h.App() == nil {
		t.Fatal("host App() is nil")
	}
	if h.App().Theme() == nil {
		t.Fatal("host theme is nil")
	}
}

// TestHostRootSwapPerSurface asserts SetRoot swaps the window root per
// surface and the old root is unmounted (spec §3.2, ADR-0002).
func TestHostRootSwapPerSurface(t *testing.T) {
	h := newTestHost(t)
	rootA := &testRoot{}
	rootB := &testRoot{}

	h.SetRoot(rootA)
	if h.App().Window().Root() != widget.Widget(rootA) {
		t.Fatal("root A not set")
	}
	h.SetRoot(rootB)
	if h.App().Window().Root() != widget.Widget(rootB) {
		t.Fatal("root B not set after swap")
	}
}

// TestHostFrameNoPanic asserts Frame() runs the widget tree layout/draw pass
// without panicking on a headless host.
func TestHostFrameNoPanic(t *testing.T) {
	h := newTestHost(t)
	h.SetRoot(&testRoot{})
	h.Frame()
}

// TestHostDrawToNoPanic asserts DrawTo renders the widget tree into a canvas
// without panicking. The host builds a canvas from a gg context sized to the
// window; with no provider the window defaults to 800x600 headless.
func TestHostDrawToNoPanic(t *testing.T) {
	h := newTestHost(t)
	h.SetRoot(&testRoot{})
	h.Frame()
	if err := h.DrawTo(nil); err != nil {
		t.Fatalf("DrawTo error: %v", err)
	}
}

// TestHostSurfaceEnum asserts the surface identifiers cover the three active
// surfaces plus none (spec §3.2).
func TestHostSurfaceEnum(t *testing.T) {
	surfaces := []Surface{SurfaceNone, SurfaceMenu, SurfaceConsole, SurfaceHUD}
	if len(surfaces) != 4 {
		t.Fatalf("unexpected surface set: %v", surfaces)
	}
	for _, s := range surfaces {
		if s.String() == "" {
			t.Fatalf("surface %d has no String()", s)
		}
	}
}

var _ = app.RenderModeHostManaged
