//go:build !gogpu
// +build !gogpu

package renderer

import (
	iinput "github.com/darkliquid/ironwail-go/internal/input"
	stubimpl "github.com/darkliquid/ironwail-go/internal/renderer/stub"
)

// InputBackendForSystem is a no-op on builds that don't provide a platform backend.
func (r *Renderer) InputBackendForSystem(sys *iinput.System) iinput.Backend {
	return stubimpl.InputBackendForSystem(sys)
}
