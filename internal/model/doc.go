// Package model provides data structures and loading functionality for Quake models.
//
// Quake uses three types of models:
//   - Brush models (.bsp): World geometry loaded from BSP files
//   - Alias models (.mdl): Character and item models with skeletal animation
//   - Sprite models (.spr): 2D billboard sprites for effects
//
// This package provides the in-memory representations (m* types) for all model types,
// as well as the on-disk formats (d* types) for loading from files.
//
// The architecture follows the original Quake design where d*_t structures represent
// on-disk data and m*_t structures represent optimized in-memory data.
package model
