# Internals

## Logic

The VM model is intentionally explicit:
- globals stored in a flat float32 array
- edicts stored as contiguous field memory
- strings, functions, defs, and statements exposed through typed slices
- call stack and local stack tracked directly on the VM

This explicit model replaces pointer-cast-heavy C access with typed helper methods while preserving QuakeC’s memory layout conventions.

## Constraints

- Offset-based access is fundamental; misaligned field/global assumptions would corrupt execution semantics.
- VM state is shared mutable state for loader, executor, and builtins, so call ordering matters.

## Decisions

### Typed helper layer over raw QC memory model

Observed decision:
- The Go port wraps the raw QuakeC memory model in named structs and accessor helpers.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- the subsystem is easier to read and test while still following QuakeC’s offset-driven model
