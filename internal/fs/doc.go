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
// resolution walks a single Quake-style search stack: later game directories
// override earlier ones, and within each game directory higher-numbered paks
// override lower-numbered paks, which in turn override loose files. Pack
// discovery follows deterministic numeric `pakN.pak` ordering, and pack-entry
// lookup is case-insensitive to match Quake data behavior on case-sensitive
// hosts.
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
