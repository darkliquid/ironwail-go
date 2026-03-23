# Responsibility

`qgo/compiler-tests` captures the evidence used to validate the reverse-engineered compiler. The tests assert header layout, opcode selection, allocator behavior, synthetic package definitions, string interning, and—most importantly—round-trip compatibility by compiling fixtures, loading them into `internal/qc.VM`, and executing the generated functions.

This node is not responsible for implementing compiler behavior. Its job is to pin down what the compiler is expected to emit and which sample programs demonstrate that behavior.
