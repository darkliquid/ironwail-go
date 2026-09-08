// Command bspdec decompiles Quake BSP29 files into editable .map files.
// See docs/superpowers/specs/2026-09-07-bspdec-design.md section 3 for the
// CLI contract (flags, exit codes, --json schema).
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

const (
	exitOK        = 0
	exitInput     = 1
	exitInternal  = 2
	exitSelfCheck = 3
)

// jsonSummary is the spec section 3 --json schema (stable key names).
type jsonSummary struct {
	Input         string          `json:"input"`
	Output        string          `json:"output"`
	BSPXBrushlist bool            `json:"bspx_brushlist"`
	Models        []jsonModelStat `json:"models"`
	ML            jsonML          `json:"ml"`
	Exit          int             `json:"exit"`
}

type jsonModelStat struct {
	Model       int `json:"model"`
	Brushes     int `json:"brushes"`
	LeavesSolid int `json:"leaves_solid"`
	PlanesUsed  int `json:"planes_used"`
	Warnings    int `json:"warnings"`
}

type jsonML struct {
	Stage string  `json:"stage"`
	Model *string `json:"model"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	// Accept the spec's `bspdec <input.bsp> [flags]` order as well as the
	// flag-package's `flags first`: Go's flag parser stops at the first
	// non-flag argument, so move a leading positional to the end.
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = append(args[1:], args[0])
	}
	fs := flag.NewFlagSet("bspdec", flag.ContinueOnError)
	fs.SetOutput(stderr)
	out := fs.String("o", "", "output .map path (default <input stem>.bspdec.map; - = stdout)")
	format := fs.String("format", "map", "output format (only map)")
	noBrushlist := fs.Bool("no-brushlist", false, "ignore a BSPX BRUSHLIST lump even if present")
	hull := fs.Int("decompile-hull", 0, "decompile collision hull N (1-3) instead of the render hull")
	merge := fs.Bool("merge-convex", true, "merge same-contents coplanar-adjacent convex cells")
	grid := fs.Int("grid-snap", 8, "quantize output brush points to integer lattice N (0 = off)")
	texFallback := fs.String("texture-fallback", "nearest", "policy for sides with no matching face: skip|nearest|trigger")
	ml := fs.String("ml", "", "enable ML stages: seams|group|all (requires -model-dir; M2+)")
	modelDir := fs.String("model-dir", "models/bspdec", "directory of provisioned model artifacts")
	jsonOut := fs.Bool("json", false, "print a machine-readable summary on stdout")
	loglevel := fs.String("loglevel", "info", "slog level: debug|info|warn|error")
	if err := fs.Parse(args); err != nil {
		return exitInput
	}

	var lv slog.Level
	if err := lv.UnmarshalText([]byte(*loglevel)); err != nil {
		_, _ = fmt.Fprintf(stderr, "bspdec: bad -loglevel %q\n", *loglevel)
		return exitInput
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{Level: lv})))

	if fs.NArg() != 1 {
		_, _ = fmt.Fprintln(stderr, "usage: bspdec [-o out.map] [flags] input.bsp")
		return exitInput
	}
	if *format != "map" {
		_, _ = fmt.Fprintf(stderr, "bspdec: unsupported -format %q (only map)\n", *format)
		return exitInput
	}
	if *ml != "" {
		_, _ = fmt.Fprintf(stderr, "bspdec: -ml %q needs the M2 training pipeline (not built); see docs/superpowers/specs/2026-09-07-bspdec-design.md section 7\n", *ml)
		return exitInput
	}
	_ = *modelDir // reserved for M2
	switch *texFallback {
	case "skip", "nearest", "trigger":
	default:
		_, _ = fmt.Fprintf(stderr, "bspdec: bad -texture-fallback %q\n", *texFallback)
		return exitInput
	}
	if *hull < 0 || *hull > 3 {
		_, _ = fmt.Fprintf(stderr, "bspdec: bad -decompile-hull %d (want 0-3)\n", *hull)
		return exitInput
	}

	input := fs.Arg(0)
	output := *out
	if output == "" {
		output = strings.TrimSuffix(input, filepath.Ext(input)) + ".bspdec.map"
	}
	if output == "-" && *jsonOut {
		_, _ = fmt.Fprintln(stderr, "bspdec: -o - and -json both write stdout; pick one")
		return exitInput
	}

	data, err := os.ReadFile(input)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bspdec: %v\n", err)
		return exitInput
	}
	m, stats, err := bspdec.Decompile(data, bspdec.Options{
		NoBrushlist:     *noBrushlist,
		DecompileHull:   *hull,
		MergeConvex:     *merge,
		GridSnap:        *grid,
		TextureFallback: *texFallback,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bspdec: %v\n", err)
		return exitInternal
	}

	// Marshal first, then self-check the exact bytes that will ship.
	var buf bytes.Buffer
	if err := mapfile.Write(&buf, m, mapfile.WriteOptions{GridSnap: *grid}); err != nil {
		_, _ = fmt.Fprintf(stderr, "bspdec: write: %v\n", err)
		return exitInternal
	}
	chk, err := mapfile.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "bspdec: self-check re-parse: %v\n", err)
		return exitSelfCheck
	}
	if err := bspdec.SelfCheck(chk); err != nil {
		_, _ = fmt.Fprintf(stderr, "bspdec: self-check: %v\n", err)
		return exitSelfCheck
	}

	if output == "-" {
		if _, err := stdout.Write(buf.Bytes()); err != nil {
			return exitInternal
		}
	} else if err := os.WriteFile(output, buf.Bytes(), 0o644); err != nil {
		_, _ = fmt.Fprintf(stderr, "bspdec: write %s: %v\n", output, err)
		return exitInput
	}

	if *jsonOut {
		s := jsonSummary{
			Input: input, Output: output,
			BSPXBrushlist: hasBSPXBrushlist(data),
			ML:            jsonML{Stage: "none", Model: nil},
			Exit:          exitOK,
		}
		for _, ms := range stats {
			s.Models = append(s.Models, jsonModelStat{
				Model: ms.Model, Brushes: ms.Brushes, LeavesSolid: ms.LeavesSolid,
				PlanesUsed: ms.PlanesUsed, Warnings: ms.Warnings,
			})
		}
		enc := json.NewEncoder(stdout)
		if err := enc.Encode(s); err != nil {
			return exitInternal
		}
	}
	return exitOK
}

// hasBSPXBrushlist reports whether the BSP carries a BSPX BRUSHLIST lump, by
// structurally validating the BSPX header and lump table. (The full reader
// moves to internal/bsp with the M1 BRUSHLIST path, bead ironwail-go-xxy.5.)
func hasBSPXBrushlist(data []byte) bool {
	for off := 0; off < len(data); {
		i := bytes.Index(data[off:], []byte("BSPX"))
		if i < 0 {
			return false
		}
		idx := off + i
		if idx+8+32 <= len(data) {
			n := binary.LittleEndian.Uint32(data[idx+4:])
			if n > 0 && n < 64 && idx+8+int(n)*32 <= len(data) {
				for k := 0; k < int(n); k++ {
					ent := idx + 8 + k*32
					if string(bytes.TrimRight(data[ent:ent+24], "\x00")) == "BRUSHLIST" {
						return true
					}
				}
			}
		}
		off = idx + 4
	}
	return false
}