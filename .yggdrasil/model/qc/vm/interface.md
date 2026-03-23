# Interface

## Main consumers

- `qc/core`, which loads and executes using the VM state model
- `qc/builtins`, which reads and mutates VM state for engine integration
- `qc/csqc`, which wraps a VM instance for client-side use
- `internal/server`, which synchronizes server state with QC globals and edicts

## Main API

Observed surfaces include:
- VM construction
- typed global and entity-field accessors
- string allocation and lookup helpers
- shared layout types for global and entity variables

## Contracts

- VM instances are not thread-safe.
- Each execution context should own its own VM instance.
- Layout and offset semantics must remain aligned with the `progs.dat` format contract.
