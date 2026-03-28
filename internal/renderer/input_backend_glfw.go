//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	iinput "github.com/darkliquid/ironwail-go/internal/input"
	openglimpl "github.com/darkliquid/ironwail-go/internal/renderer/opengl"
)

// InputBackendForSystem returns a GLFW-based input backend for the OpenGL/CGO renderer.
func (r *Renderer) InputBackendForSystem(sys *iinput.System) iinput.Backend {
	return openglimpl.NewInputBackend(r.window, sys)
}
