package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// runDisasm implements `qcmod disasm [flags] [progs.dat]`, printing a readable
// disassembly of progs.dat bytecode (bead wa0, acceptance criterion 1).
//
// With no progs path it compiles pkg/qgo/quakego, mirroring `qcmod vm`, so the
// dev kit can disassemble the project's own gameplay sources out of the box.
func runDisasm(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("disasm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	function := fs.String("func", "", "restrict output to the named function")
	output := fs.String("o", "", "write disassembly to file instead of stdout")
	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod disasm: %v\n", err)
		return 2
	}

	var progs []byte
	switch {
	case fs.NArg() > 0:
		data, err := os.ReadFile(fs.Arg(0))
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod disasm: read progs: %v\n", err)
			return 1
		}
		progs = data
	default:
		compiled, err := compileProgs()
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod disasm: compile progs: %v\n", err)
			return 1
		}
		progs = compiled
	}

	vm := qc.NewVM()
	if err := vm.LoadProgs(bytes.NewReader(progs)); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod disasm: load progs: %v\n", err)
		return 1
	}

	// Disassembly needs no builtins, globals initialisation, or edict storage —
	// it reads only the parsed progs tables — so the VM is used as-is.

	out := io.Writer(stdout)
	if *output != "" {
		f, err := os.Create(*output)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod disasm: create %s: %v\n", *output, err)
			return 1
		}
		defer func() { _ = f.Close() }()
		out = f
	}

	if err := vm.Disassemble(out, qc.DisasmOptions{Function: *function}); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod disasm: %v\n", err)
		return 1
	}
	return 0
}
