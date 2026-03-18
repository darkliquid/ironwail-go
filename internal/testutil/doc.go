// Package testutil provides helpers for locating Quake assets and writing
// concise engine tests.
//
// # Purpose
//
// The package reduces duplication in tests that need QUAKE_DIR, pak0.pak, or
// common assertion helpers.
//
// # High-level design
//
// It probes environment variables and nearby directories for test assets, then
// exposes skip helpers and comparison helpers so package tests can stay small
// and readable.
//
// # Role in the engine
//
// This package belongs to the project's testing layer. It does not participate
// in runtime engine behavior, but it helps validate asset-dependent subsystems.
//
// # Original C lineage
//
// There is no direct original Quake or Ironwail runtime counterpart; it is a
// project-level testing convenience that reflects the Go port's stronger focus
// on automated verification.
//
// # Deviations and improvements
//
// Having a dedicated reusable internal test package is itself a departure from
// the original C codebase. Standard testing helpers, environment-based asset
// discovery, and readable assertion helpers make the growing automated suite
// easier to maintain.
package testutil
