package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func roomPairDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_, err := CompileMapPair(fixtureMapPath(t), dir)
	if err != nil {
		t.Fatalf("CompileMapPair: %v", err)
	}
	// copy the map alongside so EvaluatePair can find map+bsp by stem
	data, err := os.ReadFile(fixtureMapPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "room.map"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestEvaluatePairRoom(t *testing.T) {
	dir := roomPairDir(t)
	res, err := EvaluatePair(dir, "fixture", "room")
	if err != nil {
		t.Fatalf("EvaluatePair: %v", err)
	}
	if res.Error != "" {
		t.Fatalf("unexpected error: %s", res.Error)
	}
	if res.OrigLump.Faces == 0 || res.RecompLump.Faces == 0 {
		t.Fatalf("lump stats empty: %+v vs %+v", res.OrigLump, res.RecompLump)
	}
	if res.VoxelIoU < 0.9 {
		t.Fatalf("voxel IoU = %v, want >= 0.9", res.VoxelIoU)
	}
	if res.BrushDelta < 0 {
		t.Fatalf("brush delta negative: %d", res.BrushDelta)
	}
}

func TestEvaluateCorpusRoom(t *testing.T) {
	dir := t.TempDir()
	pairDir := filepath.Join(dir, "fixture")
	_ = os.MkdirAll(pairDir, 0o755)
	data, err := os.ReadFile(fixtureMapPath(t))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(pairDir, "room.map"), data, 0o644)
	if _, err := CompileMapPair(filepath.Join(pairDir, "room.map"), pairDir); err != nil {
		t.Fatalf("CompileMapPair: %v", err)
	}
	results, err := EvaluateCorpus(dir)
	if err != nil {
		t.Fatalf("EvaluateCorpus: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if results[0].Error != "" {
		t.Fatalf("pair errored: %s", results[0].Error)
	}
}