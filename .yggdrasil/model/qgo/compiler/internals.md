# Internals

## Logic

### Pipeline

The compiler is a four-stage pipeline:

1. `Compiler.Compile` parses every Go file in the target directory and type-checks them with a `types.Config` that only knows about the synthetic `quake` packages.
2. `Lowerer` performs a two-pass walk over the package: first it collects globals and function signatures, then it lowers function bodies into `IRProgram`.
3. a lightweight IR optimizer first folds supported scalar float operations with known constant operands, trims no-op self-store instructions (`OPStore* x -> x`), prunes blocks that become unreachable once explicit terminators are honored, then runs a narrow dead-code elimination pass that removes dead pure virtual-register definitions across straight-line IR and simple label/branch control flow. After DCE, it prunes unused non-parameter `IRFunc.Locals` entries so codegen allocates fewer QC slots. Immediate pseudo-store records (`ImmFloat` / `ImmStr`) remain preserved for constant materialization.
4. `CodeGen` maps IR virtual registers to QC global/local offsets, emits `qc.DStatement` and `qc.DFunction` tables, and patches branch labels in a second pass.
5. `Emit` serializes those tables into the `progs.dat` section layout expected by `internal/qc`, including the canonical header CRC that matches the current progdefs layout used by the runtime.

`Compiler.Compile` is the optimizer integration point: lowering always returns IR first, then `optimizeIRProgram` runs before code generation so downstream emission never sees removable self-copy store instructions and local-slot allocation reflects post-DCE live virtual-register use.

### Incremental source-hash cache seam

`Compiler.Compile` now computes a deterministic SHA-256 hash across every top-level `.go` and `.qgo` source file in the target package (sorted by filename, hashing both name and content). The hash maps to `<dir>/.qgo-cache/<hash>.progs.dat`.

Execution path:
- if cache file exists, compilation short-circuits and returns cached bytes (`lastCacheHit=true`)
- if cache file is missing/unreadable, normal lowering/codegen/emission runs (`lastCacheHit=false`)
- on successful emit, compiler best-effort writes the artifact back to the hash path for subsequent runs

This is intentionally package-local and content-addressed; it does not yet include dependency graph hashing or cache eviction.

### Lowering model

The lowerer tracks a `types.Object -> VReg` mapping plus constant pools for floats and strings. IR instructions can either refer to virtual registers or encode direct QC global offsets when a value is already a reserved VM slot. Labels are represented as pseudo-instructions and resolved later by code generation.

Lowering intentionally processes only the packages passed in from `packages.Load` for the compile target. Imported dependency package bodies are not lowered. Their symbols remain available through `types.Info` resolution, but syntax walks for declarations/bodies are restricted to target-package files.

To keep emitted `progs.dat` function/global ordering deterministic within the target package, lowering explicitly sorts:

- syntax file lowering order by source filename from the package file-set for both declaration and body passes.

This makes function table order stable across runs and machines for the same source tree, and prevents imported implementation details from introducing unrelated unsupported-syntax failures during user package compilation.

Builtin directives in function doc comments now accept either explicit numeric IDs or canonical builtin names mapped from the runtime builtin table (`setorigin`, `spawn`, `remove`, `bprint`, `walkmove`, `droptofloor`, `write*`, etc). Alias resolution is case-insensitive and falls back to numeric parsing first.

### QCVM-oriented allocation

`GlobalAllocator` starts at `qc.OFSParmStart`, preserving QCVM-reserved slots. `StringTable` interns all strings and guarantees offset `0` is the empty string. `slotsForType` handles the special three-slot width of vectors so globals, locals, and parameter sizes match QCVM expectations.

### Constant materialization, first-pass folding, and bitwise-not lowering

Float and string constants are represented in IR with `IRInst.ImmFloat` / `IRInst.ImmStr` pseudo-store instructions emitted by `constFloat` and `constString`. Float immediates additionally carry `IRInst.HasImmFloat` so `0.0` remains an explicit immediate instead of being conflated with "no immediate". Code generation recognizes these pseudo-stores and seeds `GlobalAllocator`/`StringTable` directly instead of emitting runtime `OPStore*` statements that would require initialized source slots.

