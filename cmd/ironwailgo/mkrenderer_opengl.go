//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package main

import "github.com/darkliquid/ironwail-go/internal/renderer"

func newRendererBackend(cfg renderer.Config) (gameRenderer, error) {
	return renderer.NewWithConfig(cfg)
}
