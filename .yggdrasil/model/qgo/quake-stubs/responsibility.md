# Responsibility

`qgo/quake-stubs` owns the executable Go stub surface exposed by `pkg/qgo/quake`, especially the `Vec3` API, operator-emulation helpers, and type-safe entity flag helpers that let translated Quake logic run under standard `go test` execution.

It is responsible for preserving QCVM-facing semantics for vector arithmetic and float-backed entity flag storage at the API level while providing pure-Go behavior for local testing and development. It is not responsible for bytecode lowering, opcode selection, or emitting `progs.dat`; those concerns remain in `qgo/compiler`.
