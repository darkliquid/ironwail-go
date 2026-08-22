package quakeui

import (
	"fmt"
	"image"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/draw"
	legacymenu "github.com/darkliquid/ironwail-go/internal/menu"
	quakeuiconsole "github.com/darkliquid/ironwail-go/internal/quakeui/console"
	quakeuidemobar "github.com/darkliquid/ironwail-go/internal/quakeui/demobar"
	"github.com/darkliquid/ironwail-go/internal/quakeui/gfx"
	quakeuihud "github.com/darkliquid/ironwail-go/internal/quakeui/hud"
	quakeuimenu "github.com/darkliquid/ironwail-go/internal/quakeui/menu"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	"github.com/gogpu/gg"
	"github.com/gogpu/gg/integration/ggcanvas"
	uiapp "github.com/gogpu/ui/app"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
)

// OverlayRenderer coordinates 2D UI rendering directly onto the engine's
// active swapchain texture view (Scenario A composite, ADR-0011). It hosts
// the widget stack (HUD, Console, Menu) in an engine-owned overlay pass using
// a GPU-backed ggcanvas: the stack draws into widget.Canvas, then
// canvas.RenderDirect(sv, sw, sh) composites with LoadOp::Load so the 3D
// world pass is preserved.
type OverlayRenderer struct {
	host     Host
	stack    *Stack
	menuRoot *quakeuimenu.MenuRoot
	conRoot  *quakeuiconsole.ConsoleRoot
	hudRoot  *quakeuihud.HUDRoot
	demoBar  *quakeuidemobar.DemoBarRoot

	// uiApp is the gogpu/ui application hosting the widget root. It is
	// created once at startup on the ui_backend=1 path (G11) and reused for
	// the session; the engine-owned loop drives Window.DrawTo each frame
	// rather than App.Frame (no desktop.Run).
	uiApp *uiapp.App

	// gpuCanvas is the GPU-accelerated ggcanvas, created on first draw once a
	// provider is available (lazily, so WASM-after-Run ordering is safe P2).
	// When nil (no provider yet / software fallback), drawing uses a plain CPU
	// gg.Context and reads back via target.DrawRGBA as v3 did.
	gpuCanvas *ggcanvas.Canvas

	// cpuDC is the software fallback gg.Context (created lazily; also used as
	// the backing context when no GPU provider is present).
	cpuDC *gg.Context

	// lastProvider records the provider that created gpuCanvas so nil-provider
	// polls do not recreate it every frame.
	lastProviderName string

	width          int
	height         int
	drawCount      uint64
	lastLogMenuVis bool
	lastLogConVis  bool
	lastLogHUDVis  bool
}

// hostWindowProvider adapts the engine Host to gpucontext.WindowProvider so
// the ui window mirrors the engine's real viewport (size + redraw requests).
// The engine owns the frame loop; RequestRedraw is a no-op here because the
// engine drives each frame (continuous on native, rAF on wasm).
type hostWindowProvider struct {
	host Host
}

func (w *hostWindowProvider) Size() (width, height int) {
	if w == nil || w.host == nil {
		return 0, 0
	}
	return w.host.WindowSize()
}

func (w *hostWindowProvider) ScaleFactor() float64 { return 1.0 }

func (w *hostWindowProvider) RequestRedraw() {}

// NewOverlayRenderer constructs an OverlayRenderer with the standard Quake UI
// widget stack. It also creates the gogpu/ui app (headless construction is
// safe before gogpu.Run, ADR-0011) and installs the stack as its window root
// so the engine-owned loop can drive Window.DrawTo on the GPU canvas.
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
	demoBar := quakeuidemobar.NewDemoBarRoot(nil, conchars, palette)

	stack := NewStack(hudRoot, conRoot, menuRoot, demoBar)

	// Create the ui app once at startup (G11). The window provider mirrors the
	// engine's real framebuffer size so widget layout and the 320x200 menu
	// transform match the viewport; the engine still owns the frame loop and
	// the app window is only the retained widget-tree host (DrawTo contract).
	a := uiapp.New(
		uiapp.WithRenderMode(uiapp.RenderModeHostManaged),
		uiapp.WithWindowProvider(&hostWindowProvider{host: host}),
	)
	a.SetRoot(stack)

	return &OverlayRenderer{
		host:     host,
		stack:    stack,
		menuRoot: menuRoot,
		conRoot:  conRoot,
		hudRoot:  hudRoot,
		demoBar:  demoBar,
		uiApp:    a,
	}
}