The first constant-folding pass runs in `foldLiteralConstFloatOps` as a deterministic local walk over each non-builtin IR function. It tracks literal-origin float constants by VReg and rewrites supported operations (`OPAddF`, `OPSubF`, `OPMulF`, `OPDivF`, `OPEqF`, `OPNeF`, `OPLE`, `OPGE`, `OPLT`, `OPGT`) into immediate `OPStoreF` pseudo-stores targeting the original destination VReg. Folded immediate stores are fed back into the same local-known map, so deterministic multi-op arithmetic chains (for example add→mul→sub on literal-derived values) collapse in one pass. Non-immediate copy stores and unary/bitwise boolean-style ops remain out of scope for this slice.

After folding and self-store cleanup, `pruneUnreachableBlocks` computes minimal basic blocks from labels and control-flow terminators and removes blocks unreachable from entry. This specifically trims post-terminator fallthrough fragments that are not targeted by any reachable branch label, while preserving any explicitly targeted label blocks.

Then `eliminateDeadVirtualStores` constructs the same minimal basic-block view and computes conservative backward liveness across block successors before reverse per-block filtering. The pass only removes pure instructions whose destination is an auto-allocated virtual register (`vreg >= vregBase`) and not live at that point on any successor path. Side-effecting instructions (calls, pointer stores, returns, control flow) are always retained, and unknown branch labels conservatively disable optimization for that function.

Unary bitwise not (`^x`) is lowered in `lowerUnaryExpr` using the QC-compatible two's-complement identity `^x == -1 - x`. Because QCVM numeric values are float-backed, this is emitted as `OPSubF` with a `-1` constant left operand.

### Temporary lifetime audit for slot-reuse planning

This audit narrows safe candidates for future temp-slot reuse without changing current lowering/codegen behavior.

Observed temp model in current pipeline:

- function-local VRegs are monotonic (`allocVReg`) and start at `vregBase` per function; no reuse occurs during lowering.
- every newly created temp is appended into `IRFunc.Locals`; codegen allocates one contiguous local/global slot range from that list.
- existing optimizer passes can delete dead virtual defs and prune unused locals, but they do not renumber or reuse surviving VRegs.

Safe reuse candidates (for allocator follow-up):

1. **Pure expression-result temps inside one basic block**
   - Source evidence: `lowerBinaryExpr`, `lowerUnaryExpr`, vector indexing/load helpers create result VRegs and append them to locals.
   - Safety condition: candidate VReg is virtual (`vreg >= vregBase`), pure-defined, and dead at all block successors (same condition already used by `eliminateDeadVirtualStores` liveness logic).
   - Why low risk: aligns with existing purity/liveness model already used for removal; reuse extends that model from deletion-only to lifetime packing.

2. **Switch comparison condition temps**
   - Source evidence: `lowerSwitchStmt` emits per-case compare result `cond` VRegs (`opcodeForEq` result) then consumes them immediately in `OPIF`.
   - Safety condition: reuse only after compare+branch pair is emitted and liveness confirms no later use.
   - Why low risk: short, explicit lifetime and no pointer/call side effects.

3. **Address-pointer temps for dynamic/entity field stores**
   - Source evidence: `lowerFieldStore` and `SetFieldFloat` intrinsic emit `OPAddress` into a pointer temp then immediate `OPStoreP*`.
   - Safety condition: pointer temp not used after corresponding store and not carried across labels/branches.
   - Why medium-low risk: two-instruction lifetime; must respect control-flow boundaries.

4. **Call-return copy temps when immediately consumed**
   - Source evidence: `lowerCallExpr` stores `OFS_RETURN` into a fresh temp result for non-void calls.
   - Safety condition: result temp dies before any branch join and before any subsequent value-consuming operation requiring persistence.
   - Why medium risk (still candidate): call sites are side-effecting; reuse must not collapse lifetimes across later reads.

High-risk / defer zones:

