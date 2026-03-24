# Internals

## Logic

The CLI remains intentionally thin, but now has a small dispatch seam:

- the default path parses compile flags, builds one `compiler.Compiler`, threads the `-v`
  flag into `Compiler.Verbose`, calls `Compile(dir)`, and persists the returned byte slice
  with `os.WriteFile`
- the `source-order` path parses its own `flag.FlagSet`, validates supported
  format/scope combinations, calls `compiler.SourceOrder(dir)`, and renders payloads for:
  - text/functions
  - text/files
  - json/functions
  - json/files
  before writing to stdout or `-o`

Both paths return status codes through a shared `run(...)` helper so tests can assert the
`qgo:`-prefixed stderr contract without shelling out to a subprocess.

## Constraints

- the command assumes exactly one Go package directory is being compiled
- it emits a single `progs.dat` blob rather than an intermediate representation or multiple artifacts

## Decisions

### Thin entrypoint over compiler-owned logic

Observed decision:
- the CLI delegates compilation and source-order traversal behavior to `cmd/qgo/compiler`
  and only owns subcommand detection, flag parsing, payload formatting, and output writes

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- shell-facing behavior stays easy to reason about
- compiler internals remain reusable from tests without spawning a process

### Ship source-order as a narrow first subcommand slice

Observed decision:
- implement only the default `text` + `functions` path first, while still parsing the
  planned flag names so later slices can expand behavior without reshaping the command

Rationale:
- enables incremental implementation slices without revisiting external behavior decisions
- prevents accidental divergence from already-landed deterministic traversal behavior in compiler tests

Rejected alternatives:
- implement all planned formats/scopes in one pass:
  - rejected because this slice only needs the deterministic default traversal path
- add a standalone tool with unrelated stderr/exit conventions:
  - rejected because existing `qgo` consumers already rely on consistent `qgo:` error
    prefix and status semantics

### Lock JSON output contract for parity/debug consumers

Observed decision:
- when `-format json` is requested, emit compact JSON arrays with explicit struct-backed
  field ordering so object keys stay stable (`index`, then `file`, then optional `function`)
- preserve source-order indices as zero-based integers and keep path rendering delegated
  to compiler traversal (`displayPath`) so output stays relative and slash-normalized

Rationale:
- deterministic JSON output is easier to snapshot in test fixtures and safer for downstream
  tooling that compares source-order runs byte-for-byte
- key-order and path-shape stability avoid accidental contract drift when refactoring

Rejected alternatives:
- pretty-printed/indented JSON:
  - rejected because the CLI contract prioritizes compact deterministic payloads
- map-based JSON rows:
  - rejected because map iteration order would make key order unstable
