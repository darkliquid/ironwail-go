# Interface

## Main consumer

- developers invoking `qgo` from the shell or build scripts

## CLI contract

- `qgo [flags] [dir]`
- `-o <path>` chooses the output file path and defaults to `progs.dat`
- `-v` enables a success message that prints the destination path and byte count
- `[dir]` defaults to `.` and is passed to `compiler.(*Compiler).Compile`

## Failure modes

- parsing/type-checking/compiler failures are printed to stderr with a `qgo:` prefix and exit status `1`
- output write failures are surfaced immediately and also terminate with exit status `1`