- **Cross-branch/join live ranges** (`OPIF`/`OPIFNot`/`OPGoto`, labels): reuse requires successor-aware liveness to avoid clobbering values needed on alternate paths.
- **Reserved VM slots** (`OFS_PARM*`, `OFS_RETURN`) used for call/return ABI: never part of reusable temp pool.
- **Vector locals**: 3-slot width must remain contiguous; scalar-slot packing must not fragment vector allocations.
- **Non-virtual operands and direct globals** (`vreg < vregBase`, plus unresolved-object placeholders later mapped by codegen): treat as non-reusable storage identities.

Recommended narrow implementation slice after this audit:

- implement optional **post-liveness slot assignment** for virtual locals only, keyed by non-overlapping live intervals within CFG blocks;
- preserve existing VReg identifiers and IR semantics; change only `IRLocal`→slot mapping stage;
- keep explicit no-reuse guards for reserved ABI slots and multi-slot vector alignment.

### Dynamic entity-field helper intrinsics

Dynamic helper lowering is now enabled for a narrow FieldOffset contract:

- `quake.FieldFloat(entity, fieldOffset)` lowers directly to `OP_LOAD_F`
- `quake.SetFieldFloat(entity, fieldOffset, value)` lowers directly to `OP_ADDRESS` + `OP_STOREP_F`
- `ent.FieldFloat(fieldOffset)` for `quake.Entity` receiver form lowers directly to `OP_LOAD_F`

Lowering performs strict intrinsic gating before generic call handling:

- helper name must be one of the recognized intrinsic names
- arity must match exactly (`2` for read, `3` for write)
- argument QC types are validated (`entity`, `field`, `float` where applicable)
- receiver-form read validates receiver QC type `entity` and field-offset QC type `field`

This keeps dynamic field access opcode-correct without lowering imported helper bodies.

Calls that match the broader dynamic-helper naming family (`quake.Field*` / `quake.SetField*`) but are not part of this narrow pair now produce an explicit defer diagnostic. Receiver-form `quake.Entity.SetField*` is also deferred, including `ent.SetFieldFloat(...)`. These guards prevent accidental fallback to generic call lowering for unimplemented dynamic helper variants and keep scope decisions observable in tests.

## Constraints

- the compiler currently targets a narrow Go subset tailored for QC-like programs
- only `quake` and `quake/engine` imports are legal during type-checking
- binary compatibility is anchored to QCVM struct/layout constants from `internal/qc`

## Decisions

### Synthetic packages instead of importing runtime Go packages

Observed decision:
- the frontend type-checks against synthetic `go/types.Package` definitions for `quake` and `quake/engine`

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- source programs can use a Go-shaped API without linking against runtime engine packages
- builtin numbering and engine globals remain compiler-controlled rather than inferred from executable code

### Two-pass code generation for branches

Observed decision:
- labels are collected during statement emission and branch displacements are patched in a dedicated second pass

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- lowering can emit structured control flow without knowing final statement indices up front
- undefined labels fail during generation rather than silently producing bad jumps

### Chose target-package-only lowering with deterministic file traversal

Observed decision:
- lowering now walks only syntax for explicitly requested compile-target packages, and sorts those files by filename before declaration and body passes.

Rationale:
- traversing imported package bodies can trigger unsupported-language failures that are unrelated to the package being compiled.
- stable function ordering is required for repeatable `progs.dat` layout comparisons in parity tooling.

Rejected alternatives:
- recursively lowering imported package syntax:
  - rejected because it expands compiler scope beyond the active package and couples compilation success to dependency implementation details.
- keep default syntax traversal order and accept occasional order drift:
  - rejected because parity smoke outputs become noisy and hard to trust.

### Numeric builtin IDs with named aliases at lowering boundary

Observed decision:
- preserve numeric builtin IDs in IR/function tables, but allow a named alias layer in directive parsing (`//qgo:builtin <name>`)
- enforce a deterministic directive failure matrix in lowering: unknown alias, malformed payload, duplicate same-id directives, and ambiguous differing-id directives all produce explicit compile-time diagnostics instead of silent fallback.
- keep alias coverage explicitly curated rather than auto-generated from all runtime builtin declarations; numeric directives remain the fallback for runtime-only/extension IDs.

