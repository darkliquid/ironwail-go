// Package types contains shared types and constants used by the server
// package and its future sub-packages. Types defined here have no methods
// and no dependency on the Server struct, allowing sub-packages to import
// them without creating import cycles.
//
// The server package re-exports all types and constants defined here via
// type aliases and const aliases, so existing code that references
// server.MoveType, server.SolidType, etc. continues to work unchanged.
//
// # Original C lineage
//
// These types mirror the entity_state_t, movevars_t, and protocol constants
// from server.h, server.h, and quakedef.h in the original C engine.
package types
