package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/bspdec"
	"github.com/darkliquid/ironwail-go/internal/bspdec/eval"
	"github.com/darkliquid/ironwail-go/tools/bspdec_synth"
)

// buildFixtureSynthPair generates one synthetic pair in a temp dir.
func buildFixtureSynthPair(t *testing.T) synthPair {
	t.Helper()
	dir := t.TempDir()
	g := synth.NewGenerator(7, 3)
	m := g.GenMap(0)
	f, err := os.Create(filepath.Join(dir, "m.map"))
	if err != nil {
		t.Fatal(err)
	}
	if err := g.Emit(m, f); err != nil {
		t.Fatalf("emit: %v", err)
	}
	_ = f.Close()
	mapPath := filepath.Join(dir, "m.map")
	p, err := eval.CompileMapPair(mapPath, dir)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return synthPair{PkgID: "fixture", MapID: "synth-7-0", BSPPath: p.BSPPath}
}

func TestPilotPairCompileParity(t *testing.T) {
	r := buildFixtureSynthPair(t)
	res := pilotPair(r, false)
	if res.Error != "" {
		t.Fatalf("pilot error: %s", res.Error)
	}
	if !res.SelfCheck || !res.Compile {
		t.Fatalf("decoded map failed validity-constrained emission: %+v", res)
	}
	if res.Programs == 0 || res.Valid != res.Programs {
		t.Fatalf("programs valid %d/%d", res.Valid, res.Programs)
	}
	if !res.Pass {
		t.Fatalf("parity IoU = %v, want >= 0.95", res.ParityIoU)
	}
}

func TestPilotPairBeamSearchParity(t *testing.T) {
	r := buildFixtureSynthPair(t)
	res := pilotPair(r, true)
	if res.Error != "" {
		t.Fatalf("search pilot error: %s", res.Error)
	}
	if res.Programs == 0 || res.Valid != res.Programs {
		t.Fatalf("search programs valid %d/%d", res.Valid, res.Programs)
	}
	if !res.Pass {
		t.Fatalf("search parity IoU = %v, want >= 0.95", res.ParityIoU)
	}
}

func TestSearchProgramFindsPlanes(t *testing.T) {
	r := buildFixtureSynthPair(t)
	data, err := os.ReadFile(r.BSPPath)
	if err != nil {
		t.Fatal(err)
	}
	f, err := bsp.Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	ls, err := bspdec.BrushListFromBSP(data)
	if err != nil || len(ls) == 0 {
		t.Fatalf("no BRUSHLIST: %v", err)
	}
	prog, ok := searchProgram(f, ls[0], 16)
	if !ok || len(prog.Planes) != len(ls[0].Faces) {
		t.Fatalf("search decode failed: ok=%v planes=%d faces=%d", ok, len(prog.Planes), len(ls[0].Faces))
	}
	// re-derive the AABB from the decoded brush and compare to the target
	tb, err := brushFromTokens(f, prog, ls[0])
	if err != nil {
		t.Fatal(err)
	}
	if !approx(tb.Mins.X, ls[0].Mins.X) || !approx(tb.Maxs.X, ls[0].Maxs.X) {
		t.Fatalf("decoded AABB x (%v,%v) != target (%v,%v)", tb.Mins.X, tb.Maxs.X, ls[0].Mins.X, ls[0].Maxs.X)
	}
}

func TestTokenizeProgramPlaneTable(t *testing.T) {
	r := buildFixtureSynthPair(t)
	data, err := os.ReadFile(r.BSPPath)
	if err != nil {
		t.Fatal(err)
	}
	f, err := bsp.Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	ls, err := bspdec.BrushListFromBSP(data)
	if err != nil || len(ls) == 0 {
		t.Fatalf("no BRUSHLIST: %v", err)
	}
	prog, ok := tokenizeProgram(f, ls[0])
	if !ok {
		t.Fatal("first brush failed to tokenize")
	}
	if len(prog.Planes) != len(ls[0].Faces) {
		t.Fatalf("tokens %d != faces %d", len(prog.Planes), len(ls[0].Faces))
	}
	for i, tok := range prog.Planes {
		if tok < 0 || int(tok) >= len(f.Planes) {
			t.Fatalf("token %d out of plane table", tok)
		}
		// the SIGNED token plane must re-derive the brush face plane
		p := f.Planes[tok]
		n := [3]float64{float64(p.Normal.X), float64(p.Normal.Y), float64(p.Normal.Z)}
		d := float64(p.Dist)
		if prog.Neg[i] {
			n[0], n[1], n[2] = -n[0], -n[1], -n[2]
			d = -d
		}
		face := ls[0].Faces[i]
		if !approx(n[0], face.Normal.X) || !approx(n[1], face.Normal.Y) ||
			!approx(n[2], face.Normal.Z) || !approx(d, face.Dist) {
			t.Fatalf("signed token %d plane (%v,%.1f) != face %v", i, n, d, face)
		}
	}
}
