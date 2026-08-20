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

type dummyCanvas struct {
	widget.Canvas
}

func TestStack_Draw(t *testing.T) {
	child1 := &dummyChildWidget{}
	child1.SetVisible(true)

	child2 := &dummyChildWidget{}
	child2.SetVisible(false)

	child3 := &dummyChildWidget{}
	child3.SetVisible(true)

	stack := NewStack(child1, child2, child3)
	canvas := &dummyCanvas{}
	stack.Draw(nil, canvas)

	if !child1.drawn {
		t.Fatalf("expected child1 drawn")
	}
	if child2.drawn {
		t.Fatalf("expected child2 not drawn (invisible)")
	}
	if !child3.drawn {
		t.Fatalf("expected child3 drawn")
	}
}

