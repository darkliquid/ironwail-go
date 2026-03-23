# Internals

## Logic

The compiler tests combine three layers of evidence:

- fixture compilation checks that `Compile` produces structurally valid `progs.dat` sections
- helper tests isolate small invariants like opcode/type mapping and string/global allocation
- round-trip tests compile sample programs, load them with `qc.NewVM().LoadProgs`, execute functions such as `Add` and `Max`, and verify the VM-visible results

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
