package quakeui

import (
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// Stack is a container that lays out all children to the same bounds,
// overlapping them in insertion order (last child drawn on top). This is the
// surface-stacking model for the quakeui host (ADR-0002, G.14): the HUD,
// menu, and console surfaces are stacked children, each toggled by visibility,
// so overlapping surfaces (console forced-up over menu at boot, menu over
// frozen world + HUD) are layered rather than mutually exclusive.
type Stack struct {
	widget.WidgetBase

	children []widget.Widget
}

// NewStack builds a Stack with the given children in z-order (bottom to top).
func NewStack(children ...widget.Widget) *Stack {
	s := &Stack{children: children}
	s.SetVisible(true)
	s.SetEnabled(true)
	for _, child := range children {
		if ps, ok := child.(interface{ SetParent(widget.Widget) }); ok {
			ps.SetParent(s)
		}
	}
	return s
}

// Children returns the stacked children in z-order.
func (s *Stack) Children() []widget.Widget {
	if s == nil {
		return nil
	}
	return s.children
}

// visibilitySetter is implemented by widgets embedding widget.WidgetBase.
type visibilitySetter interface {
	IsVisible() bool
	SetVisible(bool)
}

// boundsSetter is implemented by widgets embedding widget.WidgetBase.
type boundsSetter interface {
	SetBounds(geometry.Rect)
}

// ChildVisible reports whether the child at index is visible.
func (s *Stack) ChildVisible(index int) bool {
	if s == nil || index < 0 || index >= len(s.children) {
		return false
	}
	if vs, ok := s.children[index].(visibilitySetter); ok {
		return vs.IsVisible()
	}
	return true
}

// Layout lays out all children to the same constrained size (overlapping).
func (s *Stack) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := c.Constrain(geometry.Sz(320, 200))
	for _, child := range s.children {
		child.Layout(ctx, c)
		if bs, ok := child.(boundsSetter); ok {
			bs.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
		}
	}
	s.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw renders the visible children in z-order (bottom to top).
func (s *Stack) Draw(ctx widget.Context, canvas widget.Canvas) {
	if s == nil {
		return
	}
	for _, child := range s.children {
		visible := true
		if vs, ok := child.(visibilitySetter); ok {
			visible = vs.IsVisible()
		}
		if visible {
			child.Draw(ctx, canvas)
		}
	}
}

// Event dispatches to the topmost visible child first (last in z-order).
func (s *Stack) Event(ctx widget.Context, e event.Event) bool {
	if s == nil {
		return false
	}
	for i := len(s.children) - 1; i >= 0; i-- {
		child := s.children[i]
		visible := true
		if vs, ok := child.(visibilitySetter); ok {
			visible = vs.IsVisible()
		}
		if visible && child.Event(ctx, e) {
			return true
		}
	}
	return false
}
