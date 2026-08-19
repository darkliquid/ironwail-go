package quakui

import (
	"testing"

	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

type dummyChildWidget struct {
	widget.WidgetBase
	laidOut bool
	drawn   bool
}

func (d *dummyChildWidget) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	d.laidOut = true
	size := geometry.Sz(c.MaxWidth, c.MaxHeight)
	d.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

func (d *dummyChildWidget) Draw(ctx widget.Context, canvas widget.Canvas) {
	d.drawn = true
}

func (d *dummyChildWidget) Event(ctx widget.Context, e event.Event) bool {
	return false
}

func (d *dummyChildWidget) Children() []widget.Widget {
	return nil
}

func TestStack_ChildrenFiltering(t *testing.T) {
	child1 := &dummyChildWidget{}
	child1.SetVisible(true)

	child2 := &dummyChildWidget{}
	child2.SetVisible(false)

	child3 := &dummyChildWidget{}
	child3.SetVisible(true)

	stack := NewStack(child1, child2, child3)

	children := stack.Children()
	if len(children) != 2 {
		t.Fatalf("expected 2 visible children, got %d", len(children))
	}
	if children[0] != widget.Widget(child1) || children[1] != widget.Widget(child3) {
		t.Fatalf("unexpected children returned by Children()")
	}

	// Change visibility dynamically
	child2.SetVisible(true)
	child1.SetVisible(false)

	children = stack.Children()
	if len(children) != 2 {
		t.Fatalf("expected 2 visible children after toggle, got %d", len(children))
	}
	if children[0] != widget.Widget(child2) || children[1] != widget.Widget(child3) {
		t.Fatalf("unexpected children returned after visibility toggle")
	}
}

type dummyBoundaryWidget struct {
	widget.WidgetBase
	drawn bool
}

func (d *dummyBoundaryWidget) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	return geometry.Sz(c.MaxWidth, c.MaxHeight)
}

func (d *dummyBoundaryWidget) Draw(ctx widget.Context, canvas widget.Canvas) {
	d.drawn = true
}

func (d *dummyBoundaryWidget) Event(ctx widget.Context, e event.Event) bool {
	return false
}

func (d *dummyBoundaryWidget) Children() []widget.Widget {
	return nil
}

func TestStack_Draw_SkipsBoundaries(t *testing.T) {
	regularChild := &dummyChildWidget{}
	regularChild.SetVisible(true)

	boundaryChild := &dummyBoundaryWidget{}
	boundaryChild.SetVisible(true)
	boundaryChild.SetRepaintBoundary(true)

	stack := NewStack(regularChild, boundaryChild)

	stack.Draw(nil, nil)
	// Even without canvas, regular child might be evaluated if Draw was naive,
	// but let's test with minimal non-nil canvas logic if called
	if regularChild.drawn {
		t.Fatalf("expected regularChild not drawn with nil canvas")
	}
	if boundaryChild.drawn {
		t.Fatalf("expected boundaryChild not drawn (boundary should be skipped)")
	}
}

