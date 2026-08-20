package quakui

import (
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// Stack is a container that lays out all children to the same bounds,
// overlapping them in insertion order (last child drawn on top). This is the
// surface-stacking model for the v2 UI (spec §3.2): the gpuview world texture
// is the base layer and the menu/console/HUD surfaces stack above, each
// toggled by visibility so overlapping surfaces are layered rather than
// mutually exclusive. It is a BYO-kit container (AC8 — no stock core/* widget).
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

// Children returns the visible stacked children in z-order.
func (s *Stack) Children() []widget.Widget {
	if s == nil {
		return nil
	}
	visible := make([]widget.Widget, 0, len(s.children))
	for _, child := range s.children {
		if vs, ok := child.(visibilityChecker); ok && !vs.IsVisible() {
			continue
		}
		visible = append(visible, child)
	}
	return visible
}

// visibilityChecker is implemented by widgets that can report visibility.
type visibilityChecker interface {
	IsVisible() bool
}

// boundsSetter is implemented by widgets embedding widget.WidgetBase.
type boundsSetter interface {
	SetBounds(geometry.Rect)
}

// Layout lays out all children to the same constrained size (overlapping).
func (s *Stack) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := geometry.Sz(c.MaxWidth, c.MaxHeight)
	for _, child := range s.children {
		child.Layout(ctx, c)
		if bs, ok := child.(boundsSetter); ok {
			bs.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
		}
	}
	s.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw renders stacked children in z-order. External texture widgets
// (such as WorldTexture) are handled by the compositor blit path.
func (s *Stack) Draw(ctx widget.Context, canvas widget.Canvas) {
	if s == nil || canvas == nil {
		return
	}
	type boundaryChecker interface {
		IsRepaintBoundary() bool
	}
	for _, child := range s.children {
		if bc, ok := child.(boundaryChecker); ok && bc.IsRepaintBoundary() {
			continue
		}
		if _, ok := child.(interface{ Texture() gpucontext.TextureView }); ok {
			continue
		}
		visible := true
		if vs, ok := child.(visibilityChecker); ok {
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
		if vs, ok := child.(visibilityChecker); ok {
			visible = vs.IsVisible()
		}
		if visible && child.Event(ctx, e) {
			return true
		}
	}
	return false
}