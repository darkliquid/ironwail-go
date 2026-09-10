package main

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/bspdec"
	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
	synth "github.com/darkliquid/ironwail-go/tools/bspdec_synth"
)

// cloneQuakeMapSource fetches the reference map-source repo when missing.
// A local -source dir always wins (tests and offline runs).
func cloneQuakeMapSource(ctx *stageCtx) error {
	if _, err := os.Stat(ctx.sourceDir); err == nil {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(ctx.sourceDir), 0o755)
	slog.Info("bspdec-corpus: cloning quake_map_source", "rev", quakeMapSourceRev)
	cmd := exec.Command("git", "clone", "https://github.com/fzwoch/quake_map_source", ctx.sourceDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("clone quake_map_source: %v: %s", err, out)
	}
	return nil
}

const quakeMapSourceRev = "27abebaa3886bb0e3156cce3a604673d22b243f8"

// scanLocalSource walks srcDir for .map files. Maps directly at the source
// root (single-author packages like quake_map_source) get a per-map pkg_id;
// maps in subdirectories group by directory.
func scanLocalSource(srcDir string) ([]eval.ManifestEntry, error) {
	var entries []eval.ManifestEntry
	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".map") {
			return nil
		}
		dir := filepath.Dir(path)
		pkgID := filepath.Base(dir)
		if dir == srcDir {
			pkgID = strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
		}
		relMap := filepath.ToSlash(strings.TrimPrefix(path, srcDir+string(filepath.Separator)))
		note := "unlicensed-local"
		dirEntries, _ := os.ReadDir(dir)
		for _, de := range dirEntries {
			if strings.HasPrefix(strings.ToUpper(de.Name()), "LICENSE") || strings.HasPrefix(strings.ToUpper(de.Name()), "COPYING") {
				note = "GPL-2.0" // quake_map_source is GPL-2.0 per spec section 9.1
				break
			}
		}
		entry := eval.ManifestEntry{
			PkgID:       pkgID,
			SourceURL:   "file://" + dir,
			LicenseNote: note,
			MapFiles:    []string{relMap},
			Flags:       eval.Flags{Era: "classic", ToolchainGuess: "vanilla"},
		}
		// sibling .bsp (precompiled map+bsp package)
		bspPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".bsp"
		if _, err := os.Stat(bspPath); err == nil {
			entry.BSPFiles = append(entry.BSPFiles, filepath.ToSlash(strings.TrimPrefix(bspPath, srcDir+string(filepath.Separator))))
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// stageEnumerate records local packages in the manifest (idempotent upsert).
func stageEnumerate(ctx *stageCtx) error {
	if err := cloneQuakeMapSource(ctx); err != nil {
		return err
	}
	entries, err := scanLocalSource(ctx.sourceDir)
	if err != nil {
		return err
	}
	if err := eval.AppendToManifest(ctx.manifestPath, entries); err != nil {
		return err
	}
	ctx.provenance("enumerate", len(entries), "entries")
	return nil
}

// stageCanonicalize compiles map-only entries into paired/<pkg_id>/ and
// copies pairs; bsp-only entries land in the classic holdout listing. Each
// map is compiled in a subprocess (canonicalize-onemap) so a compiler
// panic on one map cannot abort the pipeline: failures are recorded and the
// run continues.
func stageCanonicalize(ctx *stageCtx) error {
	entries, err := eval.LoadManifest(ctx.manifestPath)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(ctx.pairedDir, 0o755)
	_ = os.MkdirAll(ctx.holdoutDir, 0o755)
	paired := 0
	holdout := 0
	failures := 0
	for i := range entries {
		e := &entries[i]
		if e.Flags.Era == "synthetic" {
			continue
		}
		if len(e.MapFiles) == 0 && len(e.BSPFiles) > 0 {
			e.Flags.ClassicHoldout = true
			holdout++
			continue
		}
		if len(e.MapFiles) == 0 {
			continue
		}
		outDir := filepath.Join(ctx.pairedDir, e.PkgID)
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		for _, mf := range e.MapFiles {
			src := filepath.Join(ctx.sourceDir, filepath.FromSlash(mf))
			dst := filepath.Join(outDir, filepath.Base(mf))
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("copy %s: %w", src, err)
			}
			if _, err := os.Stat(strings.TrimSuffix(dst, ".map") + ".bsp"); err == nil {
				paired++
				continue
			}
			if _, err := ctx.compile(dst, outDir); err != nil {
				slog.Warn("bspdec-corpus: compile failed", "pkg", e.PkgID, "map", mf, "err", err)
				failures++
				continue
			}
			paired++
		}
	}
	if err := eval.AppendToManifest(ctx.manifestPath, entries); err != nil {
		return err
	}
	ctx.provenance("canonicalize", paired, "compiled", holdout, "holdout", failures, "failed")
	return nil
}

