# Responsibility

`qgo/compiler` owns the compilation pipeline that turns a Go package into a QCVM-compatible `progs.dat` image. It parses and type-checks a directory, constructs synthetic `quake` and `quake/engine` packages, lowers the typed AST into an internal IR, assigns globals/locals/string offsets, and emits QCVM tables and bytecode records.

It is not responsible for executing the produced bytecode or for server/client runtime integration. The runtime meaning of the emitted program belongs to `internal/qc` and its consumers.
