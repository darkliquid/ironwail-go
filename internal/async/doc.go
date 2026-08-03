// Package async provides a minimal thread-safe work queue used to marshal
// work from background goroutines back onto a single "main" thread/frame
// pump, matching the semantics of C Ironwail's host.c AsyncQueue.
//
// # Purpose
//
// The package implements a bounded FIFO queue that background goroutines
// (asset loading, audio streaming, network I/O) can push work onto,
// and the main loop drains and executes synchronously each frame.
//
// # Original C lineage
//
// Mirrors the AsyncQueue struct and Host_AccumulateAsyncWork /
// Host_FlushAsyncWork functions in host.c. The C version used a
// linked list of callback function pointers; the Go version uses
// func() closures.
//
// # Role in the engine
//
// Sits between the host frame loop (internal/host) and background
// workers. The host calls Drain() each frame; background goroutines
// call Enqueue(). This ensures all game state mutations happen on
// the main thread, avoiding data races without mutexes on game state.
//
// # Testing
//
// Unit tests verify FIFO ordering, concurrent enqueue safety, and
// drain semantics. Run with:
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/async -count=1
package async
