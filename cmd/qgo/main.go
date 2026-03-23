// Command qgo compiles Go source code to QCVM progs.dat bytecode.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/ironwail/ironwail-go/cmd/qgo/compiler"
)

func main() {
	output := flag.String("o", "progs.dat", "output file path")
	verbose := flag.Bool("v", false, "verbose output")
	flag.Parse()

	dir := "."
	if flag.NArg() > 0 {
		dir = flag.Arg(0)
	}

	c := compiler.New()
	c.Verbose = *verbose

	data, err := c.Compile(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "qgo: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*output, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "qgo: write %s: %v\n", *output, err)
		os.Exit(1)
	}

	if *verbose {
		fmt.Printf("wrote %s (%d bytes)\n", *output, len(data))
	}
}
