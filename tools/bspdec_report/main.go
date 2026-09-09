// Command bspdec_report prints a markdown + JSON summary of the eval matrix
// stored at <data>/eval.json, and exits 1 when fewer than -min-pairs pairs
// were evaluated (honest gates). The headroom subcommand runs the M1
// baseline-vs-BRUSHLIST-ceiling study over paired and synthetic maps and
// prints the go/no-go verdict for the ML routes (spec section 13).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "headroom" {
		os.Args = append(os.Args[:1], os.Args[2:]...)
		runHeadroom()
		return
	}
	dataDir := flag.String("data", "dataset/bspdec", "dataset root")
	minPairs := flag.Int("min-pairs", 1, "minimum evaluated pairs for exit 0")
	jsonOut := flag.Bool("json", false, "print raw results JSON instead of markdown")
	flag.Parse()

	path := filepath.Join(*dataDir, "eval.json")
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bspdec-report: %v\n", err)
		os.Exit(1)
	}
	var results []eval.PairResult
	if err := json.Unmarshal(data, &results); err != nil {
		fmt.Fprintf(os.Stderr, "bspdec-report: bad eval.json: %v\n", err)
		os.Exit(1)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].PkgID != results[j].PkgID {
			return results[i].PkgID < results[j].PkgID
		}
		return results[i].MapID < results[j].MapID
	})

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(results); err != nil {
			os.Exit(1)
		}
		os.Exit(summaryExit(results, *minPairs))
	}

	fmt.Printf("# bspdec eval matrix (%d pairs)\n\n", len(results))
	fmt.Println("| pkg | map | voxel IoU | faces Δ | planes Δ | brushΔ | err |")
	fmt.Println("| --- | --- | --- | --- | --- | --- | --- |")
	for _, r := range results {
		fd := r.RecompLump.Faces - r.OrigLump.Faces
		pd := r.RecompLump.Planes - r.OrigLump.Planes
		fmt.Printf("| %s | %s | %.3f | %+d | %+d | %+d | %s |\n",
			r.PkgID, r.MapID, r.VoxelIoU, fd, pd, r.BrushDelta, r.Error)
	}
	fmt.Println()
	good, total := 0, len(results)
	for _, r := range results {
		if r.Error == "" && r.VoxelIoU >= 0.9 {
			good++
		}
	}
	fmt.Printf("pass (error-free, IoU>=0.9): %d/%d\n", good, total)
	os.Exit(summaryExit(results, *minPairs))
}

func summaryExit(results []eval.PairResult, minPairs int) int {
	if len(results) < minPairs {
		fmt.Fprintf(os.Stderr, "bspdec-report: %d pairs < min %d\n", len(results), minPairs)
		return 1
	}
	return 0
}

// runHeadroom executes the M1 headroom study: for every manifest pair (both
// the canonicalized paired/ slice and the synthetic synth/<seed>/ slice),
// score the treewalk baseline and the BRUSHLIST-direct ceiling, then print
// the per-map table plus the verdict for the ML routes.
func runHeadroom() {
	dataDir := flag.String("data", "dataset/bspdec", "dataset root")
	threshold := flag.Float64("threshold", 0.02, "minimum mean ceiling gain for a go verdict")
	flag.Parse()

	entries, err := eval.LoadManifest(filepath.Join(*dataDir, "raw", "manifest.jsonl"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "bspdec-report headroom: %v\n", err)
		os.Exit(1)
	}
	type target struct {
		pkgID, mapID, pairDir string
	}
	var targets []target
	for _, e := range entries {
		if len(e.MapFiles) == 0 {
			continue
		}
		for _, mf := range e.MapFiles {
			mapID := strings.TrimSuffix(filepath.Base(mf), filepath.Ext(mf))
			var pairDir string
			if e.Flags.Era == "synthetic" {
				seed := strings.TrimPrefix(e.PkgID, "synth-")
				seed = seed[:strings.Index(seed, "-")]
				pairDir = filepath.Join(*dataDir, "synth", seed)
			} else {
				pairDir = filepath.Join(*dataDir, "paired", e.PkgID)
			}
			targets = append(targets, target{e.PkgID, mapID, pairDir})
		}
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].pkgID != targets[j].pkgID {
			return targets[i].pkgID < targets[j].pkgID
		}
		return targets[i].mapID < targets[j].mapID
	})

	var rows []eval.HeadroomRow
	for _, tgt := range targets {
		row := eval.HeadroomRow{PkgID: tgt.pkgID, MapID: tgt.mapID}
		base, baseBr, errB := eval.EvaluateHeadroom(tgt.pairDir, tgt.pkgID, tgt.mapID, false)
		if errB != nil {
			row.Error = "baseline: " + errB.Error()
		} else {
			row.BaselineIoU, row.BaselineBrushes = base, baseBr
		}
		ceil, ceilBr, errC := eval.EvaluateHeadroom(tgt.pairDir, tgt.pkgID, tgt.mapID, true)
		if errC != nil {
			row.Error = "ceiling: " + errC.Error()
		} else if row.Error == "" {
			row.CeilingIoU, row.CeilingBrushes = ceil, ceilBr
		}
		if counts, err := eval.BrushCounts(filepath.Join(tgt.pairDir, tgt.mapID+".bsp")); err == nil && len(counts) > 0 {
			row.OracleBrushes = counts[0]
		}
		rows = append(rows, row)
	}

	rows, _ = eval.ComposeHeadroomReport(rows, true)
	fmt.Printf("# bspdec headroom (baseline treewalk vs BRUSHLIST ceiling, %d pairs)\n\n", len(rows))
	fmt.Println("| pkg | map | base IoU | ceil IoU | Δ | baseBr | ceilBr | oracleBr | err |")
	fmt.Println("| --- | --- | --- | --- | --- | --- | --- | --- | --- |")
	for _, r := range rows {
		fmt.Printf("| %s | %s | %.3f | %.3f | %+.3f | %d | %d | %d | %s |\n",
			r.PkgID, r.MapID, r.BaselineIoU, r.CeilingIoU, r.Gain,
			r.BaselineBrushes, r.CeilingBrushes, r.OracleBrushes, r.Error)
	}

	// verdict on the synthetic slice: intact geometry isolates the direct
	// path from the CSG-fidelity failures that still block real maps
	var synthGains []float64
	for _, r := range rows {
		if r.Error == "" && strings.HasPrefix(r.PkgID, "synth-") {
			synthGains = append(synthGains, r.Gain)
		}
	}
	meanGain := 0.0
	if len(synthGains) > 0 {
		for _, g := range synthGains {
			meanGain += g
		}
		meanGain /= float64(len(synthGains))
	}
	verdict := eval.GoVerdict(meanGain, *threshold)
	fmt.Printf("\nM1 gate: %s (mean Δ IoU = %.4f over %d synthetic pairs, threshold %.2f)\n",
		verdict, meanGain, len(synthGains), *threshold)
	if verdict == "go" {
		fmt.Println("Routes A/B proceed: the BRUSHLIST ceiling clears the threshold and ML has room to approach it.")
	}
}