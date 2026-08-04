// Package physics provides per-entity physics simulation, bounding box tracing, and monster movement.
//
// # Original C lineage
//
// Mirrored from original Quake / Ironwail C sources:
//   - sv_phys.c: Physics integration, movetypes (walk, fly, toss, push, noclip), collision resolution.
//   - sv_move.c: Monster movement, step checking, pathfinding toward goals.
package physics
