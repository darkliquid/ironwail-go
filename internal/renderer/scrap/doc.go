// Package scrap implements skyline bin-packing for rectangular regions
// within a fixed-size atlas page. This is the classic Quake scrap texture
// approach: track per-column allocation height and find the best-fit
// slot for each incoming rectangle.
//
// # Purpose
//
// The scrap allocator packs small UI textures (console characters,
// HUD icons, menu images) into a shared atlas texture, reducing the
// number of GPU texture bindings needed during 2D overlay rendering.
//
// # Original C lineage
//
// Mirrors the scrap allocator in gl_draw.c (Scrap_AllocBlock,
// Scrap_Upload). The C version used a 256×256 scrap texture;
// the Go version uses a configurable page size.
//
// # Role in the engine
//
// Used by the renderer's 2D overlay system (overlay2d) and the
// image package (internal/image) for caching QPic character
// glyphs. The scrap texture is uploaded once and re-used across
// frames.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/renderer/scrap -count=1
package scrap