// subprocessCompile is the production compile function: the real corpus
// binary re-executes itself in a child process so a qbsp panic on one map
// cannot kill the pipeline. When running under go test (binary name carries
// ".test"), it falls back to in-process compilation — re-executing a test
// binary would rerun the whole suite recursively and exhaust memory.
func subprocessCompile(mapPath, outDir string) (evalPair, error) {
	exe, err := os.Executable()
	if err != nil {
		return evalPair{}, err
	}
	if strings.Contains(filepath.Base(exe), ".test") {
		p, err := eval.CompileMapPair(mapPath, outDir)
		return evalPair{MapPath: p.MapPath, BSPPath: p.BSPPath}, err
	}
	cmd := exec.Command(exe, "canonicalize-onemap", "-data", filepath.Dir(filepath.Dir(outDir)), "-pkg", filepath.Base(outDir))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return evalPair{}, fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	bspPath := filepath.Join(outDir, strings.TrimSuffix(filepath.Base(mapPath), ".map")+".bsp")
	if _, err := os.Stat(bspPath); err != nil {
		return evalPair{}, fmt.Errorf("canonicalize-onemap %s did not produce %s: %w", filepath.Base(outDir), bspPath, err)
	}
	return evalPair{
		MapPath: mapPath,
		BSPPath: bspPath,
	}, nil
}

// canonicalizeOneMap compiles a single package's maps (subprocess-isolated
// so qbsp panics cannot kill the whole pipeline).
func canonicalizeOneMap(ctx *stageCtx, pkgID string) error {
	entries, err := eval.LoadManifest(ctx.manifestPath)
	if err != nil {
		return err
	}
	outDir := filepath.Join(ctx.pairedDir, pkgID)
	_ = os.MkdirAll(outDir, 0o755)
	for _, e := range entries {
		if e.PkgID != pkgID {
			continue
		}
		for _, mf := range e.MapFiles {
			src := filepath.Join(ctx.sourceDir, filepath.FromSlash(mf))
			dst := filepath.Join(outDir, filepath.Base(mf))
			if err := copyFile(src, dst); err != nil {
				return err
			}
			if _, err := os.Stat(strings.TrimSuffix(dst, ".map") + ".bsp"); err == nil {
				continue
			}
			if _, err := eval.CompileMapPair(dst, outDir); err != nil {
				return err
			}
		}
	}
	return nil
}

// stageLabels runs cell/seam label derivation per paired map+bsp.
func stageLabels(ctx *stageCtx) error {
	entries, err := eval.LoadManifest(ctx.manifestPath)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(ctx.labeledDir, 0o755)
	done := 0
	for i := range entries {
		e := &entries[i]
		if len(e.MapFiles) == 0 {
			continue
		}
		for _, mf := range e.MapFiles {
			stem := strings.TrimSuffix(filepath.Base(mf), filepath.Ext(mf))
			mapPath := filepath.Join(ctx.pairedDir, e.PkgID, filepath.Base(mf))
			bspPath := filepath.Join(ctx.pairedDir, e.PkgID, stem+".bsp")
			if _, err := os.Stat(bspPath); err != nil {
				continue // map-only package failed canonicalization
			}
			out := filepath.Join(ctx.labeledDir, e.PkgID, stem+".labels.json")
			if _, err := os.Stat(out); err == nil {
				done++
				continue // idempotent
			}
			res, err := deriveLabels(mapPath, bspPath)
			if err != nil {
				return fmt.Errorf("labels %s/%s: %w", e.PkgID, stem, err)
			}
			_ = os.MkdirAll(filepath.Dir(out), 0o755)
			if err := writeJSON(out, res); err != nil {
				return err
			}
			done++
		}
	}
	ctx.provenance("labels", done, "labeled")
	return nil
}

