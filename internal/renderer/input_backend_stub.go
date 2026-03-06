//go:build !gogpu && !opengl && !cgo
// +build !gogpu,!opengl,!cgo

package renderer

import iinput "github.com/ironwail/ironwail-go/internal/input"

// InputBackendForSystem is a no-op on builds that don't provide a platform backend.
func (r *Renderer) InputBackendForSystem(sys *iinput.System) iinput.Backend {
	return nil
}
