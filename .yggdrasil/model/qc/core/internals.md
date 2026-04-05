# Internals

## Logic

### Loading

`LoadProgs` reads the QuakeC program header, seeks to each table, and loads statements, defs, functions, strings, and globals into the VM. It also derives `EntityFields` and `EdictSize` from the program metadata.

### Execution

`ExecuteProgram` distinguishes between:
- builtin-backed functions
- bytecode-backed functions

The interpreter loop advances through statements, dispatches opcodes, handles control flow and function calls, tracks profile counters, and copies return values into `OFSReturn`.

`OPAddress` now permits pointers into edict `0` when the target field offset is valid. That keeps the VM compatible with mods that treat `worldspawn` as a writable entity context and assign world fields such as `self.gravity`.

### Stack handling

Function entry saves caller state and local values, copies parameters from the reserved parameter globals, and sets the new execution context. Leave restores locals and unwinds the stack.

## Constraints

- Stack depth and local-stack size are bounded.
- Execution must preserve QuakeC calling and return semantics exactly enough for progs compatibility.
- Out-of-bounds statements or invalid function numbers are hard errors.
- The interpreter enforces the Quakespasm/Ironwail runaway-loop guard of `0x1000000` executed statements per `ExecuteProgram` invocation and raises `"runaway loop error"` on overflow.
- Regression tests pin guard parity with C by asserting both the `0x1000000` limit constant and trap behavior for a tight infinite loop.
- A first-pass fixture slice additionally verifies an opt-in VM test override path (`VM.RunawayLoopLimit`) still traps with the same runaway-loop error while leaving default behavior unchanged.

## Decisions

### Explicit interpreter with profile and trace hooks

Observed decision:
- The Go interpreter keeps explicit trace/profile hooks instead of hiding all execution inside a minimally visible loop.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- execution is more observable and testable than a minimal black-box interpreter

### OP_DIV_F keeps C/IEEE-754 divide-by-zero semantics

Observed decision:
- `OPDivF` executes raw floating-point division without guard logic, including zero denominators.
- Tests pin a behavior matrix across operand/sign cases: `±1/±0 -> ±Inf` with sign derived from IEEE-754 signed-zero rules, and `0/±0 -> NaN`, validated through VM globals and `OFSReturn`.

Rationale:
- Match C Ironwail `pr_exec.c` behavior (`OPC->_float = OPA->_float / OPB->_float;`) and avoid introducing non-parity runtime errors.

Rejected alternatives:
- Throwing `PR_RunError`/Go errors for divide-by-zero:
  - rejected because C VM does not do this, and would change gameplay/QC behavior for existing `progs.dat`.

### World-entity field pointers stay valid during bytecode execution

Observed decision:
- `OPAddress` no longer rejects edict `0` solely because it is the world entity.
- The opcode still errors on out-of-range edict indices or field offsets; only the blanket "world entity" rejection was removed.

Rationale:
- qbj2 `worldspawn` stores through `self.gravity`, which is valid pointer math against edict `0` in this VM layout.
- Rejecting all world-entity field pointers broke otherwise valid mod startup before the menu-driven `map start` path could finish.

Rejected alternatives:
- Keeping the classic guard and treating qbj2 as unsupported:
  - rejected because the surrounding VM storage model already supports in-bounds world-field access, and the compatibility failure blocked a normal mod bootstrap path.
