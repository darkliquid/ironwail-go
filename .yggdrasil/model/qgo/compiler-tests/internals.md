# Internals

## Logic

The compiler tests combine three layers of evidence:

- fixture compilation checks that `Compile` produces structurally valid `progs.dat` sections
- helper tests isolate small invariants like opcode/type mapping and string/global allocation
- GlobalAllocator helper tests assert the allocator baseline tracks pre-registered QCVM/system globals (`self`, `parm*`, tracing globals, `msg_entity`) and begins compiler-owned allocation at `qc.OFSMsgEntity + 1`
- round-trip tests compile sample programs, load them with `qc.NewVM().LoadProgs`, execute functions such as `Add` and `Max`, and verify the VM-visible results
- round-trip arithmetic coverage now includes bitwise-not semantics (`^x`) and mask-clearing (`a & ^b`) via an ephemeral source package compiled in-test
- unsupported-feature tests create temporary package directories under `cmd/qgo/testdata` and assert deterministic lowering errors for syntax outside the supported subset
- general-struct-literal defer test compiles an ephemeral package and asserts the explicit `general struct literals are deferred` diagnostic contract
- incremental cache tests use ephemeral packages to validate compile-cache semantics without introducing persistent fixture directories
- builtin-directive tests now cover numeric and named alias parsing (`//qgo:builtin 23`, `//qgo:builtin bprint`, `//qgo:builtin SPAWN`) and verify compiled function metadata uses negative builtin IDs
- builtin-directive diagnostic tests additionally pin the failure matrix: unknown alias (`unknown //qgo:builtin alias "<name>"`), malformed directive payload, duplicate same-id directives on one function, and ambiguous differing-id directives on one function
- IR pipeline tests include a direct optimizer unit assertion that no-op self-store instructions are removed from function bodies, plus an end-to-end compile assertion that generated statements do not contain self-copy stores
- optimizer unit coverage now explicitly includes builtin-function IR bodies and asserts they are left untouched while non-builtin bodies are trimmed
- IR optimizer unit coverage now includes phase-0 constant-folding assertions that supported literal-only scalar float arithmetic/comparison operations collapse into immediate `OPStoreF` pseudo-stores, including folded zero-valued results
- optimizer boundary assertions now pin that fold tracking does not propagate through copy stores and that unary `OPNotF` remains unfurled in this slice
- IR optimizer unit coverage now includes local-slot pruning assertions that confirm dead-temp locals are removed after DCE while parameter locals are retained
- IR optimizer unit coverage now includes control-flow DCE assertions that verify dead pure virtual-register defs are removed across simple label/branch patterns while side-effecting pointer stores, branch conditions, and jump/return structure are retained
- IR optimizer unit coverage now includes unreachable-block assertions that verify instructions after an explicit terminator are removed when no reachable label targets them
- compile-level constant-folding coverage builds an ephemeral package with `2 + 3` and asserts the resulting function body has no runtime `OPAddF` statements
- source-order tests compile multi-file ephemeral packages and assert function-table order follows filename order, protecting deterministic lowering traversal for parity tooling
  - current assertion pins `a_first.go` (`Able`) before `main.go` (`MainValue`) before `z_last.go` (`Zed`) in emitted function order
- deterministic smoke tests compile the same fixture twice and assert byte-identical output to catch nondeterministic table/section emission drift
- structural parity smoke tests parse compiled `controlflow` output and pin stable layout/function/opcode invariants (including `Max`/`Sum` arity and positive first statements) so section-shape drift is detected even when output remains deterministic
- parity smoke tests evaluate `Add` in QCVM and compare output with equivalent native Go arithmetic over multiple signed/decimal vectors
- parity smoke harness adds a deterministic QCVM baseline matrix that executes `Add`, `Max`, and `Sum` against pinned vectors and native-Go expected returns, catching VM-visible lowering drift without broad golden tooling
- parity smoke harness now also asserts shallow structural signals per fixture (header/version/CRC/core-section sanity, required function-arity contracts, and required opcode presence) so compile-shape regressions are caught without introducing broad cross-binary diff infrastructure
- import-isolation tests compile an ephemeral package that imports a local dependency whose body includes unsupported type-switch syntax, asserting successful compile to prove imported bodies are not lowered
- dynamic-field intrinsic tests now include a runtime round-trip that executes compiled `ReadWrite(ent, ofs, value)` against a loaded VM and verifies both return value (pre-write read) and post-call entity field mutation

The `cmd/qgo/testdata/*/progs.go` programs are part of the persistent reverse-engineering story because they document the supported subset in executable form: globals, arithmetic, and basic control flow.

## Constraints

- tests assume `internal/qc` remains the authority for `progs.dat` layout and execution semantics
- fixture programs are intentionally tiny so failures identify compiler stages rather than application logic

## Decisions

### Runtime round-trip tests as compatibility proof

Observed decision:
- compiler tests load emitted binaries into the real QC VM instead of only checking byte slices

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- reverse engineering is anchored to executable QC behavior, not only to serialization snapshots
- helper utilities like `loadVM` become central evidence for future compiler work

### Explicitly pin unsupported advanced type forms

Observed decision:
- compiler tests include negative cases that assert deterministic failures for `*ast.TypeAssertExpr` and `*ast.TypeSwitchStmt`

Rationale:
- near-term qgo scope defers advanced type features because lowering/codegen has no runtime type-tag model and no interface-value runtime representation
- explicit tests prevent accidental partial support from silently changing language expectations

