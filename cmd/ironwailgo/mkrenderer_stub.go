//go:build !gogpu && !opengl && !cgo
// +build !gogpu,!opengl,!cgo

package main

import "github.com/ironwail/ironwail-go/internal/renderer"

func newRendererBackend(cfg renderer.Config) (gameRenderer, error) {
	return renderer.NewWithConfig(cfg)
}
