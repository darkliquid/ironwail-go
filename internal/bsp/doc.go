// Package bsp reads Quake BSP map files and exposes typed on-disk and
// in-memory structures.
//
// # Purpose
//
// The package decodes map lumps such as planes, nodes, faces, textures,
// visibility, clipnodes, and submodels from Quake BSP files.
//
// # High-level design
//
// The package is largely data-oriented: binary readers parse the header and
// lumps, while loaders translate those bytes into strongly typed structures
// suitable for runtime use. It already reflects classic Quake BSP plus larger-
// map variants such as BSP2 and Quake 64 constants.
//
// # Role in the engine
//
// This package underpins world loading, collision hull construction,
// visibility data handling, and later renderer/model setup.
//
// # Original C lineage
//
// The corresponding original concepts live in bspfile.h, model.c, gl_model.c,
// and the BSP tree and hull code spread through world/model loading.
//
// # Deviations and improvements
//
// The Go port isolates BSP format knowledge into one package instead of mixing
// file decoding with server and renderer concerns. Standard-library binary I/O,
// explicit version constants, and safer slice/reader usage replace manual C
// struct walking while keeping Quake's lump model intact.
package bsp
