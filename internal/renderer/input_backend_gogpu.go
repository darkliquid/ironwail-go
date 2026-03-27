//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	iinput "github.com/ironwail/ironwail-go/internal/input"
	gogpuimpl "github.com/ironwail/ironwail-go/internal/renderer/gogpu"
)

// InputBackendForSystem returns a Backend implementation wired to this renderer's app.
func (r *Renderer) InputBackendForSystem(sys *iinput.System) iinput.Backend {
	return gogpuimpl.NewInputBackend(r.app, sys)
}
