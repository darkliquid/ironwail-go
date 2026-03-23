# Responsibility

## Purpose

`ironwailgo/runtime-loop` owns the host callback implementation, per-frame runtime ordering, demo playback control flow, and shutdown sequencing.

## Owns

- `gameCallbacks` and frame-phase coordination.
- The ordering of event polling, console command execution, server frame, client send/read, activation transitions, and runtime synchronization.
- Demo playback, rewind, EOF handling, and demo-world bootstrap.
- Headless/dedicated/runtime loop helpers.
- Final shutdown/write-config/teardown behavior.

## Does not own

- Startup-time subsystem construction.
- Detailed camera/view calculations or entity collection logic.