Rejected alternatives:
- infer entity type via ad-hoc `classname` string checks for assertions/switches:
  - rejected because it bakes one convention into the compiler and is not a general type system contract
- silently keep these forms unsupported without tests:
  - rejected because behavior can drift unnoticed during unrelated lowering work

### Dynamic field access uses explicit helper contracts

Observed decision:
- add focused compiler tests for the first intrinsic helper pair (`FieldFloat`, `SetFieldFloat`) and keep broader dynamic syntax work deferred.

Rationale:
- import/body isolation is a required prerequisite and is now covered by dedicated tests.
- intrinsic lowering now exists for the narrow read/write float helper seam, so opcode assertions can validate direct field-op emission and type gating.

Rejected alternatives:
- skip negative tests and only assert happy-path opcode presence:
  - rejected because strict helper argument type gating is part of the contract.
- rely on Go type-check arity errors alone:
  - rejected because intrinsic lowering must enforce its own helper contract even when helper signatures are broad/variadic in synthetic or test stubs.
- stop at opcode assertions without VM execution:
  - rejected because this cannot prove `OP_LOAD_F`/`OP_ADDRESS`/`OP_STOREP_F` cooperate correctly with runtime field pointers and entity memory layout.
- avoid explicit defer-boundary tests for additional helper names:
  - rejected because compiler users need a deterministic diagnostic that broader dynamic helper variants are intentionally not in scope yet.

### Keep deferred feature boundaries explicit in tests

Observed decision:
- add focused struct-literal boundary tests: one negative case for non-Vec3 struct literals (asserting the dedicated defer diagnostic and type context), and one positive case proving Vec3 literals remain supported.

Rationale:
- this guards the intentional boundary between shipped vector-literal support and deferred general-struct lowering.
- explicit assertion makes future broadening a deliberate change instead of accidental behavior drift.

Rejected alternatives:
- rely on a generic unsupported-composite error match:
  - rejected because it does not encode the product decision that this boundary is intentional and currently safest.

### Verify incremental behavior via deterministic cache-hit/cold-miss tests

Observed decision:
- tests compile the same ephemeral package twice and then recompile after editing source, asserting `LastCacheHit` transitions and output changes

Rationale:
- the cache seam is intentionally narrow; tests must pin expected semantics so future compiler refactors do not regress incremental behavior

Rejected alternatives:
- only asserting cache directory/file creation:
  - rejected because file existence alone does not prove compile-path reuse or invalidation

### Add focused IR optimization assertions without broad pass churn

Observed decision:
- add a narrow optimizer contract slice that now covers first-pass constant folding, dead self-store elimination, and minimal control-flow-aware virtual-register DCE rather than broad multi-pass optimization churn

Rationale:
- keeps the slice small, reviewable, and directly tied to current lowering/codegen shapes
- establishes a pass hook and test harness for future optimization work

Rejected alternatives:
- implementing multiple optimization passes (constant folding, full dead code elimination, temp-slot compaction) in one change:
  - rejected because it expands blast radius and weakens confidence in round-trip behavior for a first pipeline slice

### Keep unreachable-block cleanup as an isolated optimizer contract

Observed decision:
- add a focused optimizer unit test that verifies post-terminator unreachable blocks are pruned independently of value-liveness dead-code elimination.

Rationale:
- this pins a structural control-flow invariant in a deterministic unit test without expanding fixture or runtime smoke scope.
- explicit pass-level coverage helps keep this optimization isolated from constant folding and local-slot pruning behavior.

Rejected alternatives:
- only assert final emitted bytecode shape:
  - rejected because bytecode-only checks can hide IR-pass boundary regressions and make optimizer failures harder to localize.

### Pin smallest-safe temp/global reuse behavior with local-pruning tests

Observed decision:
- add a focused optimizer test that validates dead virtual-store elimination plus `IRFunc.Locals` pruning, while asserting parameter locals remain present.

Rationale:
- this captures the intended "smallest safe reuse" seam at compiler-IR level without requiring broad end-to-end fixture churn.
- test keeps behavior deterministic by asserting both surviving instructions and exact surviving local set.

Rejected alternatives:
- only asserting instruction removal:
  - rejected because slot reuse improvement is realized through reduced local allocation metadata, not just body changes.

### Add deterministic smoke checks before broader parity tooling slices

Observed decision:
- add smoke-level tests for byte-stable output and Go-vs-QC arithmetic agreement rather than introducing broad golden-file parity infrastructure in this slice

Rationale:
- these checks validate the most immediate parity invariants needed for follow-up tooling while keeping the test surface cheap to maintain in-repo
- byte-identity and shared-input execution comparison catch practical regressions early without overfitting to full fixture snapshots

Rejected alternatives:
- introducing full cross-tooling golden comparisons for all sections and all fixtures in one pass:
  - rejected because it broadens scope significantly and would require additional harness/plumbing not needed for this focused follow-up

### Add narrow deterministic parity smoke harness for core fixture behavior

Observed decision:
- add a table-driven QCVM smoke harness in compiler tests that compiles existing arithmetic/controlflow fixtures and asserts stable VM-visible outputs for selected calls.

Rationale:
- keeps parity detection focused on a small, repeatable baseline while reusing existing compiler/VM test infrastructure and fixtures.
- makes future lowering regressions easier to spot by pinning function-level behavior without introducing extra tooling.

Rejected alternatives:
- introducing a standalone external parity runner:
  - rejected because this scope is intentionally narrow and existing test helpers already provide compile/load/execute coverage.
