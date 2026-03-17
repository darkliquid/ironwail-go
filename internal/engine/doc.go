// Package engine provides generic, reusable data structures for the Ironwail
// game engine. These types eliminate boilerplate across subsystems by offering
// type-safe, thread-safe containers that replace ad-hoc map-based patterns
// inherited from the C codebase.
//
// The package is intentionally dependency-free — it imports only the standard
// library — so that any internal package can use it without creating circular
// imports.
//
// Key types:
//
//   - [Cache] — a thread-safe key/value store for runtime object caching
//     (models, textures, sounds). Supports Get/Set/Delete/Clear with
//     sync.RWMutex for concurrent read access.
//
//   - [Registry] — a write-once, read-many lookup table for configuration
//     data that is built during initialization and then frozen. Panics on
//     duplicate registration to catch wiring bugs early.
//
//   - [Set] — a minimal mathematical set backed by a map. Used anywhere the
//     C code used a "map[string]struct{}" pattern for membership testing.
//
//   - [Queue] — a bounded ring buffer for FIFO command/event processing.
//     Grows dynamically when full, making it suitable for bursty workloads
//     like console command batching.
//
//   - [EventBus] — a typed publish/subscribe system for decoupled
//     communication between subsystems. Subscribers receive events
//     synchronously in registration order.
package engine
