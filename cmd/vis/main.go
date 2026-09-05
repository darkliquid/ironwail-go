// Command vis computes PVS visibility for a compiled BSP and writes it back
// into the BSP's visibility lump (bead ironwail-go-t63, M3). It consumes the
// .prt portal file produced by qbsp.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/vis"
)

func main() {
	outPath := flag.String("o", "", "output .bsp path (default: overwrite input)")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: vis [-o out.bsp] map.bsp")
		os.Exit(2)
	}
	bspPath := flag.Arg(0)
	prtPath := strings.TrimSuffix(bspPath, filepath.Ext(bspPath)) + ".prt"

	bspData, err := os.ReadFile(bspPath)
	if err != nil {
		log.Fatalf("vis: read %s: %v", bspPath, err)
	}
	prtData, err := os.ReadFile(prtPath)
	if err != nil {
		log.Fatalf("vis: read %s: %v (run qbsp first to produce the portal file)", prtPath, err)
	}

	outData, err := vis.Run(bspData, prtData)
	if err != nil {
		log.Fatalf("vis: %v", err)
	}

	out := *outPath
	if out == "" {
		out = bspPath
	}
	if err := os.WriteFile(out, outData, 0o644); err != nil {
		log.Fatalf("vis: write %s: %v", out, err)
	}
	fmt.Printf("vis: wrote PVS to %s (%d bytes visibility)\n", out, len(outData))
}