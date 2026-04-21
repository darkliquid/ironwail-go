package main

import (
	"github.com/darkliquid/ironwail-go/internal/game"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func newRendererBackend(cfg renderer.Config) (game.Renderer, error) {
	return renderer.NewWithConfig(cfg)
}
