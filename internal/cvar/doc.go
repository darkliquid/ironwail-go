// Package cvar manages Quake console variables and their typed views.
//
// # Purpose
//
// The package registers named variables, stores their canonical string values,
// exposes float/int/bool conversions, and supports configuration persistence.
//
// # High-level design
//
// CVarSystem owns a case-insensitive map of variables together with flags,
// defaults, descriptions, and optional callbacks. Runtime code uses it to
// register values, update them, enumerate archivable variables, and perform
// prefix completion.
//
// # Role in the engine
//
// This package is the configuration backbone used by host timing, renderer and
// input options, gameplay flags, and console commands.
//
// # Original C lineage
//
// The direct counterpart is cvar.c together with declarations from cvar.h.
//
// # Deviations and improvements
//
// The current Go package is narrower and cleaner than the original C layer,
// even though it still offers a convenient global instance. Mutex-protected
// maps, typed helper methods, and callback hooks replace macro-heavy access and
// loosely coordinated globals while preserving Quake cvar semantics.
package cvar