Rationale:
- builtin names are easier to read and review in source than raw numbers
- keeping IR/storage numeric avoids widening downstream codegen/emitter interfaces
- strict diagnostics prevent accidental non-builtin lowering when directive text is present but invalid, which previously made failures indirect and harder to triage.

Rejected alternatives:
- replacing all builtin references with names through codegen/emitter:
  - rejected for this slice because it increases API churn beyond a focused compiler increment
- requiring every runtime builtin stub ID to have a compiler alias:
  - rejected because many runtime/extension IDs are rarely referenced by name in authored qgo and are adequately addressable through numeric directives; enforcing full alias parity adds maintenance churn with low user value.

### Chose narrow intrinsic helper lowering for FieldOffset read/write

Observed decision:
- implement only `FieldFloat`/`SetFieldFloat` as compiler intrinsics in this slice, with strict type gating and direct opcode emission.

Rationale:
- `self.(fld_var)` is not valid Go AST and would require parser-level divergence.
- generic helper-call lowering is unsafe for this seam because imported helper bodies are intentionally excluded from lowering.
- direct intrinsic lowering guarantees expected QC field opcodes for validated helper calls while keeping change scope tight.

Rejected alternatives:
- extend parser/lowering for `self.(fld_var)` directly:
  - rejected for now because it introduces grammar divergence and larger risk beyond this blocker.
- infer dynamic field access from generic `ent[idx]` without strict type gating:
  - rejected because it can silently mis-lower non-entity indexes and weakens safety guarantees.
- rely on imported helper implementations without compiler-intrinsic lowering:
  - rejected because imported helper bodies are intentionally not lowered, so helper semantics are not guaranteed to materialize as required QC field opcodes.
- implement all field-value helper variants (`FieldVector`, `FieldString`, etc.) in one pass:
  - rejected for this slice to keep blast radius limited to the unblock seam.
- silently allow other `Field*`/`SetField*` helper names to pass through generic call lowering:
  - rejected because it obscures the intentional defer boundary and makes non-float dynamic helper usage fail in less actionable ways.

### Chose explicit defer diagnostics for non-Vec3 struct literals

Observed decision:
- keep `lowerCompositeLit` support narrow to `Vec3` and add a dedicated compile-time error path for general struct literals with type-qualified context.

Rationale:
- non-`Vec3` structs currently collapse to scalar-slot QC typing (`EvFloat`) in `goTypeToQC`, so naive acceptance would silently mis-lower field layout and stores.
- a deterministic defer diagnostic is safer than partial support because it prevents accidental semantic corruption while preserving forward progress for users and tests.

Rejected alternatives:
- infer arbitrary struct layout from Go field order and emit sequential stores immediately:
  - rejected because current QC type mapping/opcode selection does not encode multi-slot non-vector struct layout and would require broader allocator/codegen contracts.
- leave the old generic unsupported-composite error:
  - rejected because it does not clearly communicate that this is an intentional defer boundary and slows triage for compiler users.

### First IR constant-folding slice uses explicit immediate-presence semantics

Observed decision:
- implement a narrow IR constant-folding pass over scalar float operations and represent folded float immediates with `HasImmFloat`.

Rationale:
- this introduces optimization value with low blast radius and deterministic behavior.
- folded expressions can produce `0.0`, and explicit immediate presence avoids silently dropping valid zero immediates during later optimization/codegen checks.

Rejected alternatives:
- keep optimizer limited only to self-store pruning:
  - rejected because straightforward constant arithmetic remained in emitted statements and was a low-risk optimization target.
- use `ImmFloat != 0` as the only immediate-presence check:
  - rejected because it cannot distinguish a real `0.0` immediate from "no immediate".

### Phase-1 arithmetic-chain folding extends phase-0 local folding

Observed decision:
- keep the optimizer single-pass/local but allow folded immediate `OPStoreF` results to seed subsequent fold decisions in the same function walk.

Rationale:
- arithmetic-chain folding provides meaningful instruction-count reduction without introducing branch pruning, CFG-wide value propagation, or broader optimizer churn in this phase.
- deterministic behavior is preserved because the pass remains an ordered linear traversal over non-builtin function bodies.

