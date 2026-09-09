// Command bspdec_report prints a markdown + JSON summary of the eval matrix
// stored at <data>/eval.json, and exits 1 when fewer than -min-pairs pairs
// were evaluated (honest gates).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
)

func main() {
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