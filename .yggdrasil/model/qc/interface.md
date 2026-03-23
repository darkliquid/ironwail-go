# Interface

## Main consumers

- `internal/server` for SSQC execution and builtin integration
- host/client-facing CSQC callers for client-side HUD/draw flows

## Main exposed shape

The package exposes:
- VM state and typed helpers
- progs loading and bytecode execution
- builtin registration and hook interfaces
- CSQC wrapper state and entry points

Detailed contracts live in the child nodes.
