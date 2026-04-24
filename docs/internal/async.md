# Async Package

## Purpose
The `async` package provides a minimal thread-safe work queue used to marshal work from background goroutines back onto a single "main" thread or frame pump. This matches the semantics of the original C Ironwail's `host.c` AsyncQueue. In the context of a game engine like Quake, many systems (like save workers or mod downloaders) run in the background but need to update the game state safely without racing against the client or server state.

## Key Types & Interfaces
- **`Queue`**: A bounded FIFO (First-In-First-Out) queue of work items (functions). It uses a `sync.Mutex` for thread safety and a `sync.Cond` to manage blocking behavior when the queue is full.

## Core Workflow
1. **Creation**: A `Queue` is initialized with a specific capacity using `NewQueue(capacity)`.
2. **Pushing Work**: Background goroutines use `Push(fn)` to add a work item. If the queue is full, `Push` blocks until space is available. Alternatively, `TryPush(fn)` can be used for non-blocking attempts.
3. **Draining Work**: The main thread (typically once per frame in `Host.Frame`) calls `Drain()`. This method grabs all currently pending functions, clears the queue, and executes them synchronously in the order they were pushed.
4. **Shutdown**: `Shutdown()` marks the queue as closed, unblocks any waiting producers, and performs one final drain.

## Integration
The `async` package is primarily integrated into the **Host** system. The `Host.Frame` loop is responsible for calling `Drain()` on the global async queue(s) to ensure that background tasks (like the completion of a map download or a save game operation) are processed at a predictable point in the frame lifecycle.

## Learning Tips
- **Idiomatic Go vs. C Parity**: While idiomatic Go might use an unbounded channel for this purpose, `async.Queue` mirrors the C implementation's bounded, blocking behavior and atomic drain semantics. This is a great example of balancing Go idioms with the requirements of a ported architecture.
- **Blocking & Synchronization**: Examine how `sync.Cond` is used in the `Push` method to wait for space in the queue and how `Broadcast` is used in `Drain` and `Shutdown` to wake up blocked goroutines.

## Tests

**`TestQueuePushDrainFIFO`** — Pushes 5 closures capturing indices 0–4, drains them, and asserts the output slice contains `[0,1,2,3,4]` in order. The `Queue` dispatches work from the game thread to the render/audio thread; FIFO ordering ensures commands execute in the order they were issued.

**`TestQueueTryPushRespectsCapacity`** — On a capacity-2 queue, the first two `TryPush` calls succeed and the third returns false. After `Drain`, the fourth succeeds. `TryPush` is a non-blocking variant; callers must handle the false return when the queue is full without deadlocking.

**`TestQueuePushBlocksUntilDrain`** — Fills a capacity-1 queue, starts a goroutine calling `Push`, verifies after 20 ms the goroutine is still blocked, then drains and verifies unblocking. `Push` must block when the queue is full to provide backpressure; otherwise the producer would overflow memory. Uses `time.After` and channels for synchronization.

**`TestQueueShutdownUnblocksProducers`** — Fills a capacity-1 queue, starts a goroutine blocked on `Push`, calls `Shutdown`. Asserts the goroutine unblocks and `Push` returns false, `IsShutdown()` returns true, and `TryPush` fails. On engine shutdown, blocked producers must not hang indefinitely.

**`TestQueueConcurrentProducers`** — 8 goroutines each push 100 closures that atomically increment a counter. The test drains periodically until all 800 closures have run within 5 seconds. The queue must be safe under concurrent producer load without deadlock or lost increments. Uses `sync.WaitGroup` and `atomic.Int32`.

**`TestQueueNilFuncIsNoop`** — Both `Push(nil)` and `TryPush(nil)` return false and do not change `Len()`. Nil function references must be rejected silently rather than panicking when `Drain` tries to call them.
