# Interface

## Main consumers

- `cmd/qgo/main.go`
- compiler-focused tests under `cmd/qgo/compiler/*_test.go`

## Main API

Observed exported surface:
- `New() *Compiler`
- `(*Compiler).Compile(dir string) ([]byte, error)`
- `(*Compiler).LastCacheHit() bool`
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
- successful `Compile` writes per-source-hash cache files under `<dir>/.qgo-cache/<sha256>.progs.dat` and reuses cached output when inputs are unchanged
- `LastCacheHit` reports whether the previous `Compile` call reused a cached artifact
- generated binaries must remain loadable by `internal/qc.VM.LoadProgs`
- emitted `progs.dat` headers must carry the canonical Quake progdefs CRC expected by the current QC runtime layout
- builtin functions are represented as QC functions whose `FirstStatement` is the negative builtin number
- builtin directives accept either numeric form (`//qgo:builtin 23`) or a compiler-known name alias (`//qgo:builtin bprint`)
- dynamic helper intrinsics `quake.FieldFloat(entity, fieldOffset)` and `quake.SetFieldFloat(entity, fieldOffset, value)` are compiler-recognized and lowered directly to QC field opcodes (`OP_LOAD_F`, `OP_ADDRESS`, `OP_STOREP_F`) with strict arity/type gating.

## Failure modes

- parse, type-check, lowering, branch patching, and emission errors are surfaced as Go errors
- unsupported imports are rejected by the synthetic importer with a descriptive path error
- cache read failures other than missing files are treated as cache misses, while source hashing and cache write errors can still fail compilation when they come from source reads
