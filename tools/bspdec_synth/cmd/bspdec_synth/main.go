// Command bspdec_synth generates a deterministic synthetic map corpus
// (spec section 9.6 P6): room-grammar maps on an 8-unit lattice, compiled
// with the pinned qbsp so every pair carries its BRUSHLIST oracle.
//
// Usage: bspdec_synth [-data dataset/bspdec] [-count 1000] [-seed 0x5EED]
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
	synth "github.com/darkliquid/ironwail-go/tools/bspdec_synth"
)

func main() {
	dataDir := flag.String("data", "dataset/bspdec", "dataset root")
	count := flag.Int("count", 1000, "number of maps to generate")
	seed := flag.Int64("seed", 0x5EED, "generation seed")
	out := flag.String("out", "", "output root for the pairs (default: <data>/synth/<seed>)")
	flag.Parse()

	seedDir := fmt.Sprintf("%d", *seed)
	root := *dataDir
	if *out != "" {
		root = *out
	}
	dir := filepath.Join(root, "synth", seedDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatal(err)
	}

	g := synth.NewGenerator(*seed, *count)
	var entries []eval.ManifestEntry
	failed := 0
	for n := 0; n < *count; n++ {
		mapID := fmt.Sprintf("synth-%d-%d", *seed, n)
		m := g.GenMap(n)
		f, err := os.Create(filepath.Join(dir, mapID+".map"))
		if err != nil {
			fatal(err)
		}
		if err := g.Emit(m, f); err != nil {
			fatal(err)
		}
		if err := f.Close(); err != nil {
			fatal(err)
		}
		mapPath := filepath.Join(dir, mapID+".map")
		if _, err := eval.CompileMapPair(mapPath, dir); err != nil {
			slog.Error("bspdec_synth: compile failed", "map", mapID, "err", err)
			failed++
			continue
		}
		entries = append(entries, eval.ManifestEntry{
			PkgID:       mapID,
			LicenseNote: "synthetic",
			MapFiles:    []string{filepath.ToSlash(strings.TrimPrefix(mapPath, filepath.Join(root, "")+string(filepath.Separator)))},
			BSPFiles:    []string{filepath.ToSlash(strings.TrimPrefix(strings.TrimSuffix(mapPath, ".map")+".bsp", filepath.Join(root, "")+string(filepath.Separator)))},
			Flags:       eval.Flags{Era: "synthetic", Brushlist: true},
		})
		if n%200 == 0 {
			slog.Info("bspdec_synth: progress", "maps", n)
		}
	}
	if err := eval.AppendToManifest(filepath.Join(*dataDir, "raw", "manifest.jsonl"), entries); err != nil {
		fatal(err)
	}
	slog.Info("bspdec_synth: done", "maps", len(entries), "failures", failed, "dir", dir)
	if failed > 0 {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "bspdec_synth: %v\n", err)
	os.Exit(1)
}
