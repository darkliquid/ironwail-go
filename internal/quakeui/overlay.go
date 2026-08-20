package quakeui

import (
	"fmt"
	"image"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/draw"
	legacymenu "github.com/darkliquid/ironwail-go/internal/menu"
	quakeuiconsole "github.com/darkliquid/ironwail-go/internal/quakeui/console"
	"github.com/darkliquid/ironwail-go/internal/quakeui/gfx"
	quakeuihud "github.com/darkliquid/ironwail-go/internal/quakeui/hud"
	quakeuimenu "github.com/darkliquid/ironwail-go/internal/quakeui/menu"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/gogpu/gg"
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
	menuRoot *quakeuimenu.MenuRoot
	conRoot  *quakeuiconsole.ConsoleRoot
	hudRoot  *quakeuihud.HUDRoot

	dc             *gg.Context
	width          int
	height         int
	drawCount      uint64
	lastLogMenuVis bool
	lastLogConVis  bool
	lastLogHUDVis  bool
}

// NewOverlayRenderer constructs an OverlayRenderer with the standard Quake UI widget stack.
func NewOverlayRenderer(
	host Host,
	mgr *legacymenu.Manager,
	con *console.Console,
	hudProv quakeuihud.HUDStateProvider,
	drawMgr *draw.Manager,
	conchars []byte,
	palette []byte,
) *OverlayRenderer {
	atlas := gfx.NewConcharsAtlas(conchars, palette)
	menuRoot := quakeuimenu.NewMenuRoot(mgr, drawMgr, conchars, palette)
	conRoot := quakeuiconsole.NewConsoleRoot(con, drawMgr, atlas)
	hudRoot := quakeuihud.NewHUDRoot(hudProv, drawMgr, conchars, palette)

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
func (r *OverlayRenderer) MenuRoot() *quakeuimenu.MenuRoot {
	if r == nil {
		return nil
	}
	return r.menuRoot
}

// ConsoleRoot returns the underlying ConsoleRoot widget.
func (r *OverlayRenderer) ConsoleRoot() *quakeuiconsole.ConsoleRoot {
	if r == nil {
		return nil
	}
	return r.conRoot
}

// HUDRoot returns the underlying HUDRoot widget.
func (r *OverlayRenderer) HUDRoot() *quakeuihud.HUDRoot {
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

// DrawOverlay records and draws the 2D widget overlay onto the target RenderContext.
func (r *OverlayRenderer) DrawOverlay(target renderer.RenderContext, width, height int) error {
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

	r.drawCount++
	menuVis := r.menuRoot != nil && r.menuRoot.IsVisible()
	conVis := r.conRoot != nil && r.conRoot.IsVisible()
	hudVis := r.hudRoot != nil && r.hudRoot.IsVisible()
	if r.drawCount <= 5 || r.drawCount%300 == 0 || menuVis != r.lastLogMenuVis || conVis != r.lastLogConVis || hudVis != r.lastLogHUDVis {
		r.lastLogMenuVis = menuVis
		r.lastLogConVis = conVis
		r.lastLogHUDVis = hudVis
		slog.Debug("quakeui overlay draw",
			"frame", r.drawCount,
			"width", width, "height", height,
			"menu_vis", menuVis,
			"con_vis", conVis,
			"hud_vis", hudVis,
			"target_nil", target == nil,
		)
	}

	if target != nil {
		img := r.dc.Image()
		if rgba, ok := img.(*image.RGBA); ok {
			target.DrawRGBA(0, 0, rgba)
		}
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
	handled := r.stack.Event(ctx, e)
	slog.Debug("quakeui overlay event",
		"event", fmt.Sprintf("%T", e),
		"handled", handled,
		"menu_vis", r.menuRoot != nil && r.menuRoot.IsVisible(),
		"con_vis", r.conRoot != nil && r.conRoot.IsVisible(),
	)
	return handled
}

// HandleEvent implements the HandleEvents interface so KeyForwarder can dispatch into the stack.
func (r *OverlayRenderer) HandleEvent(e event.Event) {
	r.Event(e)
}
