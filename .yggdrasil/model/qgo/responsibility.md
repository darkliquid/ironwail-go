# Responsibility

`cmd/qgo` is responsible for the executable-facing `qgo` command. It parses the small CLI surface (`-o`, `-v`, and an optional source directory), invokes the compiler package, and writes the resulting `progs.dat` bytes to disk.

It is not responsible for implementing compilation itself, defining synthetic quake packages, or understanding QCVM binary layout details. Those concerns live in the `qgo/compiler` node and, ultimately, in `internal/qc`.
