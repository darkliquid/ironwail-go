package quakui

import (
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/desktop"
	"github.com/gogpu/ui/widget"
)

// Run boots the gogpu/ui host on the ui_backend=1 path (ADR-0006, research
// 0006). It builds the ui app from the engine's gogpu.App (window/platform
// provider), sets the given root (the gpuview world texture plus the active
// surface widgets), and calls desktop.Run, which owns the window render loop
// and composites the gpuview texture (world) under the UI widgets. The world
// is never cleared by the UI composite (research 0006 §3); the engine's
// OnUpdate simulation keeps running via desktop.Run's loop.
func Run(host Host, root widget.Widget) error {
	uiApp := app.New(
		app.WithWindowProvider(host.GogpuApp()),
		app.WithPlatformProvider(host.GogpuApp()),
		app.WithTheme(QuakeTheme()),
	)
	uiApp.SetRoot(root)
	return desktop.Run(host.GogpuApp(), uiApp)
}
