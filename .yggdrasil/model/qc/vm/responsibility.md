# Responsibility

## Purpose

`qc/vm` owns the in-memory QuakeC VM model: globals, entity/edict layout, string table, function metadata, call stack, and the typed helper layer that makes those structures usable from Go.

## Owns

- `VM` and its core runtime fields.
- Typed layouts such as `GlobalVars` and `EntVars`.
- Fundamental constants and offsets such as `OFSParm*`/`OFSReturn`-style conventions.
- Shared typed accessors for globals, strings, vectors, functions, and edict fields.

## Does not own

- `progs.dat` binary loading mechanics.
- The bytecode execution loop.
- Builtin registration policy.
