# Internals

## Logic

The package groups small, reusable building blocks rather than one unified runtime. `SizeBuf` provides Quake-style sized-buffer semantics for reading and writing primitive values, intrusive lists and bit arrays provide tiny structural helpers used by engine subsystems, and the `COM_*` helpers preserve C-style token/argv/path utility behavior for callers that still depend on those contracts. The `binary` subpackage isolates endian conversion and stream I/O helpers that support binary file and protocol decoding.

## Constraints

- Several helpers preserve C-style global/stateful behavior for compatibility rather than idiomatic Go purity.
- `SizeBuf` is intentionally low-level and should not silently absorb higher-level protocol semantics.
- Small parsing and angle-encoding differences can create parity drift if callers assume exact C behavior.

## Decisions

### Keep only the reusable primitive subset in `common`

Observed decision:
- The Go port keeps `common` focused on low-level primitives while moving broader engine responsibilities into dedicated packages.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The package is smaller and more reusable than C `common.c`, but callers still depend on a few compatibility-oriented helpers and global parsing conventions.