type labelRecord struct {
	MapID string             `json:"map_id"`
	Cells []bspdec.CellLabel `json:"cells"`
	Seams []bspdec.SeamLabel `json:"seams"`
	Stats map[string]int     `json:"stats"`
}

func deriveLabels(mapPath, bspPath string) (*labelRecord, error) {
	mapData, err := os.ReadFile(mapPath)
	if err != nil {
		return nil, err
	}
	m, err := mapfile.Parse(bytes.NewReader(mapData))
	if err != nil {
		return nil, err
	}
	bspData, err := os.ReadFile(bspPath)
	if err != nil {
		return nil, err
	}
	tree, err := bsp.LoadTree(bytes.NewReader(bspData))
	if err != nil {
		return nil, err
	}
	cells, seams, err := bspdec.LabelCells(tree, m, bspdec.Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		return nil, err
	}
	stats := map[string]int{"cells": len(cells), "seams": len(seams)}
	for _, c := range cells {
		stats["label_"+c.Confidence]++
	}
	return &labelRecord{
		MapID: filepath.Base(mapPath),
		Cells: cells,
		Seams: seams,
		Stats: stats,
	}, nil
}

// stageEval runs the tier-1/2 matrix over paired maps and writes eval.json.
func stageEval(ctx *stageCtx) error {
	results, err := eval.EvaluateCorpus(ctx.pairedDir)
	if err != nil {
		return err
	}
	// synthetic pairs live under synth/<seed>/; evaluate them through the
	// same EvaluatePair path so the report covers the whole corpus.
	entries, err := eval.LoadManifest(ctx.manifestPath)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Flags.Era != "synthetic" || len(e.MapFiles) == 0 {
			continue
		}
		seed := strings.TrimPrefix(e.PkgID, "synth-")
		seed = seed[:strings.Index(seed, "-")]
		mapID := strings.TrimSuffix(filepath.Base(e.MapFiles[0]), filepath.Ext(e.MapFiles[0]))
		r, err := eval.EvaluatePair(filepath.Join(ctx.synthDir, seed), e.PkgID, mapID)
		if err != nil && r.Error == "" {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].PkgID != results[j].PkgID {
			return results[i].PkgID < results[j].PkgID
		}
		return results[i].MapID < results[j].MapID
	})
	if err := writeJSON(filepath.Join(ctx.dataDir, "eval.json"), results); err != nil {
		return err
	}
	ctx.provenance("eval", len(results), "pairs")
	return nil
}

// synthCompile compiles a generated map in-process. Real-world maps use the
// subprocess-isolated ctx.compile (qbsp panics on some), but synthesized
// maps are sealed by construction, so in-process compile is safe and keeps
// the stage hermetic for tests.
func synthCompile(mapPath, outDir string) (evalPair, error) {
	p, err := eval.CompileMapPair(mapPath, outDir)
	return evalPair{MapPath: p.MapPath, BSPPath: p.BSPPath}, err
}

