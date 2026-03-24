# Interface

## Main consumer

- developers invoking `qgo` from the shell or build scripts

## CLI contract

- `qgo [flags] [dir]`
- `-o <path>` chooses the output file path and defaults to `progs.dat`
- `-v` enables a success message that prints the destination path and byte count
- `[dir]` defaults to `.` and is passed to `compiler.(*Compiler).Compile`

## Source-order CLI contract

- command shape: `qgo source-order [flags] [dir]`
- purpose: expose deterministic source/function ordering used by compilation for parity/debug tooling
- currently implemented formats/scopes:
  - `-format text -scope functions` emits `<index>\t<relative-file>\t<function-name>`
  - `-format text -scope files` emits `<index>\t<relative-file>`
  - `-format json -scope functions` emits a JSON array of objects with keys ordered as
    `index`, `file`, `function`
  - `-format json -scope files` emits a JSON array of objects with keys ordered as
    `index`, `file`
- output destination:
  - stdout by default
  - `-o <path>` writes payload to file
- forward-compatibility flags:
  - `-strict` is parsed so later slices can tighten declaration handling without changing the CLI shape
- currently rejected values:
  - unknown `-format` values (anything other than `text` and `json`)
  - unknown `-scope` values (anything other than `functions` and `files`)
- diagnostics and status:
  - stderr diagnostics retain `qgo:` prefix
  - exit status `0` on success, `1` on failure
- determinism requirements:
  - identical input + flags must produce byte-identical output
  - ordering must not depend on filesystem enumeration order
  - traversal logic must reuse existing compiler source-order semantics instead of a second ordering algorithm
  - source-order `index` fields are zero-based
  - relative file paths are normalized with `/` separators

## Failure modes

- parsing/type-checking/compiler failures are printed to stderr with a `qgo:` prefix and exit status `1`
- output write failures are surfaced immediately and also terminate with exit status `1`
