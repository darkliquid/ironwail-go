# Interface

## Main consumer

- developers invoking `qgo` from the shell or build scripts

## CLI contract

- `qgo [flags] [dir]`
- `-o <path>` chooses the output file path and defaults to `progs.dat`
- `-v` enables a success message that prints the destination path and byte count
- `[dir]` defaults to `.` and is passed to `compiler.(*Compiler).Compile`

## Planned CLI contract (not yet implemented)

- future command shape: `qgo source-order [flags] [dir]`
- purpose: expose deterministic source/function ordering used by compilation for parity/debug tooling
- expected formats:
  - `-format text` emits stable line-based output
  - `-format json` emits stable machine-readable output with `version`, `dir`, `scope`, and `order`
- expected scopes:
  - `-scope files` for file ordering only
  - `-scope functions` for file + function ordering (default)
- output destination:
  - stdout by default
  - `-o <path>` writes payload to file
- diagnostics and status:
  - stderr diagnostics retain `qgo:` prefix
  - exit status `0` on success, `1` on failure
- determinism requirements:
  - identical input + flags must produce byte-identical output
  - ordering must not depend on filesystem enumeration order
  - traversal logic must reuse existing compiler source-order semantics instead of a second ordering algorithm

## Failure modes

- parsing/type-checking/compiler failures are printed to stderr with a `qgo:` prefix and exit status `1`
- output write failures are surfaced immediately and also terminate with exit status `1`