// stageSynth generates the deterministic synthetic slice: room-grammar maps
// compiled by the pinned qbsp (BRUSHLIST oracle), each with full-coverage
// cell/seam labels (spec section 9.6 P6 + 9.4).
func stageSynth(ctx *stageCtx) error {
	const seed = 0x5EED
	seedDir := filepath.Join(ctx.synthDir, fmt.Sprintf("%d", seed))
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		return err
	}
	g := synth.NewGenerator(seed, ctx.count)
	var entries []eval.ManifestEntry
	labeled := 0
	for n := 0; n < ctx.count; n++ {
		mapID := fmt.Sprintf("synth-%d-%d", seed, n)
		m := g.GenMap(n)
		var buf bytes.Buffer
		if err := g.Emit(m, &buf); err != nil {
			return err
		}
		mapPath := filepath.Join(seedDir, mapID+".map")
		if err := os.WriteFile(mapPath, buf.Bytes(), 0o644); err != nil {
			return err
		}
		p, err := synthCompile(mapPath, seedDir)
		if err != nil {
			slog.Warn("bspdec-corpus: synth compile failed", "map", mapID, "err", err)
			continue
		}
		rel := func(p string) string {
			return filepath.ToSlash(strings.TrimPrefix(p, ctx.dataDir+string(filepath.Separator)))
		}
		entries = append(entries, eval.ManifestEntry{
			PkgID:       mapID,
			LicenseNote: "synthetic",
			MapFiles:    []string{rel(p.MapPath)},
			BSPFiles:    []string{rel(p.BSPPath)},
			Flags:       eval.Flags{Era: "synthetic", Brushlist: true},
		})
		res, err := deriveLabels(p.MapPath, p.BSPPath)
		if err != nil {
			return fmt.Errorf("synth labels %s: %w", mapID, err)
		}
		out := filepath.Join(ctx.labeledDir, mapID, mapID+".labels.json")
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := writeJSON(out, res); err != nil {
			return err
		}
		labeled++
	}
	if err := eval.AppendToManifest(ctx.manifestPath, entries); err != nil {
		return err
	}
	ctx.provenance("synth", len(entries), "pairs", labeled, "labeled")
	return nil
}

// runSplits computes package-level train/val/test + the classic holdout set
// over the manifest's paired packages, folding in synthetic packages found
// under synth/<seed>/ so the split always covers the generated slice.
func runSplits(dataDir string) (train, val, test, holdout []string, err error) {
	entries, err := eval.LoadManifest(filepath.Join(dataDir, "raw", "manifest.jsonl"))
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var pairedPkgs []string
	var holdoutPkgs []string
	for _, e := range entries {
		if e.Flags.ClassicHoldout || len(e.MapFiles) == 0 {
			holdoutPkgs = append(holdoutPkgs, e.PkgID)
			continue
		}
		pairedPkgs = append(pairedPkgs, e.PkgID)
	}
	// synthetic packages exist on disk as sure as in the manifest; scan the
	// synth dirs so a missing manifest entry cannot silently drop them
	if dirs, derr := os.ReadDir(filepath.Join(dataDir, "synth")); derr == nil {
		for _, d := range dirs {
			if !d.IsDir() {
				continue
			}
			pkg := "synth-" + d.Name()
			if !slices.Contains(pairedPkgs, pkg) {
				pairedPkgs = append(pairedPkgs, pkg)
			}
		}
	}
	for _, p := range pairedPkgs {
		h := fnv.New32a()
		h.Write([]byte(p))
		switch h.Sum32() % 10 {
		case 8, 9:
			test = append(test, p)
		case 7:
			val = append(val, p)
		default:
			train = append(train, p)
		}
	}
	sort.Strings(train)
	sort.Strings(val)
	sort.Strings(test)
	sort.Strings(holdoutPkgs)
	if err := writeJSON(filepath.Join(dataDir, "splits.json"), map[string]any{
		"train":           train,
		"val":             val,
		"test":            test,
		"classic_holdout": holdoutPkgs,
	}); err != nil {
		return nil, nil, nil, nil, err
	}
	return train, val, test, holdoutPkgs, nil
}

// stageSplits writes package-level train/val/test + the classic holdout set.
func stageSplits(ctx *stageCtx) error {
	train, val, test, holdout, err := runSplits(ctx.dataDir)
	if err != nil {
		return err
	}
	ctx.provenance("splits", len(train), len(val), len(test), len(holdout), "split")
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}
