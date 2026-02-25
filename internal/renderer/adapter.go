// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package renderer

// RendererAdapter wraps renderer.Renderer to implement host.Renderer interface
type RendererAdapter struct {
	renderer *Renderer
}

func NewRendererAdapter(r *Renderer) *RendererAdapter {
	return &RendererAdapter{renderer: r}
}

func (a *RendererAdapter) Init() error {
	// Renderer is already initialized via its own Init method
	return nil
}

func (a *RendererAdapter) UpdateScreen() {
	// For now, this is a no-op since the renderer manages its own frame loop
	// The renderer's OnDraw callback is where screen updates happen
}

func (a *RendererAdapter) Shutdown() {
	a.renderer.Shutdown()
}
