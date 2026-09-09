package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/bspdec"
	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
	synth "github.com/darkliquid/ironwail-go/tools/bspdec_synth"
)

// seedFixtureSource creates a local "quake_map_source"-style dir with one
// package containing the committed room fixture.
func seedFixtureSource(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	pkgDir := filepath.Join(src, "fixture")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "bspdec", "room.map"))
	if err != nil {
		t.Fatalf("ReadFile fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "room.map"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return src
}

func TestPipelineOnFixture(t *testing.T) {
	src := seedFixtureSource(t)
	dataDir := filepath.Join(t.TempDir(), "dataset")
	ctx := newStageCtx(dataDir)
	ctx.sourceDir = src
	ctx.compile = func(mapPath, outDir string) (evalPair, error) {
		p, err := eval.CompileMapPair(mapPath, outDir)
		return evalPair{MapPath: p.MapPath, BSPPath: p.BSPPath}, err
	}

	if err := stageEnumerate(ctx); err != nil {
		t.Fatalf("enumerate: %v", err)
	}
	entries, err := eval.LoadManifest(ctx.manifestPath)
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].PkgID != "fixture" {
		t.Fatalf("pkg = %q", entries[0].PkgID)
	}

	if err := stageCanonicalize(ctx); err != nil {
		t.Fatalf("canonicalize: %v", err)
	}
	bspPath := filepath.Join(ctx.pairedDir, "fixture", "room.bsp")
	if _, err := os.Stat(bspPath); err != nil {
		t.Fatalf("paired bsp missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ctx.pairedDir, "fixture", "room.qbsp.log")); err != nil {
		t.Fatalf("qbsp log missing: %v", err)
	}

	if err := stageLabels(ctx); err != nil {
		t.Fatalf("labels: %v", err)
	}
	labelsPath := filepath.Join(ctx.labeledDir, "fixture", "room.labels.json")
	if _, err := os.Stat(labelsPath); err != nil {
		t.Fatalf("labels file missing: %v", err)
	}

	if err := stageSplits(ctx); err != nil {
		t.Fatalf("splits: %v", err)
	}
	splitPath := filepath.Join(ctx.dataDir, "splits.json")
	if _, err := os.Stat(splitPath); err != nil {
		t.Fatalf("splits.json missing: %v", err)
	}

	if err := stageEval(ctx); err != nil {
		t.Fatalf("eval: %v", err)
	}
	if _, err := os.Stat(filepath.Join(ctx.dataDir, "eval.json")); err != nil {
		t.Fatalf("eval.json missing: %v", err)
	}

	// idempotency: re-runs must not error or duplicate
	for _, fn := range []func(*stageCtx) error{stageEnumerate, stageCanonicalize, stageLabels, stageSplits, stageEval} {
		if err := fn(ctx); err != nil {
			t.Fatalf("idempotent re-run: %v", err)
		}
	}
	entries2, err := eval.LoadManifest(ctx.manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries2) != 1 {
		t.Fatalf("manifest grew on re-run: %d", len(entries2))
	}
}

// TestSynthLabelsFullCoverage: every world-model cell of a generated map
// receives exactly one original-brush label (BRUSHLIST ground truth), and
// every derived seam edge carries exact truth (intact synthetic geometry has
// no cracks, so "none" labels are a labelling bug, not map reality).
func TestSynthLabelsFullCoverage(t *testing.T) {
	dir := t.TempDir()
	g := synth.NewGenerator(7, 1)
	m := g.GenMap(0)
	var buf bytes.Buffer
	if err := g.Emit(m, &buf); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "m.map")
	if err := os.WriteFile(src, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := eval.CompileMapPair(src, dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p.BSPPath)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	orig, err := mapfile.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	cells, seams, err := bspdec.LabelCells(tree, orig, bspdec.Options{GridSnap: 8})
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) == 0 {
		t.Fatal("no labeled cells")
	}
	labeled, total := bspdec.LabelCoverage(cells)
	if labeled != total {
		t.Fatalf("label coverage = %d/%d, want 100%%", labeled, total)
	}
	for i, c := range cells {
		if c.OriginalBrush < 0 || c.OriginalBrush >= len(orig.Entities[0].Brushes) {
			t.Fatalf("cell %d labeled with out-of-range brush %d", i, c.OriginalBrush)
		}
	}
	for i, s := range seams {
		if !s.Seam {
			t.Fatalf("edge %d unlabeled (expected exact seam truth on synthetic)", i)
		}
	}
}

// TestStageSynth exercises the synth corpus stage end-to-end with the
// in-process compile injection: pairs emitted, label files written with
// full coverage, manifest registered.
func TestStageSynth(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "dataset")
	ctx := newStageCtx(dataDir)
	ctx.count = 3
	ctx.compile = func(mapPath, outDir string) (evalPair, error) {
		p, err := eval.CompileMapPair(mapPath, outDir)
		return evalPair{MapPath: p.MapPath, BSPPath: p.BSPPath}, err
	}
	ctx.synthDir = filepath.Join(dataDir, "synth")
	if err := stageSynth(ctx); err != nil {
		t.Fatalf("synth: %v", err)
	}
	maps, err := filepath.Glob(filepath.Join(ctx.synthDir, "24301", "*.map"))
	if err != nil {
		t.Fatal(err)
	}
	if len(maps) != 3 {
		t.Fatalf("synth maps = %d, want 3", len(maps))
	}
	for n := 0; n < 3; n++ {
		id := fmt.Sprintf("synth-24301-%d", n)
		b, err := os.ReadFile(filepath.Join(ctx.labeledDir, id, id+".labels.json"))
		if err != nil {
			t.Fatalf("labels %d: %v", n, err)
		}
		var rec labelRecord
		if err := json.Unmarshal(b, &rec); err != nil {
			t.Fatal(err)
		}
		covered := rec.Stats["label_assigned"] + rec.Stats["label_multi"]
		if covered != rec.Stats["cells"] {
			t.Fatalf("map %d label coverage %d/%d, want full", n, covered, rec.Stats["cells"])
		}
	}
	entries, err := eval.LoadManifest(ctx.manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("manifest entries = %d, want 3", len(entries))
	}
}

// writeSynthFixture fabricates a synth/<seed>/ pair directory and a manifest
// entry for a normal (non-synth) package, so "synth-0" can only reach the
// split through the synth-dir scan, not the manifest fold.
func writeSynthFixture(t *testing.T, dataDir string) {
	t.Helper()
	seedDir := filepath.Join(dataDir, "synth", "0")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// a fake sealed pair; splits only scans directories so contents are moot
	if err := os.WriteFile(filepath.Join(seedDir, "synth-0-0.map"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "synth-0-0.bsp"), []byte("dummy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := eval.AppendToManifest(filepath.Join(dataDir, "raw", "manifest.jsonl"), []eval.ManifestEntry{
		{PkgID: "real-pkg", LicenseNote: "unlicensed-local", MapFiles: []string{"maps/a.map"}},
	}); err != nil {
		t.Fatal(err)
	}
}

// TestSplitsIncludeSynth: the split file must contain synth packages when
// the synth corpus exists.
func TestSplitsIncludeSynth(t *testing.T) {
	dataDir := t.TempDir()
	writeSynthFixture(t, dataDir)
	if _, _, _, _, err := runSplits(dataDir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dataDir, "splits.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "synth-0") {
		t.Fatalf("splits.json missing synth packages:\n%s", s)
	}
}
