package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompileMapPair(t *testing.T) {
	dir := t.TempDir()
	pair, err := CompileMapPair(fixtureMapPath(t), dir)
	if err != nil {
		t.Fatalf("CompileMapPair: %v", err)
	}
	if _, err := os.Stat(pair.BSPPath); err != nil {
		t.Fatalf("bsp missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "room.qbsp.log")); err != nil {
		t.Fatalf("qbsp log missing: %v", err)
	}
	counts, err := BrushCounts(pair.BSPPath)
	if err != nil {
		t.Fatalf("BrushCounts: %v", err)
	}
	if len(counts) != 1 || counts[0] != 6 {
		t.Fatalf("BRUSHLIST oracle = %v, want [6]", counts)
	}
}

func TestBrushCountsMissingLump(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plain.bsp")
	if err := os.WriteFile(path, []byte("not a bsp"), 0o644); err != nil {
		t.Fatal(err)
	}
	counts, err := BrushCounts(path)
	if err == nil && counts != nil {
		t.Fatal("expected error or nil counts for garbage file")
	}
}