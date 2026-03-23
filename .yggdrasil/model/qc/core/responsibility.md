# Responsibility

## Purpose

`qc/core` owns `progs.dat` loading, symbol lookup, function-entry/leave stack handling, and the bytecode interpreter.

## Owns

- `LoadProgs` and binary ingestion of statements, defs, functions, strings, and globals.
- Symbol lookup helpers such as function/global/field searches.
- Call-stack and local-stack transitions for function entry/leave.
- The interpreter loop in `ExecuteProgram`.

## Does not own

- VM memory layout definitions themselves.
- Builtin implementation behavior beyond dispatching to registered builtins.
