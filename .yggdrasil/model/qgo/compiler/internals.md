# Internals

## Logic

### Pipeline

The compiler is a four-stage pipeline:

1. `Compiler.Compile` parses every Go file in the target directory and type-checks them with a `types.Config` that only knows about the synthetic `quake` packages.
2. `Lowerer` performs a two-pass walk over the package: first it collects globals and function signatures, then it lowers function bodies into `IRProgram`.
3. `CodeGen` maps IR virtual registers to QC global/local offsets, emits `qc.DStatement` and `qc.DFunction` tables, and patches branch labels in a second pass.
4. `Emit` serializes those tables into the `progs.dat` section layout expected by `internal/qc`.

### Lowering model

The lowerer tracks a `types.Object -> VReg` mapping plus constant pools for floats and strings. IR instructions can either refer to virtual registers or encode direct QC global offsets when a value is already a reserved VM slot. Labels are represented as pseudo-instructions and resolved later by code generation.

### QCVM-oriented allocation

`GlobalAllocator` starts at `qc.OFSParmStart`, preserving QCVM-reserved slots. `StringTable` interns all strings and guarantees offset `0` is the empty string. `slotsForType` handles the special three-slot width of vectors so globals, locals, and parameter sizes match QCVM expectations.

## Constraints

- the compiler currently targets a narrow Go subset tailored for QC-like programs
- only `quake` and `quake/engine` imports are legal during type-checking
- binary compatibility is anchored to QCVM struct/layout constants from `internal/qc`

## Decisions

### Synthetic packages instead of importing runtime Go packages

Observed decision:
- the frontend type-checks against synthetic `go/types.Package` definitions for `quake` and `quake/engine`

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- source programs can use a Go-shaped API without linking against runtime engine packages
- builtin numbering and engine globals remain compiler-controlled rather than inferred from executable code

### Two-pass code generation for branches

Observed decision:
- labels are collected during statement emission and branch displacements are patched in a dedicated second pass

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- lowering can emit structured control flow without knowing final statement indices up front
- undefined labels fail during generation rather than silently producing bad jumps
