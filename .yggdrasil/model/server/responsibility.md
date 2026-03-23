# Responsibility

## Purpose

`server` owns the authoritative gameplay runtime for a loaded Quake map: edicts, QuakeC integration, collision, physics, client/server session state, snapshot serialization, and save/load.

## Owns

- The per-map authoritative simulation state.
- The server-side bridge between Go runtime structures and the QuakeC VM.
- Map bootstrap, world linking, and per-frame simulation.
- Server-side network message generation and signon state.
- Save/load capture and restore.
- Optional debug telemetry for QC and trigger/physics debugging.

## Does not own

- Top-level engine orchestration and process lifecycle, which belong to `host`.
- Client-side parsing, prediction, and rendering.
- The QuakeC VM implementation itself; `server` drives and synchronizes it.
