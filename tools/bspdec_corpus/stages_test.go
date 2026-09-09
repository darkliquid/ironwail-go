package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
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