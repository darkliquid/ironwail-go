# Package: engine

## Purpose
The `engine` package provides a suite of generic, reusable data structures and utilities that form the backbone of the Ironwail engine. It is designed to be dependency-free (relying only on the Go standard library) to avoid circular dependencies between internal packages. Its primary goal is to replace the ad-hoc, map-based, or unsafe patterns inherited from the original C codebase with type-safe, thread-safe, and idiomatic Go implementations.

## Key Types & Interfaces
- **`Cache[T]`**: A thread-safe key/value store using `sync.RWMutex`. It is used for caching runtime objects like models, textures, and sounds.
- **`Registry[T]`**: A "write-once, read-many" lookup table. It is intended for configuration data populated during initialization. It panics on duplicate registration to help catch developer errors early.
- **`Set[T]`**: A mathematical set implementation backed by a map, used for membership testing.
- **`Queue[T]`**: A bounded ring buffer for FIFO (First-In-First-Out) processing, capable of growing dynamically. Ideal for handling bursts of console commands or events.
- **`EventBus`**: A typed publish/subscribe system that allows decoupled communication between different subsystems.
- **`ParallelLoad[T]`**: A utility function for concurrent asset loading using a worker pool pattern.
- **`LoadPipeline[T]`**: A channel-based pipeline for managing concurrent asset loading requests and results.

## Core Workflow
The package is primarily used as a utility layer:
1. **Initialization**: Subsystems create `Registry` or `Cache` instances to store their long-lived data.
2. **Execution**: During the game loop, the `Queue` and `EventBus` facilitate communication and command flow.
3. **Asset Loading**: When a level loads, `ParallelLoad` or `LoadPipeline` are used to fetch multiple assets (sounds, models) in parallel, maximizing CPU utilization.

## Integration
`engine` is the most low-level package in `internal/`. Almost every other package (like `renderer`, `audio`, `host`) imports `engine` to use its thread-safe containers. By providing these generics, `engine` ensures consistent behavior for data management across the entire project.

## Learning Tips
- **Generics in Action**: This package is an excellent example of using Go generics (`[T any]`) to create reusable engine primitives.
- **Thread Safety**: Examine how `sync.RWMutex` and `sync.Once` are used in `Cache` and `LoadPipeline` to ensure correctness in a multi-threaded environment.
- **Semaphore Pattern**: Look at `ParallelLoad` in `loader.go` to see a classic Go pattern for limiting concurrency using a buffered channel as a semaphore.
