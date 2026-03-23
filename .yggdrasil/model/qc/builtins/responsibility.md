# Responsibility

## Purpose

`qc/builtins` owns the QuakeC builtin-function bridge between VM execution and engine services.

## Owns

- Builtin registration by canonical builtin number.
- The server hook interface and adapter bridge for engine-backed builtins.
- Builtin implementations for entity/world/math/string/IO-facing operations.

## Does not own

- The VM execution loop.
- Concrete server/client/render/audio implementations behind the registered hooks.
