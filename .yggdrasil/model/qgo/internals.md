# Internals

## Logic

The CLI is intentionally thin. `main.go` builds one `compiler.Compiler`, threads the `-v` flag into `Compiler.Verbose`, calls `Compile(dir)`, and persists the returned byte slice with `os.WriteFile`.

That design keeps the executable stable while allowing reverse-engineering and compiler work to accumulate in the package beneath it without changing the shell contract.

## Constraints

- the command assumes exactly one Go package directory is being compiled
- it emits a single `progs.dat` blob rather than an intermediate representation or multiple artifacts

## Decisions

### Thin entrypoint over embedded compilation logic

Observed decision:
- the CLI delegates all compilation behavior to `cmd/qgo/compiler` and only owns flag parsing plus file output

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- shell-facing behavior stays easy to reason about
- compiler internals remain reusable from tests without spawning a process

### Define source-order behavior as a CLI contract before implementation

Observed decision:
- document a precise `qgo source-order` CLI/input-output contract in `QGO_SPEC.md` before adding command code

Rationale:
- enables incremental implementation slices without revisiting external behavior decisions
- prevents accidental divergence from already-landed deterministic traversal behavior in compiler tests

Rejected alternatives:
- implement command first and infer contract from emergent behavior
  - rejected because it risks churn in flags/output shape and weakens downstream parity tooling assumptions
- add a standalone tool with unrelated stderr/exit conventions
  - rejected because existing `qgo` consumers already rely on consistent `qgo:` error prefix and status semantics
