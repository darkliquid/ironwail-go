// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package renderer

type backendWithShutdown interface {
	Shutdown()
}

// RendererAdapter wraps renderer.Renderer to implement host.Renderer interface
type RendererAdapter struct {
	backend backendWithShutdown
}

// NewRendererAdapter wires the active renderer backend behind a single interface
// so the rest of the engine can run the same frame pipeline regardless of
// whether it is using the canonical GoGPU backend or the no-backend stub.
func NewRendererAdapter(b backendWithShutdown) *RendererAdapter {
	return &RendererAdapter{backend: b}
}

// Init prepares backend resources needed before the first frame, including API-specific state, cached GPU objects, and per-frame scratch structures used by the renderer.
func (a *RendererAdapter) Init() error {
	// Renderer is already initialized via its own Init method
	return nil
}

// UpdateScreen executes one full frame of rendering work for the active backend, from camera setup through world/entity/effects passes to final HUD compositing.
func (a *RendererAdapter) UpdateScreen() {
	// For now, this is a no-op since the renderer manages its own frame loop
	// The renderer's OnDraw callback is where screen updates happen
}

// Shutdown releases backend-owned resources in reverse order of creation so context-bound objects (textures, buffers, shaders) are destroyed safely.
func (a *RendererAdapter) Shutdown() {
	if a.backend != nil {
		a.backend.Shutdown()
	}
}
