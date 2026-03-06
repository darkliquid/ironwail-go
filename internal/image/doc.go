// Package image parses Quake image container formats and simple
// palette-indexed picture data.
//
// # Purpose
//
// The package loads WAD2 lumps, normalizes lump names, decodes QPic resources,
// and provides small helpers for palette-backed image conversion.
//
// # High-level design
//
// The package is format-centric. It defines WAD headers and lump records,
// reads them from a random-access source, stores lump payloads in memory, and
// decodes compact Quake picture formats used by menus, console assets, and
// tests.
//
// # Role in the engine
//
// This is the low-level image decoding layer consumed by draw and related
// asset loaders.
//
// # Original C lineage
//
// The corresponding original sources are wad.c, wad.h, and the classic image
// format handling that fed Quake UI and texture systems.
//
// # Deviations and improvements
//
// The Go port intentionally keeps image parsing separate from rendering and
// texture upload, which is a useful break from the original renderer-heavy C
// structure. io.ReaderAt, io.SectionReader, encoding/binary, and typed lump
// maps replace manual pointer walking and ad hoc byte swapping.
package image