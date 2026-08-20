package quakui

import (
	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/draw"
	legacymenu "github.com/darkliquid/ironwail-go/internal/menu"
	quakuiconsole "github.com/darkliquid/ironwail-go/internal/quakui/console"
	"github.com/darkliquid/ironwail-go/internal/quakui/gfx"
	quakuihud "github.com/darkliquid/ironwail-go/internal/quakui/hud"
	quakuimenu "github.com/darkliquid/ironwail-go/internal/quakui/menu"
	"github.com/gogpu/gg"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
)

// OverlayRenderer coordinates 2D UI rendering directly onto the engine's
// active swapchain texture view (SPEC-003, ADR-0010). It hosts the widget
// stack (HUD, Console, Menu) in an engine-owned overlay pass using
// FlushGPUWithViewPreserveContent (LoadOpLoad) so the 3D world is preserved.
type OverlayRenderer struct {
	host     Host
	stack    *Stack
	menuRoot *quakuimenu.MenuRoot
	conRoot  *quakuiconsole.ConsoleRoot
	hudRoot  *quakuihud.HUDRoot

	dc     *gg.Context
	width  int
	height int
}

// NewOverlayRenderer constructs an OverlayRenderer with the standard Quake UI widget stack.
func NewOverlayRenderer(
	host Host,
	mgr *legacymenu.Manager,
	con *console.Console,
	hudProv quakuihud.HUDStateProvider,
	drawMgr *draw.Manager,
	conchars []byte,
	palette []byte,
) *OverlayRenderer {
	atlas := gfx.NewConcharsAtlas(conchars, palette)
	menuRoot := quakuimenu.NewMenuRoot(mgr, drawMgr, conchars, palette)
	conRoot := quakuiconsole.NewConsoleRoot(con, drawMgr, atlas)
	hudRoot := quakuihud.NewHUDRoot(hudProv, drawMgr, conchars, palette)

	stack := NewStack(hudRoot, conRoot, menuRoot)

	return &OverlayRenderer{
		host:     host,
		stack:    stack,
		menuRoot: menuRoot,
		conRoot:  conRoot,
		hudRoot:  hudRoot,
	}
}

// MenuRoot returns the underlying MenuRoot widget.
func (r *OverlayRenderer) MenuRoot() *quakuimenu.MenuRoot {
	if r == nil {
		return nil
	}
	return r.menuRoot
}

// ConsoleRoot returns the underlying ConsoleRoot widget.
func (r *OverlayRenderer) ConsoleRoot() *quakuiconsole.ConsoleRoot {
	if r == nil {
		return nil
	}
	return r.conRoot
}

// HUDRoot returns the underlying HUDRoot widget.
func (r *OverlayRenderer) HUDRoot() *quakuihud.HUDRoot {
	if r == nil {
		return nil
	}
	return r.hudRoot
}

// SetConsoleSlideFraction updates the dropdown animation fraction.
func (r *OverlayRenderer) SetConsoleSlideFraction(f float32) {
	if r != nil && r.conRoot != nil {
		r.conRoot.SetSlideFraction(f)
	}
}

// SetConsoleForcedUp updates whether the console is forced to occupy the screen.
func (r *OverlayRenderer) SetConsoleForcedUp(forced bool) {
	if r != nil && r.conRoot != nil {
		r.conRoot.SetForcedUp(forced)
	}
}

// DrawOverlay records and flushes the 2D widget overlay onto the target GPU texture view.
func (r *OverlayRenderer) DrawOverlay(targetView gpucontext.TextureView, width, height int) error {
	if r == nil || width <= 0 || height <= 0 {
		return nil
	}

	if r.dc == nil || r.width != width || r.height != height {
		r.width = width
		r.height = height
		r.dc = gg.NewContext(width, height)
	}

	r.dc.Clear()

	canvas := render.NewCanvas(r.dc, width, height)
	ctx := widget.NewContext()
	ctx.SetWindowSize(geometry.Sz(float32(width), float32(height)))

	r.stack.Layout(ctx, geometry.Loose(geometry.Sz(float32(width), float32(height))))
	r.stack.Draw(ctx, canvas)

	if !targetView.IsNil() {
		return r.dc.FlushGPUWithViewPreserveContent(targetView, uint32(width), uint32(height))
	}
	return nil
}

// Event routes an event to the topmost active widget in the stack.
func (r *OverlayRenderer) Event(e event.Event) bool {
	if r == nil || r.stack == nil {
		return false
	}
	ctx := widget.NewContext()
	ctx.SetWindowSize(geometry.Sz(float32(r.width), float32(r.height)))
	return r.stack.Event(ctx, e)
}

// HandleEvent implements the HandleEvents interface so KeyForwarder can dispatch into the stack.
func (r *OverlayRenderer) HandleEvent(e event.Event) {
	r.Event(e)
}
