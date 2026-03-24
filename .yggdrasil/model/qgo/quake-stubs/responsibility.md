# Responsibility

`qgo/quake-stubs` owns the executable Go stub surface exposed by `pkg/qgo/quake`, including both vector/operator helpers and engine builtin shims used by translated QuakeGo logic.

It is responsible for preserving compiler-facing signatures while providing optional runtime behavior hooks for local `go test` execution. It is not responsible for bytecode lowering, opcode selection, or VM dispatch in the engine runtime; those concerns remain in `qgo/compiler` and runtime QC subsystems.
