package renderer

import (
	iinput "github.com/darkliquid/ironwail-go/internal/input"
	gogpuimpl "github.com/darkliquid/ironwail-go/internal/renderer/gogpu"
)

// InputBackendForSystem returns a Backend implementation wired to this
// renderer's app. It registers EventSource callbacks and falls back to
// polling app.Input() when callbacks are not observed.
func (r *Renderer) InputBackendForSystem(sys *iinput.System) iinput.Backend {
	return gogpuimpl.NewInputBackend(r.app, sys)
}

// PollOnlyInputBackendForSystem returns a Backend implementation that polls
// app.Input() exclusively (ADR-0012 §5.2: engine gameplay input migrates to
// polling gogpuApp.Input()). It never registers EventSource callbacks, so the
// UI owns the EventSource on the gogpu/ui path without double-delivery.
func (r *Renderer) PollOnlyInputBackendForSystem(sys *iinput.System) iinput.Backend {
	return gogpuimpl.NewPollOnlyInputBackend(r.app, sys)
}
