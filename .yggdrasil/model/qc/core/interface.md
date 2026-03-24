# Interface

## Main consumers

- `internal/server` for SSQC execution
- `qc/csqc` for client-side entry-point execution
- tests that validate opcode and stack semantics

## Main API

Observed surfaces:
- `LoadProgs`
- `FindFunction`, `FindGlobal`, `FindField`
- `EnterFunction`, `LeaveFunction`
- `ExecuteProgram`
- `ProfileResults(top int)` for sorted function-level QC profile counters (reset on read)
- runaway-loop guard remains Quake-compatible by default (`0x1000000`); tests may opt into `VM.RunawayLoopLimit` override for focused fixture coverage

## Contracts

- `LoadProgs` requires a compatible `progs.dat` version and layout.
- `ExecuteProgram` expects a valid function index and previously loaded VM tables.
- Negative `FirstStatement` values denote builtins and are dispatched through the builtin registry.
