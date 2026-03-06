// Package draw loads and caches Quake 2D drawing assets such as pics, fonts,
// and palette-backed UI resources.
//
// # Purpose
//
// The package reads gfx.wad, resolves standalone .lmp pictures, and serves
// parsed QPic data to HUD, menu, and console rendering code.
//
// # High-level design
//
// Manager owns the loaded WAD, the Quake palette, a cache of decoded pics, and
// optional filesystem or directory-based fallback loading. Asset lookup prefers
// the cache, then tries WAD entries, pak files, and loose files in Quake-like
// order.
//
// # Role in the engine
//
// This package is the 2D asset bridge between filesystem/image decoding and
// higher-level UI packages.
//
// # Original C lineage
//
// The closest original sources are draw.c, gl_draw.c, draw.h, and the WAD/UI
// resource concepts handled by wad.c.
//
// # Deviations and improvements
//
// The Go port cleanly separates asset loading from the actual GPU or software
// drawing operations, unlike the original renderer-centric organization.
// Explicit caching, clearer errors, and composition with the fs and image
// packages replace the old monolithic draw path while keeping Quake's asset
// lookup behavior.
package draw