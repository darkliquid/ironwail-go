# Responsibility

`cmd/qgo` is responsible for the executable-facing `qgo` command. It owns the thin CLI
surface for both package compilation and deterministic source-order inspection:

- compile mode parses `-o`, `-v`, and an optional source directory, invokes the compiler
  package, and writes the resulting `progs.dat` bytes to disk
- `source-order` parses its own narrow flag slice, emits deterministic traversal rows, and
  preserves the `qgo:` stderr/exit-status contract

It is not responsible for implementing compilation itself, defining synthetic quake
packages, or understanding QCVM binary layout details. Those concerns live in the
`qgo/compiler` node and, ultimately, in `internal/qc`.