Rejected alternatives:
- add copy-propagation and branch-aware constant analysis in the same slice:
  - rejected because it broadens risk beyond the targeted arithmetic-chain folding goal.

### Chose local-slot pruning as the smallest safe temp/global reuse follow-up

Observed decision:
- after constant folding and straight-line dead virtual-store elimination, prune `IRFunc.Locals` entries that are no longer referenced by any kept IR instruction, while always retaining parameter locals.

Rationale:
- codegen allocates local/global storage from `IRFunc.Locals`, so removing dead locals immediately reduces allocated QC slots without changing opcode emission contracts.
- this is a narrow deterministic change: no register renumbering, no control-flow rewriting, and no semantic changes to retained instructions.

Rejected alternatives:
- implement full virtual-register renumbering/compaction:
  - rejected for this slice because it increases blast radius and complicates debugability.
- implement broad CFG/SSA dataflow in the same change:
  - rejected because this slice only needs simple label/branch support and should avoid large optimizer architecture churn.

### Chose audit-first narrowing before temp-slot allocator changes

Observed decision:
- perform a precise temporary-lifetime audit and identify explicit safe/restricted reuse zones before introducing any allocator-level temp-slot reuse implementation.

Rationale:
- temp-slot reuse touches lowering, liveness, and codegen slot assignment simultaneously; premature implementation risks semantic regressions in QC ABI-sensitive paths.
- a documented candidate map allows incremental implementation with smaller blast radius and test targeting.

Rejected alternatives:
- implement broad temp-slot reuse immediately across all VRegs:
  - rejected because branch joins, call ABI slots, and vector width constraints require explicit safety boundaries first.

### Chose smallest-safe control-flow DCE over full CFG/dataflow optimization

Observed decision:
- extend the minimal dead-code elimination pass to support simple label/branch patterns with conservative block-level liveness, while still limiting scope to existing IR control-flow opcodes.

Rationale:
- this gives immediate IR cleanup value after constant folding with low semantic risk.
- conservative per-block liveness across branch successors removes obvious dead defs in control-flow-heavy lowering output without altering jump structure.
- avoiding full CFG/SSA rewrites keeps implementation deterministic and reviewable.

Rejected alternatives:
- full CFG-based DCE in the same slice:
  - rejected because it increases blast radius and correctness risk around branch labels and jump targets.
- removing dead writes to direct QC global offsets:
  - rejected because those writes may target reserved VM slots or externally observed state and are not safe for a first pass.

### Chose explicit unreachable-block pruning before liveness DCE

Observed decision:
- add a dedicated control-flow pass (`pruneUnreachableBlocks`) that drops whole basic blocks unreachable from entry once explicit terminators are respected, and run it before virtual-register liveness DCE.

Rationale:
- lowering can emit instructions after explicit `return`/`done`/unconditional branch boundaries that are not branch targets; retaining those blocks adds avoidable IR noise and can mask optimization intent.
- doing this as a separate pass keeps scope independent from value-level DCE and avoids coupling with constant-folding or temp-slot reuse behavior.
- pruning at block granularity preserves branch-target semantics: reachable labeled blocks remain intact, while only truly unreachable blocks are removed.

Rejected alternatives:
- fold unreachable-block handling into `eliminateDeadVirtualStores`:
  - rejected because it entangles structural control-flow cleanup with value liveness logic and makes the pass harder to reason about and test in isolation.
- rely on codegen/runtime to ignore post-terminator IR:
  - rejected because optimizer stages should hand downstream codegen a structurally cleaner IR and deterministic pass boundaries.

### Local source hash cache before wider dependency hashing

Observed decision:
- introduce a narrow cache seam keyed only by local source-file hashes

Rationale:
- provides immediate incremental wins for iterative edits without committing to a larger dependency-aware invalidation model
- keeps risk low while compiler/lowering behavior is still changing rapidly

Rejected alternatives:
- no caching until full module/dependency hash graph exists:
  - rejected because it delays practical DX gains and makes every iteration pay full compile cost
- mtime-based cache keys:
  - rejected because timestamps are less deterministic and can produce false hits/misses across toolchains
