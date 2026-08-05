// Package arena provides high-performance arena memory allocators and scratch buffer pools.
//
// # Purpose
//
// Allocate frame-scoped scratch memory with zero GC overhead, recycling temporary vertex,
// particle, and network buffers across frames.
//
// # Original C lineage
//
// Mirrored from original Quake / Ironwail C sources:
//   - zone.c: Hunk and Zone memory pool management.
//
// # Key types
//
//   - Arena: Linear bump allocator cleared at frame end.
//   - BufferPool: Thread-safe pool for reusable byte buffers.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/engine/arena -count=1
package arena
