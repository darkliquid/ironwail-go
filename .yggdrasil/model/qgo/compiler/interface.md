# Interface

## Main consumers

- `cmd/qgo/main.go`
- compiler-focused tests under `cmd/qgo/compiler/*_test.go`

## Main API

Observed exported surface:
- `New() *Compiler`
- `(*Compiler).Compile(dir string) ([]byte, error)`
- `NewLowerer(synth, info, fset) *Lowerer`
- `NewCodeGen(globals, strings) *CodeGen`
- `Emit(in *EmitInput) ([]byte, error)`
- `NewSyntheticPackages() *SyntheticPackages`
- `NewSyntheticImporter(pkgs) *SyntheticImporter`

Key exported data structures:
- `Compiler`
- `SyntheticPackages`, `SyntheticImporter`, `BuiltinDef`
- `IRProgram`, `IRFunc`, `IRInst`, `IRGlobal`, `IRField`, `IRLocal`, `IRParam`
- `EmitInput`
- `GlobalAllocator`
- `StringTable`
- `CompileError`, `ErrorList`

## Contracts

- `Compile` expects a directory containing exactly one Go package that only imports the synthetic `quake` and `quake/engine` packages
- generated binaries must remain loadable by `internal/qc.VM.LoadProgs`
- emitted `progs.dat` headers must carry the canonical Quake progdefs CRC expected by the current QC runtime layout
- builtin functions are represented as QC functions whose `FirstStatement` is the negative builtin number

## Failure modes

- parse, type-check, lowering, branch patching, and emission errors are surfaced as Go errors
- unsupported imports are rejected by the synthetic importer with a descriptive path error
