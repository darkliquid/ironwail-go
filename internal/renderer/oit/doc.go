// Package oit provides configuration constants and resource management
// for Order-Independent Transparency (OIT) rendering.
//
// # Purpose
//
// This package declares the alpha mode constant (AlphaModeOIT) and
// provides a helper function (ShouldUseResources) that determines
// whether the OIT framebuffer resources should be allocated based
// on the configured alpha mode.
//
// OIT is a rendering technique that correctly sorts translucent
// fragments regardless of draw order, avoiding z-fighting and
// incorrect blending. It uses a two-pass approach: first rendering
// translucent fragments to a weighted accumulation buffer, then
// compositing the result.
//
// # Original C lineage
//
// Not present in C Ironwail. OIT is a modern rendering technique
// added in the Go port as an optional improvement over Quake's
// traditional back-to-front translucent sorting.
//
// # Role in the engine
//
// The renderer root package checks ShouldUseResources during
// initialization to decide whether to allocate OIT framebuffers.
// Currently disabled (AlphaModeOIT = 2, which is above the
// threshold), so OIT resources are not allocated.
//
// # Testing
//
// No test files currently exist. Future tests should verify
// ShouldUseResources logic for different alpha mode values.
package oit
