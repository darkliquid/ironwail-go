// Command qbsp compiles Quake .map files into BSP files (bead
// ironwail-go-t63, M1). It is the pure-Go counterpart of ericw-tools'
// qbsp: parse (QuakeEd + Valve 220), CSG, hulls, leak detection, and
// BSP29/BSP2 output.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

func main() {
	outPath := flag.String("o", "", "output .bsp path (default: <map>.bsp)")
	bsp2 := flag.Bool("bsp2", false, "emit the extended BSP2 format (32-bit lumps)")
	twoPSB := flag.Bool("2psb", false, "emit the BSP2RMQ variant (32-bit indices, 16-bit bounds; implies -bsp2)")
	leaktest := flag.Bool("leaktest", false, "exit 1 if the map leaks")
	margin := flag.Float64("margin", 64, "void ring around the map (units)")
	omitDetail := flag.Bool("omitdetail", false, "drop func_detail* entities entirely")
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: qbsp [-o out.bsp] [-bsp2] [-leaktest] map.map")
		os.Exit(2)
	}
	mapPath := flag.Arg(0)
	out := *outPath
	if out == "" {
		ext := filepath.Ext(mapPath)
		out = strings.TrimSuffix(mapPath, ext) + ".bsp"
	}

	f, err := os.Open(mapPath)
	if err != nil {
		log.Fatalf("qbsp: %v", err)
	}
	m, err := qbsp.ParseMap(f)
	_ = f.Close()
	if err != nil {
		log.Fatalf("qbsp: parse %s: %v", mapPath, err)
	}

	res, err := qbsp.Compile(m, qbsp.Options{
		BSP2:       *bsp2 || *twoPSB,
		TwoPSB:     *twoPSB,
		Margin:     *margin,
		OmitDetail: *omitDetail,
		Log:        func(f string, a ...any) { fmt.Printf("  "+f+"\n", a...) },
	})
	if err != nil {
		log.Fatalf("qbsp: %v", err)
	}

	fmt.Printf("---- qbsp / ironwail-go ----\n")
	fmt.Printf("%s -> %s\n", mapPath, out)
	for _, line := range res.Log {
		fmt.Printf("  %s\n", line)
	}

	if res.Leaked {
		fmt.Printf("LEAK: map leaks to the void\n")
		pts := strings.TrimSuffix(out, filepath.Ext(out)) + ".pts"
		if err := writePTS(pts, res.LeakPath); err != nil {
			log.Fatalf("qbsp: %v", err)
		}
		fmt.Printf("  wrote %s (%d points)\n", pts, len(res.LeakPath))
		if *leaktest {
			log.Fatalf("qbsp: leaktest failed")
		}
	}

	if err := os.WriteFile(out, res.Data, 0o644); err != nil {
		log.Fatalf("qbsp: write %s: %v", out, err)
	}
	if res.PortalFile != nil && len(res.PortalFile.Serialize()) > 0 {
		prt := strings.TrimSuffix(out, filepath.Ext(out)) + ".prt"
		if err := os.WriteFile(prt, res.PortalFile.Serialize(), 0o644); err != nil {
			log.Fatalf("qbsp: write %s: %v", prt, err)
		}
		fmt.Printf("%s written (%d portals)\n", prt, len(res.PortalFile.Portals))
	}
	version := "BSP29"
	if *bsp2 {
		version = "BSP2"
	}
	fmt.Printf("%s written (%d bytes, %s)\n", out, len(res.Data), version)
}

// writePTS writes the leak trail as Quake-style point lines.
func writePTS(path string, trail []qbsp.Point) error {
	var b strings.Builder
	for _, p := range trail {
		fmt.Fprintf(&b, "%.1f %.1f %.1f\n", p[0], p[1], p[2])
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
