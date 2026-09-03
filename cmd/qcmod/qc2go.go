package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/darkliquid/ironwail-go/internal/qc/transpile"
)

// runQC2Go implements `qcmod qc2go [-pkg name] [-o out.go] <file.qc>`,
// converting QuakeC source to QuakeGo (bead wa0, acceptance criterion 3).
// The output is a migration starting point: constructs needing human
// judgment carry TODO(transpile) comments.
func runQC2Go(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("qc2go", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	pkg := fs.String("pkg", "progs", "Go package clause for the output")
	output := fs.String("o", "", "write Go source to file instead of stdout")
	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod qc2go: %v\n", err)
		return 2
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "qcmod qc2go: exactly one .qc input file is required")
		return 2
	}

	src, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod qc2go: read %s: %v\n", fs.Arg(0), err)
		return 1
	}

	out, err := transpile.Transpile(string(src), transpile.Options{Package: *pkg})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod qc2go: %v\n", err)
		return 1
	}

	if *output == "" {
		if _, err := stdout.Write([]byte(out)); err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod qc2go: write stdout: %v\n", err)
			return 1
		}
		return 0
	}
	if err := os.WriteFile(*output, []byte(out), 0o644); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod qc2go: write %s: %v\n", *output, err)
		return 1
	}
	return 0
}
