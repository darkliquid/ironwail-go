# Package `qc`

## Purpose
The `qc` package implements the QuakeC Virtual Machine (VM). QuakeC is the domain-specific scripting language used to define Quake's game logic, monster AI, and player interactions. This package loads the compiled `progs.dat` files, interprets the bytecode, and provides a bridge between the scripts and the Go-based engine.

## Key Types & Interfaces
- **`VM`**: The core state of the virtual machine, including the function table, statement (bytecode) array, globals, and the string table.
- **`GlobalVars`**: A typed view into the VM's global variable array, mapping fixed offsets to familiar QuakeC variables like `self`, `other`, `time`, and `trace_fraction`.
- **`EntVars`**: A typed view into an entity's (edict's) fields, such as `origin`, `velocity`, `health`, and `classname`.
- **`BuiltinFunc`**: A function signature for Go-native functions that are exposed to QuakeC (e.g., `print`, `spawn`, `traceline`).
- **`CSQC`**: A specialized wrapper for the Client-Side QuakeC VM, which handles HUD rendering and client-side logic.

## Core Workflow
1. **Loading**: `LoadProgs` reads a `.dat` file, populating the VM's statements, functions, and initial global/entity definitions.
2. **Execution**: The engine calls `ExecuteProgram` with a function index (like the `StartFrame` entry point).
3. **Interpreter Loop**: Inside `exec.go`, the VM iterates through bytecode statements. Each statement consists of an opcode and up to three operands (usually offsets into the globals or entity fields).
4. **Builtins**: When a statement calls a negative function index, the VM dispatches to a Go function registered in the `Builtins` array, allowing QuakeC to perform "engine-level" tasks like physics or networking.

## Integration
- **Server**: The primary user of the `qc` package. The server runs the "SSQC" (Server-Side QuakeC) to drive the game simulation.
- **Renderer/HUD**: In mods that support it, the client runs "CSQC" to draw custom HUD elements or handle client-side effects.
- **Cvar System**: The VM can interact with engine cvars via builtins like `cvar()` and `cvar_set()`.

## Learning Tips
- **Interpreter Loop**: Read `internal/qc/exec.go` to see how the core switch-statement for opcodes is implemented. It's the most performance-critical part of the VM.
- **Data Layout**: Compare `internal/qc/types.go` with the original `progdefs.h` or `progdefs.q1` to see how the engine maintains bit-perfect compatibility with QuakeC data layouts.
- **Builtin Bridge**: Check `internal/qc/builtins.go` to see how Go functions are mapped to VM callbacks.
