// Package model defines Quake model formats and in-memory representations for
// brush, alias, and sprite models.
//
// # Purpose
//
// The package describes how world geometry, animated MDL models, and sprite
// effects are loaded and represented at runtime.
//
// # High-level design
//
// It separates on-disk structures from optimized in-memory structures such as
// planes, surfaces, hulls, textures, alias headers, and sprite data. Brush,
// alias, and sprite model concepts share one vocabulary so collision, loading,
// and rendering code can refer to the same types.
//
// # Role in the engine
//
// This is the shared model layer consumed by server collision, renderer world
// drawing, and asset loading.
//
// # Original C lineage
//
// The main references are gl_model.c, gl_model.h, modelgen.h, and the broader
// Quake model-loading concepts that sit between BSP parsing and rendering.
//
// # Deviations and improvements
//
// The Go port uses a dedicated package for model definitions instead of
// scattering them between renderer and world files. Typed enums, explicit
// structs for major runtime concepts, and cleaner relationships between
// on-disk and runtime data make the model layer easier to understand and reuse
// across subsystems.
package model
