// Package main provides the offline BSP diagnostic CLI tool (bspdiag).
//
// # Purpose
//
// Inspect Quake BSP map lumps, entity key-value fields, leaf contents, face attributes,
// lightmaps, texture palettes, and lit-water translucency settings offline without running the game engine.
//
// # Original C lineage
//
// Offline inspection tool for structures mirrored from:
//   - bspfile.c: BSP lump headers, faces, planes, leafs, nodes, textures.
//   - gl_rsurf.c: Surface lightmaps, face attributes, liquid translucency algorithms.
//
// # Commands
//
//   - `bspdiag info <quake_dir> <map.bsp>`: Summary of BSP lumps, textures, texinfo flags, atlas simulation.
//   - `bspdiag entities <quake_dir> <map.bsp>`: Parsed worldspawn and entity key-value fields.
//   - `bspdiag point <x> <y> <z> <quake_dir> <map.bsp>`: Leaf contents query at 3D point.
//   - `bspdiag face <face_id> <quake_dir> <map.bsp>`: Detailed face attributes and lightmap sample grid.
//   - `bspdiag texture <name> <quake_dir> <map.bsp>`: Texture dimensions and RGBA converted colors.
//   - `bspdiag liquids <quake_dir> <map.bsp>`: Surface liquid and lit-water alpha settings analysis.
//
// # Testing
//
//	mise run build-bspdiag
//	./cmd/bspdiag/bspdiag info ./quake-data id1/start.bsp
package main
