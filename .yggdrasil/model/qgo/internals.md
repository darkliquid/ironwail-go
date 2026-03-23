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
