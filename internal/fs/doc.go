// Package fs implements the Quake virtual filesystem over loose directories
// and .pak archives.
//
// # Purpose
//
// The package mounts id1 and optional game directories, searches them in Quake
// override order, and loads files by virtual path.
//
// # High-level design
//
// FileSystem models search paths and loaded pack directories explicitly. File
// resolution checks loose files first and then pack contents in reverse-priority
// order so later game directories and higher-numbered paks override earlier
// content as they do in Quake.
//
// # Role in the engine
//
// This is the canonical asset source for maps, progs, graphics, sounds,
// configs, and tests.
//
// # Original C lineage
//
// The main reference is the filesystem portion of common.c, plus pack and WAD
// concepts historically associated with wad.c.
//
// # Deviations and improvements
//
// The Go port isolates filesystem logic into its own package instead of hiding
// it inside a broad common layer. It also adds explicit path sanitization and
// root checks, and it uses io/fs, os.DirFS, and structured pack metadata in
// place of manual C string/path handling.
package fs