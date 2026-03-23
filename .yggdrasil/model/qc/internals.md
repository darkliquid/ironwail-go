# Internals

## Structure

The package is split into focused concerns:
- `qc/vm` — VM state model, globals, entity-field layout, and core types
- `qc/core` — `progs.dat` loading and bytecode execution
- `qc/builtins` — server/engine builtin bridge and builtin registration
- `qc/csqc` — client-side QuakeC wrapper, hooks, and entry-point handling

## Decisions

### Single QC package with focused implementation slices

Observed decision:
- The Go port keeps one `qc` package but separates the VM model, interpreter core, builtin bridge, and CSQC support by file/function groups.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- server-side and client-side QC support share a common VM foundation without forcing one monolithic implementation file
