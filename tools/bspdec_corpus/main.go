package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// stageCtx carries the dataset root and shared paths.
type stageCtx struct {
	dataDir       string // dataset/bspdec root
	manifestPath  string // raw/manifest.jsonl
	pairedDir     string
	labeledDir    string
	holdoutDir    string
	synthDir      string
	sourceDir     string // local package source (enumerate)
	defaultSource string
	count         int // maps per synth run
	// compile compiles one map into outDir. Production uses subprocess
	// isolation (qbsp panics on some real maps must not kill the pipeline);
	// tests inject in-process eval.CompileMapPair so no test binary is ever
	// re-executed (which would recurse and exhaust memory).
	compile func(mapPath, outDir string) (evalPair, error)
}

func newStageCtx(dataDir string) *stageCtx {
	return &stageCtx{
		dataDir:       dataDir,
		manifestPath:  filepath.Join(dataDir, "raw", "manifest.jsonl"),
		pairedDir:     filepath.Join(dataDir, "paired"),
		labeledDir:    filepath.Join(dataDir, "labeled"),
		holdoutDir:    filepath.Join(dataDir, "classic-holdout"),
		synthDir:      filepath.Join(dataDir, "synth"),
		defaultSource: filepath.Join(dataDir, "raw", "quake_map_source"),
		count:         50,
		compile:       subprocessCompile,
	}
}

// evalPair mirrors eval.Pair to keep main.go import-free of internal deps
// in this helper position.
type evalPair struct {
	MapPath string
	BSPPath string
}

func (c *stageCtx) provenance(stage string, a ...any) {
	line := fmt.Sprint(append([]any{stage}, a...)...)
	f, err := os.OpenFile(filepath.Join(c.dataDir, "provenance.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("bspdec-corpus: provenance", "err", err)
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintln(f, line)
}

var stageFuncs = map[string]func(*stageCtx) error{
	"enumerate":    stageEnumerate,
	"canonicalize": stageCanonicalize,
	"labels":       stageLabels,
	"splits":       stageSplits,
	"eval":         stageEval,
	"synth":        stageSynth,
}

func main() {
	stage := ""
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		stage = os.Args[1]
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
	if stage == "canonicalize-onemap" {
		fs := flag.NewFlagSet("bspdec-corpus canonicalize-onemap", flag.ExitOnError)
		dataDir := fs.String("data", "dataset/bspdec", "dataset root")
		pkgID := fs.String("pkg", "", "package to canonicalize")
		_ = fs.Parse(os.Args[1:])
		ctx := newStageCtx(*dataDir)
		ctx.sourceDir = ctx.defaultSource
		if err := canonicalizeOneMap(ctx, *pkgID); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "bspdec-corpus canonicalize-onemap %s: %v\n", *pkgID, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	fs := flag.NewFlagSet("bspdec-corpus", flag.ExitOnError)
	dataDir := fs.String("data", "dataset/bspdec", "dataset root")
	srcDir := fs.String("source", "", "local source dir of map packages (default: <data>/raw/quake_map_source)")
	count := fs.Int("count", 50, "maps to generate per synth run (synth stage)")
	quaddicted := fs.Bool("quaddicted", false, "also enumerate the Quaddicted API (offline-first: degrades to a notice)")
	_ = quaddicted
	_ = fs.Parse(os.Args[1:])

	if stage == "" {
		fmt.Fprintln(os.Stderr, "usage: bspdec-corpus <stage> [-data dir] [-source dir]")
		fmt.Fprintln(os.Stderr, "stages: enumerate canonicalize labels splits eval synth")
		os.Exit(2)
	}
	fn, ok := stageFuncs[stage]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown stage %q\n", stage)
		os.Exit(2)
	}
	ctx := newStageCtx(*dataDir)
	ctx.count = *count
	ctx.sourceDir = ctx.defaultSource
	if *srcDir != "" {
		ctx.sourceDir = *srcDir
	}
	if err := fn(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "bspdec-corpus %s: %v\n", stage, err)
		os.Exit(1)
	}
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
