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
- builtin directive parsing is strict and diagnostic-driven: unknown aliases fail with `unknown //qgo:builtin alias "<name>"`, empty/multi-token payloads fail as malformed, and multiple directives on one function fail as duplicate (same builtin id) or ambiguous (different builtin ids)
- dynamic helper intrinsics `quake.FieldFloat(entity, fieldOffset)` and `quake.SetFieldFloat(entity, fieldOffset, value)` are compiler-recognized and lowered directly to QC field opcodes (`OP_LOAD_F`, `OP_ADDRESS`, `OP_STOREP_F`) with strict arity/type gating.
- other `quake.Field*` / `quake.SetField*` dynamic helpers are intentionally deferred for now and fail with an explicit defer diagnostic that points users back to the supported `FieldFloat`/`SetFieldFloat` surface.
- composite literal support is intentionally narrow: `Vec3` literals are supported as vector values, while non-`Vec3` struct literals are explicitly deferred with a dedicated compile-time diagnostic (`general struct literals are deferred ...`).
- IR optimization now includes a first literal-only constant-folding pass for scalar float arithmetic/comparison opcodes (`OPAddF`, `OPSubF`, `OPMulF`, `OPDivF`, `OPEqF`, `OPNeF`, `OPLE`, `OPGE`, `OPLT`, `OPGT`) when both operands are known literal immediate sources in the same traversal.
- folded float immediates are represented with `IRInst.HasImmFloat=true` so zero-valued constants remain explicit and are preserved through codegen.
- IR optimization includes a dedicated unreachable-block pruning pass that removes basic blocks not reachable from entry after explicit terminators (`OPGoto`, `OPIF`, `OPIFNot`, `OPReturn`, `OPDone`) are honored.
- IR optimization includes a minimal dead-code elimination pass that now supports simple label/branch control flow (`OPIF`/`OPIFNot`/`OPGoto`) via conservative block-level liveness, removing dead pure definitions to auto-allocated virtual registers while preserving jump semantics and side effects.

## Failure modes

- parse, type-check, lowering, branch patching, and emission errors are surfaced as Go errors
- unsupported imports are rejected by the synthetic importer with a descriptive path error
- cache read failures other than missing files are treated as cache misses, while source hashing and cache write errors can still fail compilation when they come from source reads
