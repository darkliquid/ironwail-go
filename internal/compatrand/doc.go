// Package compatrand provides a deterministic, thread-safe random number
// generator that produces identical sequences to C Ironwail's rand()
// function, ensuring multiplayer demo compatibility and parity in
// any gameplay path that uses randomness (monster AI, item drops,
// spread patterns).
//
// # Purpose
//
// C's rand() is platform-dependent and non-thread-safe. This package
// wraps a seeded PRNG that matches the Quake engine's expected rand()
// output sequence, so that demos recorded on the C engine play back
// identically on the Go port.
//
// # Original C lineage
//
// Mirrors the rand()/srand() calls throughout sv_phys.c, cl_tent.c,
// and pr_edict.c. The C engine used libc rand() directly; the Go
// port must replicate the exact sequence to maintain demo parity.
//
// # Role in the engine
//
// Used by the server (physics, AI), client (temp entities, effects),
// and QuakeC VM (when builtins call random()). All randomness in the
// engine flows through this package to guarantee determinism.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/compatrand -count=1
package compatrand
