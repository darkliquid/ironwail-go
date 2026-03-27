//go:build gogpu && !cgo

package main

import "github.com/ironwail/ironwail-go/internal/renderer"

func newRendererBackend(cfg renderer.Config) (gameRenderer, error) {
	return renderer.NewWithConfig(cfg)
}