// SetDemoBarStateProvider wires the per-frame demo playback state source
// (the engine's DemoState adapter) into the display-only demo bar.
func (r *OverlayRenderer) SetDemoBarStateProvider(p quakeuidemobar.StateProvider) {
	if r == nil || r.demoBar == nil {
		return
	}
	r.demoBar.SetProvider(p)
}

// DemoBar returns the underlying demo bar widget.
func (r *OverlayRenderer) DemoBar() *quakeuidemobar.DemoBarRoot {
	if r == nil {
		return nil
	}
	return r.demoBar
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

// DrawOverlay records and draws the 2D widget overlay onto the target
// RenderContext. Per-frame it lays out and draws the widget stack into a
// canvas and composites the result:
//   - GPU path (Scenario A): the stack draws into a GPU-backed ggcanvas, then
//     RenderDirect(sv, sw, sh) flushes it onto the swapchain surface with
//     LoadOp::Load (the world pass is preserved via MarkPreserveContent).
//   - Software fallback (no provider / headless): CPU gg.Context readback via
//     target.DrawRGBA as v3 did.
//
// The engine-owned loop invokes this inside RenderFrame's overlay phase; the
// uiApp window root is only used for widget state (DrawTo contract), the
// engine drives drawing.
func (r *OverlayRenderer) DrawOverlay(target renderer.RenderContext, width, height int) error {
	if r == nil || width <= 0 || height <= 0 {
		return nil
	}

	// Resolve the backing canvas: create the GPU canvas once a provider is
	// available, resize on dimension change, else fall back to software.
	gpu, err := r.ensureCanvas(width, height)
	if err != nil {
		slog.Warn("quakeui overlay: GPU canvas unavailable; software fallback", "error", err)
		gpu = nil
	}

	var canvas widget.Canvas
	var cc *gg.Context
	if gpu != nil {
		cc = gpu.Context()
	} else {
		if r.cpuDC == nil || r.width != width || r.height != height {
			r.cpuDC = gg.NewContext(width, height)
		}
		cc = r.cpuDC
	}
	canvas = render.NewCanvas(cc, width, height)

	ctx := widget.NewContext()
	ctx.SetWindowSize(geometry.Sz(float32(width), float32(height)))

	r.stack.Layout(ctx, geometry.Loose(geometry.Sz(float32(width), float32(height))))

	if gpu != nil {
		// GPU canvas: clear transparent first — HostManaged DrawTo does NOT
		// clear (the host paints the background), so without this the widget
		// tree accumulates onto the previous frame: menu items ghost, sub
		// menus draw over each other, and the cursor stays painted. The
		// transparent clear leaves the world pass visible under the overlay
		// (RenderDirect/LoadOp::Load composites on top).
		cc.Identity()
		cc.Clear()

		// GPU canvas: let the ui window drive the full retained tree
		// (Window.DrawTo runs layout + draw of the root stack) so widget
		// state (SetNeedsRedraw/invalidation) is honored, then RenderDirect
		// composites onto the preserved surface.
		if r.uiApp == nil || r.uiApp.Window() == nil {
			// Fall back to the direct-stack draw if the ui app is missing.
			gpu.Draw(func(dcc *gg.Context) {
				wi := widget.NewContext()
				wi.SetWindowSize(geometry.Sz(float32(width), float32(height)))
				r.stack.Draw(wi, canvas)
			})
		} else {
			gpu.Draw(func(dcc *gg.Context) {
				r.uiApp.Window().DrawTo(canvas)
			})
		}
	} else {
		// Software fallback: clear + draw the tree, then blit via DrawRGBA
		// (v3 behavior, CPU readback).
		cc.Clear()
		r.stack.Draw(ctx, canvas)
		if target != nil {
			img := cc.Image()
			if rgba, ok := img.(*image.RGBA); ok {
				target.DrawRGBA(0, 0, rgba)
			}
		}
		return r.drawCountLogged(target, width, height)
	}

	// GPU path finalize: present the pixmap onto the current surface.
	// Production uses the target-aware ggcanvas Render (borrows the frame's
	// shared encoder, never submits against a retired surface). Tests and
	// targets without a live RenderTarget fall back to RenderDirect against
	// the surface view, then to a readback blit.
	// GPU path finalize: composite the overlay pixmap onto the preserved
	// surface via the engine's overlay blend (LoadOpLoad over the world).
	// Without the gg/gpu accelerator (which corrupts the engine's swapchain
	// lifetime when bound into the same device), RenderDirect/GPU-direct is
	// unavailable; the proven v3 composite is the CPU-readback blit.
	if target == nil {
		r.drawCountLogged(target, width, height)
		return nil
	}
	r.blitToTarget(target, r.gpuCanvasContextImage(gpu))
	return r.drawCountLogged(target, width, height)
}

// gpuCanvasContextImage returns the ggcanvas's CPU pixmap as RGBA (the readback
// used when the GPU-direct composite is unavailable).
func (r *OverlayRenderer) gpuCanvasContextImage(gpu *ggcanvas.Canvas) *image.RGBA {
	if gpu == nil || gpu.Context() == nil {
		return nil
	}
	img := gpu.Context().Image()
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	return nil
}

// blitToTarget draws the RGBA pixmap through the render context. Uses the
// fresh-texture draw when available so the cached engine overlay texture
// (whose lifetime may be retired by a surface retarget) is never reused.
func (r *OverlayRenderer) blitToTarget(target renderer.RenderContext, rgba *image.RGBA) {
	if target == nil || rgba == nil {
		return
	}
	if fr, ok := target.(interface{ DrawRGBAFresh(int, int, *image.RGBA) }); ok {
		fr.DrawRGBAFresh(0, 0, rgba)
	} else {
		target.DrawRGBA(0, 0, rgba)
	}
}

// drawCountLogged centralizes the per-frame draw-count telemetry that the v3
// overlay emitted after every overlay pass.
func (r *OverlayRenderer) drawCountLogged(_ renderer.RenderContext, width, height int) error {
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
		)
	}
	return nil
}

// ensureCanvas returns the GPU-backed canvas for the current dimensions,
// creating it lazily once a provider is available. A nil provider means the
// overlay must fall back to software rendering (headless, pre-Run, or a
// software adapter — AC3c fail-open).
func (r *OverlayRenderer) ensureCanvas(width, height int) (*ggcanvas.Canvas, error) {
	provider := r.host.GPUContextProvider()
	if provider == nil {
		r.gpuCanvas = nil
		return nil, nil
	}
	if r.gpuCanvas != nil && r.width == width && r.height == height {
		return r.gpuCanvas, nil
	}
	canvas, err := ggcanvas.New(provider, width, height)
	if err != nil {
		r.gpuCanvas = nil
		return nil, err
	}
	r.gpuCanvas = canvas
	r.width = width
	r.height = height
	return canvas, nil
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

// Close releases the GPU canvas owned by the overlay. Called from the engine
// teardown (gogpuApp.OnClose, G11 lifecycle). Safe on the software path where
// no GPU canvas was created.
func (r *OverlayRenderer) Close() {
	if r == nil {
		return
	}
	if r.gpuCanvas != nil {
		_ = r.gpuCanvas.Close()
		r.gpuCanvas = nil
	}
}
