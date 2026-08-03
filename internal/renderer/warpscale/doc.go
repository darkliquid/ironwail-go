// Package warpscale provides water warp post-processing configuration
// and FOV scaling math for the GoGPU/WebGPU renderer.
//
// # Purpose
//
// This package computes the horizontal FOV scale factor for
// r_waterwarp > 1 (the "warp FOV" effect that distorts the view
// when the player is underwater), manages the offscreen scene
// render target used for water warp compositing, and provides
// the scene render target lifecycle (enable, disable, composite).
//
// # Original C lineage
//
// The water warp effect mirrors R_ViewChanged (view.c) and
// R_RenderView with underwater warp (gl_rmain.c). The C version
// applied a sinusoidal distortion to the entire framebuffer when
// the player was underwater; the Go version uses an offscreen
// render target and a composite shader pass.
//
// # Role in the engine
//
// The renderer frame loop (renderer_gogpu_frame.go) calls
// shouldUseSceneRenderTarget to decide whether to render the
// world to an offscreen target. If true, the world renders
// offscreen, then compositeSceneRenderTarget applies the water
// warp distortion and copies the result to the swapchain.
//
// The scene render target is also used for translucent liquid
// faces that require compositing (see hasTranslucentWorldLiquidFacesGoGPU).
//
// # Testing
//
// No test files currently exist. Future tests should verify:
//   - WaterwarpFOVScale produces correct FOV multipliers
//   - Scene render target enable/disable lifecycle
//   - Composite pass with and without water warp
package warpscale
