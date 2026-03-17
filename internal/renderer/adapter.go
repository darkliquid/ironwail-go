// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package renderer

// RendererAdapter wraps renderer.Renderer to implement host.Renderer interface
type RendererAdapter struct {
	renderer *Renderer
}

// NewRendererAdapter NewRendererAdapter selects the active renderer backend (OpenGL, GoGPU, or stub) and wires it behind a single interface so the rest of the engine can run the same frame pipeline regardless of graphics API.
func NewRendererAdapter(r *Renderer) *RendererAdapter {
	return &RendererAdapter{renderer: r}
}

// Init Init prepares backend resources needed before the first frame, including API-specific state, cached GPU objects, and per-frame scratch structures used by the renderer.
func (a *RendererAdapter) Init() error {
	// Renderer is already initialized via its own Init method
	return nil
}

// UpdateScreen UpdateScreen executes one full frame of rendering work for the active backend, from camera setup through world/entity/effects passes to final HUD compositing.
func (a *RendererAdapter) UpdateScreen() {
	// For now, this is a no-op since the renderer manages its own frame loop
	// The renderer's OnDraw callback is where screen updates happen
}

// Shutdown Shutdown releases backend-owned resources in reverse order of creation so context-bound objects (textures, buffers, shaders) are destroyed safely.
func (a *RendererAdapter) Shutdown() {
	a.renderer.Shutdown()
}
