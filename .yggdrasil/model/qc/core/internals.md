# Internals

## Logic

### Loading

`LoadProgs` reads the QuakeC program header, seeks to each table, and loads statements, defs, functions, strings, and globals into the VM. It also derives `EntityFields` and `EdictSize` from the program metadata.

### Execution

`ExecuteProgram` distinguishes between:
- builtin-backed functions
- bytecode-backed functions

The interpreter loop advances through statements, dispatches opcodes, handles control flow and function calls, tracks profile counters, and copies return values into `OFSReturn`.

### Stack handling

Function entry saves caller state and local values, copies parameters from the reserved parameter globals, and sets the new execution context. Leave restores locals and unwinds the stack.

## Constraints

- Stack depth and local-stack size are bounded.
- Execution must preserve QuakeC calling and return semantics exactly enough for progs compatibility.
- Out-of-bounds statements or invalid function numbers are hard errors.

## Decisions

### Explicit interpreter with profile and trace hooks

Observed decision:
- The Go interpreter keeps explicit trace/profile hooks instead of hiding all execution inside a minimally visible loop.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- execution is more observable and testable than a minimal black-box interpreter
