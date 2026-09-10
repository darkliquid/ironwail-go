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

func TestCheckEricwToolsLeak(t *testing.T) {
	qbsp := FindEricwQBSP()
	if qbsp == "" {
		t.Skip("ericw-tools qbsp not available")
	}

	// 1. Sealed room test
	sealedRes, err := CheckEricwToolsLeak(fixtureMapPath(t))
	if err != nil {
		t.Fatalf("CheckEricwToolsLeak sealed: %v", err)
	}
	if !sealedRes.Available {
		t.Fatal("expected ericw-tools to be available")
	}
	if sealedRes.Leaked {
		t.Errorf("expected sealed map to not leak in ericw-tools: %s", sealedRes.Output)
	}

	// 2. Leaky map test (box with missing ceiling so inside leaks to outside)
	leakyMapContent := "{\n\"classname\" \"worldspawn\"\n" +
		"{\n( 0 0 0 ) ( 0 256 0 ) ( 256 256 0 ) floor 0 0 0 1 1\n" +
		"( 0 0 -16 ) ( 256 0 -16 ) ( 256 256 -16 ) floor 0 0 0 1 1\n" +
		"( 0 0 0 ) ( 0 0 -16 ) ( 0 256 -16 ) floor 0 0 0 1 1\n" +
		"( 256 0 0 ) ( 256 256 0 ) ( 256 256 -16 ) floor 0 0 0 1 1\n" +
		"( 0 256 0 ) ( 0 256 -16 ) ( 256 256 -16 ) floor 0 0 0 1 1\n" +
		"( 0 0 0 ) ( 256 0 0 ) ( 256 0 -16 ) floor 0 0 0 1 1\n}\n" +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"128 128 64\"\n}\n"
	tmpMap := filepath.Join(t.TempDir(), "leaky.map")
	if err := os.WriteFile(tmpMap, []byte(leakyMapContent), 0o644); err != nil {
		t.Fatal(err)
	}

	leakyRes, err := CheckEricwToolsLeak(tmpMap)
	if err != nil {
		t.Fatalf("CheckEricwToolsLeak leaky: %v", err)
	}
	if !leakyRes.Leaked {
		t.Errorf("expected leaky map to leak in ericw-tools: %s", leakyRes.Output)
	}
}