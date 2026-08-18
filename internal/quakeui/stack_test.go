package quakeui

import (
	"testing"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// stackTestWidget is a minimal widget for stacking tests.
type stackTestWidget struct {
	widget.WidgetBase
	name string
}

func (s *stackTestWidget) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := c.Constrain(geometry.Sz(320, 200))
	s.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

func (s *stackTestWidget) Draw(ctx widget.Context, canvas widget.Canvas) {}
func (s *stackTestWidget) Event(ctx widget.Context, e event.Event) bool   { return false }

// TestStackLaysOutChildren asserts the Stack container lays out all children
// to the same bounds (overlapping layers, ADR-0002).
func TestStackLaysOutChildren(t *testing.T) {
	a := &stackTestWidget{name: "a"}
	b := &stackTestWidget{name: "b"}
	st := NewStack(a, b)

	ctx := widget.NewContext()
	size := st.Layout(ctx, geometry.Tight(geometry.Sz(320, 200)))
	if size.Width != 320 || size.Height != 200 {
		t.Fatalf("Stack Layout size = %v, want 320x200", size)
	}
	if len(st.Children()) != 2 {
		t.Fatalf("Stack children = %d, want 2", len(st.Children()))
	}
}

// TestStackVisibility asserts children can be toggled independently (surface
// stacking: HUD below menu below console).
func TestStackVisibility(t *testing.T) {
	hud := &stackTestWidget{name: "hud"}
	menu := &stackTestWidget{name: "menu"}
	console := &stackTestWidget{name: "console"}
	st := NewStack(hud, menu, console)

	// Game: HUD only.
	hud.SetVisible(true)
	menu.SetVisible(false)
	console.SetVisible(false)
	if !st.ChildVisible(0) || st.ChildVisible(1) || st.ChildVisible(2) {
		t.Fatal("game stacking: expected HUD visible only")
	}

	// Game + menu: HUD + menu.
	menu.SetVisible(true)
	if !st.ChildVisible(0) || !st.ChildVisible(1) || st.ChildVisible(2) {
		t.Fatal("game+menu stacking: expected HUD+menu visible")
	}

	// Console over menu: console on top (last child).
	console.SetVisible(true)
	if !st.ChildVisible(2) {
		t.Fatal("console over menu: expected console visible")
	}
}

// TestStackOrder asserts the z-order is insertion order (last child on top).
func TestStackOrder(t *testing.T) {
	hud := &stackTestWidget{name: "hud"}
	console := &stackTestWidget{name: "console"}
	st := NewStack(hud, console)

	children := st.Children()
	if children[0] != widget.Widget(hud) || children[1] != widget.Widget(console) {
		t.Fatal("Stack z-order not insertion order (console should be on top)")
	}
}
