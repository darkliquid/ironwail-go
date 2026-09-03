package compiler

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

func compileFixtureSourceMap(t *testing.T, fixture string) (*qc.SourceMap, int) {
	t.Helper()
	c := New()
	data, sm, err := c.CompileWithSourceMap(filepath.Join("..", "testdata", fixture))
	if err != nil {
		t.Fatalf("CompileWithSourceMap: %v", err)
	}
	return sm, len(data)
}

func TestSourceMapMapsStatementsToSourceLines(t *testing.T) {
	sm, _ := compileFixtureSourceMap(t, "controlflow")
	if sm == nil {
		t.Fatal("nil source map")
	}

	// Every function's body statements must map to progs.go lines 3-17.
	for _, m := range sm.Mappings {
		if m.Source < 0 {
			continue // sentinel/prologue statements
		}
		src := sm.Sources[m.Source]
		if want := "progs.go"; filepath.Base(src) != want {
			t.Errorf("mapping stmt %d: source = %q, want base %q", m.Stmt, src, want)
		}
		if m.Line < 3 || m.Line > 17 {
			t.Errorf("mapping stmt %d: line %d outside fixture range 3..17", m.Stmt, m.Line)
		}
	}
}

func TestSourceMapSentinelStatementHasNoOrigin(t *testing.T) {
	sm, _ := compileFixtureSourceMap(t, "controlflow")
	m := sm.Lookup(0)
	if m == nil {
		t.Fatal("no mapping for sentinel statement 0")
	}
	if m.Source != -1 {
		t.Errorf("sentinel mapping source = %d, want -1 (no origin)", m.Source)
	}
}

func TestSourceMapMaxBodyPointsAtReturnLines(t *testing.T) {
	sm, _ := compileFixtureSourceMap(t, "controlflow")

	// Max's `return a` is line 5 and `return b` is line 7: statements whose
	// mapping is line 5 or 7 must exist.
	found := map[int]bool{}
	for _, m := range sm.Mappings {
		if m.Source >= 0 && (m.Line == 5 || m.Line == 7) {
			found[m.Line] = true
		}
	}
	if !found[5] || !found[7] {
		t.Errorf("expected mappings for lines 5 and 7, got %v", found)
	}
}

func TestSourceMapStatementsForLineResolvesBreakpoints(t *testing.T) {
	sm, _ := compileFixtureSourceMap(t, "controlflow")

	src := ""
	if len(sm.Sources) > 0 {
		src = sm.Sources[0]
	}
	if src == "" {
		t.Fatal("no sources recorded")
	}
	// Line 14 (`result = result + i`) is an assignment inside Sum's loop.
	stmts := sm.StatementsForLine(src, 14)
	if len(stmts) == 0 {
		t.Fatal("no statements resolve to line 14")
	}
	for _, s := range stmts {
		m := sm.Lookup(s)
		if m == nil || m.Line != 14 {
			t.Errorf("StatementsForLine returned stmt %d with wrong mapping", s)
		}
	}
}

func TestSourceMapRoundTripsThroughJSON(t *testing.T) {
	sm, _ := compileFixtureSourceMap(t, "controlflow")

	var buf bytes.Buffer
	if err := sm.Write(&buf); err != nil {
		t.Fatalf("Write: %v", err)
	}
	loaded, err := qc.LoadSourceMap(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("qc.LoadSourceMap: %v", err)
	}
	if loaded.Version != qc.SourceMapVersion || loaded.File != sm.File {
		t.Errorf("roundtrip mismatch: version=%d file=%q", loaded.Version, loaded.File)
	}
	if len(loaded.Mappings) != len(sm.Mappings) {
		t.Errorf("mappings = %d, want %d", len(loaded.Mappings), len(sm.Mappings))
	}
	for i := range sm.Mappings {
		if loaded.Mappings[i] != sm.Mappings[i] {
			t.Errorf("mapping %d differs: %+v vs %+v", i, loaded.Mappings[i], sm.Mappings[i])
			break
		}
	}
}

func TestSourceMapLoadRejectsUnknownVersion(t *testing.T) {
	r := strings.NewReader(`{"version": 99, "file": "x", "sources": [], "mappings": []}`)
	if _, err := qc.LoadSourceMap(r); err == nil {
		t.Fatal("expected error for unknown source map version")
	}
}
