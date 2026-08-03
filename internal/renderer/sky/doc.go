// Package sky implements external skybox loading, configuration, and
// rendering for the GoGPU/WebGPU renderer.
//
// # Purpose
//
// This package handles loading 6-face external skybox textures from
// PNG files (gfx/env/<name>_{rt,bk,lf,ft,up,dn}.png), wind effect
// configuration (gfx/env/<name>_wind.cfg), and GPU texture creation
// for the skybox cubemap rendering path.
//
// # Original C lineage
//
// Mirrors the external skybox subsystem in gl_sky.c (SkyLoader_Init,
// SkyLoader_LoadFace, R_DrawSky). The C version used SOIL or
// stb_image for PNG loading; the Go version uses image/png.
//
// # Role in the engine
//
// The renderer root package calls into this package during world
// upload (UploadWorld) and each frame during the sky render pass.
// The skybox textures are bound as a separate bind group (group 3)
// during the sky render pass.
//
// # Testing
//
// No test files currently exist. Future tests should cover:
//   - Wind config parsing
//   - Skybox face name resolution
//   - Fallback behavior when sky textures are missing
package sky
