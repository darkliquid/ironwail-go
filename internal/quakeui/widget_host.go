package quakeui

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/quakeui/theme"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/widget"
)

// Surface identifies the active UI surface (spec §3.2). The engine's KeyDest
// router decides which surface is active; the host keeps one root per surface
// and swaps the window root to match (ADR-0002).
type Surface int

const (
	// SurfaceNone means no gogpu/ui surface is active (gameplay only).
	SurfaceNone Surface = iota
	// SurfaceMenu is the main menu + submenu page tree.
	SurfaceMenu
	// SurfaceConsole is the dropdown console.
	SurfaceConsole
	// SurfaceHUD is the in-game HUD.
	SurfaceHUD
)

// String returns a stable identifier for the surface.
func (s Surface) String() string {
	switch s {
	case SurfaceMenu:
		return "menu"
	case SurfaceConsole:
		return "console"
	case SurfaceHUD:
		return "hud"
	default:
		return "none"
	}
}

// HostOptions configures the gogpu/ui host.
type HostOptions struct {
	// Provider is the gogpu GPU context provider (gogpu.App.GPUContextProvider()).
	// It satisfies gpucontext.DeviceProvider + WindowProvider + PlatformProvider.
	// When nil the host runs headless (app.New with no providers), used by
	// unit tests and the headless boot path.
	Provider gpucontext.DeviceProvider
	// ProviderFunc lazily resolves the GPU context provider on first DrawTo.
	// This is the preferred path for the engine: the gogpu provider is only
	// available after the app's renderer is initialized (during Run), so the
	// host must resolve it lazily rather than at construction.
	ProviderFunc func() gpucontext.DeviceProvider
	// EventSource feeds OS input into the ui tree (ADR-0003 gateway). When nil
	// no events are delivered (headless tests).
	EventSource gpucontext.EventSource
	// Gateway is the engine input gateway (ADR-0003). When non-nil it is used
	// as the EventSource and stored on the host so the game can feed engine
	// input events into it per frame.
	Gateway *Gateway
}

// Host owns the gogpu/ui app lifecycle inside the engine frame (spec §3.1,
// ADR-0002). The engine keeps its own window/swapchain/render loop; per frame
// the host runs uiApp.Frame() then DrawTo(render.NewCanvas(cc,w,h)) and
// composites the widget canvas onto the engine surface.
type Host struct {
	app      *app.App
	provider gpucontext.DeviceProvider
	providerFunc func() gpucontext.DeviceProvider
	gateway  *Gateway
	canvas   *canvasBridge
}

// NewHost constructs the gogpu/ui host. With a nil provider (and no
// ProviderFunc) it runs headless (used by tests and the headless boot path);
// with a provider it wires the ui app to the engine's gogpu.App so the widget
// tree draws into the engine surface (AC3: boots).
func NewHost(opts HostOptions) *Host {
	// Resolve the initial provider: the explicit provider wins, otherwise the
	// ProviderFunc is called lazily (the gogpu provider is only available after
	// the app renderer initializes during Run).
	initialProvider := opts.Provider
	if initialProvider == nil && opts.ProviderFunc != nil {
		initialProvider = opts.ProviderFunc()
	}
	h := &Host{
		provider:     initialProvider,
		providerFunc: opts.ProviderFunc,
		gateway:      opts.Gateway,
	}

	appOpts := []app.Option{
		app.WithTheme(theme.QuakeTheme()),
		app.WithRenderMode(app.RenderModeHostManaged),
	}
	if wp, ok := initialProvider.(gpucontext.WindowProvider); ok && wp != nil {
		appOpts = append(appOpts, app.WithWindowProvider(wp))
	}
	if pp, ok := initialProvider.(gpucontext.PlatformProvider); ok && pp != nil {
		appOpts = append(appOpts, app.WithPlatformProvider(pp))
	}
	es := opts.EventSource
	if opts.Gateway != nil {
		es = opts.Gateway
	}
	if es != nil {
		appOpts = append(appOpts, app.WithEventSource(es))
	}

	h.app = app.New(appOpts...)
	h.canvas = newCanvasBridge(h.resolveProvider)
	return h
}

// resolveProvider returns the current GPU context provider, resolving it
// lazily from ProviderFunc when the initial provider was nil (the gogpu
// provider becomes available once the app renderer initializes during Run).
func (h *Host) resolveProvider() gpucontext.DeviceProvider {
	if h == nil {
		return nil
	}
	if h.provider != nil {
		return h.provider
	}
	if h.providerFunc != nil {
		h.provider = h.providerFunc()
	}
	return h.provider
}

// Gateway returns the engine input gateway, or nil if none was configured.
func (h *Host) Gateway() *Gateway {
	if h == nil {
		return nil
	}
	return h.gateway
}

// App returns the underlying gogpu/ui app.
func (h *Host) App() *app.App {
	if h == nil {
		return nil
	}
	return h.app
}

// SetRoot swaps the window root widget. The old root tree is unmounted and
// the new root is mounted on the next Frame (spec §3.2, ADR-0002).
func (h *Host) SetRoot(root widget.Widget) {
	if h == nil || h.app == nil {
		return
	}
	h.app.SetRoot(root)
}

// Frame runs one widget-tree frame (layout, draw, state). Call once per
// engine frame before DrawTo (spec §5.1).
func (h *Host) Frame() {
	if h == nil || h.app == nil {
		return
	}
	h.app.Frame()
}

// DrawTo renders the widget tree into a canvas bridged from the engine's gg
// context and composites it onto the engine surface via the given render
// target (the engine's gogpu.ContextRenderTarget, which implements
// ggcanvas.RenderTarget). With a nil provider it renders into a software
// canvas (headless tests) and never presents. Returns an error if the canvas
// could not be created or composited.
func (h *Host) DrawTo(dc ggcanvas.RenderTarget) error {
	if h == nil || h.app == nil {
		return nil
	}
	win := h.app.Window()

	// Resolve the real window size from the provider (the gogpu provider is
	// only available after Run). The widget tree must lay out at the engine's
	// surface size, not the headless default.
	provider := h.resolveProvider()
	w, hgt := 800, 600
	if wp, ok := provider.(gpucontext.WindowProvider); ok && wp != nil {
		if cw, ch := wp.Size(); cw > 0 && ch > 0 {
			w, hgt = cw, ch
		}
	}
	sz := win.WindowSize()
	if int(sz.Width) != w || int(sz.Height) != hgt {
		win.HandleResize(w, hgt)
	}

	if err := h.canvas.ensure(w, hgt); err != nil {
		return err
	}
	widgetCanvas := h.canvas.widgetCanvas(w, hgt)
	if widgetCanvas == nil {
		return nil
	}

	win.DrawTo(widgetCanvas)
	return h.canvas.present(dc, w, hgt)
}

// Close releases the canvas and any GPU resources owned by the host.
func (h *Host) Close() {
	if h == nil {
		return
	}
	if h.canvas != nil {
		h.canvas.close()
	}
	if h.app != nil {
		h.app.Window().Close()
	}
}

var _ = slog.Default
