# Responsibility

## Purpose

`server/debug-telemetry` owns opt-in diagnostics for understanding authoritative server behavior, especially trigger/touch/QC/physics execution.

## Owns

- server debug cvar registration and config parsing
- per-frame/per-event telemetry filtering and output
- QC trace instrumentation helpers
- batching/coalescing of debug output for readable logs

## Does not own

- The core gameplay logic being instrumented.
- Normal production networking or persistence behavior.
